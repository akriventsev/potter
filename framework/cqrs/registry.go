// Package cqrs предоставляет реестр для управления обработчиками команд и запросов.
package cqrs

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"potter/framework/transport"
)

// Registry реестр команд и запросов
type Registry struct {
	mu                sync.RWMutex
	commandHandlers   map[string]transport.CommandHandler
	queryHandlers     map[string]transport.QueryHandler
	commandTypes      map[string]reflect.Type
	queryTypes        map[string]reflect.Type
	handlerGroups     map[string][]string // группы обработчиков
	handlerStats      map[string]*HandlerStats
}

// HandlerStats статистика по обработчику
type HandlerStats struct {
	RegisteredAt int64
	Group        string
	Type         string
}

// NewRegistry создает новый реестр
func NewRegistry() *Registry {
	return &Registry{
		commandHandlers: make(map[string]transport.CommandHandler),
		queryHandlers:   make(map[string]transport.QueryHandler),
		commandTypes:    make(map[string]reflect.Type),
		queryTypes:      make(map[string]reflect.Type),
		handlerGroups:   make(map[string][]string),
		handlerStats:    make(map[string]*HandlerStats),
	}
}

// RegisterCommandHandler регистрирует обработчик команды
func (r *Registry) RegisterCommandHandler(handler transport.CommandHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	commandName := handler.CommandName()
	if _, exists := r.commandHandlers[commandName]; exists {
		return fmt.Errorf("command handler already registered: %s", commandName)
	}

	r.commandHandlers[commandName] = handler

	// Определяем тип команды через рефлексию
	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() == reflect.Ptr {
		handlerType = handlerType.Elem()
	}

	// Ищем метод Handle для определения типа команды
	handleMethod, found := handlerType.MethodByName("Handle")
	if found && handleMethod.Type.NumIn() >= 2 {
		// Второй параметр должен быть Command
		cmdType := handleMethod.Type.In(1)
		r.commandTypes[commandName] = cmdType
	}

	// Сохраняем статистику
	r.handlerStats[commandName] = &HandlerStats{
		RegisteredAt: time.Now().Unix(),
		Type:         "command",
	}

	return nil
}

// RegisterQueryHandler регистрирует обработчик запроса
func (r *Registry) RegisterQueryHandler(handler transport.QueryHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	queryName := handler.QueryName()
	if _, exists := r.queryHandlers[queryName]; exists {
		return fmt.Errorf("query handler already registered: %s", queryName)
	}

	r.queryHandlers[queryName] = handler

	// Определяем тип запроса через рефлексию
	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() == reflect.Ptr {
		handlerType = handlerType.Elem()
	}

	// Ищем метод Handle для определения типа запроса
	handleMethod, found := handlerType.MethodByName("Handle")
	if found && handleMethod.Type.NumIn() >= 2 {
		// Второй параметр должен быть Query
		queryType := handleMethod.Type.In(1)
		r.queryTypes[queryName] = queryType
	}

	// Сохраняем статистику
	r.handlerStats[queryName] = &HandlerStats{
		RegisteredAt: time.Now().Unix(),
		Type:         "query",
	}

	return nil
}

// UnregisterCommandHandler удаляет обработчик команды
func (r *Registry) UnregisterCommandHandler(commandName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.commandHandlers[commandName]; !exists {
		return fmt.Errorf("command handler not found: %s", commandName)
	}

	delete(r.commandHandlers, commandName)
	delete(r.commandTypes, commandName)
	delete(r.handlerStats, commandName)

	return nil
}

// UnregisterQueryHandler удаляет обработчик запроса
func (r *Registry) UnregisterQueryHandler(queryName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.queryHandlers[queryName]; !exists {
		return fmt.Errorf("query handler not found: %s", queryName)
	}

	delete(r.queryHandlers, queryName)
	delete(r.queryTypes, queryName)
	delete(r.handlerStats, queryName)

	return nil
}

// GetCommandHandler возвращает обработчик команды
func (r *Registry) GetCommandHandler(commandName string) (transport.CommandHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.commandHandlers[commandName]
	return handler, exists
}

// GetQueryHandler возвращает обработчик запроса
func (r *Registry) GetQueryHandler(queryName string) (transport.QueryHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.queryHandlers[queryName]
	return handler, exists
}

// GetAllCommandHandlers возвращает все зарегистрированные обработчики команд
func (r *Registry) GetAllCommandHandlers() map[string]transport.CommandHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]transport.CommandHandler)
	for k, v := range r.commandHandlers {
		result[k] = v
	}
	return result
}

// GetAllQueryHandlers возвращает все зарегистрированные обработчики запросов
func (r *Registry) GetAllQueryHandlers() map[string]transport.QueryHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]transport.QueryHandler)
	for k, v := range r.queryHandlers {
		result[k] = v
	}
	return result
}

// GetStats возвращает статистику по зарегистрированным обработчикам
func (r *Registry) GetStats() map[string]*HandlerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*HandlerStats)
	for k, v := range r.handlerStats {
		result[k] = v
	}
	return result
}

// AddHandlerGroup добавляет группу обработчиков
func (r *Registry) AddHandlerGroup(groupName string, handlerNames []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlerGroups[groupName] = handlerNames
}

// GetHandlerGroup возвращает обработчики группы
func (r *Registry) GetHandlerGroup(groupName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlerGroups[groupName]
}

// RegisterAllHandlers регистрирует все обработчики в шинах
func (r *Registry) RegisterAllHandlers(commandBus transport.CommandBus, queryBus transport.QueryBus) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Регистрация обработчиков команд
	for _, handler := range r.commandHandlers {
		if err := commandBus.Register(handler); err != nil {
			return fmt.Errorf("failed to register command handler %s: %w", handler.CommandName(), err)
		}
	}

	// Регистрация обработчиков запросов
	for _, handler := range r.queryHandlers {
		if err := queryBus.Register(handler); err != nil {
			return fmt.Errorf("failed to register query handler %s: %w", handler.QueryName(), err)
		}
	}

	return nil
}

