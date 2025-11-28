// Package invoke предоставляет тесты для модуля errors.
package invoke

import (
	"testing"
)

func TestNewErrorEventReceivedError_NilErrorEvent(t *testing.T) {
	// Тест на безопасное поведение при nil ErrorEvent
	err := NewErrorEventReceivedError(nil)
	
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	
	if err.Code != ErrErrorEventReceived {
		t.Errorf("expected error code %s, got %s", ErrErrorEventReceived, err.Code)
	}
	
	if err.Message != "error event received: unknown error event" {
		t.Errorf("expected message 'error event received: unknown error event', got '%s'", err.Message)
	}
	
	// Проверяем, что cause равен nil
	if err.Cause != nil {
		t.Errorf("expected nil cause, got %v", err.Cause)
	}
}

func TestNewErrorEventReceivedError_ValidErrorEvent(t *testing.T) {
	// Тест на корректное поведение при валидном ErrorEvent
	testError := NewTestErrorEvent("test error message")
	err := NewErrorEventReceivedError(testError)
	
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	
	if err.Code != ErrErrorEventReceived {
		t.Errorf("expected error code %s, got %s", ErrErrorEventReceived, err.Code)
	}
	
	expectedMessage := "error event received: test error: test error message"
	if err.Message != expectedMessage {
		t.Errorf("expected message '%s', got '%s'", expectedMessage, err.Message)
	}
	
	// Проверяем, что cause установлен
	if err.Cause == nil {
		t.Error("expected non-nil cause")
	}
}

func TestNewErrorEventReceivedError_ErrorEventWithoutError(t *testing.T) {
	// Тест на ErrorEvent без вложенной ошибки
	errorEvent := NewBaseErrorEvent(
		"test_error",
		"aggregate-1",
		"TEST_ERROR",
		"test error message",
		nil, // без вложенной ошибки
		false,
	)
	
	err := NewErrorEventReceivedError(errorEvent)
	
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	
	if err.Code != ErrErrorEventReceived {
		t.Errorf("expected error code %s, got %s", ErrErrorEventReceived, err.Code)
	}
	
	expectedMessage := "error event received: test error message"
	if err.Message != expectedMessage {
		t.Errorf("expected message '%s', got '%s'", expectedMessage, err.Message)
	}
	
	// Проверяем, что cause равен nil, так как вложенной ошибки нет
	if err.Cause != nil {
		t.Errorf("expected nil cause, got %v", err.Cause)
	}
}

