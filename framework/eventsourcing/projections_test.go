package eventsourcing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/akriventsev/potter/framework/events"
)

// TestProjection тестовая проекция
type TestProjection struct {
	name            string
	processedEvents []StoredEvent
	resetCalled     bool
	mu              sync.RWMutex
}

func NewTestProjection(name string) *TestProjection {
	return &TestProjection{
		name:            name,
		processedEvents: make([]StoredEvent, 0),
		resetCalled:     false,
	}
}

func (p *TestProjection) Name() string {
	return p.name
}

func (p *TestProjection) HandleEvent(ctx context.Context, event StoredEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.processedEvents = append(p.processedEvents, event)
	return nil
}

func (p *TestProjection) Reset(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.processedEvents = make([]StoredEvent, 0)
	p.resetCalled = true
	return nil
}

func (p *TestProjection) GetProcessedCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.processedEvents)
}

func (p *TestProjection) WasReset() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.resetCalled
}

func TestProjectionManager_Register(t *testing.T) {
	eventStore := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	checkpointStore := NewInMemoryCheckpointStore()

	manager := NewProjectionManager(eventStore, checkpointStore)

	projection := NewTestProjection("test-projection")
	err := manager.Register(projection)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Попытка зарегистрировать повторно должна вернуть ошибку
	err = manager.Register(projection)
	if err == nil {
		t.Error("Expected error when registering duplicate projection")
	}
}

func TestProjectionManager_Start(t *testing.T) {
	eventStore := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	checkpointStore := NewInMemoryCheckpointStore()

	manager := NewProjectionManager(eventStore, checkpointStore)

	projection := NewTestProjection("test-projection")
	if err := manager.Register(projection); err != nil {
		t.Fatalf("Failed to register projection: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Даем время на запуск
	time.Sleep(100 * time.Millisecond)

	// Проверяем статус
	status, err := manager.GetStatus("test-projection")
	if err != nil {
		t.Fatalf("Expected no error getting status, got %v", err)
	}

	if status.State != "running" {
		t.Errorf("Expected state 'running', got %s", status.State)
	}
}

func TestProjectionManager_Stop(t *testing.T) {
	eventStore := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	checkpointStore := NewInMemoryCheckpointStore()

	manager := NewProjectionManager(eventStore, checkpointStore)

	projection := NewTestProjection("test-projection")
	if err := manager.Register(projection); err != nil {
		t.Fatalf("Failed to register projection: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err := manager.Stop(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Проверяем что проекция остановлена
	status, err := manager.GetStatus("test-projection")
	if err == nil {
		if status.State != "stopped" {
			t.Errorf("Expected state 'stopped', got %s", status.State)
		}
	}
}

func TestProjectionManager_Rebuild(t *testing.T) {
	eventStore := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	checkpointStore := NewInMemoryCheckpointStore()

	// Создаем события для обработки
	ctx := context.Background()
	event1 := events.NewBaseEvent("test.event", "agg-1")
	event2 := events.NewBaseEvent("test.event", "agg-2")
	if err := eventStore.AppendEvents(ctx, "agg-1", 0, []events.Event{event1}); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}
	if err := eventStore.AppendEvents(ctx, "agg-2", 0, []events.Event{event2}); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}

	manager := NewProjectionManager(eventStore, checkpointStore)

	projection := NewTestProjection("test-projection")
	if err := manager.Register(projection); err != nil {
		t.Fatalf("Failed to register projection: %v", err)
	}

	err := manager.Rebuild(ctx, "test-projection")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Проверяем что проекция была сброшена
	if !projection.WasReset() {
		t.Error("Expected Reset() to be called during rebuild")
	}

	// Даем время на обработку
	time.Sleep(200 * time.Millisecond)

	// Проверяем что события были обработаны
	count := projection.GetProcessedCount()
	if count < 2 {
		t.Errorf("Expected at least 2 events processed, got %d", count)
	}
}

func TestProjectionManager_CheckpointSave(t *testing.T) {
	checkpointStore := NewInMemoryCheckpointStore()
	ctx := context.Background()

	projectionName := "test-projection"
	position := int64(100)

	err := checkpointStore.SaveCheckpoint(ctx, projectionName, position)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Проверяем что checkpoint был сохранен
	savedPosition, err := checkpointStore.GetCheckpoint(ctx, projectionName)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if savedPosition != position {
		t.Errorf("Expected position %d, got %d", position, savedPosition)
	}
}

func TestProjectionManager_CheckpointGet(t *testing.T) {
	checkpointStore := NewInMemoryCheckpointStore()
	ctx := context.Background()

	projectionName := "test-projection"
	
	// Проверяем что для несуществующего checkpoint возвращается 0
	position, err := checkpointStore.GetCheckpoint(ctx, projectionName)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if position != 0 {
		t.Errorf("Expected position 0 for new projection, got %d", position)
	}

	// Сохраняем checkpoint и проверяем
	savedPosition := int64(50)
	if err := checkpointStore.SaveCheckpoint(ctx, projectionName, savedPosition); err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	position, err = checkpointStore.GetCheckpoint(ctx, projectionName)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if position != savedPosition {
		t.Errorf("Expected position %d, got %d", savedPosition, position)
	}
}

func TestProjectionManager_ProcessEvents(t *testing.T) {
	eventStore := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	checkpointStore := NewInMemoryCheckpointStore()

	ctx := context.Background()

	// Создаем события
	event1 := events.NewBaseEvent("test.event1", "agg-1")
	event2 := events.NewBaseEvent("test.event2", "agg-2")
	if err := eventStore.AppendEvents(ctx, "agg-1", 0, []events.Event{event1}); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}
	if err := eventStore.AppendEvents(ctx, "agg-2", 0, []events.Event{event2}); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}

	manager := NewProjectionManager(eventStore, checkpointStore)

	projection := NewTestProjection("test-projection")
	if err := manager.Register(projection); err != nil {
		t.Fatalf("Failed to register projection: %v", err)
	}

	// Запускаем проекцию
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Даем время на обработку
	time.Sleep(200 * time.Millisecond)

	// Проверяем что события были обработаны
	count := projection.GetProcessedCount()
	if count < 2 {
		t.Errorf("Expected at least 2 events processed, got %d", count)
	}

	// Проверяем checkpoint
	position, err := checkpointStore.GetCheckpoint(ctx, "test-projection")
	if err != nil {
		t.Fatalf("Failed to get checkpoint: %v", err)
	}

	if position == 0 {
		t.Error("Expected checkpoint position to be saved")
	}
}

func TestProjectionManager_ResumeFromCheckpoint(t *testing.T) {
	eventStore := NewInMemoryEventStore(DefaultInMemoryEventStoreConfig())
	checkpointStore := NewInMemoryCheckpointStore()

	ctx := context.Background()

	// Создаем первые события
	event1 := events.NewBaseEvent("test.event1", "agg-1")
	event2 := events.NewBaseEvent("test.event2", "agg-2")
	if err := eventStore.AppendEvents(ctx, "agg-1", 0, []events.Event{event1}); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}
	if err := eventStore.AppendEvents(ctx, "agg-2", 0, []events.Event{event2}); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}

	manager := NewProjectionManager(eventStore, checkpointStore)

	projection := NewTestProjection("test-projection")
	if err := manager.Register(projection); err != nil {
		t.Fatalf("Failed to register projection: %v", err)
	}

	// Запускаем и обрабатываем первые события
	ctx1, cancel1 := context.WithCancel(context.Background())
	if err := manager.Start(ctx1); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	cancel1()
	manager.Stop(ctx)

	// Получаем позицию checkpoint
	position, err := checkpointStore.GetCheckpoint(ctx, "test-projection")
	if err != nil {
		t.Fatalf("Failed to get checkpoint: %v", err)
	}

	initialCount := projection.GetProcessedCount()

	// Создаем новые события
	event3 := events.NewBaseEvent("test.event3", "agg-3")
	if err := eventStore.AppendEvents(ctx, "agg-3", 0, []events.Event{event3}); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}

	// Перезапускаем проекцию - она должна продолжить с checkpoint
	projection2 := NewTestProjection("test-projection")
	if err := manager.Register(projection2); err == nil {
		// Если уже зарегистрирована, удаляем и регистрируем заново
		manager.Register(projection2)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	if err := manager.Start(ctx2); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Проекция должна продолжить с последней позиции
	newPosition, err := checkpointStore.GetCheckpoint(ctx, "test-projection")
	if err != nil {
		t.Fatalf("Failed to get checkpoint: %v", err)
	}

	if newPosition <= position {
		t.Errorf("Expected new position > %d, got %d", position, newPosition)
	}
	
	_ = initialCount
}
