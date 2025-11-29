package saga

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBaseStep_Execute(t *testing.T) {
	step := NewBaseStep("test-step")
	executed := false
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		executed = true
		return nil
	})

	sagaCtx := NewSagaContext()
	err := step.Execute(context.Background(), sagaCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !executed {
		t.Error("Execute action was not called")
	}
}

func TestBaseStep_Execute_Error(t *testing.T) {
	step := NewBaseStep("test-step")
	expectedErr := errors.New("test error")
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return expectedErr
	})

	sagaCtx := NewSagaContext()
	err := step.Execute(context.Background(), sagaCtx)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestBaseStep_Compensate(t *testing.T) {
	step := NewBaseStep("test-step")
	compensated := false
	step.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		compensated = true
		return nil
	})

	sagaCtx := NewSagaContext()
	err := step.Compensate(context.Background(), sagaCtx)
	if err != nil {
		t.Fatalf("Compensate failed: %v", err)
	}

	if !compensated {
		t.Error("Compensate action was not called")
	}
}

func TestBaseStep_CanExecute(t *testing.T) {
	step := NewBaseStep("test-step")
	step.WithGuard(func(ctx context.Context, sagaCtx SagaContext) bool {
		return sagaCtx.GetBool("can_execute")
	})

	sagaCtx := NewSagaContext()
	if step.CanExecute(context.Background(), sagaCtx) {
		t.Error("Expected CanExecute to return false")
	}

	sagaCtx.Set("can_execute", true)
	if !step.CanExecute(context.Background(), sagaCtx) {
		t.Error("Expected CanExecute to return true")
	}
}

func TestBaseStep_Timeout(t *testing.T) {
	step := NewBaseStep("test-step")
	timeout := 5 * time.Second
	step.WithTimeout(timeout)

	if step.Timeout() != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, step.Timeout())
	}
}

func TestBaseStep_RetryPolicy(t *testing.T) {
	step := NewBaseStep("test-step")
	policy := SimpleRetry(3)
	step.WithRetry(policy)

	if step.RetryPolicy() != policy {
		t.Error("Retry policy mismatch")
	}
}

func TestRetryPolicy_ShouldRetry(t *testing.T) {
	policy := SimpleRetry(3)

	if !policy.ShouldRetry(errors.New("test"), 0) {
		t.Error("Expected ShouldRetry to return true")
	}

	if policy.ShouldRetry(errors.New("test"), 3) {
		t.Error("Expected ShouldRetry to return false after max attempts")
	}
}

func TestRetryPolicy_CalculateDelay(t *testing.T) {
	policy := ExponentialBackoff(3, time.Second, 2.0)

	delay1 := policy.CalculateDelay(0)
	if delay1 != 2*time.Second {
		t.Errorf("Expected delay %v, got %v", 2*time.Second, delay1)
	}

	delay2 := policy.CalculateDelay(1)
	if delay2 != 4*time.Second {
		t.Errorf("Expected delay %v, got %v", 4*time.Second, delay2)
	}
}

func TestCommandStep(t *testing.T) {
	// Mock command bus
	mockBus := &mockCommandBus{
		commands: make(map[string]bool),
	}

	forwardCmd := &mockCommand{name: "forward"}
	compensateCmd := &mockCommand{name: "compensate"}

	step := NewCommandStep("test-step", mockBus, forwardCmd, compensateCmd)

	sagaCtx := NewSagaContext()
	sagaCtx.SetCorrelationID("test-correlation")

	err := step.Execute(context.Background(), sagaCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !mockBus.commands["forward"] {
		t.Error("Forward command was not sent")
	}

	err = step.Compensate(context.Background(), sagaCtx)
	if err != nil {
		t.Fatalf("Compensate failed: %v", err)
	}

	if !mockBus.commands["compensate"] {
		t.Error("Compensate command was not sent")
	}
}

func TestParallelStep(t *testing.T) {
	step1 := NewBaseStep("step1")
	step1.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("step1", "done")
		return nil
	})

	step2 := NewBaseStep("step2")
	step2.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("step2", "done")
		return nil
	})

	parallelStep := NewParallelStep("parallel", step1, step2)

	sagaCtx := NewSagaContext()
	err := parallelStep.Execute(context.Background(), sagaCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if sagaCtx.GetString("step1") != "done" {
		t.Error("Step1 not executed")
	}
	if sagaCtx.GetString("step2") != "done" {
		t.Error("Step2 not executed")
	}
}

func TestConditionalStep(t *testing.T) {
	innerStep := NewBaseStep("inner")
	innerStep.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		sagaCtx.Set("inner", "done")
		return nil
	})

	conditionalStep := NewConditionalStep("conditional",
		func(ctx context.Context, sagaCtx SagaContext) bool {
			return sagaCtx.GetBool("should_execute")
		},
		innerStep,
	)

	sagaCtx := NewSagaContext()

	// Test when condition is false
	err := conditionalStep.Execute(context.Background(), sagaCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if sagaCtx.GetString("inner") != "" {
		t.Error("Inner step should not be executed when condition is false")
	}

	// Test when condition is true
	sagaCtx.Set("should_execute", true)
	err = conditionalStep.Execute(context.Background(), sagaCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if sagaCtx.GetString("inner") != "done" {
		t.Error("Inner step should be executed when condition is true")
	}
}

// Mock implementations for testing

type mockCommand struct {
	name string
}

func (c *mockCommand) CommandName() string {
	return c.name
}

