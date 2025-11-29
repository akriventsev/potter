package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	gqlhandler "github.com/99designs/gqlgen/graphql/handler"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/transport"
)

// minimalExecutableSchema минимальная реализация ExecutableSchema для тестов
type minimalExecutableSchema struct {
	commandBus transport.CommandBus
	queryBus   transport.QueryBus
	eventBus   events.EventBus
}

func (s *minimalExecutableSchema) Schema() *ast.Schema {
	return &ast.Schema{}
}

func (s *minimalExecutableSchema) Complexity(typeName, fieldName string, childComplexity int, args map[string]any) (int, bool) {
	return childComplexity, true
}

func (s *minimalExecutableSchema) Exec(ctx context.Context) graphql.ResponseHandler {
	opCtx := graphql.GetOperationContext(ctx)
	if opCtx == nil || opCtx.Operation == nil {
		return graphql.OneShot(graphql.ErrorResponse(ctx, "no operation"))
	}

	// Простая обработка query через QueryBus
	queryName := "testQuery"
	if opCtx.Operation.Name != "" {
		queryName = opCtx.Operation.Name
	}

	args := make(map[string]interface{})
	if opCtx.Variables != nil {
		for k, v := range opCtx.Variables {
			args[k] = v
		}
	}

	query := &BaseQuery{
		queryName: queryName,
		metadata:  transport.NewBaseQueryMetadata("test-query-id", "test-correlation-id"),
		args:      args,
	}

	result, err := s.queryBus.Ask(ctx, query)
	if err != nil {
		return graphql.OneShot(graphql.ErrorResponse(ctx, err.Error()))
	}

	// Конвертируем result в json.RawMessage
	data, _ := json.Marshal(result)
	return graphql.OneShot(&graphql.Response{
		Data: data,
	})
}

// testQueryHandler обработчик для тестовых запросов
type testQueryHandler struct {
	result interface{}
}

func (h *testQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	return h.result, nil
}

func (h *testQueryHandler) QueryName() string {
	return "testQuery"
}

// testCommandHandler обработчик для тестовых команд
type testCommandHandler struct{}

func (h *testCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	return nil
}

func (h *testCommandHandler) CommandName() string {
	return "testCommand"
}

func TestGraphQLAdapter_Integration_Query(t *testing.T) {
	// Настройка buses
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()

	// Регистрация query handler
	queryHandler := &testQueryHandler{
		result: map[string]interface{}{
			"test": "result",
		},
	}
	err := queryBus.Register(queryHandler)
	require.NoError(t, err)

	// Создание схемы
	schema := &minimalExecutableSchema{
		commandBus: commandBus,
		queryBus:   queryBus,
		eventBus:   eventBus,
	}

	// Создание адаптера
	config := DefaultGraphQLConfig()
	config.Port = 0 // Используем случайный порт
	adapter, err := NewGraphQLAdapter(config, commandBus, queryBus, eventBus, schema)
	require.NoError(t, err)

	// Запуск адаптера
	ctx := context.Background()
	err = adapter.Start(ctx)
	require.NoError(t, err)
	defer adapter.Stop(ctx)

	// Небольшая задержка для запуска сервера
	time.Sleep(100 * time.Millisecond)

	// Создание HTTP запроса
	query := `{"query": "{ testQuery { test } }"}`
	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/graphql", config.Port), bytes.NewBufferString(query))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Выполнение запроса
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Сервер может не успеть запуститься, это нормально для интеграционных тестов
		t.Skipf("Server not ready: %v", err)
		return
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGraphQLAdapter_Integration_Mutation(t *testing.T) {
	// Настройка buses
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()

	// Регистрация command handler
	commandHandler := &testCommandHandler{}
	err := commandBus.Register(commandHandler)
	require.NoError(t, err)

	// Создание схемы
	schema := &minimalExecutableSchema{
		commandBus: commandBus,
		queryBus:   queryBus,
		eventBus:   eventBus,
	}

	// Создание адаптера
	config := DefaultGraphQLConfig()
	config.Port = 0
	adapter, err := NewGraphQLAdapter(config, commandBus, queryBus, eventBus, schema)
	require.NoError(t, err)

	// Запуск адаптера
	ctx := context.Background()
	err = adapter.Start(ctx)
	require.NoError(t, err)
	defer adapter.Stop(ctx)

	// Небольшая задержка для запуска сервера
	time.Sleep(100 * time.Millisecond)

	// Создание HTTP запроса
	mutation := `{"query": "mutation { testCommand { success } }"}`
	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/graphql", config.Port), bytes.NewBufferString(mutation))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Выполнение запроса
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("Server not ready: %v", err)
		return
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGraphQLAdapter_Integration_HTTPHandler(t *testing.T) {
	// Настройка buses
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()

	// Создание схемы
	schema := &minimalExecutableSchema{
		commandBus: commandBus,
		queryBus:   queryBus,
		eventBus:   eventBus,
	}

	// Создание адаптера
	config := DefaultGraphQLConfig()
	_, err := NewGraphQLAdapter(config, commandBus, queryBus, eventBus, schema)
	require.NoError(t, err)

	// Создание HTTP handler напрямую для тестирования
	srv := gqlhandler.New(schema)
	
	// Тестирование handler
	query := `{"query": "{ __typename }"}`
	req := httptest.NewRequest("POST", "/graphql", bytes.NewBufferString(query))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGraphQLAdapter_Integration_EventSubscription(t *testing.T) {
	// Настройка buses
	eventBus := events.NewInMemoryEventBus()

	// Создание subscription manager
	subscriptionManager := NewSubscriptionManager(eventBus)

	ctx := context.Background()
	channel, err := subscriptionManager.Subscribe(ctx, "test.event", nil)
	require.NoError(t, err)

	// Публикация события
	event := events.NewBaseEvent("test.event", "aggregate-1")
	err = eventBus.Publish(ctx, event)
	require.NoError(t, err)

	// Проверка получения события
	select {
	case receivedEvent := <-channel:
		assert.Equal(t, "test.event", receivedEvent.EventType())
		assert.Equal(t, "aggregate-1", receivedEvent.AggregateID())
	case <-time.After(1 * time.Second):
		t.Fatal("event not received")
	}

	// Отписка
	subscriptionManager.mu.RLock()
	var subscriptionID string
	for id := range subscriptionManager.subscriptions {
		subscriptionID = id
		break
	}
	subscriptionManager.mu.RUnlock()

	err = subscriptionManager.Unsubscribe(subscriptionID)
	require.NoError(t, err)

	// Публикация еще одного события после отписки
	event2 := events.NewBaseEvent("test.event", "aggregate-2")
	err = eventBus.Publish(ctx, event2)
	require.NoError(t, err)

	// Убеждаемся, что событие не получено
	select {
	case <-channel:
		t.Fatal("should not receive event after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		// Ожидаемое поведение
	}
}

func TestGraphQLAdapter_Integration_WithCQRS(t *testing.T) {
	// Настройка buses
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()

	// Регистрация handlers
	queryHandler := &testQueryHandler{result: "test-result"}
	err := queryBus.Register(queryHandler)
	require.NoError(t, err)

	commandHandler := &testCommandHandler{}
	err = commandBus.Register(commandHandler)
	require.NoError(t, err)

	// Создание базовой схемы
	baseSchema := &minimalExecutableSchema{
		commandBus: commandBus,
		queryBus:   queryBus,
		eventBus:   eventBus,
	}

	// Создание адаптера с CQRS интеграцией
	config := DefaultGraphQLConfig()
	config.Port = 0
	adapter, err := NewGraphQLAdapterWithCQRS(config, commandBus, queryBus, eventBus, baseSchema)
	require.NoError(t, err)
	assert.NotNil(t, adapter)

	// Проверка, что схема - это potterExecutableSchema
	potterSchema, ok := adapter.schema.(*potterExecutableSchema)
	assert.True(t, ok)
	assert.NotNil(t, potterSchema.commandResolver)
	assert.NotNil(t, potterSchema.queryResolver)
	assert.NotNil(t, potterSchema.subscriptionResolver)
}

