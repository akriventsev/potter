// Package events предоставляет базовые интерфейсы для работы с доменными событиями.
package events

import (
	"context"
	"fmt"
	"time"
)

// Event представляет доменное событие
type Event interface {
	// EventID возвращает уникальный идентификатор события
	EventID() string
	// EventType возвращает тип события
	EventType() string
	// OccurredAt возвращает время возникновения события
	OccurredAt() time.Time
	// AggregateID возвращает идентификатор агрегата
	AggregateID() string
	// Metadata возвращает метаданные события
	Metadata() EventMetadata
}

// EventMetadata метаданные события
type EventMetadata map[string]interface{}

// Get получает значение метаданных по ключу
func (m EventMetadata) Get(key string) (interface{}, bool) {
	val, ok := m[key]
	return val, ok
}

// Set устанавливает значение метаданных
func (m EventMetadata) Set(key string, value interface{}) {
	if m == nil {
		m = make(EventMetadata)
	}
	m[key] = value
}

// CorrelationID возвращает correlation ID
func (m EventMetadata) CorrelationID() string {
	val, ok := m.Get("correlation_id")
	if !ok {
		return ""
	}
	if id, ok := val.(string); ok {
		return id
	}
	return ""
}

// CausationID возвращает causation ID
func (m EventMetadata) CausationID() string {
	val, ok := m.Get("causation_id")
	if !ok {
		return ""
	}
	if id, ok := val.(string); ok {
		return id
	}
	return ""
}

// UserID возвращает ID пользователя
func (m EventMetadata) UserID() string {
	val, ok := m.Get("user_id")
	if !ok {
		return ""
	}
	if id, ok := val.(string); ok {
		return id
	}
	return ""
}

// BaseEvent базовая реализация события
type BaseEvent struct {
	eventID     string
	eventType   string
	occurredAt  time.Time
	aggregateID string
	metadata    EventMetadata
}

// NewBaseEvent создает новое базовое событие
func NewBaseEvent(eventType, aggregateID string) *BaseEvent {
	return &BaseEvent{
		eventID:     generateEventID(),
		eventType:   eventType,
		occurredAt:  time.Now(),
		aggregateID: aggregateID,
		metadata:    make(EventMetadata),
	}
}

// WithMetadata добавляет метаданные к событию
func (e *BaseEvent) WithMetadata(key string, value interface{}) *BaseEvent {
	e.metadata.Set(key, value)
	return e
}

// WithCorrelationID устанавливает correlation ID
func (e *BaseEvent) WithCorrelationID(id string) *BaseEvent {
	e.metadata.Set("correlation_id", id)
	return e
}

// WithCausationID устанавливает causation ID
func (e *BaseEvent) WithCausationID(id string) *BaseEvent {
	e.metadata.Set("causation_id", id)
	return e
}

// WithUserID устанавливает user ID
func (e *BaseEvent) WithUserID(id string) *BaseEvent {
	e.metadata.Set("user_id", id)
	return e
}

func (e *BaseEvent) EventID() string {
	return e.eventID
}

func (e *BaseEvent) EventType() string {
	return e.eventType
}

func (e *BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

func (e *BaseEvent) AggregateID() string {
	return e.aggregateID
}

func (e *BaseEvent) Metadata() EventMetadata {
	return e.metadata
}

// EventHandler обработчик доменных событий
type EventHandler interface {
	// Handle обрабатывает событие
	Handle(ctx context.Context, event Event) error
	// EventType возвращает тип события, который обрабатывает этот handler
	EventType() string
}

// EventPublisher публикатор событий
type EventPublisher interface {
	// Publish публикует событие
	Publish(ctx context.Context, event Event) error
}

// EventSubscriber подписчик на события
type EventSubscriber interface {
	// Subscribe подписывается на тип события
	Subscribe(eventType string, handler EventHandler) error
	// Unsubscribe отписывается от типа события
	Unsubscribe(eventType string, handler EventHandler) error
}

// EventBus объединяет Publisher и Subscriber
type EventBus interface {
	EventPublisher
	EventSubscriber
}

// generateEventID генерирует уникальный ID события
func generateEventID() string {
	// Используем timestamp + наносекунды для уникальности
	return fmt.Sprintf("event-%d", time.Now().UnixNano())
}

