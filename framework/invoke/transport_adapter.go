// Package invoke предоставляет адаптеры для работы EventAwaiter с различными транспортами.
package invoke

import (
	"context"
	"fmt"
	"sync"

	"potter/framework/events"
	"potter/framework/transport"
)

// EventSource интерфейс для унификации источников событий
type EventSource interface {
	// Subscribe подписывается на тип события и вызывает handler при получении
	Subscribe(eventType string, handler events.EventHandler) error
	// Unsubscribe отписывается от типа события
	Unsubscribe(eventType string, handler events.EventHandler) error
}

// EventBusAdapter адаптер для events.EventBus, реализующий EventSource
type EventBusAdapter struct {
	events.EventBus
}

// NewEventBusAdapter создает новый адаптер для EventBus
func NewEventBusAdapter(eventBus events.EventBus) *EventBusAdapter {
	return &EventBusAdapter{
		EventBus: eventBus,
	}
}

// Subscribe делегирует подписку к EventBus
func (a *EventBusAdapter) Subscribe(eventType string, handler events.EventHandler) error {
	return a.EventBus.Subscribe(eventType, handler)
}

// Unsubscribe делегирует отписку к EventBus
func (a *EventBusAdapter) Unsubscribe(eventType string, handler events.EventHandler) error {
	return a.EventBus.Unsubscribe(eventType, handler)
}

// TransportSubscriberAdapter адаптер для transport.Subscriber, реализующий EventSource
type TransportSubscriberAdapter struct {
	subscriber     transport.Subscriber
	serializer     transport.MessageSerializer
	subjectResolver SubjectResolver
	handlers       map[string][]events.EventHandler
	mu             sync.RWMutex
	subscribedSubjects map[string]bool
}

// NewTransportSubscriberAdapter создает новый адаптер для transport.Subscriber
func NewTransportSubscriberAdapter(
	subscriber transport.Subscriber,
	serializer transport.MessageSerializer,
	resolver SubjectResolver,
) *TransportSubscriberAdapter {
	return &TransportSubscriberAdapter{
		subscriber:         subscriber,
		serializer:         serializer,
		subjectResolver:    resolver,
		handlers:           make(map[string][]events.EventHandler),
		subscribedSubjects: make(map[string]bool),
	}
}

// Subscribe подписывается на тип события через transport.Subscriber
func (a *TransportSubscriberAdapter) Subscribe(eventType string, handler events.EventHandler) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Определяем subject через resolver
	subject := a.subjectResolver.ResolveEventSubject(eventType)
	if subject == "" {
		return fmt.Errorf("failed to resolve subject for event type: %s", eventType)
	}

	// Добавляем handler в список
	a.handlers[eventType] = append(a.handlers[eventType], handler)

	// Если еще не подписаны на subject, создаем подписку
	if !a.subscribedSubjects[subject] {
		// Создаем обертку для обработки сообщений из транспорта
		messageHandler := func(ctx context.Context, msg *transport.Message) error {
			// Десериализуем событие
			// Для упрощения предполагаем, что событие можно десериализовать
			// В реальной реализации нужно знать тип события заранее
			// Здесь используем базовый подход - пытаемся найти handler по типу из headers
			eventTypeFromHeader := msg.Headers["event_type"]
			if eventTypeFromHeader == "" {
				eventTypeFromHeader = eventType
			}

			// Получаем handlers для этого типа события
			a.mu.RLock()
			handlers := a.handlers[eventTypeFromHeader]
			a.mu.RUnlock()

			// Вызываем все handlers
			for _, h := range handlers {
				// Создаем базовое событие из сообщения
				// В реальной реализации нужно десериализовать конкретный тип события
				// Здесь используем упрощенный подход
				baseEvent := events.NewBaseEvent(eventTypeFromHeader, msg.Headers["aggregate_id"])
				if correlationID := msg.Headers["correlation_id"]; correlationID != "" {
					baseEvent.WithCorrelationID(correlationID)
				}
				if causationID := msg.Headers["causation_id"]; causationID != "" {
					baseEvent.WithCausationID(causationID)
				}

				// Вызываем handler
				if err := h.Handle(ctx, baseEvent); err != nil {
					return fmt.Errorf("handler error: %w", err)
				}
			}

			return nil
		}

		// Подписываемся на subject
		// Используем background context для подписки, так как это долгоживущая операция
		if err := a.subscriber.Subscribe(context.Background(), subject, messageHandler); err != nil {
			return fmt.Errorf("failed to subscribe to subject %s: %w", subject, err)
		}

		a.subscribedSubjects[subject] = true
	}

	return nil
}

// Unsubscribe отписывается от типа события
func (a *TransportSubscriberAdapter) Unsubscribe(eventType string, handler events.EventHandler) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	handlers := a.handlers[eventType]
	for i, h := range handlers {
		if h == handler {
			a.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}

	// Если handlers больше нет, можно отписаться от subject
	if len(a.handlers[eventType]) == 0 {
		subject := a.subjectResolver.ResolveEventSubject(eventType)
		if subject != "" {
			if err := a.subscriber.Unsubscribe(subject); err != nil {
				return fmt.Errorf("failed to unsubscribe from subject %s: %w", subject, err)
			}
			delete(a.subscribedSubjects, subject)
		}
	}

	return nil
}

