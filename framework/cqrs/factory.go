// Package cqrs предоставляет фабрику для создания обработчиков.
package cqrs

import (
	"context"
	"fmt"

	"github.com/akriventsev/potter/framework/transport"
)

// HandlerFactory фабрика для создания обработчиков
type HandlerFactory struct {
	registry *Registry
}

// NewHandlerFactory создает новую фабрику обработчиков
func NewHandlerFactory(registry *Registry) *HandlerFactory {
	return &HandlerFactory{
		registry: registry,
	}
}

// CreateCommandHandler создает обработчик команды с автоматической регистрацией
func (f *HandlerFactory) CreateCommandHandler(
	name string,
	handlerFunc func(ctx context.Context, cmd transport.Command) error,
) transport.CommandHandler {
	handler := NewFuncCommandHandler(name, handlerFunc)
	return handler
}

// CreateQueryHandler создает обработчик запроса с автоматической регистрацией
func (f *HandlerFactory) CreateQueryHandler(
	name string,
	handlerFunc func(ctx context.Context, q transport.Query) (interface{}, error),
) transport.QueryHandler {
	handler := NewFuncQueryHandler(name, handlerFunc)
	return handler
}

// RegisterCommandHandler регистрирует обработчик команды через фабрику
func (f *HandlerFactory) RegisterCommandHandler(
	bus transport.CommandBus,
	name string,
	handlerFunc func(ctx context.Context, cmd transport.Command) error,
) error {
	handler := f.CreateCommandHandler(name, handlerFunc)
	return RegisterCommandHandler(f.registry, bus, handler)
}

// RegisterQueryHandler регистрирует обработчик запроса через фабрику
func (f *HandlerFactory) RegisterQueryHandler(
	bus transport.QueryBus,
	name string,
	handlerFunc func(ctx context.Context, q transport.Query) (interface{}, error),
) error {
	handler := f.CreateQueryHandler(name, handlerFunc)
	return RegisterQueryHandler(f.registry, bus, handler)
}

// HandlerDependencies зависимости для обработчиков (generic версия)
type HandlerDependencies map[string]interface{}

// NewHandlerDependencies создает зависимости для обработчиков
func NewHandlerDependencies() HandlerDependencies {
	return make(HandlerDependencies)
}

// Set устанавливает зависимость
func (d HandlerDependencies) Set(key string, value interface{}) {
	d[key] = value
}

// Get получает зависимость
func (d HandlerDependencies) Get(key string) (interface{}, bool) {
	val, ok := d[key]
	return val, ok
}

// GetTyped получает типизированную зависимость
func GetTyped[T any](deps HandlerDependencies, key string) (T, bool) {
	var zero T
	val, ok := deps[key]
	if !ok {
		return zero, false
	}
	typed, ok := val.(T)
	return typed, ok
}

// CreateHandlerFromFunc создает обработчик из функции с автоматическим определением зависимостей
// NOTE: Автоматическое создание обработчиков через reflection планируется в будущих версиях.
// Это сложная задача, требующая анализа сигнатуры функции и инъекции зависимостей.
// Используйте CreateCommandHandler или CreateQueryHandler для создания обработчиков из функций.
func CreateHandlerFromFunc(fn interface{}) (transport.CommandHandler, error) {
	return nil, fmt.Errorf("not implemented: use CreateCommandHandler or CreateQueryHandler instead")
}

// HandlerDecorator декоратор для обработчиков
type HandlerDecorator interface {
	Decorate(handler transport.CommandHandler) transport.CommandHandler
}

// DecoratedCommandHandler обработчик с декоратором
type DecoratedCommandHandler struct {
	handler   transport.CommandHandler
	decorator HandlerDecorator
}

// NewDecoratedCommandHandler создает декорированный обработчик
func NewDecoratedCommandHandler(handler transport.CommandHandler, decorator HandlerDecorator) *DecoratedCommandHandler {
	return &DecoratedCommandHandler{
		handler:   handler,
		decorator: decorator,
	}
}

func (h *DecoratedCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	decorated := h.decorator.Decorate(h.handler)
	return decorated.Handle(ctx, cmd)
}

func (h *DecoratedCommandHandler) CommandName() string {
	return h.handler.CommandName()
}

