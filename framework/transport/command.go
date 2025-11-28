// Package transport предоставляет интерфейсы и реализации для работы с командами CQRS.
package transport

import (
	"context"
	"time"
)

// Command представляет команду CQRS
type Command interface {
	CommandName() string
}

// CommandMetadata интерфейс для метаданных команд
type CommandMetadata interface {
	// ID возвращает уникальный идентификатор команды
	ID() string
	// Timestamp возвращает время создания команды
	Timestamp() time.Time
	// CorrelationID возвращает correlation ID для трассировки
	CorrelationID() string
	// CausationID возвращает causation ID (ID команды, вызвавшей эту)
	CausationID() string
}

// CommandHandler обработчик команд
type CommandHandler interface {
	Handle(ctx context.Context, cmd Command) error
	CommandName() string
}

// CommandValidator интерфейс для валидации команд
type CommandValidator interface {
	Validate(ctx context.Context, cmd Command) error
}

// CommandInterceptor интерфейс для перехвата команд
type CommandInterceptor interface {
	// Intercept вызывается перед выполнением команды
	Intercept(ctx context.Context, cmd Command, next func(ctx context.Context, cmd Command) error) error
}

// CommandBus шина команд
type CommandBus interface {
	Send(ctx context.Context, cmd Command) error
	Register(handler CommandHandler) error
}

// BaseCommandMetadata базовая реализация метаданных команды
type BaseCommandMetadata struct {
	id            string
	timestamp     time.Time
	correlationID string
	causationID   string
}

// NewBaseCommandMetadata создает новые метаданные команды
func NewBaseCommandMetadata(id, correlationID, causationID string) *BaseCommandMetadata {
	return &BaseCommandMetadata{
		id:            id,
		timestamp:     time.Now(),
		correlationID: correlationID,
		causationID:   causationID,
	}
}

func (m *BaseCommandMetadata) ID() string {
	return m.id
}

func (m *BaseCommandMetadata) Timestamp() time.Time {
	return m.timestamp
}

func (m *BaseCommandMetadata) CorrelationID() string {
	return m.correlationID
}

func (m *BaseCommandMetadata) CausationID() string {
	return m.causationID
}

