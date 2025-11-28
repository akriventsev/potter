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
}

// NewInMemoryAdapter создает новый InMemory адаптер
func NewInMemoryAdapter(config InMemoryConfig) *InMemoryAdapter {
	return &InMemoryAdapter{
		config:      config,
		subscribers: make(map[string][]transport.MessageHandler),
		responders:  make(map[string]func(ctx context.Context, request *transport.Message) (*transport.Message, error)),
		channels:    make(map[string]chan *transport.Message),
		running:     false,
		requestChs:  make(map[string]chan *transport.Message),
	}
}

// Start запускает адаптер (реализация core.Lifecycle)
func (i *InMemoryAdapter) Start(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.running {
		return nil
	}

	i.running = true
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (i *InMemoryAdapter) Stop(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.running {
		return nil
	}

	// Закрываем все channels
	for _, ch := range i.channels {
		close(ch)
	}

	i.running = false
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
	i.mu.RLock()
	handlers := i.subscribers[subject]
	// Проверяем wildcard подписки
	for subj, h := range i.subscribers {
		if i.matchSubject(subject, subj) && subj != subject {
			handlers = append(handlers, h...)
		}
	}
	i.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	msg := &transport.Message{
		Subject: subject,
		Data:    data,
		Headers: headers,
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
