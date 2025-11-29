package saga

import (
	"context"
	"testing"

	"potter/framework/events"
)

func TestDefaultOrchestrator_Execute(t *testing.T) {
	persistence := NewInMemoryPersistence()
	mockEventBus := &mockEventBus{events: make([]events.Event, 0)}
	orchestrator := NewDefaultOrchestrator(persistence, mockEventBus)

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
	err = orchestrator.Execute(ctx, saga)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if saga.Status() != SagaStatusCompleted {
		t.Errorf("Expected status Completed, got %s", saga.Status())
	}

	// Check that events were published
	if len(mockEventBus.events) == 0 {
		t.Error("Expected events to be published")
	}
}

func TestDefaultOrchestrator_Resume(t *testing.T) {
	persistence := NewInMemoryPersistence()
	registry := NewSagaRegistry()
	mockEventBus := &mockEventBus{events: make([]events.Event, 0)}
	orchestrator := NewDefaultOrchestrator(persistence, mockEventBus).WithRegistry(registry)

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

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSagaWithEventBus("test-id", definition, sagaCtx, persistence, mockEventBus)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = persistence.Save(ctx, saga)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	err = orchestrator.Resume(ctx, "test-id")
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	loaded, err := persistence.Load(ctx, "test-id")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Status() != SagaStatusCompleted {
		t.Errorf("Expected status Completed, got %s", loaded.Status())
	}
}

func TestDefaultOrchestrator_GetStatus(t *testing.T) {
	persistence := NewInMemoryPersistence()
	mockEventBus := &mockEventBus{events: make([]events.Event, 0)}
	orchestrator := NewDefaultOrchestrator(persistence, mockEventBus)

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

	status, err := orchestrator.GetStatus(ctx, "test-id")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status != SagaStatusPending {
		t.Errorf("Expected status Pending, got %s", status)
	}
}

func TestDefaultOrchestrator_Compensate(t *testing.T) {
	persistence := NewInMemoryPersistence()
	mockEventBus := &mockEventBus{events: make([]events.Event, 0)}
	orchestrator := NewDefaultOrchestrator(persistence, mockEventBus)

	definition := NewBaseSagaDefinition("test-saga")
	step1 := NewBaseStep("step1")
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("step1", "done")
		return nil
	})
	step1.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("step1", "compensated")
		return nil
	})
	definition.AddStep(step1)

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

	err = orchestrator.Compensate(ctx, saga)
	if err != nil {
		t.Fatalf("Compensate failed: %v", err)
	}

	if saga.Status() != SagaStatusCompensated {
		t.Errorf("Expected status Compensated, got %s", saga.Status())
	}
}


