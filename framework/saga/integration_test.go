package saga

import (
	"context"
	"fmt"
	"testing"

	"potter/framework/events"
)

// TestSaga_EndToEnd выполняет полный цикл выполнения саги с persistence и orchestrator
func TestSaga_EndToEnd(t *testing.T) {
	// Setup
	persistence := NewInMemoryPersistence()
	mockEventBus := &mockEventBus{events: make([]events.Event, 0)}
	orchestrator := NewDefaultOrchestrator(persistence, mockEventBus)

	// Create saga definition
	definition := NewBaseSagaDefinition("order-saga")
	
	step1 := NewBaseStep("reserve-inventory")
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		if orderID == "" {
			return fmt.Errorf("order_id not found")
		}
		sagaCtx.Set("inventory_reserved", true)
		return nil
	})
	step1.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("inventory_reserved", false)
		return nil
	})
	definition.AddStep(step1)

	step2 := NewBaseStep("charge-payment")
	step2.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		amount := sagaCtx.GetInt("amount")
		if amount <= 0 {
			return fmt.Errorf("invalid amount")
		}
		sagaCtx.Set("payment_charged", true)
		return nil
	})
	step2.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("payment_charged", false)
		return nil
	})
	definition.AddStep(step2)

	step3 := NewBaseStep("create-order")
	step3.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("order_created", true)
		return nil
	})
	definition.AddStep(step3)

	// Create saga instance
	sagaCtx := NewSagaContext()
	sagaCtx.Set("order_id", "order-123")
	sagaCtx.Set("amount", 100)
	sagaCtx.SetCorrelationID("correlation-123")

	saga, err := NewBaseSaga("saga-123", definition, sagaCtx, persistence)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	// Execute saga through orchestrator
	ctx := context.Background()
	err = orchestrator.Execute(ctx, saga)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify saga completed
	if saga.Status() != SagaStatusCompleted {
		t.Errorf("Expected status Completed, got %s", saga.Status())
	}

	// Verify context was updated
	if !sagaCtx.GetBool("inventory_reserved") {
		t.Error("Inventory not reserved")
	}
	if !sagaCtx.GetBool("payment_charged") {
		t.Error("Payment not charged")
	}
	if !sagaCtx.GetBool("order_created") {
		t.Error("Order not created")
	}

	// Verify events were published
	if len(mockEventBus.events) == 0 {
		t.Error("Expected events to be published")
	}

	// Verify saga can be loaded from persistence
	loaded, err := persistence.Load(ctx, "saga-123")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Status() != SagaStatusCompleted {
		t.Errorf("Loaded saga status mismatch: expected Completed, got %s", loaded.Status())
	}
}

// TestSaga_Compensation выполняет сценарий с компенсацией
func TestSaga_Compensation(t *testing.T) {
	persistence := NewInMemoryPersistence()
	mockEventBus := &mockEventBus{events: make([]events.Event, 0)}
	orchestrator := NewDefaultOrchestrator(persistence, mockEventBus)

	definition := NewBaseSagaDefinition("order-saga")
	
	step1 := NewBaseStep("reserve-inventory")
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("inventory_reserved", true)
		return nil
	})
	step1.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("inventory_reserved", false)
		return nil
	})
	definition.AddStep(step1)

	step2 := NewBaseStep("charge-payment")
	step2.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		// Simulate failure
		return fmt.Errorf("payment failed")
	})
	step2.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	definition.AddStep(step2)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("saga-456", definition, sagaCtx, persistence)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = orchestrator.Execute(ctx, saga)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Verify saga was compensated
	if saga.Status() != SagaStatusCompensated {
		t.Errorf("Expected status Compensated, got %s", saga.Status())
	}

	// Verify inventory was released
	if sagaCtx.GetBool("inventory_reserved") {
		t.Error("Inventory should be released after compensation")
	}
}

// TestSaga_Resume проверяет возобновление выполнения саги
func TestSaga_Resume(t *testing.T) {
	persistence := NewInMemoryPersistence()
	registry := NewSagaRegistry()
	mockEventBus := &mockEventBus{events: make([]events.Event, 0)}
	orchestrator := NewDefaultOrchestrator(persistence, mockEventBus)

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

	registry.RegisterSaga("test-saga", definition)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("saga-789", definition, sagaCtx, persistence)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = persistence.Save(ctx, saga)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Resume saga
	err = orchestrator.Resume(ctx, "saga-789")
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	// Verify saga completed
	loaded, err := persistence.Load(ctx, "saga-789")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Status() != SagaStatusCompleted {
		t.Errorf("Expected status Completed, got %s", loaded.Status())
	}
}

