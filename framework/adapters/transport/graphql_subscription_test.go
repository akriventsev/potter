package transport

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/akriventsev/potter/framework/events"
)

func TestSubscriptionManager_Subscribe(t *testing.T) {
	eventBus := events.NewInMemoryEventBus()
	manager := NewSubscriptionManager(eventBus)

	ctx := context.Background()
	channel, err := manager.Subscribe(ctx, "test.event", nil)
	require.NoError(t, err)
	assert.NotNil(t, channel)

	// Проверка регистрации
	manager.mu.RLock()
	assert.Equal(t, 1, len(manager.subscriptions))
	manager.mu.RUnlock()
}

func TestSubscriptionManager_Unsubscribe(t *testing.T) {
	eventBus := events.NewInMemoryEventBus()
	manager := NewSubscriptionManager(eventBus)

	ctx := context.Background()
	channel, err := manager.Subscribe(ctx, "test.event", nil)
	require.NoError(t, err)

	// Получаем subscription ID
	manager.mu.RLock()
	var subscriptionID string
	for id := range manager.subscriptions {
		subscriptionID = id
		break
	}
	manager.mu.RUnlock()

	err = manager.Unsubscribe(subscriptionID)
	require.NoError(t, err)

	// Проверка удаления
	manager.mu.RLock()
	assert.Equal(t, 0, len(manager.subscriptions))
	manager.mu.RUnlock()

	// Проверка закрытия канала
	_, ok := <-channel
	assert.False(t, ok, "channel should be closed")

	// Проверка, что после отписки события не доставляются
	event := events.NewBaseEvent("test.event", "aggregate-1")
	err = eventBus.Publish(ctx, event)
	require.NoError(t, err)

	// Убеждаемся, что событие не получено (канал закрыт)
	select {
	case <-channel:
		t.Fatal("should not receive event after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		// Ожидаемое поведение - событие не получено
	}
}

func TestSubscriptionManager_Broadcast(t *testing.T) {
	eventBus := events.NewInMemoryEventBus()
	manager := NewSubscriptionManager(eventBus)

	ctx := context.Background()
	channel, err := manager.Subscribe(ctx, "test.event", nil)
	require.NoError(t, err)

	// Публикуем событие
	event := events.NewBaseEvent("test.event", "aggregate-1")
	err = eventBus.Publish(ctx, event)
	require.NoError(t, err)

	// Проверяем получение события
	select {
	case receivedEvent := <-channel:
		assert.Equal(t, "test.event", receivedEvent.EventType())
		assert.Equal(t, "aggregate-1", receivedEvent.AggregateID())
	case <-time.After(1 * time.Second):
		t.Fatal("event not received")
	}
}

func TestSubscriptionManager_Filters(t *testing.T) {
	eventBus := events.NewInMemoryEventBus()
	manager := NewSubscriptionManager(eventBus)

	ctx := context.Background()
	correlationID := "corr-123"
	filter := &CorrelationIDFilter{CorrelationID: correlationID}
	
	channel, err := manager.Subscribe(ctx, "test.event", filter)
	require.NoError(t, err)

	// Событие с правильным correlation ID
	event1 := events.NewBaseEvent("test.event", "aggregate-1")
	event1.WithCorrelationID(correlationID)
	err = eventBus.Publish(ctx, event1)
	require.NoError(t, err)

	// Событие с неправильным correlation ID
	event2 := events.NewBaseEvent("test.event", "aggregate-2")
	event2.WithCorrelationID("corr-456")
	err = eventBus.Publish(ctx, event2)
	require.NoError(t, err)

	// Должно получить только первое событие
	select {
	case receivedEvent := <-channel:
		assert.Equal(t, correlationID, receivedEvent.Metadata().CorrelationID())
	case <-time.After(1 * time.Second):
		t.Fatal("event not received")
	}

	// Второе событие не должно пройти фильтр
	select {
	case <-channel:
		t.Fatal("should not receive second event")
	case <-time.After(100 * time.Millisecond):
		// Ожидаемое поведение
	}
}

func TestSubscriptionManager_ContextCancellation(t *testing.T) {
	eventBus := events.NewInMemoryEventBus()
	manager := NewSubscriptionManager(eventBus)

	ctx, cancel := context.WithCancel(context.Background())
	channel, err := manager.Subscribe(ctx, "test.event", nil)
	require.NoError(t, err)

	// Отменяем контекст
	cancel()

	// Ждем отписки
	time.Sleep(100 * time.Millisecond)

	// Проверка удаления
	manager.mu.RLock()
	assert.Equal(t, 0, len(manager.subscriptions))
	manager.mu.RUnlock()

	// Проверка закрытия канала
	_, ok := <-channel
	assert.False(t, ok, "channel should be closed")
}

func TestSubscriptionManager_Close(t *testing.T) {
	eventBus := events.NewInMemoryEventBus()
	manager := NewSubscriptionManager(eventBus)

	ctx := context.Background()
	_, err := manager.Subscribe(ctx, "test.event", nil)
	require.NoError(t, err)
	_, err = manager.Subscribe(ctx, "test.event2", nil)
	require.NoError(t, err)

	// Проверка регистрации
	manager.mu.RLock()
	assert.Equal(t, 2, len(manager.subscriptions))
	manager.mu.RUnlock()

	// Закрытие всех подписок
	err = manager.Close()
	require.NoError(t, err)

	// Проверка очистки
	manager.mu.RLock()
	assert.Equal(t, 0, len(manager.subscriptions))
	manager.mu.RUnlock()
}

func TestCorrelationIDFilter(t *testing.T) {
	filter := &CorrelationIDFilter{CorrelationID: "corr-123"}

	event1 := events.NewBaseEvent("test.event", "aggregate-1")
	event1.WithCorrelationID("corr-123")
	assert.True(t, filter.Match(event1))

	event2 := events.NewBaseEvent("test.event", "aggregate-2")
	event2.WithCorrelationID("corr-456")
	assert.False(t, filter.Match(event2))
}

func TestAggregateIDFilter(t *testing.T) {
	filter := &AggregateIDFilter{AggregateID: "aggregate-1"}

	event1 := events.NewBaseEvent("test.event", "aggregate-1")
	assert.True(t, filter.Match(event1))

	event2 := events.NewBaseEvent("test.event", "aggregate-2")
	assert.False(t, filter.Match(event2))
}

func TestCompositeFilter(t *testing.T) {
	corrFilter := &CorrelationIDFilter{CorrelationID: "corr-123"}
	aggFilter := &AggregateIDFilter{AggregateID: "aggregate-1"}

	// AND filter
	andFilter := &CompositeFilter{
		Filters: []EventFilter{corrFilter, aggFilter},
		Op:      "AND",
	}

	event1 := events.NewBaseEvent("test.event", "aggregate-1")
	event1.WithCorrelationID("corr-123")
	assert.True(t, andFilter.Match(event1))

	event2 := events.NewBaseEvent("test.event", "aggregate-2")
	event2.WithCorrelationID("corr-123")
	assert.False(t, andFilter.Match(event2))

	// OR filter
	orFilter := &CompositeFilter{
		Filters: []EventFilter{corrFilter, aggFilter},
		Op:      "OR",
	}

	event3 := events.NewBaseEvent("test.event", "aggregate-2")
	event3.WithCorrelationID("corr-123")
	assert.True(t, orFilter.Match(event3))
}

