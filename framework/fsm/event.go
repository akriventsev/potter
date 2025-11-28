// Package fsm предоставляет определения событий для FSM.
package fsm

import (
	"context"
	"time"
)

// Event представляет событие, которое может вызвать переход в FSM
type Event interface {
	// Name возвращает имя события
	Name() string
	// Data возвращает данные события
	Data() interface{}
	// Timestamp возвращает время создания события
	Timestamp() time.Time
}

// EventMetadata метаданные события
type EventMetadata map[string]interface{}

// Get получает значение метаданных
func (m EventMetadata) Get(key string) (interface{}, bool) {
	val, ok := m[key]
	return val, ok
}

// Set устанавливает значение метаданных
func (m EventMetadata) Set(key string, value interface{}) {
	m[key] = value
}

// BaseEvent базовая реализация события
type BaseEvent struct {
	name      string
	data      interface{}
	timestamp time.Time
	metadata  EventMetadata
	priority  int
}

// NewEvent создает новое событие
func NewEvent(name string, data interface{}) *BaseEvent {
	return &BaseEvent{
		name:      name,
		data:      data,
		timestamp: time.Now(),
		metadata:  make(EventMetadata),
		priority:  0,
	}
}

// WithMetadata добавляет метаданные к событию
func (e *BaseEvent) WithMetadata(key string, value interface{}) *BaseEvent {
	e.metadata.Set(key, value)
	return e
}

// WithPriority устанавливает приоритет события
func (e *BaseEvent) WithPriority(priority int) *BaseEvent {
	e.priority = priority
	return e
}

func (e *BaseEvent) Name() string {
	return e.name
}

func (e *BaseEvent) Data() interface{} {
	return e.data
}

func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

// Priority возвращает приоритет события
func (e *BaseEvent) Priority() int {
	return e.priority
}

// EventData типизированные данные события
type EventData map[string]interface{}

// Get получает значение по ключу
func (d EventData) Get(key string) (interface{}, bool) {
	val, ok := d[key]
	return val, ok
}

// GetString получает строковое значение
func (d EventData) GetString(key string) (string, bool) {
	val, ok := d.Get(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt получает целочисленное значение
func (d EventData) GetInt(key string) (int, bool) {
	val, ok := d.Get(key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// TransitionHook функция-хук, вызываемая при переходе
type TransitionHook func(ctx context.Context, from State, to State, event Event) error

// Guard функция-охранник, проверяющая возможность перехода
type Guard func(ctx context.Context, from State, to State, event Event) (bool, error)

