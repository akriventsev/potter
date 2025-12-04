// Package events предоставляет реализации EventPublisher.
package events

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RetryConfig конфигурация retry для публикатора
type RetryConfig struct {
	MaxAttempts      int
	InitialDelay     time.Duration
	MaxDelay         time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig возвращает конфигурацию retry по умолчанию
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:      3,
		InitialDelay:     time.Second,
		MaxDelay:         30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// InMemoryEventPublisher реализация публикатора событий в памяти
type InMemoryEventPublisher struct {
	subscribers map[string][]EventHandler
	mu          sync.RWMutex
	ordered     bool
	retryConfig *RetryConfig
}

// NewInMemoryEventPublisher создает новый in-memory публикатор
func NewInMemoryEventPublisher() *InMemoryEventPublisher {
	return &InMemoryEventPublisher{
		subscribers: make(map[string][]EventHandler),
		ordered:     false,
	}
}

// WithOrdering включает упорядочивание событий
func (p *InMemoryEventPublisher) WithOrdering(ordered bool) *InMemoryEventPublisher {
	p.ordered = ordered
	return p
}

// WithRetry настраивает retry логику
func (p *InMemoryEventPublisher) WithRetry(config RetryConfig) *InMemoryEventPublisher {
	p.retryConfig = &config
	return p
}

// retryPublish выполняет публикацию с retry
func (p *InMemoryEventPublisher) retryPublish(ctx context.Context, event Event, handler EventHandler) error {
	if p.retryConfig == nil {
		return handler.Handle(ctx, event)
	}

	var lastErr error
	delay := p.retryConfig.InitialDelay

	for attempt := 0; attempt < p.retryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := handler.Handle(ctx, event)
		if err == nil {
			return nil
		}

		lastErr = err
		delay = time.Duration(float64(delay) * p.retryConfig.BackoffMultiplier)
		if delay > p.retryConfig.MaxDelay {
			delay = p.retryConfig.MaxDelay
		}
	}

	return fmt.Errorf("publish failed after %d attempts: %w", p.retryConfig.MaxAttempts, lastErr)
}

// Publish публикует событие
func (p *InMemoryEventPublisher) Publish(ctx context.Context, event Event) error {
	p.mu.RLock()
	handlers := p.subscribers[event.EventType()]
	ordered := p.ordered
	p.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	// Если включена упорядоченная доставка, обрабатываем последовательно
	if ordered {
		var errors []error
		for _, handler := range handlers {
			var err error
			if p.retryConfig != nil {
				err = p.retryPublish(ctx, event, handler)
			} else {
				err = handler.Handle(ctx, event)
			}
			if err != nil {
				errors = append(errors, fmt.Errorf("handler %s failed: %w", handler.EventType(), err))
			}
		}

		if len(errors) > 0 {
			return fmt.Errorf("publish failed: %v", errors)
		}
		return nil
	}

	// Параллельная обработка (по умолчанию)
	var wg sync.WaitGroup
	errCh := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			var err error
			if p.retryConfig != nil {
				err = p.retryPublish(ctx, event, h)
			} else {
				err = h.Handle(ctx, event)
			}
			if err != nil {
				errCh <- fmt.Errorf("handler %s failed: %w", h.EventType(), err)
			}
		}(handler)
	}

	wg.Wait()
	close(errCh)

	// Собираем ошибки
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("publish failed: %v", errors)
	}

	return nil
}

// Subscribe подписывается на события (для совместимости с EventBus)
// ВАЖНО: InMemoryEventPublisher должен использовать общий subscribers с EventSubscriber
func (p *InMemoryEventPublisher) Subscribe(eventType string, handler EventHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.subscribers == nil {
		p.subscribers = make(map[string][]EventHandler)
	}
	p.subscribers[eventType] = append(p.subscribers[eventType], handler)
	return nil
}

// AsyncEventPublisher асинхронный публикатор событий
type AsyncEventPublisher struct {
	*InMemoryEventPublisher
	queue    chan eventMessage
	workers  int
	stopCh   chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
}

type eventMessage struct {
	ctx   context.Context
	event Event
}

// NewAsyncEventPublisher создает новый асинхронный публикатор
func NewAsyncEventPublisher(workers int, queueSize int) *AsyncEventPublisher {
	p := &AsyncEventPublisher{
		InMemoryEventPublisher: NewInMemoryEventPublisher(),
		queue:                  make(chan eventMessage, queueSize),
		workers:                workers,
		stopCh:                 make(chan struct{}),
	}

	// Запускаем воркеры
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	return p
}

func (p *AsyncEventPublisher) worker() {
	defer p.wg.Done()
	for {
		select {
		case msg, ok := <-p.queue:
			if !ok {
				return
			}
			_ = p.InMemoryEventPublisher.Publish(msg.ctx, msg.event)
		case <-p.stopCh:
			// Drain queue before stopping
			for {
				select {
				case msg := <-p.queue:
					_ = p.InMemoryEventPublisher.Publish(msg.ctx, msg.event)
				default:
					return
				}
			}
		}
	}
}

// WithRetry настраивает retry логику для асинхронного публикатора
func (p *AsyncEventPublisher) WithRetry(config RetryConfig) *AsyncEventPublisher {
	p.InMemoryEventPublisher.WithRetry(config)
	return p
}

// Publish публикует событие асинхронно
func (p *AsyncEventPublisher) Publish(ctx context.Context, event Event) error {
	select {
	case p.queue <- eventMessage{ctx: ctx, event: event}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-p.stopCh:
		return fmt.Errorf("publisher is stopped")
	}
}

// Stop останавливает публикатор с graceful shutdown
// Метод идемпотентен: повторные вызовы не приведут к panic
func (p *AsyncEventPublisher) Stop(ctx context.Context) error {
	var err error
	p.stopOnce.Do(func() {
		close(p.stopCh)
		
		// Ждем завершения всех воркеров
		done := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			err = nil
		case <-ctx.Done():
			err = ctx.Err()
		}
	})
	return err
}

// BatchEventPublisher пакетный публикатор событий
type BatchEventPublisher struct {
	*InMemoryEventPublisher
	batch     []Event
	batchSize int
	flushInterval time.Duration
	mu        sync.Mutex
	ticker    *time.Ticker
	stopCh    chan struct{}
}

// NewBatchEventPublisher создает новый пакетный публикатор
func NewBatchEventPublisher(batchSize int, flushInterval time.Duration) *BatchEventPublisher {
	p := &BatchEventPublisher{
		InMemoryEventPublisher: NewInMemoryEventPublisher(),
		batch:                  make([]Event, 0, batchSize),
		batchSize:              batchSize,
		flushInterval:          flushInterval,
		stopCh:                 make(chan struct{}),
	}

	p.ticker = time.NewTicker(flushInterval)
	go p.flushLoop()

	return p
}

func (p *BatchEventPublisher) flushLoop() {
	for {
		select {
		case <-p.ticker.C:
			_ = p.Flush(context.Background())
		case <-p.stopCh:
			return
		}
	}
}

// Publish добавляет событие в пакет
func (p *BatchEventPublisher) Publish(ctx context.Context, event Event) error {
	p.mu.Lock()
	p.batch = append(p.batch, event)
	shouldFlush := len(p.batch) >= p.batchSize
	p.mu.Unlock()

	if shouldFlush {
		return p.Flush(ctx)
	}

	return nil
}

// Flush публикует все события из пакета
func (p *BatchEventPublisher) Flush(ctx context.Context) error {
	p.mu.Lock()
	events := make([]Event, len(p.batch))
	copy(events, p.batch)
	p.batch = p.batch[:0]
	p.mu.Unlock()

	for _, event := range events {
		if err := p.InMemoryEventPublisher.Publish(ctx, event); err != nil {
			return err
		}
	}

	return nil
}

// Stop останавливает публикатор
func (p *BatchEventPublisher) Stop(ctx context.Context) error {
	p.ticker.Stop()
	close(p.stopCh)
	return p.Flush(ctx)
}

// WithRetry настраивает retry логику для пакетного публикатора
func (p *BatchEventPublisher) WithRetry(config RetryConfig) *BatchEventPublisher {
	p.InMemoryEventPublisher.WithRetry(config)
	return p
}

