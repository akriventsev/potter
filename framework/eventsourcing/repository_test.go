package eventsourcing

import (
	"context"
	"testing"

	"potter/framework/events"
)

func createTestRepository() (*EventSourcedRepository[*TestAggregate], *InMemoryEventStore, *InMemorySnapshotStore) {
	eventStore := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	snapshotStore := NewInMemorySnapshotStore()
	config := DefaultRepositoryConfig()
	config.UseSnapshots = true
	factory := func(id string) *TestAggregate {
		return NewTestAggregate(id)
	}
	repo := NewEventSourcedRepository[*TestAggregate](eventStore, snapshotStore, config, factory)
	return repo, eventStore, snapshotStore
}

func createTestAggregate(id string) *TestAggregate {
	agg := NewTestAggregate(id)
	event := &TestCreatedEvent{
		BaseEvent: events.NewBaseEvent("test.created", id),
		Name:      "Test",
		Value:     10,
	}
	agg.RaiseEvent(event)
	return agg
}

func generateEvents(count int, aggregateID string) []events.Event {
	evts := make([]events.Event, count)
	for i := 0; i < count; i++ {
		evts[i] = &TestUpdatedEvent{
			BaseEvent: events.NewBaseEvent("test.updated", aggregateID),
			Value:     10 + i,
		}
	}
	return evts
}

func TestEventSourcedRepository_Save(t *testing.T) {
	repo, _, _ := createTestRepository()
	ctx := context.Background()

	agg := createTestAggregate("test-1")
	err := repo.Save(ctx, agg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	uncommitted := agg.GetUncommittedEvents()
	if len(uncommitted) != 0 {
		t.Error("Expected no uncommitted events after save")
	}
}

func TestEventSourcedRepository_GetByID(t *testing.T) {
	repo, _, _ := createTestRepository()
	ctx := context.Background()

	agg := createTestAggregate("test-1")
	err := repo.Save(ctx, agg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	loaded, err := repo.GetByID(ctx, "test-1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if loaded == nil {
		t.Fatal("Expected loaded aggregate, got nil")
	}

	if loaded.ID() != "test-1" {
		t.Errorf("Expected ID 'test-1', got '%s'", loaded.ID())
	}

	// Проверяем, что состояние восстановлено
	if loaded.name != "Test" {
		t.Errorf("Expected name 'Test', got '%s'", loaded.name)
	}

	if loaded.value != 10 {
		t.Errorf("Expected value 10, got %d", loaded.value)
	}

	// Проверяем версию
	if loaded.Version() != 1 {
		t.Errorf("Expected version 1, got %d", loaded.Version())
	}
}

func TestEventSourcedRepository_SaveWithSnapshot(t *testing.T) {
	repo, _, snapshotStore := createTestRepository()
	ctx := context.Background()

	agg := createTestAggregate("test-1")
	// Генерируем много событий для создания снапшота
	for i := 0; i < 150; i++ {
		event := &TestUpdatedEvent{
			BaseEvent: events.NewBaseEvent("test.updated", "test-1"),
			Value:     10 + i,
		}
		agg.RaiseEvent(event)
	}

	err := repo.Save(ctx, agg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Проверяем, что снапшот создан
	snapshot, err := snapshotStore.GetSnapshot(ctx, "test-1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if snapshot == nil {
		t.Error("Expected snapshot to be created")
	}
}

func TestEventSourcedRepository_GetByIDWithSnapshot(t *testing.T) {
	repo, _, _ := createTestRepository()
	ctx := context.Background()

	agg := createTestAggregate("test-1")
	// Генерируем события для создания снапшота
	for i := 0; i < 150; i++ {
		event := &TestUpdatedEvent{
			BaseEvent: events.NewBaseEvent("test.updated", "test-1"),
			Value:     10 + i,
		}
		agg.RaiseEvent(event)
	}

	err := repo.Save(ctx, agg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Загружаем агрегат
	loaded, err := repo.GetByID(ctx, "test-1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if loaded == nil {
		t.Fatal("Expected loaded aggregate, got nil")
	}
}

func TestEventSourcedRepository_ConcurrencyConflict(t *testing.T) {
	repo, _, _ := createTestRepository()
	ctx := context.Background()

	agg1 := createTestAggregate("test-1")
	err := repo.Save(ctx, agg1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Загружаем агрегат дважды
	agg2, _ := repo.GetByID(ctx, "test-1")
	agg3, _ := repo.GetByID(ctx, "test-1")

	// Модифицируем оба
	event2 := &TestUpdatedEvent{
		BaseEvent: events.NewBaseEvent("test.updated", "test-1"),
		Value:     20,
	}
	agg2.RaiseEvent(event2)

	event3 := &TestUpdatedEvent{
		BaseEvent: events.NewBaseEvent("test.updated", "test-1"),
		Value:     30,
	}
	agg3.RaiseEvent(event3)

	// Сохраняем первый
	err = repo.Save(ctx, agg2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Второй должен вызвать конфликт
	err = repo.Save(ctx, agg3)
	if err == nil {
		t.Error("Expected concurrency conflict error")
	}
}

func TestEventSourcedRepository_SnapshotFrequency(t *testing.T) {
	repo, _, snapshotStore := createTestRepository()
	ctx := context.Background()

	agg := createTestAggregate("test-1")

	// Генерируем ровно 100 событий (частота снапшотов)
	for i := 0; i < 100; i++ {
		event := &TestUpdatedEvent{
			BaseEvent: events.NewBaseEvent("test.updated", "test-1"),
			Value:     10 + i,
		}
		agg.RaiseEvent(event)
	}

	err := repo.Save(ctx, agg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Проверяем снапшот
	snapshot, err := snapshotStore.GetSnapshot(ctx, "test-1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if snapshot == nil {
		t.Error("Expected snapshot to be created at frequency threshold")
	}
}

func TestEventSourcedRepository_OptimisticConcurrency(t *testing.T) {
	repo, _, _ := createTestRepository()
	ctx := context.Background()

	agg := createTestAggregate("test-1")
	err := repo.Save(ctx, agg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Проверяем версию
	version, err := repo.GetVersion(ctx, "test-1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if version != 1 {
		t.Errorf("Expected version 1, got %d", version)
	}
}

func TestEventSourcedRepository_Exists(t *testing.T) {
	repo, _, _ := createTestRepository()
	ctx := context.Background()

	exists, err := repo.Exists(ctx, "test-1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if exists {
		t.Error("Expected aggregate to not exist")
	}

	agg := createTestAggregate("test-1")
	err = repo.Save(ctx, agg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	exists, err = repo.Exists(ctx, "test-1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !exists {
		t.Error("Expected aggregate to exist")
	}
}

