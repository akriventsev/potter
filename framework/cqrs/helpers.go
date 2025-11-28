// Package cqrs предоставляет вспомогательные функции для работы с CQRS.
package cqrs

import (
	"context"
	"fmt"

	"potter/framework/transport"
)

// RegisterCommandHandler регистрирует обработчик команды в реестре и шине
func RegisterCommandHandler(registry *Registry, bus transport.CommandBus, handler transport.CommandHandler) error {
	// Регистрируем в реестре
	if err := registry.RegisterCommandHandler(handler); err != nil {
		return fmt.Errorf("failed to register in registry: %w", err)
	}

	// Регистрируем в шине
	if err := bus.Register(handler); err != nil {
		return fmt.Errorf("failed to register in bus: %w", err)
	}

	return nil
}

// RegisterQueryHandler регистрирует обработчик запроса в реестре и шине
func RegisterQueryHandler(registry *Registry, bus transport.QueryBus, handler transport.QueryHandler) error {
	// Регистрируем в реестре
	if err := registry.RegisterQueryHandler(handler); err != nil {
		return fmt.Errorf("failed to register in registry: %w", err)
	}

	// Регистрируем в шине
	if err := bus.Register(handler); err != nil {
		return fmt.Errorf("failed to register in bus: %w", err)
	}

	return nil
}

// BatchRegisterCommandHandlers регистрирует несколько обработчиков команд
func BatchRegisterCommandHandlers(registry *Registry, bus transport.CommandBus, handlers ...transport.CommandHandler) error {
	for _, handler := range handlers {
		if err := RegisterCommandHandler(registry, bus, handler); err != nil {
			return fmt.Errorf("failed to register handler %s: %w", handler.CommandName(), err)
		}
	}
	return nil
}

// BatchRegisterQueryHandlers регистрирует несколько обработчиков запросов
func BatchRegisterQueryHandlers(registry *Registry, bus transport.QueryBus, handlers ...transport.QueryHandler) error {
	for _, handler := range handlers {
		if err := RegisterQueryHandler(registry, bus, handler); err != nil {
			return fmt.Errorf("failed to register handler %s: %w", handler.QueryName(), err)
		}
	}
	return nil
}

// CommandHandlerFunc функция-обработчик команды
type CommandHandlerFunc func(ctx context.Context, cmd transport.Command) error

// FuncCommandHandler адаптер для функции в CommandHandler
type FuncCommandHandler struct {
	name    string
	handler CommandHandlerFunc
}

// NewFuncCommandHandler создает обработчик команды из функции
func NewFuncCommandHandler(name string, handler CommandHandlerFunc) *FuncCommandHandler {
	return &FuncCommandHandler{
		name:    name,
		handler: handler,
	}
}

func (h *FuncCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	return h.handler(ctx, cmd)
}

func (h *FuncCommandHandler) CommandName() string {
	return h.name
}

// QueryHandlerFunc функция-обработчик запроса
type QueryHandlerFunc func(ctx context.Context, q transport.Query) (interface{}, error)

// FuncQueryHandler адаптер для функции в QueryHandler
type FuncQueryHandler struct {
	name    string
	handler QueryHandlerFunc
}

// NewFuncQueryHandler создает обработчик запроса из функции
func NewFuncQueryHandler(name string, handler QueryHandlerFunc) *FuncQueryHandler {
	return &FuncQueryHandler{
		name:    name,
		handler: handler,
	}
}

func (h *FuncQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	return h.handler(ctx, q)
}

func (h *FuncQueryHandler) QueryName() string {
	return h.name
}

// TypedCommandHandler[T] generic обработчик команды с типизацией
type TypedCommandHandler[T transport.Command] struct {
	name    string
	handler func(ctx context.Context, cmd T) error
}

// NewTypedCommandHandler создает типизированный обработчик команды
func NewTypedCommandHandler[T transport.Command](name string, handler func(ctx context.Context, cmd T) error) *TypedCommandHandler[T] {
	return &TypedCommandHandler[T]{
		name:    name,
		handler: handler,
	}
}

func (h *TypedCommandHandler[T]) Handle(ctx context.Context, cmd transport.Command) error {
	typedCmd, ok := cmd.(T)
	if !ok {
		return fmt.Errorf("invalid command type for handler %s", h.name)
	}
	return h.handler(ctx, typedCmd)
}

func (h *TypedCommandHandler[T]) CommandName() string {
	return h.name
}

// TypedQueryHandler[T, R] generic обработчик запроса с типизацией
type TypedQueryHandler[T transport.Query, R any] struct {
	name    string
	handler func(ctx context.Context, q T) (R, error)
}

// NewTypedQueryHandler создает типизированный обработчик запроса
func NewTypedQueryHandler[T transport.Query, R any](name string, handler func(ctx context.Context, q T) (R, error)) *TypedQueryHandler[T, R] {
	return &TypedQueryHandler[T, R]{
		name:    name,
		handler: handler,
	}
}

func (h *TypedQueryHandler[T, R]) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	typedQuery, ok := q.(T)
	if !ok {
		return nil, fmt.Errorf("invalid query type for handler %s", h.name)
	}
	return h.handler(ctx, typedQuery)
}

func (h *TypedQueryHandler[T, R]) QueryName() string {
	return h.name
}

// AsyncCommandHandler асинхронный обработчик команды
type AsyncCommandHandler struct {
	*FuncCommandHandler
	done chan error
}

// NewAsyncCommandHandler создает асинхронный обработчик команды
func NewAsyncCommandHandler(name string, handler CommandHandlerFunc) *AsyncCommandHandler {
	return &AsyncCommandHandler{
		FuncCommandHandler: NewFuncCommandHandler(name, handler),
		done:               make(chan error, 1),
	}
}

func (h *AsyncCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	go func() {
		h.done <- h.handler(ctx, cmd)
	}()
	return nil
}

// Wait ждет завершения обработки
func (h *AsyncCommandHandler) Wait() error {
	return <-h.done
}

// StreamingQueryHandler обработчик запроса с потоковой передачей результатов
type StreamingQueryHandler struct {
	name    string
	handler func(ctx context.Context, q transport.Query) (<-chan interface{}, error)
}

// NewStreamingQueryHandler создает потоковый обработчик запроса
func NewStreamingQueryHandler(name string, handler func(ctx context.Context, q transport.Query) (<-chan interface{}, error)) *StreamingQueryHandler {
	return &StreamingQueryHandler{
		name:    name,
		handler: handler,
	}
}

func (h *StreamingQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	ch, err := h.handler(ctx, q)
	if err != nil {
		return nil, err
	}

	// Собираем все результаты в слайс
	results := make([]interface{}, 0)
	for result := range ch {
		results = append(results, result)
	}
	return results, nil
}

func (h *StreamingQueryHandler) QueryName() string {
	return h.name
}

