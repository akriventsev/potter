package transport

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/transport"
)

func TestCommandResolver_Resolve(t *testing.T) {
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()
	
	baseResolver := NewBaseResolver(commandBus, queryBus, eventBus)
	resolver := NewCommandResolver(baseResolver)

	// Регистрируем mock handler
	mockHandler := &mockCommandHandler{
		commandName: "TestCommand",
	}
	err := commandBus.Register(mockHandler)
	require.NoError(t, err)

	ctx := context.Background()
	args := map[string]interface{}{
		"id": "test-id",
	}

	result, err := resolver.Resolve(ctx, "TestCommand", args)
	require.NoError(t, err)
	assert.NotNil(t, result)
	
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, resultMap["success"].(bool))
	assert.NotEmpty(t, resultMap["command_id"])
}

func TestQueryResolver_Resolve(t *testing.T) {
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()
	
	baseResolver := NewBaseResolver(commandBus, queryBus, eventBus)
	resolver := NewQueryResolver(baseResolver)

	// Регистрируем mock handler
	mockHandler := &mockQueryHandler{
		queryName: "TestQuery",
		result:    "test-result",
	}
	err := queryBus.Register(mockHandler)
	require.NoError(t, err)

	ctx := context.Background()
	args := map[string]interface{}{
		"id": "test-id",
	}

	result, err := resolver.Resolve(ctx, "TestQuery", args)
	require.NoError(t, err)
	assert.Equal(t, "test-result", result)
}

func TestSubscriptionResolver_Subscribe(t *testing.T) {
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()
	
	subscriptionManager := NewSubscriptionManager(eventBus)
	baseResolver := NewBaseResolver(commandBus, queryBus, eventBus)
	resolver := NewSubscriptionResolver(baseResolver, subscriptionManager)

	ctx := context.Background()
	channel, err := resolver.Subscribe(ctx, "test.event")
	require.NoError(t, err)
	assert.NotNil(t, channel)
}

func TestResolverRegistry(t *testing.T) {
	registry := NewResolverRegistry()

	resolver := func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return "test", nil
	}

	registry.Register("Query", "test", resolver)
	
	resolver2, exists := registry.Get("Query", "test")
	assert.True(t, exists)
	assert.NotNil(t, resolver2)
	
	_, exists = registry.Get("Query", "nonexistent")
	assert.False(t, exists)
}

// mockCommandHandler мок для CommandHandler
type mockCommandHandler struct {
	commandName string
}

func (m *mockCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	return nil
}

func (m *mockCommandHandler) CommandName() string {
	return m.commandName
}

// mockQueryHandler мок для QueryHandler
type mockQueryHandler struct {
	queryName string
	result    interface{}
}

func (m *mockQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	return m.result, nil
}

func (m *mockQueryHandler) QueryName() string {
	return m.queryName
}

