// Package transport предоставляет базовые классы и утилиты для REST, gRPC, WebSocket транспортов.
package transport

import (
	"fmt"
	"strings"

	"potter/framework/transport"
)

// CommandRouter автоматическая маршрутизация requests к commands
type CommandRouter struct {
	routes map[string]transport.Command
}

// NewCommandRouter создает новый CommandRouter
func NewCommandRouter() *CommandRouter {
	return &CommandRouter{
		routes: make(map[string]transport.Command),
	}
}

// RegisterCommand регистрирует command
func (r *CommandRouter) RegisterCommand(name string, command transport.Command) {
	r.routes[name] = command
}

// Route определяет command для request
func (r *CommandRouter) Route(request interface{}) (transport.Command, error) {
	// Простая реализация - можно расширить
	commandName := extractCommandName(request)
	command, exists := r.routes[commandName]
	if !exists {
		return nil, fmt.Errorf("command not found: %s", commandName)
	}
	return command, nil
}

// QueryRouter автоматическая маршрутизация requests к queries
type QueryRouter struct {
	routes map[string]transport.Query
}

// NewQueryRouter создает новый QueryRouter
func NewQueryRouter() *QueryRouter {
	return &QueryRouter{
		routes: make(map[string]transport.Query),
	}
}

// RegisterQuery регистрирует query
func (r *QueryRouter) RegisterQuery(name string, query transport.Query) {
	r.routes[name] = query
}

// Route определяет query для request
func (r *QueryRouter) Route(request interface{}) (transport.Query, error) {
	queryName := extractQueryName(request)
	query, exists := r.routes[queryName]
	if !exists {
		return nil, fmt.Errorf("query not found: %s", queryName)
	}
	return query, nil
}

// extractCommandName извлекает имя команды из request
func extractCommandName(request interface{}) string {
	// Простая реализация - можно расширить с рефлексией
	if cmd, ok := request.(transport.Command); ok {
		return cmd.CommandName()
	}
	return ""
}

// extractQueryName извлекает имя запроса из request
func extractQueryName(request interface{}) string {
	if query, ok := request.(transport.Query); ok {
		return query.QueryName()
	}
	return ""
}

// NamingConvention автоматический mapping по имени
func NamingConvention(commandName string) (method string, path string) {
	// CreateUserCommand -> POST /users
	// UpdateUserCommand -> PUT /users/:id
	// DeleteUserCommand -> DELETE /users/:id

	method = "POST"
	path = "/"

	if strings.HasPrefix(commandName, "Create") {
		method = "POST"
		resource := strings.TrimSuffix(strings.TrimPrefix(commandName, "Create"), "Command")
		path = fmt.Sprintf("/%s", strings.ToLower(resource))
	} else if strings.HasPrefix(commandName, "Update") {
		method = "PUT"
		resource := strings.TrimSuffix(strings.TrimPrefix(commandName, "Update"), "Command")
		path = fmt.Sprintf("/%s/:id", strings.ToLower(resource))
	} else if strings.HasPrefix(commandName, "Delete") {
		method = "DELETE"
		resource := strings.TrimSuffix(strings.TrimPrefix(commandName, "Delete"), "Command")
		path = fmt.Sprintf("/%s/:id", strings.ToLower(resource))
	}

	return method, path
}

