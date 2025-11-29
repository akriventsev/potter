package saga

import (
	"context"
	"testing"
	"time"
)

func TestSagaBuilder_Build(t *testing.T) {
	builder := NewSagaBuilder("test-saga")

	step1 := NewBaseStep("step1")
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	builder.AddStep(step1)

	step2 := NewBaseStep("step2")
	step2.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	builder.AddStep(step2)

	definition, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if definition.Name() != "test-saga" {
		t.Errorf("Expected name 'test-saga', got '%s'", definition.Name())
	}

	if len(definition.Steps()) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(definition.Steps()))
	}
}

func TestSagaBuilder_Build_NoSteps(t *testing.T) {
	builder := NewSagaBuilder("test-saga")

	_, err := builder.Build()
	if err == nil {
		t.Error("Expected error when building saga without steps")
	}
}

func TestSagaBuilder_WithTimeout(t *testing.T) {
	builder := NewSagaBuilder("test-saga")
	timeout := 5 * time.Second
	builder.WithTimeout(timeout)

	step := NewBaseStep("step1")
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	builder.AddStep(step)

	definition, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if definition.Steps()[0].Timeout() != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, definition.Steps()[0].Timeout())
	}
}

func TestSagaBuilder_WithRetryPolicy(t *testing.T) {
	builder := NewSagaBuilder("test-saga")
	policy := SimpleRetry(3)
	builder.WithRetryPolicy(policy)

	step := NewBaseStep("step1")
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	builder.AddStep(step)

	definition, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if definition.Steps()[0].RetryPolicy() != policy {
		t.Error("Retry policy mismatch")
	}
}

func TestStepBuilder_Build(t *testing.T) {
	builder := NewStepBuilder("test-step")

	executed := false
	builder.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		executed = true
		return nil
	})

	step, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	sagaCtx := NewSagaContext()
	err = step.Execute(context.Background(), sagaCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !executed {
		t.Error("Execute action was not called")
	}
}

func TestStepBuilder_Build_NoExecute(t *testing.T) {
	builder := NewStepBuilder("test-step")

	_, err := builder.Build()
	if err == nil {
		t.Error("Expected error when building step without execute action")
	}
}

func TestStepBuilder_WithCompensate(t *testing.T) {
	builder := NewStepBuilder("test-step")

	compensated := false
	builder.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	builder.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		compensated = true
		return nil
	})

	step, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	sagaCtx := NewSagaContext()
	err = step.Compensate(context.Background(), sagaCtx)
	if err != nil {
		t.Fatalf("Compensate failed: %v", err)
	}

	if !compensated {
		t.Error("Compensate action was not called")
	}
}

