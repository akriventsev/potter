// Package events предоставляет реализацию EventBus.
package events

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// InMemoryEventBus реализация шины событий
type InMemoryEventBus struct {
	publisher  *InMemoryEventPublisher
	subscriber *InMemoryEventSubscriber
	middleware []EventMiddleware
	dlq        DeadLetterQueue
	mu         sync.RWMutex
	shutdown   chan struct{}
	wg         sync.WaitGroup // для отслеживания активных публикаций
	shutdownMu sync.Mutex
	stopped    bool
}

// EventMiddleware middleware для событий
type EventMiddleware func(ctx context.Context, event Event, next func(ctx context.Context, event Event) error) error

// DeadLetterQueue интерфейс для dead letter queue
type DeadLetterQueue interface {
	Publish(ctx context.Context, event Event, reason string) error
}

// NewInMemoryEventBus создает новую шину событий
func NewInMemoryEventBus() *InMemoryEventBus {
	publisher := NewInMemoryEventPublisher()
	subscriber := NewInMemoryEventSubscriber()

	// Связываем publisher и subscriber через общий subscribers map
	// Используем subscribers из subscriber для publisher
	publisher.subscribers = subscriber.handlers

	return &InMemoryEventBus{
		publisher:  publisher,
		subscriber: subscriber,
		middleware:  make([]EventMiddleware, 0),
		shutdown:    make(chan struct{}),
	}
}

// WithMiddleware добавляет middleware к шине
func (b *InMemoryEventBus) WithMiddleware(middleware EventMiddleware) *InMemoryEventBus {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.middleware = append(b.middleware, middleware)
	return b
}

// WithDeadLetterQueue устанавливает DLQ
func (b *InMemoryEventBus) WithDeadLetterQueue(dlq DeadLetterQueue) *InMemoryEventBus {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dlq = dlq
	return b
}

// Publish публикует событие
func (b *InMemoryEventBus) Publish(ctx context.Context, event Event) error {
	// Проверяем, не остановлена ли шина
	b.shutdownMu.Lock()
	if b.stopped {
		b.shutdownMu.Unlock()
		return fmt.Errorf("event bus is stopped")
	}
	b.shutdownMu.Unlock()

	// Инкрементируем WaitGroup для отслеживания активных публикаций
	b.wg.Add(1)
	defer b.wg.Done()

	// Применяем middleware
	next := func(ctx context.Context, event Event) error {
		return b.publisher.Publish(ctx, event)
	}

	for i := len(b.middleware) - 1; i >= 0; i-- {
		mw := b.middleware[i]
		prevNext := next
		next = func(ctx context.Context, event Event) error {
			return mw(ctx, event, prevNext)
		}
	}

	err := next(ctx, event)
	if err != nil && b.dlq != nil {
		_ = b.dlq.Publish(ctx, event, err.Error())
	}

	return err
}

// Subscribe подписывается на тип события
func (b *InMemoryEventBus) Subscribe(eventType string, handler EventHandler) error {
	return b.subscriber.Subscribe(eventType, handler)
}

// Unsubscribe отписывается от типа события
func (b *InMemoryEventBus) Unsubscribe(eventType string, handler EventHandler) error {
	return b.subscriber.Unsubscribe(eventType, handler)
}

// Replay воспроизводит события из истории
func (b *InMemoryEventBus) Replay(ctx context.Context, events []Event) error {
	for _, event := range events {
		if err := b.Publish(ctx, event); err != nil {
			return fmt.Errorf("failed to replay event %s: %w", event.EventID(), err)
		}
	}
	return nil
}

// Shutdown корректно завершает работу шины
func (b *InMemoryEventBus) Shutdown(ctx context.Context) error {
	b.shutdownMu.Lock()
	if b.stopped {
		b.shutdownMu.Unlock()
		return nil // Идемпотентный вызов
	}
	b.stopped = true
	close(b.shutdown)
	b.shutdownMu.Unlock()

	// Ждем завершения всех активных публикаций
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		// Жесткий таймаут после ожидания активных публикаций
		return fmt.Errorf("shutdown timeout after waiting for active publications")
	}
}

