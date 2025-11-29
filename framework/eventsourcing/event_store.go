// Package eventsourcing предоставляет полную поддержку Event Sourcing паттерна.
package eventsourcing

import (
	"context"
	"errors"
	"time"

	"potter/framework/events"
)

var (
	// ErrConcurrencyConflict возникает при конфликте версий при сохранении событий
	ErrConcurrencyConflict = errors.New("concurrency conflict: expected version does not match current version")
	// ErrStreamNotFound возникает когда поток событий агрегата не найден
	ErrStreamNotFound = errors.New("event stream not found")
	// ErrInvalidVersion возникает при некорректной версии события
	ErrInvalidVersion = errors.New("invalid event version")
)

// StoredEvent представляет сохраненное событие с метаданными
type StoredEvent struct {
	ID           string
	AggregateID  string
	AggregateType string
	EventType    string
	EventData    events.Event
	Metadata     map[string]interface{}
	Version      int64
	Position     int64
	OccurredAt   time.Time
	CreatedAt    time.Time
}

// EventStream представляет поток событий агрегата
type EventStream struct {
	AggregateID string
	Events      []StoredEvent
	Metadata    StreamMetadata
}

// StreamMetadata содержит метаданные потока событий
type StreamMetadata struct {
	CurrentVersion int64
	EventCount     int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EventDeserializer интерфейс для десериализации событий из хранилища
type EventDeserializer interface {
	// DeserializeEvent десериализует JSON/BSON данные в конкретный тип события
	DeserializeEvent(eventType string, data []byte) (events.Event, error)
}

// EventStore интерфейс для хранения событий
type EventStore interface {
	// AppendEvents добавляет события в поток агрегата с проверкой версии для оптимистичной конкурентности
	AppendEvents(ctx context.Context, aggregateID string, expectedVersion int64, events []events.Event) error

	// GetEvents возвращает все события агрегата начиная с указанной версии
	GetEvents(ctx context.Context, aggregateID string, fromVersion int64) ([]StoredEvent, error)

	// GetEventsByType возвращает события определенного типа начиная с указанного времени
	GetEventsByType(ctx context.Context, eventType string, fromTimestamp time.Time) ([]StoredEvent, error)

	// GetAllEvents возвращает все события начиная с указанной позиции для replay
	GetAllEvents(ctx context.Context, fromPosition int64) (<-chan StoredEvent, error)
}

