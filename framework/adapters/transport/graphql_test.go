package transport

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/transport"
)

// mockExecutableSchema мок для graphql.ExecutableSchema
type mockExecutableSchema struct{}

func (m *mockExecutableSchema) Schema() *ast.Schema {
	return &ast.Schema{}
}

func (m *mockExecutableSchema) Complexity(typeName, fieldName string, childComplexity int, args map[string]any) (int, bool) {
	return childComplexity, true
}

func (m *mockExecutableSchema) Exec(ctx context.Context) graphql.ResponseHandler {
	return graphql.OneShot(graphql.ErrorResponse(ctx, "not implemented"))
}

func TestGraphQLAdapter_Lifecycle(t *testing.T) {
	config := DefaultGraphQLConfig()
	config.Port = 0 // Используем случайный порт для тестов
	
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()
	schema := &mockExecutableSchema{}

	adapter, err := NewGraphQLAdapter(config, commandBus, queryBus, eventBus, schema)
	require.NoError(t, err)
	assert.NotNil(t, adapter)

	// Проверка начального состояния
	assert.False(t, adapter.IsRunning())
	assert.Equal(t, "graphql-adapter", adapter.Name())
	assert.Equal(t, core.ComponentTypeTransport, adapter.Type())

	// Запуск
	ctx := context.Background()
	err = adapter.Start(ctx)
	require.NoError(t, err)
	assert.True(t, adapter.IsRunning())

	// Остановка
	err = adapter.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, adapter.IsRunning())
}

func TestGraphQLAdapter_WithComplexityLimit(t *testing.T) {
	config := DefaultGraphQLConfig()
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()
	schema := &mockExecutableSchema{}

	adapter, err := NewGraphQLAdapter(config, commandBus, queryBus, eventBus, schema)
	require.NoError(t, err)

	adapter.WithComplexityLimit(500)
	assert.Equal(t, 500, adapter.config.ComplexityLimit)
}

func TestGraphQLAdapter_RegisterResolver(t *testing.T) {
	config := DefaultGraphQLConfig()
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()
	schema := &mockExecutableSchema{}

	adapter, err := NewGraphQLAdapter(config, commandBus, queryBus, eventBus, schema)
	require.NoError(t, err)

	resolver := func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return "test", nil
	}

	adapter.RegisterResolver("Query", "test", resolver)
	
	// Проверка регистрации
	adapter.mu.RLock()
	key := "Query.test"
	_, exists := adapter.resolvers[key]
	adapter.mu.RUnlock()
	assert.True(t, exists)
}

func TestDefaultGraphQLConfig(t *testing.T) {
	config := DefaultGraphQLConfig()
	
	assert.Equal(t, 8082, config.Port)
	assert.Equal(t, "/graphql", config.Path)
	assert.Equal(t, "/playground", config.PlaygroundPath)
	assert.True(t, config.EnablePlayground)
	assert.True(t, config.EnableIntrospection)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, 1000, config.ComplexityLimit)
	assert.Equal(t, 15, config.MaxDepth)
}

