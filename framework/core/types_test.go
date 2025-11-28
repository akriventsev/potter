package core

import (
	"context"
	"errors"
	"testing"
)

func TestFrameworkContext_GetMetadata(t *testing.T) {
	ctx := context.Background()
	fwCtx := NewFrameworkContext(ctx)

	// Устанавливаем метаданные
	fwCtx.SetMetadata("key1", "value1")
	fwCtx.SetMetadata("key2", 42)

	// Получаем метаданные
	val1, ok1 := fwCtx.GetMetadata("key1")
	if !ok1 {
		t.Error("Expected metadata key1 to exist")
	}
	if val1 != "value1" {
		t.Errorf("Expected value1, got %v", val1)
	}

	val2, ok2 := fwCtx.GetMetadata("key2")
	if !ok2 {
		t.Error("Expected metadata key2 to exist")
	}
	if val2 != 42 {
		t.Errorf("Expected 42, got %v", val2)
	}

	// Несуществующий ключ
	_, ok3 := fwCtx.GetMetadata("nonexistent")
	if ok3 {
		t.Error("Expected nonexistent key to not exist")
	}
}

func TestFrameworkContext_SetMetadata(t *testing.T) {
	ctx := context.Background()
	fwCtx := NewFrameworkContext(ctx)

	fwCtx.SetMetadata("test", "value")
	val, ok := fwCtx.GetMetadata("test")
	if !ok || val != "value" {
		t.Errorf("Expected value, got %v, ok=%v", val, ok)
	}
}

func TestFrameworkContext_GetCorrelationID(t *testing.T) {
	ctx := context.Background()
	fwCtx := NewFrameworkContext(ctx)

	// Устанавливаем correlation ID
	fwCtx.SetMetadata("correlation_id", "corr-123")
	id := fwCtx.GetCorrelationID()
	if id != "corr-123" {
		t.Errorf("Expected corr-123, got %s", id)
	}

	// Без correlation ID
	fwCtx2 := NewFrameworkContext(ctx)
	id2 := fwCtx2.GetCorrelationID()
	if id2 != "" {
		t.Errorf("Expected empty string, got %s", id2)
	}
}

func TestFrameworkContext_GetCausationID(t *testing.T) {
	ctx := context.Background()
	fwCtx := NewFrameworkContext(ctx)

	// Устанавливаем causation ID
	fwCtx.SetMetadata("causation_id", "cause-456")
	id := fwCtx.GetCausationID()
	if id != "cause-456" {
		t.Errorf("Expected cause-456, got %s", id)
	}

	// Без causation ID
	fwCtx2 := NewFrameworkContext(ctx)
	id2 := fwCtx2.GetCausationID()
	if id2 != "" {
		t.Errorf("Expected empty string, got %s", id2)
	}
}

func TestError_Error(t *testing.T) {
	cause := errors.New("root cause")
	err := &Error{
		Code:    "TEST_ERROR",
		Message: "Test error",
		Cause:   cause,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Expected error message")
	}
	if msg != "Test error: root cause" {
		t.Errorf("Expected 'Test error: root cause', got '%s'", msg)
	}

	// Без cause
	err2 := &Error{
		Code:    "TEST_ERROR",
		Message: "Test error",
	}
	msg2 := err2.Error()
	if msg2 != "Test error" {
		t.Errorf("Expected 'Test error', got '%s'", msg2)
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &Error{
		Cause: cause,
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Error("Expected unwrap to return cause")
	}
}

func TestResult_Ok(t *testing.T) {
	result := Ok("success")
	if !result.IsOk() {
		t.Error("Expected result to be ok")
	}
	if result.IsErr() {
		t.Error("Expected result to not be error")
	}
	if result.Value != "success" {
		t.Errorf("Expected 'success', got %v", result.Value)
	}
}

func TestResult_Err(t *testing.T) {
	err := errors.New("test error")
	result := Err[string](err)
	if result.IsOk() {
		t.Error("Expected result to not be ok")
	}
	if !result.IsErr() {
		t.Error("Expected result to be error")
	}
	if result.Error != err {
		t.Error("Expected error to match")
	}
}

func TestResult_IsOk(t *testing.T) {
	result1 := Ok(42)
	if !result1.IsOk() {
		t.Error("Expected IsOk to return true")
	}

	result2 := Err[int](errors.New("error"))
	if result2.IsOk() {
		t.Error("Expected IsOk to return false")
	}
}

func TestResult_IsErr(t *testing.T) {
	result1 := Ok(42)
	if result1.IsErr() {
		t.Error("Expected IsErr to return false")
	}

	result2 := Err[int](errors.New("error"))
	if !result2.IsErr() {
		t.Error("Expected IsErr to return true")
	}
}

func TestOption_Some(t *testing.T) {
	opt := Some("value")
	if !opt.IsSome() {
		t.Error("Expected option to be Some")
	}
	if opt.IsNone() {
		t.Error("Expected option to not be None")
	}
	if opt.Value() != "value" {
		t.Errorf("Expected 'value', got %v", opt.Value())
	}
}

func TestOption_None(t *testing.T) {
	opt := None[string]()
	if opt.IsSome() {
		t.Error("Expected option to not be Some")
	}
	if !opt.IsNone() {
		t.Error("Expected option to be None")
	}
}

func TestOption_IsSome(t *testing.T) {
	opt1 := Some(42)
	if !opt1.IsSome() {
		t.Error("Expected IsSome to return true")
	}

	opt2 := None[int]()
	if opt2.IsSome() {
		t.Error("Expected IsSome to return false")
	}
}

func TestOption_IsNone(t *testing.T) {
	opt1 := Some(42)
	if opt1.IsNone() {
		t.Error("Expected IsNone to return false")
	}

	opt2 := None[int]()
	if !opt2.IsNone() {
		t.Error("Expected IsNone to return true")
	}
}

func TestOption_Value(t *testing.T) {
	opt := Some("test")
	val := opt.Value()
	if val != "test" {
		t.Errorf("Expected 'test', got %v", val)
	}
}

func TestOption_ValueOr(t *testing.T) {
	opt1 := Some("value")
	val1 := opt1.ValueOr("default")
	if val1 != "value" {
		t.Errorf("Expected 'value', got %v", val1)
	}

	opt2 := None[string]()
	val2 := opt2.ValueOr("default")
	if val2 != "default" {
		t.Errorf("Expected 'default', got %v", val2)
	}
}

