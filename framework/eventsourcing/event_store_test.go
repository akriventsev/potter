package eventsourcing

import (
	"context"
	"testing"
	"time"

	"github.com/akriventsev/potter/framework/events"
)

// MockEvent для тестирования
type MockEvent struct {
	eventID     string
	eventType   string
	aggregateID string
	occurredAt  time.Time
	metadata    events.EventMetadata
}

func (e *MockEvent) EventID() string {
	return e.eventID
}

func (e *MockEvent) EventType() string {
	return e.eventType
}

func (e *MockEvent) OccurredAt() time.Time {
	return e.occurredAt
}

func (e *MockEvent) AggregateID() string {
	return e.aggregateID
}

func (e *MockEvent) Metadata() events.EventMetadata {
	return e.metadata
}

func newMockEvent(eventType, aggregateID string) *MockEvent {
	return &MockEvent{
		eventID:     "event-1",
		eventType:   eventType,
		aggregateID: aggregateID,
		occurredAt:  time.Now(),
		metadata:    make(events.EventMetadata),
	}
}

func TestInMemoryEventStore_AppendEvents(t *testing.T) {
	store := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	ctx := context.Background()

	events := []events.Event{
		newMockEvent("test.event", "agg-1"),
		newMockEvent("test.event", "agg-1"),
	}

	err := store.AppendEvents(ctx, "agg-1", 0, events)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	stored, err := store.GetEvents(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(stored) != 2 {
		t.Errorf("Expected 2 events, got %d", len(stored))
	}
}

func TestInMemoryEventStore_AppendEvents_ConcurrencyConflict(t *testing.T) {
	store := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	ctx := context.Background()

	events1 := []events.Event{newMockEvent("test.event", "agg-1")}
	err := store.AppendEvents(ctx, "agg-1", 0, events1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	events2 := []events.Event{newMockEvent("test.event", "agg-1")}
	err = store.AppendEvents(ctx, "agg-1", 0, events2)
	if err == nil {
		t.Error("Expected concurrency conflict error")
	}
}

func TestInMemoryEventStore_GetEvents(t *testing.T) {
	store := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	ctx := context.Background()

	events := []events.Event{
		newMockEvent("test.event", "agg-1"),
		newMockEvent("test.event", "agg-1"),
		newMockEvent("test.event", "agg-1"),
	}

	err := store.AppendEvents(ctx, "agg-1", 0, events)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	stored, err := store.GetEvents(ctx, "agg-1", 2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(stored) != 2 {
		t.Errorf("Expected 2 events from version 2, got %d", len(stored))
	}
}

func TestInMemoryEventStore_GetEventsByType(t *testing.T) {
	store := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	ctx := context.Background()

	events := []events.Event{
		newMockEvent("type1", "agg-1"),
		newMockEvent("type2", "agg-1"),
		newMockEvent("type1", "agg-2"),
	}

	err := store.AppendEvents(ctx, "agg-1", 0, events[:2])
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = store.AppendEvents(ctx, "agg-2", 0, events[2:])
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	stored, err := store.GetEventsByType(ctx, "type1", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(stored) != 2 {
		t.Errorf("Expected 2 events of type1, got %d", len(stored))
	}
}

func TestInMemoryEventStore_GetAllEvents(t *testing.T) {
	store := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	ctx := context.Background()

	events1 := []events.Event{newMockEvent("test.event", "agg-1")}
	err := store.AppendEvents(ctx, "agg-1", 0, events1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	events2 := []events.Event{newMockEvent("test.event", "agg-2")}
	err = store.AppendEvents(ctx, "agg-2", 0, events2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	ch, err := store.GetAllEvents(ctx, 0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	count := 0
	for range ch {
		count++
	}

	if count != 2 {
		t.Errorf("Expected 2 events, got %d", count)
	}
}

func TestInMemoryEventStore_ConcurrentAppend(t *testing.T) {
	store := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	ctx := context.Background()

	// Тест конкурентного добавления в разные агрегаты
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			events := []events.Event{newMockEvent("test.event", "agg-1")}
			err := store.AppendEvents(ctx, "agg-1", int64(id), events)
			done <- (err == nil)
		}(i)
	}

	successCount := 0
	for i := 0; i < 10; i++ {
		if <-done {
			successCount++
		}
	}

	// Только одно должно быть успешным из-за проверки версии
	if successCount != 1 {
		t.Errorf("Expected 1 successful append, got %d", successCount)
	}
}

func TestSnapshotStore_SaveAndGet(t *testing.T) {
	store := NewInMemorySnapshotStore()
	ctx := context.Background()

	snapshot := Snapshot{
		AggregateID:  "agg-1",
		AggregateType: "test",
		Version:      10,
		State:        []byte("test state"),
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
	}

	err := store.SaveSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	loaded, err := store.GetSnapshot(ctx, "agg-1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if loaded == nil {
		t.Fatal("Expected snapshot, got nil")
	}

	if loaded.Version != 10 {
		t.Errorf("Expected version 10, got %d", loaded.Version)
	}
}

func TestSnapshotStore_DeleteOldSnapshots(t *testing.T) {
	store := NewInMemorySnapshotStore()
	ctx := context.Background()

	snapshot := Snapshot{
		AggregateID:  "agg-1",
		AggregateType: "test",
		Version:      5,
		State:        []byte("test state"),
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
	}

	err := store.SaveSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = store.DeleteSnapshots(ctx, "agg-1", 10)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	loaded, err := store.GetSnapshot(ctx, "agg-1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if loaded != nil {
		t.Error("Expected snapshot to be deleted, but it still exists")
	}
}

func BenchmarkEventStore_AppendEvents(b *testing.B) {
	store := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events := []events.Event{newMockEvent("test.event", "agg-1")}
		_ = store.AppendEvents(ctx, "agg-1", int64(i), events)
	}
}

func BenchmarkEventStore_GetEvents(b *testing.B) {
	store := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	ctx := context.Background()

	// Подготавливаем данные
	events := make([]events.Event, 100)
	for i := 0; i < 100; i++ {
		events[i] = newMockEvent("test.event", "agg-1")
	}
	_ = store.AppendEvents(ctx, "agg-1", 0, events)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetEvents(ctx, "agg-1", 0)
	}
}

