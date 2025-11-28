// Package core предоставляет базовые типы для всех компонентов фреймворка.
package core

import (
	"context"
)

// Context расширенный контекст с дополнительными методами для работы с метаданными
type Context interface {
	context.Context
	// GetMetadata возвращает метаданные из контекста
	GetMetadata(key string) (interface{}, bool)
	// SetMetadata устанавливает метаданные в контекст
	SetMetadata(key string, value interface{})
	// GetCorrelationID возвращает correlation ID
	GetCorrelationID() string
	// GetCausationID возвращает causation ID
	GetCausationID() string
}

// FrameworkContext реализация расширенного контекста
type FrameworkContext struct {
	context.Context
	metadata map[string]interface{}
}

// NewFrameworkContext создает новый расширенный контекст
func NewFrameworkContext(ctx context.Context) *FrameworkContext {
	return &FrameworkContext{
		Context:  ctx,
		metadata: make(map[string]interface{}),
	}
}

// GetMetadata возвращает метаданные из контекста
func (c *FrameworkContext) GetMetadata(key string) (interface{}, bool) {
	val, ok := c.metadata[key]
	return val, ok
}

// SetMetadata устанавливает метаданные в контекст
func (c *FrameworkContext) SetMetadata(key string, value interface{}) {
	if c.metadata == nil {
		c.metadata = make(map[string]interface{})
	}
	c.metadata[key] = value
}

// GetCorrelationID возвращает correlation ID
func (c *FrameworkContext) GetCorrelationID() string {
	val, ok := c.GetMetadata("correlation_id")
	if !ok {
		return ""
	}
	if id, ok := val.(string); ok {
		return id
	}
	return ""
}

// GetCausationID возвращает causation ID
func (c *FrameworkContext) GetCausationID() string {
	val, ok := c.GetMetadata("causation_id")
	if !ok {
		return ""
	}
	if id, ok := val.(string); ok {
		return id
	}
	return ""
}

// Error структура для ошибок фреймворка с кодами, сообщениями и stack trace
type Error struct {
	Code      string
	Message   string
	Cause     error
	StackTrace string
}

// Error реализует интерфейс error
func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap возвращает причину ошибки
func (e *Error) Unwrap() error {
	return e.Cause
}

// Result[T] generic тип для результатов операций (успех/ошибка)
type Result[T any] struct {
	Value T
	Error error
}

// Ok создает успешный результат
func Ok[T any](value T) Result[T] {
	return Result[T]{Value: value}
}

// Err создает результат с ошибкой
func Err[T any](err error) Result[T] {
	return Result[T]{Error: err}
}

// IsOk проверяет, успешен ли результат
func (r Result[T]) IsOk() bool {
	return r.Error == nil
}

// IsErr проверяет, есть ли ошибка в результате
func (r Result[T]) IsErr() bool {
	return r.Error != nil
}

// Option[T] generic тип для опциональных значений
type Option[T any] struct {
	value T
	some  bool
}

// Some создает Option с значением
func Some[T any](value T) Option[T] {
	return Option[T]{value: value, some: true}
}

// None создает пустой Option
func None[T any]() Option[T] {
	return Option[T]{some: false}
}

// IsSome проверяет, есть ли значение
func (o Option[T]) IsSome() bool {
	return o.some
}

// IsNone проверяет, пуст ли Option
func (o Option[T]) IsNone() bool {
	return !o.some
}

// Value возвращает значение (panic если None)
func (o Option[T]) Value() T {
	if !o.some {
		panic("option is none")
	}
	return o.value
}

// ValueOr возвращает значение или значение по умолчанию
func (o Option[T]) ValueOr(defaultValue T) T {
	if o.some {
		return o.value
	}
	return defaultValue
}

// ComponentType enum для типов компонентов
type ComponentType string

const (
	ComponentTypeModule   ComponentType = "module"
	ComponentTypeAdapter  ComponentType = "adapter"
	ComponentTypeTransport ComponentType = "transport"
	ComponentTypeHandler   ComponentType = "handler"
)

// Priority тип для приоритетов инициализации
type Priority int

const (
	PriorityLow    Priority = 100
	PriorityNormal Priority = 50
	PriorityHigh   Priority = 10
	PriorityCritical Priority = 1
)

