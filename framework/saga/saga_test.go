package saga

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewBaseSaga(t *testing.T) {
	definition := NewBaseSagaDefinition("test-saga")
	step := NewBaseStep("step1")
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	definition.AddStep(step)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, nil)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	if saga.ID() != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", saga.ID())
	}

	if saga.Status() != SagaStatusPending {
		t.Errorf("Expected status Pending, got %s", saga.Status())
	}

	if saga.Definition() != definition {
		t.Error("Definition mismatch")
	}
}

func TestBaseSaga_Execute_Success(t *testing.T) {
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

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, nil)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = saga.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if saga.Status() != SagaStatusCompleted {
		t.Errorf("Expected status Completed, got %s", saga.Status())
	}

	history := saga.GetHistory()
	if len(history) != 2 {
		t.Errorf("Expected 2 history entries, got %d", len(history))
	}

	if sagaCtx.GetString("step1") != "done" {
		t.Error("Step1 not executed")
	}
	if sagaCtx.GetString("step2") != "done" {
		t.Error("Step2 not executed")
	}
}

func TestBaseSaga_Execute_WithError(t *testing.T) {
	definition := NewBaseSagaDefinition("test-saga")
	step1 := NewBaseStep("step1")
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("step1", "done")
		return nil
	})
	definition.AddStep(step1)

	step2 := NewBaseStep("step2")
	step2.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return fmt.Errorf("step2 failed")
	})
	definition.AddStep(step2)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, nil)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = saga.Execute(ctx)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if saga.Status() != SagaStatusCompensated {
		t.Errorf("Expected status Compensated, got %s", saga.Status())
	}
}

func TestBaseSaga_Compensate(t *testing.T) {
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
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, nil)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = saga.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	err = saga.Compensate(ctx)
	if err != nil {
		t.Fatalf("Compensate failed: %v", err)
	}

	if saga.Status() != SagaStatusCompensated {
		t.Errorf("Expected status Compensated, got %s", saga.Status())
	}

	if sagaCtx.GetString("step1") != "compensated" {
		t.Error("Step1 not compensated")
	}
}

func TestBaseSaga_GetHistory(t *testing.T) {
	definition := NewBaseSagaDefinition("test-saga")
	step1 := NewBaseStep("step1")
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	definition.AddStep(step1)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, nil)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = saga.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	history := saga.GetHistory()
	if len(history) == 0 {
		t.Error("Expected history entries")
	}

	if history[0].StepName != "step1" {
		t.Errorf("Expected step name 'step1', got '%s'", history[0].StepName)
	}
}

func TestSagaContextImpl(t *testing.T) {
	ctx := NewSagaContext()

	ctx.Set("key1", "value1")
	ctx.Set("key2", 42)
	ctx.Set("key3", true)

	if ctx.GetString("key1") != "value1" {
		t.Errorf("Expected 'value1', got '%s'", ctx.GetString("key1"))
	}

	if ctx.GetInt("key2") != 42 {
		t.Errorf("Expected 42, got %d", ctx.GetInt("key2"))
	}

	if !ctx.GetBool("key3") {
		t.Error("Expected true, got false")
	}

	correlationID := "test-correlation-id"
	ctx.SetCorrelationID(correlationID)
	if ctx.CorrelationID() != correlationID {
		t.Errorf("Expected correlation ID '%s', got '%s'", correlationID, ctx.CorrelationID())
	}

	// Test ToMap/FromMap
	data := ctx.ToMap()
	newCtx := NewSagaContext()
	if err := newCtx.FromMap(data); err != nil {
		t.Fatalf("FromMap failed: %v", err)
	}

	if newCtx.GetString("key1") != "value1" {
		t.Error("Context not restored correctly")
	}
}

func TestBaseSagaDefinition_Build(t *testing.T) {
	definition := NewBaseSagaDefinition("test-saga")
	step1 := NewBaseStep("step1")
	definition.AddStep(step1)

	fsmInstance, err := definition.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if fsmInstance == nil {
		t.Fatal("FSM instance is nil")
	}

	// Test building without steps
	emptyDef := NewBaseSagaDefinition("empty")
	_, err = emptyDef.Build()
	if err == nil {
		t.Error("Expected error when building definition without steps")
	}
}

func TestBaseSaga_Execute_WithTimeout(t *testing.T) {
	definition := NewBaseSagaDefinition("test-saga")
	step1 := NewBaseStep("step1")
	step1.WithTimeout(100 * time.Millisecond)
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return nil
		}
	})
	definition.AddStep(step1)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, nil)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = saga.Execute(ctx)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

func TestBaseSaga_Execute_WithRetry(t *testing.T) {
	attempts := 0
	definition := NewBaseSagaDefinition("test-saga")
	step1 := NewBaseStep("step1")
	step1.WithRetry(SimpleRetry(3))
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("temporary error")
		}
		return nil
	})
	definition.AddStep(step1)

	sagaCtx := NewSagaContext()
	saga, err := NewBaseSaga("test-id", definition, sagaCtx, nil)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	ctx := context.Background()
	err = saga.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

