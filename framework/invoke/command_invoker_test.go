// Package invoke предоставляет тесты для CommandInvoker.
package invoke

import (
	"context"
	"fmt"
	"testing"
	"time"

	"potter/framework/core"
	"potter/framework/events"
)

// TestCommand тестовая команда
type TestCommand struct {
	Name string
}

func (c TestCommand) CommandName() string {
	return "test_command"
}

// TestEvent тестовое событие
type TestEvent struct {
	*events.BaseEvent
	Data string
}

func NewTestEvent(data string) *TestEvent {
	return &TestEvent{
		BaseEvent: events.NewBaseEvent("test_event", "aggregate-1"),
		Data:      data,
	}
}

// MockPublisher мок для Publisher
type MockPublisher struct {
	published []struct {
		subject string
		data    []byte
		headers map[string]string
	}
}

func (m *MockPublisher) Publish(ctx context.Context, subject string, data []byte, headers map[string]string) error {
	m.published = append(m.published, struct {
		subject string
		data    []byte
		headers map[string]string
	}{subject, data, headers})
	return nil
}

// MockEventBus мок для EventBus
type MockEventBus struct {
	handlers map[string][]events.EventHandler
	events   []events.Event
}

func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		handlers: make(map[string][]events.EventHandler),
		events:   make([]events.Event, 0),
	}
}

func (m *MockEventBus) Publish(ctx context.Context, event events.Event) error {
	m.events = append(m.events, event)
	handlers := m.handlers[event.EventType()]
	for _, handler := range handlers {
		_ = handler.Handle(ctx, event)
	}
	return nil
}

func (m *MockEventBus) Subscribe(eventType string, handler events.EventHandler) error {
	m.handlers[eventType] = append(m.handlers[eventType], handler)
	return nil
}

func (m *MockEventBus) Unsubscribe(eventType string, handler events.EventHandler) error {
	handlers := m.handlers[eventType]
	for i, h := range handlers {
		if h == handler {
			m.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			return nil
		}
	}
	return nil
}

// TestErrorEvent тестовое событие об ошибке
type TestErrorEvent struct {
	*BaseErrorEvent
	Reason string
}

func NewTestErrorEvent(reason string) *TestErrorEvent {
	return &TestErrorEvent{
		BaseErrorEvent: NewBaseErrorEvent(
			"test_error",
			"aggregate-1",
			"TEST_ERROR",
			"test error: "+reason,
			fmt.Errorf("test error: %s", reason),
			false,
		),
		Reason: reason,
	}
}

func TestCommandInvoker_Invoke_ErrorEvent(t *testing.T) {
	ctx := context.Background()
	mockPub := &MockPublisher{}
	mockBus := NewMockEventBus()

	asyncBus := NewAsyncCommandBus(mockPub)
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	invoker := NewCommandInvoker[TestCommand, *TestEvent, *TestErrorEvent](
		asyncBus,
		awaiter,
		"test_event",
		"test_error",
	)

	// Запускаем goroutine для публикации ошибочного события
	go func() {
		time.Sleep(100 * time.Millisecond)
		errorEvent := NewTestErrorEvent("validation failed")
		errorEvent.WithCorrelationID("test-correlation-id")
		_ = mockBus.Publish(ctx, errorEvent)
	}()

	cmd := TestCommand{Name: "test"}
	_, err := invoker.Invoke(ctx, cmd)
	if err == nil {
		t.Fatal("expected error from error event")
	}

	// Проверяем, что ошибка содержит информацию об ошибочном событии
	if !core.WrapWithCode(err, ErrErrorEventReceived).Is(err) {
		t.Errorf("expected ERROR_EVENT_RECEIVED error, got: %v", err)
	}
}

func TestCommandInvoker_Invoke_WithBothResults(t *testing.T) {
	ctx := context.Background()
	mockPub := &MockPublisher{}
	mockBus := NewMockEventBus()

	asyncBus := NewAsyncCommandBus(mockPub)
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	invoker := NewCommandInvoker[TestCommand, *TestEvent, *TestErrorEvent](
		asyncBus,
		awaiter,
		"test_event",
		"test_error",
	)

	// Запускаем goroutine для публикации успешного события
	go func() {
		time.Sleep(100 * time.Millisecond)
		event := NewTestEvent("success data")
		event.WithCorrelationID("test-correlation-id")
		_ = mockBus.Publish(ctx, event)
	}()

	cmd := TestCommand{Name: "test"}
	success, errorEvent, err := invoker.InvokeWithBothResults(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if success == nil {
		t.Fatal("expected success event")
	}

	if errorEvent != nil {
		t.Error("expected nil error event")
	}

	if success.Data != "success data" {
		t.Errorf("expected data 'success data', got '%s'", success.Data)
	}
}

func TestCommandInvoker_Invoke_Success(t *testing.T) {
	ctx := context.Background()
	mockPub := &MockPublisher{}
	mockBus := NewMockEventBus()

	asyncBus := NewAsyncCommandBus(mockPub)
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	invoker := NewCommandInvokerWithoutError[TestCommand, *TestEvent](asyncBus, awaiter, "test_event")

	// Запускаем goroutine для публикации события
	go func() {
		time.Sleep(100 * time.Millisecond)
		event := NewTestEvent("test data")
		event.WithCorrelationID("test-correlation-id")
		_ = mockBus.Publish(ctx, event)
	}()

	cmd := TestCommand{Name: "test"}
	result, err := invoker.Invoke(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Data != "test data" {
		t.Errorf("expected data 'test data', got '%s'", result.Data)
	}
}

func TestCommandInvoker_Invoke_Timeout(t *testing.T) {
	ctx := context.Background()
	mockPub := &MockPublisher{}
	mockBus := NewMockEventBus()

	asyncBus := NewAsyncCommandBus(mockPub)
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	invoker := NewCommandInvokerWithoutError[TestCommand, *TestEvent](asyncBus, awaiter, "test_event").
		WithTimeout(100 * time.Millisecond)

	cmd := TestCommand{Name: "test"}
	_, err := invoker.Invoke(ctx, cmd)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	if !core.WrapWithCode(err, ErrEventTimeout).Is(err) {
		t.Errorf("expected EVENT_TIMEOUT error, got: %v", err)
	}
}

func TestCommandInvoker_InvokeAsync(t *testing.T) {
	ctx := context.Background()
	mockPub := &MockPublisher{}
	mockBus := NewMockEventBus()

	asyncBus := NewAsyncCommandBus(mockPub)
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	invoker := NewCommandInvokerWithoutError[TestCommand, *TestEvent](asyncBus, awaiter, "test_event")

	// Запускаем goroutine для публикации события
	go func() {
		time.Sleep(100 * time.Millisecond)
		event := NewTestEvent("async data")
		event.WithCorrelationID("test-correlation-id")
		_ = mockBus.Publish(ctx, event)
	}()

	cmd := TestCommand{Name: "test"}
	resultCh, err := invoker.InvokeAsync(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case result := <-resultCh:
		if result.IsErr() {
			t.Fatalf("unexpected error: %v", result.Error)
		}
		if result.Value.Data != "async data" {
			t.Errorf("expected data 'async data', got '%s'", result.Value.Data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

