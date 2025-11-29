package cqrs

import (
	"context"
	"testing"

	"github.com/akriventsev/potter/framework/transport"
)

// MockCommand для тестирования
type MockCommand struct {
	name string
}

func (c MockCommand) CommandName() string {
	return c.name
}

// MockCommandHandler для тестирования
type MockCommandHandler struct {
	name string
}

func (h *MockCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	return nil
}

func (h *MockCommandHandler) CommandName() string {
	return h.name
}

// MockQuery для тестирования
type MockQuery struct {
	name string
}

func (q MockQuery) QueryName() string {
	return q.name
}

// MockQueryHandler для тестирования
type MockQueryHandler struct {
	name string
}

func (h *MockQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	return "result", nil
}

func (h *MockQueryHandler) QueryName() string {
	return h.name
}

func TestRegistry_RegisterCommandHandler(t *testing.T) {
	registry := NewRegistry()
	handler := &MockCommandHandler{name: "test_command"}

	err := registry.RegisterCommandHandler(handler)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestRegistry_RegisterCommandHandler_Duplicate(t *testing.T) {
	registry := NewRegistry()
	handler := &MockCommandHandler{name: "test_command"}

	if err := registry.RegisterCommandHandler(handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}
	err := registry.RegisterCommandHandler(handler)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

func TestRegistry_RegisterQueryHandler(t *testing.T) {
	registry := NewRegistry()
	handler := &MockQueryHandler{name: "test_query"}

	err := registry.RegisterQueryHandler(handler)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestRegistry_RegisterQueryHandler_Duplicate(t *testing.T) {
	registry := NewRegistry()
	handler := &MockQueryHandler{name: "test_query"}

	if err := registry.RegisterQueryHandler(handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}
	err := registry.RegisterQueryHandler(handler)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

func TestRegistry_GetCommandHandler(t *testing.T) {
	registry := NewRegistry()
	handler := &MockCommandHandler{name: "test_command"}

	if err := registry.RegisterCommandHandler(handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	retrieved, exists := registry.GetCommandHandler("test_command")
	if !exists {
		t.Error("Expected handler to exist")
	}

	if retrieved.CommandName() != "test_command" {
		t.Errorf("Expected 'test_command', got %s", retrieved.CommandName())
	}
}

func TestRegistry_GetCommandHandler_NotFound(t *testing.T) {
	registry := NewRegistry()

	_, exists := registry.GetCommandHandler("nonexistent")
	if exists {
		t.Error("Expected handler to not exist")
	}
}

func TestRegistry_GetQueryHandler(t *testing.T) {
	registry := NewRegistry()
	handler := &MockQueryHandler{name: "test_query"}

	if err := registry.RegisterQueryHandler(handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	retrieved, exists := registry.GetQueryHandler("test_query")
	if !exists {
		t.Error("Expected handler to exist")
	}

	if retrieved.QueryName() != "test_query" {
		t.Errorf("Expected 'test_query', got %s", retrieved.QueryName())
	}
}

func TestRegistry_GetQueryHandler_NotFound(t *testing.T) {
	registry := NewRegistry()

	_, exists := registry.GetQueryHandler("nonexistent")
	if exists {
		t.Error("Expected handler to not exist")
	}
}

func TestRegistry_GetAllCommandHandlers(t *testing.T) {
	registry := NewRegistry()
	handler1 := &MockCommandHandler{name: "command1"}
	handler2 := &MockCommandHandler{name: "command2"}

	if err := registry.RegisterCommandHandler(handler1); err != nil {
		t.Fatalf("Failed to register handler1: %v", err)
	}
	if err := registry.RegisterCommandHandler(handler2); err != nil {
		t.Fatalf("Failed to register handler2: %v", err)
	}

	all := registry.GetAllCommandHandlers()
	if len(all) != 2 {
		t.Errorf("Expected 2 handlers, got %d", len(all))
	}
}

func TestRegistry_GetAllQueryHandlers(t *testing.T) {
	registry := NewRegistry()
	handler1 := &MockQueryHandler{name: "query1"}
	handler2 := &MockQueryHandler{name: "query2"}

	if err := registry.RegisterQueryHandler(handler1); err != nil {
		t.Fatalf("Failed to register handler1: %v", err)
	}
	if err := registry.RegisterQueryHandler(handler2); err != nil {
		t.Fatalf("Failed to register handler2: %v", err)
	}

	all := registry.GetAllQueryHandlers()
	if len(all) != 2 {
		t.Errorf("Expected 2 handlers, got %d", len(all))
	}
}

func TestRegistry_GetStats(t *testing.T) {
	registry := NewRegistry()
	handler := &MockCommandHandler{name: "test_command"}

	if err := registry.RegisterCommandHandler(handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	stats := registry.GetStats()
	if len(stats) != 1 {
		t.Errorf("Expected 1 stat, got %d", len(stats))
	}

	stat, exists := stats["test_command"]
	if !exists {
		t.Error("Expected stat to exist")
	}

	if stat.Type != "command" {
		t.Errorf("Expected type 'command', got %s", stat.Type)
	}

	if stat.RegisteredAt == 0 {
		t.Error("Expected RegisteredAt to be set")
	}
}

func TestRegistry_UnregisterCommandHandler(t *testing.T) {
	registry := NewRegistry()
	handler := &MockCommandHandler{name: "test_command"}

	if err := registry.RegisterCommandHandler(handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}
	err := registry.UnregisterCommandHandler("test_command")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	_, exists := registry.GetCommandHandler("test_command")
	if exists {
		t.Error("Expected handler to be unregistered")
	}
}

func TestRegistry_UnregisterCommandHandler_NotFound(t *testing.T) {
	registry := NewRegistry()

	err := registry.UnregisterCommandHandler("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent handler")
	}
}

func TestRegistry_UnregisterQueryHandler(t *testing.T) {
	registry := NewRegistry()
	handler := &MockQueryHandler{name: "test_query"}

	if err := registry.RegisterQueryHandler(handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}
	err := registry.UnregisterQueryHandler("test_query")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	_, exists := registry.GetQueryHandler("test_query")
	if exists {
		t.Error("Expected handler to be unregistered")
	}
}

func TestRegistry_UnregisterQueryHandler_NotFound(t *testing.T) {
	registry := NewRegistry()

	err := registry.UnregisterQueryHandler("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent handler")
	}
}

func TestRegistry_GetHandlerStats(t *testing.T) {
	registry := NewRegistry()
	cmdHandler := &MockCommandHandler{name: "test_command"}
	queryHandler := &MockQueryHandler{name: "test_query"}

	if err := registry.RegisterCommandHandler(cmdHandler); err != nil {
		t.Fatalf("Failed to register command handler: %v", err)
	}
	if err := registry.RegisterQueryHandler(queryHandler); err != nil {
		t.Fatalf("Failed to register query handler: %v", err)
	}

	stats := registry.GetStats()
	if len(stats) != 2 {
		t.Errorf("Expected 2 stats, got %d", len(stats))
	}

	cmdStat, exists := stats["test_command"]
	if !exists {
		t.Error("Expected command stat to exist")
	}
	if cmdStat.Type != "command" {
		t.Errorf("Expected type 'command', got %s", cmdStat.Type)
	}

	queryStat, exists := stats["test_query"]
	if !exists {
		t.Error("Expected query stat to exist")
	}
	if queryStat.Type != "query" {
		t.Errorf("Expected type 'query', got %s", queryStat.Type)
	}
}

