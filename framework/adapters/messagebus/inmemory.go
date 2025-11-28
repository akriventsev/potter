// Package messagebus предоставляет адаптеры для различных message brokers.
package messagebus

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"potter/framework/core"
	"potter/framework/transport"
)

// InMemoryConfig конфигурация для InMemory адаптера
type InMemoryConfig struct {
	BufferSize     int
	WorkerCount    int
	EnableOrdering bool // FIFO гарантии
}

// DefaultInMemoryConfig возвращает конфигурацию InMemory по умолчанию
func DefaultInMemoryConfig() InMemoryConfig {
	return InMemoryConfig{
		BufferSize:     1000,
		WorkerCount:    10,
		EnableOrdering: false,
	}
}

// InMemoryAdapter реализация MessageBus в памяти
type InMemoryAdapter struct {
	config      InMemoryConfig
	subscribers map[string][]transport.MessageHandler
	responders  map[string]func(ctx context.Context, request *transport.Message) (*transport.Message, error)
	channels    map[string]chan *transport.Message
	mu          sync.RWMutex
	running     bool
	requestChs  map[string]chan *transport.Message // correlation ID -> channel
	requestMu   sync.RWMutex
	// Очереди сообщений для воркеров
	messageQueues map[string]chan *transport.Message // subject -> queue
	workerWg      sync.WaitGroup
	stopWorkers   chan struct{}
}

// NewInMemoryAdapter создает новый InMemory адаптер
func NewInMemoryAdapter(config InMemoryConfig) *InMemoryAdapter {
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = 10
	}

	return &InMemoryAdapter{
		config:        config,
		subscribers:   make(map[string][]transport.MessageHandler),
		responders:    make(map[string]func(ctx context.Context, request *transport.Message) (*transport.Message, error)),
		channels:      make(map[string]chan *transport.Message),
		running:       false,
		requestChs:    make(map[string]chan *transport.Message),
		messageQueues: make(map[string]chan *transport.Message),
		stopWorkers:   make(chan struct{}),
	}
}

// Start запускает адаптер (реализация core.Lifecycle)
func (i *InMemoryAdapter) Start(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.running {
		return nil
	}

	// Запускаем воркеры для обработки сообщений
	for j := 0; j < i.config.WorkerCount; j++ {
		i.workerWg.Add(1)
		go i.worker(j)
	}

	i.running = true
	return nil
}

// worker обрабатывает сообщения из очередей
func (i *InMemoryAdapter) worker(id int) {
	defer i.workerWg.Done()

	for {
		select {
		case <-i.stopWorkers:
			return
		default:
			// Проверяем все очереди на наличие сообщений
			i.mu.RLock()
			queues := make([]chan *transport.Message, 0, len(i.messageQueues))
			for _, queue := range i.messageQueues {
				queues = append(queues, queue)
			}
			i.mu.RUnlock()

			// Обрабатываем сообщения из очередей
			for _, queue := range queues {
				select {
				case msg := <-queue:
					if msg != nil {
						i.processMessage(context.Background(), msg)
					}
				default:
					// Нет сообщений в этой очереди
				}
			}

			// Небольшая задержка для снижения CPU usage
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// processMessage обрабатывает одно сообщение
func (i *InMemoryAdapter) processMessage(ctx context.Context, msg *transport.Message) {
	i.mu.RLock()
	handlers := i.subscribers[msg.Subject]
	// Проверяем wildcard подписки
	for subj, h := range i.subscribers {
		if i.matchSubject(msg.Subject, subj) && subj != msg.Subject {
			handlers = append(handlers, h...)
		}
	}
	i.mu.RUnlock()

	for _, handler := range handlers {
		if i.config.EnableOrdering {
			// Синхронная обработка для FIFO
			_ = handler(ctx, msg)
		} else {
			// Асинхронная обработка
			go func(h transport.MessageHandler) {
				_ = h(ctx, msg)
			}(handler)
		}
	}
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (i *InMemoryAdapter) Stop(ctx context.Context) error {
	i.mu.Lock()
	if !i.running {
		i.mu.Unlock()
		return nil
	}
	i.running = false
	i.mu.Unlock()

	// Останавливаем воркеры
	close(i.stopWorkers)
	i.workerWg.Wait()

	// Закрываем все channels
	i.mu.Lock()
	for _, ch := range i.channels {
		close(ch)
	}
	for _, queue := range i.messageQueues {
		close(queue)
	}
	i.mu.Unlock()

	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (i *InMemoryAdapter) IsRunning() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.running
}

// Name возвращает имя компонента (реализация core.Component)
func (i *InMemoryAdapter) Name() string {
	return "inmemory-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (i *InMemoryAdapter) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// Publish публикует сообщение в subject
func (i *InMemoryAdapter) Publish(ctx context.Context, subject string, data []byte, headers map[string]string) error {
	msg := &transport.Message{
		Subject: subject,
		Data:    data,
		Headers: headers,
	}

	// Если воркеры запущены, добавляем сообщение в очередь
	if i.running && i.config.WorkerCount > 0 {
		i.mu.Lock()
		queue, exists := i.messageQueues[subject]
		if !exists {
			// Создаем очередь с буфером указанного размера
			queue = make(chan *transport.Message, i.config.BufferSize)
			i.messageQueues[subject] = queue
		}
		i.mu.Unlock()

		// Пытаемся добавить в очередь (неблокирующе)
		select {
		case queue <- msg:
			// Сообщение добавлено в очередь, воркеры обработают
		default:
			// Очередь переполнена - обрабатываем синхронно
			return i.processMessageSync(ctx, msg)
		}
		return nil
	}

	// Если воркеры не запущены, обрабатываем синхронно
	return i.processMessageSync(ctx, msg)
}

// processMessageSync обрабатывает сообщение синхронно (fallback)
func (i *InMemoryAdapter) processMessageSync(ctx context.Context, msg *transport.Message) error {
	i.mu.RLock()
	handlers := i.subscribers[msg.Subject]
	// Проверяем wildcard подписки
	for subj, h := range i.subscribers {
		if i.matchSubject(msg.Subject, subj) && subj != msg.Subject {
			handlers = append(handlers, h...)
		}
	}
	i.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	// Fan-out для всех подписчиков
	for _, handler := range handlers {
		if i.config.EnableOrdering {
			// Синхронная обработка для FIFO
			_ = handler(ctx, msg)
		} else {
			// Асинхронная обработка
			go func(h transport.MessageHandler) {
				_ = h(ctx, msg)
			}(handler)
		}
	}

	return nil
}

// Subscribe подписывается на subject
func (i *InMemoryAdapter) Subscribe(ctx context.Context, subject string, handler transport.MessageHandler) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.subscribers == nil {
		i.subscribers = make(map[string][]transport.MessageHandler)
	}

	i.subscribers[subject] = append(i.subscribers[subject], handler)
	return nil
}

// Unsubscribe отписывается от subject
func (i *InMemoryAdapter) Unsubscribe(subject string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	delete(i.subscribers, subject)
	return nil
}

// Request отправляет запрос и ждет ответа
func (i *InMemoryAdapter) Request(ctx context.Context, subject string, data []byte, timeout time.Duration) (*transport.Message, error) {
	// Создаем временный channel для ответа
	correlationID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	replyCh := make(chan *transport.Message, 1)

	i.requestMu.Lock()
	i.requestChs[correlationID] = replyCh
	i.requestMu.Unlock()

	// Публикуем запрос с correlation ID
	headers := map[string]string{
		"correlation_id": correlationID,
		"reply_subject":  fmt.Sprintf("%s.reply", subject),
	}

	if err := i.Publish(ctx, subject, data, headers); err != nil {
		i.requestMu.Lock()
		delete(i.requestChs, correlationID)
		close(replyCh)
		i.requestMu.Unlock()
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	// Ждем ответа с timeout
	select {
	case reply := <-replyCh:
		i.requestMu.Lock()
		delete(i.requestChs, correlationID)
		i.requestMu.Unlock()
		return reply, nil
	case <-time.After(timeout):
		i.requestMu.Lock()
		delete(i.requestChs, correlationID)
		close(replyCh)
		i.requestMu.Unlock()
		return nil, fmt.Errorf("request timeout")
	case <-ctx.Done():
		i.requestMu.Lock()
		delete(i.requestChs, correlationID)
		close(replyCh)
		i.requestMu.Unlock()
		return nil, ctx.Err()
	}
}

// Respond отвечает на запросы
func (i *InMemoryAdapter) Respond(ctx context.Context, subject string, handler func(ctx context.Context, request *transport.Message) (*transport.Message, error)) error {
	i.mu.Lock()
	i.responders[subject] = handler
	i.mu.Unlock()

	// Подписываемся на subject для обработки запросов
	return i.Subscribe(ctx, subject, func(ctx context.Context, msg *transport.Message) error {
		// Получаем correlation ID из headers
		correlationID, ok := msg.Headers["correlation_id"]
		if !ok {
			return nil
		}

		// Обрабатываем запрос
		reply, err := handler(ctx, msg)
		if err != nil {
			return err
		}

		// Отправляем ответ в reply channel
		i.requestMu.RLock()
		replyCh, exists := i.requestChs[correlationID]
		i.requestMu.RUnlock()

		if exists && reply != nil {
			select {
			case replyCh <- reply:
			default:
			}
		}

		return nil
	})
}

// matchSubject проверяет соответствие subject с wildcard паттерном
// Поддерживает NATS-style wildcards: * (один токен) и > (все токены)
func (i *InMemoryAdapter) matchSubject(subject, pattern string) bool {
	subjectParts := strings.Split(subject, ".")
	patternParts := strings.Split(pattern, ".")

	if len(patternParts) > len(subjectParts) {
		return false
	}

	for i, part := range patternParts {
		if part == ">" {
			return true // > matches all remaining tokens
		}
		if part == "*" {
			if i >= len(subjectParts) {
				return false
			}
			continue // * matches one token
		}
		if i >= len(subjectParts) || part != subjectParts[i] {
			return false
		}
	}

	return len(patternParts) == len(subjectParts)
}

// GetSubscriberCount возвращает количество подписчиков для subject (для тестирования)
func (i *InMemoryAdapter) GetSubscriberCount(subject string) int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.subscribers[subject])
}

// GetPendingMessages возвращает количество pending messages (для тестирования)
func (i *InMemoryAdapter) GetPendingMessages() int {
	i.requestMu.RLock()
	defer i.requestMu.RUnlock()
	return len(i.requestChs)
}
