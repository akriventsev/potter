package saga

import (
	"context"
	"fmt"
	"testing"

	"potter/framework/events"
	"potter/framework/eventsourcing"
)

func TestInMemoryPersistence_SaveAndLoad(t *testing.T) {
	persistence := NewInMemoryPersistence()

	definition := NewBaseSagaDefinition("test-saga")
	step := NewBaseStep("step1")
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	definition.AddStep(step)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, persistence)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = persistence.Save(ctx, saga)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := persistence.Load(ctx, "test-id")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID() != saga.ID() {
		t.Errorf("ID mismatch: expected %s, got %s", saga.ID(), loaded.ID())
	}
}

func TestInMemoryPersistence_LoadAll(t *testing.T) {
	persistence := NewInMemoryPersistence()

	definition := NewBaseSagaDefinition("test-saga")
	step := NewBaseStep("step1")
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	definition.AddStep(step)

	// Create and save multiple sagas
	for i := 0; i < 3; i++ {
		sagaCtx := NewSagaContext()
		saga, err := NewBaseSaga(fmt.Sprintf("test-id-%d", i), definition, sagaCtx, persistence)
		if err != nil {
			t.Fatalf("Failed to create saga: %v", err)
		}

		ctx := context.Background()
		err = persistence.Save(ctx, saga)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	ctx := context.Background()
	sagas, err := persistence.LoadAll(ctx, SagaStatusPending)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(sagas) != 3 {
		t.Errorf("Expected 3 sagas, got %d", len(sagas))
	}
}

func TestInMemoryPersistence_Delete(t *testing.T) {
	persistence := NewInMemoryPersistence()

	definition := NewBaseSagaDefinition("test-saga")
	step := NewBaseStep("step1")
	definition.AddStep(step)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, persistence)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = persistence.Save(ctx, saga)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	err = persistence.Delete(ctx, "test-id")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = persistence.Load(ctx, "test-id")
	if err == nil {
		t.Error("Expected error when loading deleted saga")
	}
}

func TestInMemoryPersistence_GetHistory(t *testing.T) {
	persistence := NewInMemoryPersistence()

	definition := NewBaseSagaDefinition("test-saga")
	step := NewBaseStep("step1")
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	definition.AddStep(step)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, persistence)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = saga.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	err = persistence.Save(ctx, saga)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	history, err := persistence.GetHistory(ctx, "test-id")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history) == 0 {
		t.Error("Expected history entries")
	}
}

func TestEventStorePersistence_SaveAndLoadWithStepEvents(t *testing.T) {
	// Создаем mock EventStore и SnapshotStore
	eventStore := eventsourcing.NewInMemoryEventStore(eventsourcing.DefaultInMemoryEventStoreConfig())
	snapshotStore := eventsourcing.NewInMemorySnapshotStore()
	registry := NewSagaRegistry()
	
	persistence := NewEventStorePersistence(eventStore, snapshotStore).WithRegistry(registry)

	definition := NewBaseSagaDefinition("test-saga")
	step1 := NewBaseStep("step1")
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("step1", "done")
		return nil
	})
	definition.AddStep(step1)

	step2 := NewBaseStep("step2")
	step2.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("step2", "done")
		return nil
	})
	definition.AddStep(step2)

	// Регистрируем определение в registry
	err := registry.RegisterSaga("test-saga", definition)
	if err != nil {
		t.Fatalf("Failed to register saga: %v", err)
	}

	mockEventBus := &mockEventBus{events: make([]events.Event, 0)}
	sagaCtx := NewSagaContext()
	saga, err := NewBaseSagaWithEventBus("test-id", definition, sagaCtx, persistence, mockEventBus)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	
	// Выполняем сагу, чтобы создать историю шагов
	err = saga.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Сохраняем сагу (должны сохраниться события шагов)
	err = persistence.Save(ctx, saga)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Загружаем сагу обратно
	loaded, err := persistence.Load(ctx, "test-id")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID() != saga.ID() {
		t.Errorf("ID mismatch: expected %s, got %s", saga.ID(), loaded.ID())
	}

	// Проверяем, что история восстановлена
	history := loaded.GetHistory()
	if len(history) == 0 {
		t.Error("Expected history entries to be restored")
	}

	// Проверяем, что события шагов были сохранены
	events, err := eventStore.GetEvents(ctx, "test-id", 0)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	// Должны быть события: SagaStateChanged, StepStarted, StepCompleted для каждого шага
	stepStartedCount := 0
	stepCompletedCount := 0
	for _, event := range events {
		if event.EventType == "StepStarted" {
			stepStartedCount++
		}
		if event.EventType == "StepCompleted" {
			stepCompletedCount++
		}
	}

	if stepStartedCount < 2 {
		t.Errorf("Expected at least 2 StepStarted events, got %d", stepStartedCount)
	}
	if stepCompletedCount < 2 {
		t.Errorf("Expected at least 2 StepCompleted events, got %d", stepCompletedCount)
	}
}

func TestEventStorePersistence_LoadAll(t *testing.T) {
	eventStore := eventsourcing.NewInMemoryEventStore(eventsourcing.DefaultInMemoryEventStoreConfig())
	snapshotStore := eventsourcing.NewInMemorySnapshotStore()
	registry := NewSagaRegistry()
	
	persistence := NewEventStorePersistence(eventStore, snapshotStore).WithRegistry(registry)

	definition := NewBaseSagaDefinition("test-saga")
	step := NewBaseStep("step1")
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	definition.AddStep(step)

	// Регистрируем определение
	err := registry.RegisterSaga("test-saga", definition)
	if err != nil {
		t.Fatalf("Failed to register saga: %v", err)
	}

	// Создаем и сохраняем несколько саг
	for i := 0; i < 3; i++ {
		sagaCtx := NewSagaContext()
		saga, err := NewBaseSagaWithEventBus(fmt.Sprintf("test-id-%d", i), definition, sagaCtx, persistence, nil)
		if err != nil {
			t.Fatalf("Failed to create saga: %v", err)
		}

		ctx := context.Background()
		err = persistence.Save(ctx, saga)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	ctx := context.Background()
	sagas, err := persistence.LoadAll(ctx, SagaStatusPending)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(sagas) != 3 {
		t.Errorf("Expected 3 sagas, got %d", len(sagas))
	}
}

func TestEventStorePersistence_RoundTripWithErrorsAndRetryAttempt(t *testing.T) {
	// Создаем mock EventStore и SnapshotStore
	eventStore := eventsourcing.NewInMemoryEventStore(eventsourcing.DefaultInMemoryEventStoreConfig())
	snapshotStore := eventsourcing.NewInMemorySnapshotStore()
	registry := NewSagaRegistry()
	
	persistence := NewEventStorePersistence(eventStore, snapshotStore).WithRegistry(registry)

	definition := NewBaseSagaDefinition("test-saga")
	step1 := NewBaseStep("step1")
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return fmt.Errorf("test error")
	})
	definition.AddStep(step1)

	// Регистрируем определение в registry
	err := registry.RegisterSaga("test-saga", definition)
	if err != nil {
		t.Fatalf("Failed to register saga: %v", err)
	}

	mockEventBus := &mockEventBus{events: make([]events.Event, 0)}
	sagaCtx := NewSagaContext()
	saga, err := NewBaseSagaWithEventBus("test-id", definition, sagaCtx, persistence, mockEventBus)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	
	// Выполняем сагу, чтобы создать историю с ошибкой
	_ = saga.Execute(ctx) // Ожидаем ошибку

	// Вручную устанавливаем RetryAttempt в истории для теста
	saga.mu.Lock()
	if len(saga.history) > 0 {
		saga.history[0].RetryAttempt = 3
	}
	saga.mu.Unlock()

	// Сохраняем сагу
	err = persistence.Save(ctx, saga)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Загружаем сагу обратно
	loaded, err := persistence.Load(ctx, "test-id")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Проверяем, что история восстановлена с ошибкой и RetryAttempt
	history := loaded.GetHistory()
	if len(history) == 0 {
		t.Fatal("Expected history entries to be restored")
	}

	firstHist := history[0]
	if firstHist.Error == nil {
		t.Error("Expected error to be restored from history")
	} else if firstHist.Error.Error() != "test error" {
		t.Errorf("Expected error message 'test error', got '%s'", firstHist.Error.Error())
	}

	if firstHist.RetryAttempt != 3 {
		t.Errorf("Expected RetryAttempt to be 3, got %d", firstHist.RetryAttempt)
	}
}

