// Package invoke предоставляет EventAwaiter для ожидания событий по correlation ID.
package invoke

import (
	"context"
	"fmt"
	"sync"
	"time"

	"potter/framework/events"
	"potter/framework/transport"
)

// EventAwaiter consumer событий по correlation ID с timeout
type EventAwaiter struct {
	eventSource EventSource
	waiters     map[string]*eventWaiter
	mu          sync.RWMutex
	stopCh      chan struct{}
	stopped     bool
	wg          sync.WaitGroup
}

// eventWaiter структура для ожидания события
type eventWaiter struct {
	eventType     string
	ch            chan events.Event
	timeout       time.Duration
	correlationID string
	createdAt     time.Time
}

// NewEventAwaiter создает новый EventAwaiter
func NewEventAwaiter(eventSource EventSource) *EventAwaiter {
	awaiter := &EventAwaiter{
		eventSource: eventSource,
		waiters:     make(map[string]*eventWaiter),
		stopCh:      make(chan struct{}),
		stopped:     false,
	}

	// Запускаем cleanup goroutine
	awaiter.wg.Add(1)
	go awaiter.cleanup()

	return awaiter
}

// NewEventAwaiterFromEventBus создает EventAwaiter из events.EventBus через адаптер
func NewEventAwaiterFromEventBus(eventBus events.EventBus) *EventAwaiter {
	return NewEventAwaiter(NewEventBusAdapter(eventBus))
}

// NewEventAwaiterFromTransport создает EventAwaiter из transport.Subscriber через адаптер
func NewEventAwaiterFromTransport(
	subscriber transport.Subscriber,
	serializer transport.MessageSerializer,
	resolver SubjectResolver,
) *EventAwaiter {
	return NewEventAwaiter(NewTransportSubscriberAdapter(subscriber, serializer, resolver))
}

// Await ожидает событие по correlation ID с timeout
func (a *EventAwaiter) Await(ctx context.Context, correlationID string, eventType string, timeout time.Duration) (events.Event, error) {
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return nil, NewEventAwaiterStoppedError()
	}

	// Создаем waiter
	waiter := &eventWaiter{
		eventType:     eventType,
		ch:            make(chan events.Event, 1),
		timeout:       timeout,
		correlationID: correlationID,
		createdAt:     time.Now(),
	}

	a.waiters[correlationID] = waiter
	a.mu.Unlock()

	// Подписываемся на тип события (если еще не подписаны)
	// EventSource будет доставлять события всем подписчикам
	handler := &correlationEventHandler{
		awaiter:       a,
		targetType:    eventType,
		correlationID: correlationID,
	}
	if err := a.eventSource.Subscribe(eventType, handler); err != nil {
		a.mu.Lock()
		delete(a.waiters, correlationID)
		a.mu.Unlock()
		return nil, fmt.Errorf("failed to subscribe to event type %s: %w", eventType, err)
	}

	// Ждем событие или timeout
	select {
	case event := <-waiter.ch:
		a.mu.Lock()
		delete(a.waiters, correlationID)
		a.mu.Unlock()
		return event, nil
	case <-time.After(timeout):
		a.mu.Lock()
		delete(a.waiters, correlationID)
		a.mu.Unlock()
		return nil, NewEventTimeoutError(correlationID, timeout.String())
	case <-ctx.Done():
		a.mu.Lock()
		delete(a.waiters, correlationID)
		a.mu.Unlock()
		return nil, ctx.Err()
	case <-a.stopCh:
		a.mu.Lock()
		delete(a.waiters, correlationID)
		a.mu.Unlock()
		return nil, NewEventAwaiterStoppedError()
	}
}

// AwaitMultiple ожидает несколько событий по correlation ID
func (a *EventAwaiter) AwaitMultiple(ctx context.Context, correlationID string, eventTypes []string, timeout time.Duration) ([]events.Event, error) {
	results := make([]events.Event, 0, len(eventTypes))
	errors := make([]error, 0)

	for _, eventType := range eventTypes {
		event, err := a.Await(ctx, correlationID, eventType, timeout)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		results = append(results, event)
	}

	if len(errors) > 0 {
		return results, fmt.Errorf("failed to await some events: %v", errors)
	}

	return results, nil
}

// AwaitAny ожидает любое из указанных событий (первое полученное)
// Возвращает событие, тип полученного события и ошибку
func (a *EventAwaiter) AwaitAny(ctx context.Context, correlationID string, eventTypes []string, timeout time.Duration) (events.Event, string, error) {
	if len(eventTypes) == 0 {
		return nil, "", fmt.Errorf("at least one event type must be specified")
	}

	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return nil, "", NewEventAwaiterStoppedError()
	}

	// Создаем waiter для любого из типов событий
	waiter := &eventWaiter{
		eventType:     "", // Пустой тип означает "любой из указанных"
		ch:            make(chan events.Event, 1),
		timeout:       timeout,
		correlationID: correlationID,
		createdAt:     time.Now(),
	}

	a.waiters[correlationID] = waiter
	a.mu.Unlock()

	// Подписываемся на все типы событий
	handlers := make([]events.EventHandler, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		handler := &correlationEventHandler{
			awaiter:       a,
			targetType:    eventType,
			correlationID: correlationID,
		}
		if err := a.eventSource.Subscribe(eventType, handler); err != nil {
			// Отписываемся от уже подписанных
			for _, h := range handlers {
				_ = a.eventSource.Unsubscribe(h.EventType(), h)
			}
			a.mu.Lock()
			delete(a.waiters, correlationID)
			a.mu.Unlock()
			return nil, "", fmt.Errorf("failed to subscribe to event type %s: %w", eventType, err)
		}
		handlers = append(handlers, handler)
	}

	// Ждем событие или timeout
	select {
	case event := <-waiter.ch:
		// Отписываемся от всех типов
		for _, handler := range handlers {
			_ = a.eventSource.Unsubscribe(handler.EventType(), handler)
		}
		a.mu.Lock()
		delete(a.waiters, correlationID)
		receivedType := event.EventType()
		a.mu.Unlock()
		return event, receivedType, nil
	case <-time.After(timeout):
		// Отписываемся от всех типов
		for _, handler := range handlers {
			_ = a.eventSource.Unsubscribe(handler.EventType(), handler)
		}
		a.mu.Lock()
		delete(a.waiters, correlationID)
		a.mu.Unlock()
		return nil, "", NewEventTimeoutError(correlationID, timeout.String())
	case <-ctx.Done():
		// Отписываемся от всех типов
		for _, handler := range handlers {
			_ = a.eventSource.Unsubscribe(handler.EventType(), handler)
		}
		a.mu.Lock()
		delete(a.waiters, correlationID)
		a.mu.Unlock()
		return nil, "", ctx.Err()
	case <-a.stopCh:
		// Отписываемся от всех типов
		for _, handler := range handlers {
			_ = a.eventSource.Unsubscribe(handler.EventType(), handler)
		}
		a.mu.Lock()
		delete(a.waiters, correlationID)
		a.mu.Unlock()
		return nil, "", NewEventAwaiterStoppedError()
	}
}

// AwaitSuccessOrError ожидает успешное или ошибочное событие
// Возвращает событие, флаг успеха (true для успеха, false для ошибки) и ошибку
func (a *EventAwaiter) AwaitSuccessOrError(ctx context.Context, correlationID string, successType, errorType string, timeout time.Duration) (events.Event, bool, error) {
	eventTypes := []string{successType, errorType}
	event, receivedType, err := a.AwaitAny(ctx, correlationID, eventTypes, timeout)
	if err != nil {
		return nil, false, err
	}

	isSuccess := receivedType == successType
	return event, isSuccess, nil
}

// Cancel отменяет ожидание события по correlation ID
func (a *EventAwaiter) Cancel(correlationID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if waiter, exists := a.waiters[correlationID]; exists {
		close(waiter.ch)
		delete(a.waiters, correlationID)
	}
}

// handleEvent обрабатывает событие и матчит его по correlation ID
func (a *EventAwaiter) handleEvent(ctx context.Context, event events.Event) {
	if event == nil {
		return
	}

	correlationID := event.Metadata().CorrelationID()
	if correlationID == "" {
		return
	}

	a.mu.RLock()
	waiter, exists := a.waiters[correlationID]
	a.mu.RUnlock()

	if !exists {
		return
	}

	// Проверяем тип события (если eventType пустой, принимаем любое событие для AwaitAny)
	if waiter.eventType != "" && event.EventType() != waiter.eventType {
		return
	}

	// Отправляем событие в канал
	select {
	case waiter.ch <- event:
	default:
		// Канал уже закрыт или полон
	}
}

// cleanup периодически очищает устаревшие waiters
func (a *EventAwaiter) cleanup() {
	defer a.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.mu.Lock()
			now := time.Now()
			for correlationID, waiter := range a.waiters {
				if now.Sub(waiter.createdAt) > waiter.timeout+5*time.Minute {
					close(waiter.ch)
					delete(a.waiters, correlationID)
				}
			}
			a.mu.Unlock()
		case <-a.stopCh:
			return
		}
	}
}

// Stop останавливает EventAwaiter
func (a *EventAwaiter) Stop(ctx context.Context) error {
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return nil
	}
	a.stopped = true
	close(a.stopCh)

	// Закрываем все waiters
	for correlationID, waiter := range a.waiters {
		close(waiter.ch)
		delete(a.waiters, correlationID)
	}
	a.mu.Unlock()

	// Ждем завершения goroutines
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// correlationEventHandler handler для матчинга событий по correlation ID
type correlationEventHandler struct {
	awaiter       *EventAwaiter
	targetType    string
	correlationID string
}

func (h *correlationEventHandler) Handle(ctx context.Context, event events.Event) error {
	// Проверяем correlation ID
	if event.Metadata().CorrelationID() != h.correlationID {
		return nil
	}

	// Проверяем тип события
	if h.targetType != "" && event.EventType() != h.targetType {
		return nil
	}

	h.awaiter.handleEvent(ctx, event)
	return nil
}

func (h *correlationEventHandler) EventType() string {
	return h.targetType
}

