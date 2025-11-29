// Package invoke предоставляет тесты для QueryInvoker.
package invoke

import (
	"context"
	"testing"

	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/transport"
)

// TestQuery тестовый запрос
type TestQuery struct {
	ID string
}

func (q TestQuery) QueryName() string {
	return "test_query"
}

// TestResult тестовый результат
type TestResult struct {
	ID   string
	Name string
}

// MockQueryBus мок для QueryBus
type MockQueryBus struct {
	handlers map[string]transport.QueryHandler
}

func NewMockQueryBus() *MockQueryBus {
	return &MockQueryBus{
		handlers: make(map[string]transport.QueryHandler),
	}
}

func (m *MockQueryBus) Ask(ctx context.Context, q transport.Query) (interface{}, error) {
	handler, exists := m.handlers[q.QueryName()]
	if !exists {
		return nil, core.NewError("NOT_FOUND", "handler not found")
	}
	return handler.Handle(ctx, q)
}

func (m *MockQueryBus) Register(handler transport.QueryHandler) error {
	m.handlers[handler.QueryName()] = handler
	return nil
}

// TestQueryHandler тестовый обработчик запросов
type TestQueryHandler struct{}

func (h *TestQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	query := q.(TestQuery)
	return TestResult{
		ID:   query.ID,
		Name: "test result",
	}, nil
}

func (h *TestQueryHandler) QueryName() string {
	return "test_query"
}

func TestQueryInvoker_Invoke_Success(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockQueryBus()
	handler := &TestQueryHandler{}
	_ = mockBus.Register(handler)

	invoker := NewQueryInvoker[TestQuery, TestResult](mockBus)

	query := TestQuery{ID: "test-id"}
	result, err := invoker.Invoke(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", result.ID)
	}
	if result.Name != "test result" {
		t.Errorf("expected Name 'test result', got '%s'", result.Name)
	}
}

func TestQueryInvoker_Invoke_InvalidType(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockQueryBus()

	// Регистрируем handler, который возвращает неправильный тип
	wrongHandler := &WrongTypeQueryHandler{}
	_ = mockBus.Register(wrongHandler)

	invoker := NewQueryInvoker[TestQuery, TestResult](mockBus)

	query := TestQuery{ID: "test-id"}
	_, err := invoker.Invoke(ctx, query)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}

	if !core.WrapWithCode(err, ErrInvalidResultType).Is(err) {
		t.Errorf("expected INVALID_RESULT_TYPE error, got: %v", err)
	}
}

// WrongTypeQueryHandler handler, возвращающий неправильный тип
type WrongTypeQueryHandler struct{}

func (h *WrongTypeQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	return "wrong type", nil
}

func (h *WrongTypeQueryHandler) QueryName() string {
	return "test_query"
}

func TestQueryInvoker_Invoke_WithValidator(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockQueryBus()
	handler := &TestQueryHandler{}
	_ = mockBus.Register(handler)

	invoker := NewQueryInvoker[TestQuery, TestResult](mockBus).
		WithValidator(func(result TestResult) error {
			if result.ID == "" {
				return core.NewError("VALIDATION_FAILED", "ID is required")
			}
			return nil
		})

	query := TestQuery{ID: "test-id"}
	result, err := invoker.Invoke(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", result.ID)
	}
}

func TestQueryInvoker_InvokeBatch(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockQueryBus()
	handler := &TestQueryHandler{}
	_ = mockBus.Register(handler)

	invoker := NewQueryInvoker[TestQuery, TestResult](mockBus)

	queries := []TestQuery{
		{ID: "id1"},
		{ID: "id2"},
		{ID: "id3"},
	}

	results, err := invoker.InvokeBatch(ctx, queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

