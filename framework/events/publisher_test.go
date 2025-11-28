package events

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// MockEvent для тестирования
type MockEvent struct {
	eventID     string
	eventType   string
	aggregateID string
	occurredAt  time.Time
	metadata    EventMetadata
}

func (e *MockEvent) EventID() string {
	return e.eventID
}

func (e *MockEvent) EventType() string {
	return e.eventType
}

func (e *MockEvent) OccurredAt() time.Time {
	return e.occurredAt
}

func (e *MockEvent) AggregateID() string {
	return e.aggregateID
}

func (e *MockEvent) Metadata() EventMetadata {
	return e.metadata
}

func newMockEvent(eventType, aggregateID string) *MockEvent {
	return &MockEvent{
		eventID:     uuid.New().String(),
		eventType:   eventType,
		aggregateID: aggregateID,
		occurredAt:  time.Now(),
		metadata:    make(EventMetadata),
	}
}

// MockEventHandler для тестирования
type MockEventHandler struct {
	mu       sync.Mutex
	handled  []Event
	err      error
	delay    time.Duration
}

func (h *MockEventHandler) Handle(ctx context.Context, event Event) error {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handled = append(h.handled, event)
	return h.err
}

func (h *MockEventHandler) EventType() string {
	return "test_event"
}

func (h *MockEventHandler) HandledCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.handled)
}

func TestInMemoryEventPublisher_Publish(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	handler := &MockEventHandler{}

	err := publisher.Subscribe("test_event", handler)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	event := newMockEvent("test_event", "agg-1")
	ctx := context.Background()

	err = publisher.Publish(ctx, event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Даем время на обработку
	time.Sleep(10 * time.Millisecond)

	if handler.HandledCount() != 1 {
		t.Errorf("Expected 1 handled event, got %d", handler.HandledCount())
	}
}

func TestInMemoryEventPublisher_Subscribe(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	handler := &MockEventHandler{}

	err := publisher.Subscribe("test_event", handler)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestInMemoryEventPublisher_WithRetry(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	retryConfig := RetryConfig{
		MaxAttempts:      3,
		InitialDelay:     10 * time.Millisecond,
		MaxDelay:         100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}
	publisher.WithRetry(retryConfig)

	handler := &MockEventHandler{
		err: errors.New("handler error"),
	}
	if err := publisher.Subscribe("test_event", handler); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	event := newMockEvent("test_event", "agg-1")
	ctx := context.Background()

	err := publisher.Publish(ctx, event)
	if err == nil {
		t.Error("Expected error after retries")
	}

	// Даем время на retry
	time.Sleep(200 * time.Millisecond)

	// Handler должен быть вызван несколько раз из-за retry
	if handler.HandledCount() < 1 {
		t.Error("Expected handler to be called at least once")
	}
}

func TestAsyncEventPublisher_Publish(t *testing.T) {
	publisher := NewAsyncEventPublisher(2, 10)
	handler := &MockEventHandler{}

	if err := publisher.Subscribe("test_event", handler); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	event := newMockEvent("test_event", "agg-1")
	ctx := context.Background()

	err := publisher.Publish(ctx, event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Даем время на асинхронную обработку
	time.Sleep(50 * time.Millisecond)

	if handler.HandledCount() != 1 {
		t.Errorf("Expected 1 handled event, got %d", handler.HandledCount())
	}
}

func TestAsyncEventPublisher_Stop(t *testing.T) {
	publisher := NewAsyncEventPublisher(2, 10)
	handler := &MockEventHandler{}

	if err := publisher.Subscribe("test_event", handler); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Публикуем несколько событий
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		event := newMockEvent("test_event", "agg-1")
		_ = publisher.Publish(ctx, event)
	}

	// Останавливаем публикатор
	stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := publisher.Stop(stopCtx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Даем время на обработку оставшихся событий
	time.Sleep(50 * time.Millisecond)
}

func TestAsyncEventPublisher_Stop_Idempotent(t *testing.T) {
	publisher := NewAsyncEventPublisher(2, 10)
	handler := &MockEventHandler{}

	if err := publisher.Subscribe("test_event", handler); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Публикуем несколько событий
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		event := newMockEvent("test_event", "agg-1")
		_ = publisher.Publish(ctx, event)
	}

	// Останавливаем публикатор первый раз
	stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := publisher.Stop(stopCtx)
	if err != nil {
		t.Errorf("Expected no error on first Stop, got %v", err)
	}

	// Повторный вызов Stop не должен вызвать panic
	stopCtx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()

	err2 := publisher.Stop(stopCtx2)
	if err2 != nil {
		t.Errorf("Expected no error on second Stop, got %v", err2)
	}

	// Третий вызов также должен быть безопасным
	err3 := publisher.Stop(context.Background())
	if err3 != nil {
		t.Errorf("Expected no error on third Stop, got %v", err3)
	}

	// Даем время на обработку оставшихся событий
	time.Sleep(50 * time.Millisecond)
}

func TestBatchEventPublisher_Publish(t *testing.T) {
	publisher := NewBatchEventPublisher(3, 100*time.Millisecond)
	handler := &MockEventHandler{}

	if err := publisher.Subscribe("test_event", handler); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	ctx := context.Background()

	// Публикуем события, но не заполняем batch
	event1 := newMockEvent("test_event", "agg-1")
	_ = publisher.Publish(ctx, event1)

	event2 := newMockEvent("test_event", "agg-2")
	_ = publisher.Publish(ctx, event2)

	// Третий event должен вызвать flush
	event3 := newMockEvent("test_event", "agg-3")
	_ = publisher.Publish(ctx, event3)

	// Даем время на обработку
	time.Sleep(50 * time.Millisecond)

	if handler.HandledCount() != 3 {
		t.Errorf("Expected 3 handled events, got %d", handler.HandledCount())
	}
}

func TestBatchEventPublisher_Flush(t *testing.T) {
	publisher := NewBatchEventPublisher(10, 1*time.Second)
	handler := &MockEventHandler{}

	if err := publisher.Subscribe("test_event", handler); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	ctx := context.Background()

	// Публикуем события
	for i := 0; i < 5; i++ {
		event := newMockEvent("test_event", "agg-1")
		_ = publisher.Publish(ctx, event)
	}

	// Принудительно flush
	err := publisher.Flush(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Даем время на обработку
	time.Sleep(50 * time.Millisecond)

	if handler.HandledCount() != 5 {
		t.Errorf("Expected 5 handled events, got %d", handler.HandledCount())
	}
}

func TestBatchEventPublisher_Stop(t *testing.T) {
	publisher := NewBatchEventPublisher(10, 1*time.Second)
	handler := &MockEventHandler{}

	if err := publisher.Subscribe("test_event", handler); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	ctx := context.Background()

	// Публикуем события
	for i := 0; i < 3; i++ {
		event := newMockEvent("test_event", "agg-1")
		_ = publisher.Publish(ctx, event)
	}

	// Останавливаем публикатор (должен вызвать flush)
	stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := publisher.Stop(stopCtx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Даем время на обработку
	time.Sleep(50 * time.Millisecond)

	if handler.HandledCount() != 3 {
		t.Errorf("Expected 3 handled events, got %d", handler.HandledCount())
	}
}

func TestInMemoryEventPublisher_MultipleSubscribers(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	handler1 := &MockEventHandler{}
	handler2 := &MockEventHandler{}

	if err := publisher.Subscribe("test_event", handler1); err != nil {
		t.Fatalf("Failed to subscribe handler1: %v", err)
	}
	if err := publisher.Subscribe("test_event", handler2); err != nil {
		t.Fatalf("Failed to subscribe handler2: %v", err)
	}

	event := newMockEvent("test_event", "agg-1")
	ctx := context.Background()

	err := publisher.Publish(ctx, event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Даем время на обработку
	time.Sleep(50 * time.Millisecond)

	if handler1.HandledCount() != 1 {
		t.Errorf("Expected handler1 to handle 1 event, got %d", handler1.HandledCount())
	}
	if handler2.HandledCount() != 1 {
		t.Errorf("Expected handler2 to handle 1 event, got %d", handler2.HandledCount())
	}
}

func TestAsyncEventPublisher_ConcurrentPublish(t *testing.T) {
	publisher := NewAsyncEventPublisher(5, 100)
	handler := &MockEventHandler{}

	if err := publisher.Subscribe("test_event", handler); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	ctx := context.Background()

	// Публикуем события конкурентно
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := newMockEvent("test_event", "agg-1")
			_ = publisher.Publish(ctx, event)
		}()
	}

	wg.Wait()

	// Даем время на обработку
	time.Sleep(200 * time.Millisecond)

	if handler.HandledCount() != 10 {
		t.Errorf("Expected 10 handled events, got %d", handler.HandledCount())
	}
}

