// Package transport предоставляет интерфейсы и реализации для работы с командами CQRS.
package transport

import (
	"context"
	"time"

	"github.com/google/uuid"
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

// BaseCommand базовая команда с метаданными
type BaseCommand struct {
	metadata    CommandMetadata
	commandName string
}

// NewBaseCommand создает новую базовую команду с метаданными и именем команды
// commandName - обязательное имя команды для маршрутизации
func NewBaseCommand(commandName string, metadata CommandMetadata) *BaseCommand {
	return &BaseCommand{
		metadata:    metadata,
		commandName: commandName,
	}
}

// NewBaseCommandSimple создает новую базовую команду с простыми параметрами (для обратной совместимости)
// Используется в examples для создания команд с именем и aggregateID
func NewBaseCommandSimple(commandName, aggregateID string) *BaseCommand {
	metadata := NewBaseCommandMetadata(
		uuid.New().String(),
		"",
		"",
	)
	return &BaseCommand{
		metadata:    metadata,
		commandName: commandName,
	}
}

// Metadata возвращает метаданные команды
func (c *BaseCommand) Metadata() CommandMetadata {
	return c.metadata
}

// CommandName возвращает имя команды
// Если commandName не установлен, возвращает пустую строку
// Команды должны переопределить этот метод или установить commandName
func (c *BaseCommand) CommandName() string {
	return c.commandName
}

// NewBaseCommandWithCorrelation создает команду с correlation ID и именем команды
// commandName - обязательное имя команды для маршрутизации
func NewBaseCommandWithCorrelation(commandName string, correlationID string) *BaseCommand {
	metadata := NewBaseCommandMetadata(
		uuid.New().String(),
		correlationID,
		"",
	)
	return NewBaseCommand(commandName, metadata)
}

