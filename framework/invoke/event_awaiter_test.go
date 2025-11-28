// Package invoke предоставляет тесты для EventAwaiter.
package invoke

import (
	"context"
	"testing"
	"time"

	"potter/framework/core"
)

func TestEventAwaiter_Await_Success(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockEventBus()
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	correlationID := "test-correlation-id"

	// Запускаем goroutine для публикации события
	go func() {
		time.Sleep(100 * time.Millisecond)
		event := NewTestEvent("test data")
		event.WithCorrelationID(correlationID)
		_ = mockBus.Publish(ctx, event)
	}()

	event, err := awaiter.Await(ctx, correlationID, "test_event", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event == nil {
		t.Fatal("expected event, got nil")
	}

	if event.EventType() != "test_event" {
		t.Errorf("expected event type 'test_event', got '%s'", event.EventType())
	}
}

func TestEventAwaiter_Await_Timeout(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockEventBus()
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	correlationID := "test-correlation-id"

	_, err := awaiter.Await(ctx, correlationID, "test_event", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	if !core.WrapWithCode(err, ErrEventTimeout).Is(err) {
		t.Errorf("expected EVENT_TIMEOUT error, got: %v", err)
	}
}

func TestEventAwaiter_AwaitMultiple(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockEventBus()
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	correlationID := "test-correlation-id"

	// Запускаем goroutine для публикации событий
	go func() {
		time.Sleep(100 * time.Millisecond)
		event1 := NewTestEvent("data1")
		event1.WithCorrelationID(correlationID)
		_ = mockBus.Publish(ctx, event1)

		time.Sleep(50 * time.Millisecond)
		event2 := NewTestEvent("data2")
		event2.WithCorrelationID(correlationID)
		_ = mockBus.Publish(ctx, event2)
	}()

	events, err := awaiter.AwaitMultiple(ctx, correlationID, []string{"test_event", "test_event"}, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestEventAwaiter_Cancel(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockEventBus()
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	correlationID := "test-correlation-id"

	// Запускаем goroutine для отмены
	go func() {
		time.Sleep(50 * time.Millisecond)
		awaiter.Cancel(correlationID)
	}()

	_, err := awaiter.Await(ctx, correlationID, "test_event", 5*time.Second)
	if err == nil {
		t.Fatal("expected error after cancel")
	}
}

func TestEventAwaiter_Stop(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockEventBus()
	awaiter := NewEventAwaiterFromEventBus(mockBus)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := awaiter.Stop(stopCtx); err != nil {
		t.Fatalf("unexpected error stopping awaiter: %v", err)
	}

	// Попытка использовать остановленный awaiter
	_, err := awaiter.Await(ctx, "test-id", "test_event", time.Second)
	if err == nil {
		t.Fatal("expected error for stopped awaiter")
	}

	if !core.WrapWithCode(err, ErrEventAwaiterStopped).Is(err) {
		t.Errorf("expected EVENT_AWAITER_STOPPED error, got: %v", err)
	}
}

func TestEventAwaiter_AwaitAny(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockEventBus()
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	correlationID := "test-correlation-id"

	// Запускаем goroutine для публикации события
	go func() {
		time.Sleep(100 * time.Millisecond)
		event := NewTestEvent("any data")
		event.WithCorrelationID(correlationID)
		_ = mockBus.Publish(ctx, event)
	}()

	event, receivedType, err := awaiter.AwaitAny(ctx, correlationID, []string{"test_event", "other_event"}, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event == nil {
		t.Fatal("expected event, got nil")
	}

	if receivedType != "test_event" {
		t.Errorf("expected event type 'test_event', got '%s'", receivedType)
	}
}

func TestEventAwaiter_AwaitSuccessOrError(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockEventBus()
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	correlationID := "test-correlation-id"

	// Запускаем goroutine для публикации успешного события
	go func() {
		time.Sleep(100 * time.Millisecond)
		event := NewTestEvent("success data")
		event.WithCorrelationID(correlationID)
		_ = mockBus.Publish(ctx, event)
	}()

	event, isSuccess, err := awaiter.AwaitSuccessOrError(ctx, correlationID, "test_event", "test_error", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !isSuccess {
		t.Error("expected success event")
	}

	if event == nil {
		t.Fatal("expected event, got nil")
	}

	if event.EventType() != "test_event" {
		t.Errorf("expected event type 'test_event', got '%s'", event.EventType())
	}
}

// TestEventAwaiter_Await_Unsubscribe проверяет, что подписчики отписываются после завершения ожидания
func TestEventAwaiter_Await_Unsubscribe(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockEventBus()
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	correlationID := "test-correlation-id"
	eventType := "test_event"

	// Выполняем несколько вызовов Await
	for i := 0; i < 5; i++ {
		// Запускаем goroutine для публикации события
		go func() {
			time.Sleep(50 * time.Millisecond)
			event := NewTestEvent("test data")
			event.WithCorrelationID(correlationID)
			_ = mockBus.Publish(ctx, event)
		}()

		_, err := awaiter.Await(ctx, correlationID, eventType, 5*time.Second)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}

		// Проверяем, что количество подписчиков не растёт
		// После каждого Await должен оставаться только один активный подписчик (если есть активные waiters)
		// или ноль (если все waiters завершены)
		// В MockEventBus мы можем проверить количество handlers
		handlers := mockBus.handlers[eventType]
		// После отписки количество handlers должно быть 0 или 1 (если есть активный waiter)
		if len(handlers) > 1 {
			t.Errorf("iteration %d: expected at most 1 handler after unsubscribe, got %d", i, len(handlers))
		}
	}

	// После всех вызовов не должно быть активных подписчиков
	handlers := mockBus.handlers[eventType]
	if len(handlers) > 0 {
		t.Errorf("expected 0 handlers after all awaits completed, got %d", len(handlers))
	}
}

// TestEventAwaiter_Await_Timeout_Unsubscribe проверяет отписку при timeout
func TestEventAwaiter_Await_Timeout_Unsubscribe(t *testing.T) {
	ctx := context.Background()
	mockBus := NewMockEventBus()
	awaiter := NewEventAwaiterFromEventBus(mockBus)
	defer awaiter.Stop(ctx)

	correlationID := "test-correlation-id"
	eventType := "test_event"

	// Выполняем несколько вызовов Await с timeout
	for i := 0; i < 3; i++ {
		_, err := awaiter.Await(ctx, correlationID, eventType, 50*time.Millisecond)
		if err == nil {
			t.Fatalf("iteration %d: expected timeout error", i)
		}

		// Проверяем, что подписчики отписаны после timeout
		handlers := mockBus.handlers[eventType]
		if len(handlers) > 0 {
			t.Errorf("iteration %d: expected 0 handlers after timeout, got %d", i, len(handlers))
		}
	}
}
