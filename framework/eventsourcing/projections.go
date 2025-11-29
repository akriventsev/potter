// Package eventsourcing предоставляет полную поддержку Event Sourcing паттерна.
package eventsourcing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Projection интерфейс для проекций
type Projection interface {
	Name() string
	HandleEvent(ctx context.Context, event StoredEvent) error
	Reset(ctx context.Context) error
}

// ProjectionStatus статус проекции
type ProjectionStatus struct {
	Name                string
	State               string // "running", "stopped", "rebuilding", "failed"
	LastProcessedPosition int64
	LastProcessedAt     time.Time
	EventsProcessed      int64
	ErrorCount          int64
	Progress            float64 // для rebuild, 0-100
}

// ProjectionManager управляет проекциями
type ProjectionManager struct {
	eventStore      EventStore
	checkpointStore CheckpointStore
	projections     map[string]Projection
	runners         map[string]*ProjectionRunner
	mu              sync.RWMutex
}

// NewProjectionManager создает новый ProjectionManager
func NewProjectionManager(eventStore EventStore, checkpointStore CheckpointStore) *ProjectionManager {
	return &ProjectionManager{
		eventStore:      eventStore,
		checkpointStore: checkpointStore,
		projections:     make(map[string]Projection),
		runners:         make(map[string]*ProjectionRunner),
	}
}

// Register регистрирует проекцию
func (m *ProjectionManager) Register(projection Projection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := projection.Name()
	if _, exists := m.projections[name]; exists {
		return fmt.Errorf("projection %s already registered", name)
	}

	m.projections[name] = projection
	return nil
}

// Start запускает все проекции
func (m *ProjectionManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, projection := range m.projections {
		runner := NewProjectionRunner(projection, m.eventStore, m.checkpointStore)
		m.runners[name] = runner

		go func(r *ProjectionRunner) {
			if err := r.Run(ctx); err != nil {
				// Логируем ошибку
				fmt.Printf("Projection %s failed: %v\n", name, err)
			}
		}(runner)
	}

	return nil
}

// Stop останавливает все проекции
func (m *ProjectionManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, runner := range m.runners {
		if err := runner.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop projection %s: %w", name, err)
		}
		delete(m.runners, name)
	}

	return nil
}

// Rebuild пересоздает проекцию
func (m *ProjectionManager) Rebuild(ctx context.Context, projectionName string) error {
	m.mu.Lock()
	projection, exists := m.projections[projectionName]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("projection %s not found", projectionName)
	}

	// Сбрасываем состояние проекции
	if err := projection.Reset(ctx); err != nil {
		return fmt.Errorf("failed to reset projection: %w", err)
	}

	// Удаляем checkpoint
	if err := m.checkpointStore.DeleteCheckpoint(ctx, projectionName); err != nil {
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}

	// Запускаем rebuild
	runner := NewProjectionRunner(projection, m.eventStore, m.checkpointStore)
	return runner.Rebuild(ctx)
}

// GetStatus возвращает статус проекции
func (m *ProjectionManager) GetStatus(projectionName string) (*ProjectionStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runner, exists := m.runners[projectionName]
	if !exists {
		return nil, fmt.Errorf("projection %s not running", projectionName)
	}

	return runner.GetStatus(), nil
}

// ProjectionRunner выполняет проекцию
type ProjectionRunner struct {
	projection      Projection
	eventStore      EventStore
	checkpointStore CheckpointStore
	status          *ProjectionStatus
	mu              sync.RWMutex
	stopChan        chan struct{}
}

// NewProjectionRunner создает новый ProjectionRunner
func NewProjectionRunner(projection Projection, eventStore EventStore, checkpointStore CheckpointStore) *ProjectionRunner {
	return &ProjectionRunner{
		projection:      projection,
		eventStore:      eventStore,
		checkpointStore: checkpointStore,
		status: &ProjectionStatus{
			Name:   projection.Name(),
			State:  "stopped",
			EventsProcessed: 0,
			ErrorCount: 0,
		},
		stopChan: make(chan struct{}),
	}
}

// Run запускает проекцию
func (r *ProjectionRunner) Run(ctx context.Context) error {
	r.mu.Lock()
	r.status.State = "running"
	r.status.LastProcessedAt = time.Now()
	r.mu.Unlock()

	// Получаем последнюю позицию
	position, err := r.checkpointStore.GetCheckpoint(ctx, r.projection.Name())
	if err != nil {
		position = 0
	}

	// Получаем события начиная с позиции
	eventsChan, err := r.eventStore.GetAllEvents(ctx, position)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.stopChan:
			return nil
		case event, ok := <-eventsChan:
			if !ok {
				// Канал закрыт, пересоздаем поток с последней позиции
				position, err := r.checkpointStore.GetCheckpoint(ctx, r.projection.Name())
				if err != nil {
					position = r.status.LastProcessedPosition
				}
				// Пересоздаем канал событий
				newChan, err := r.eventStore.GetAllEvents(ctx, position)
				if err != nil {
					time.Sleep(1 * time.Second)
					continue
				}
				eventsChan = newChan
				continue
			}

			// Обрабатываем событие
			if err := r.projection.HandleEvent(ctx, event); err != nil {
				r.mu.Lock()
				r.status.ErrorCount++
				r.mu.Unlock()
				// Продолжаем обработку несмотря на ошибку
				continue
			}

			// Сохраняем checkpoint
			if err := r.checkpointStore.SaveCheckpoint(ctx, r.projection.Name(), event.Position); err != nil {
				// Логируем ошибку, но продолжаем
				continue
			}

			r.mu.Lock()
			r.status.LastProcessedPosition = event.Position
			r.status.LastProcessedAt = time.Now()
			r.status.EventsProcessed++
			r.mu.Unlock()
		}
	}
}

// Stop останавливает проекцию
func (r *ProjectionRunner) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.status.State = "stopped"
	close(r.stopChan)
	return nil
}

// Rebuild пересоздает проекцию
func (r *ProjectionRunner) Rebuild(ctx context.Context) error {
	r.mu.Lock()
	r.status.State = "rebuilding"
	r.status.Progress = 0
	r.mu.Unlock()

	// Получаем все события с начала
	eventsChan, err := r.eventStore.GetAllEvents(ctx, 0)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	var totalEvents int64
	var processedEvents int64

	// Сначала считаем общее количество событий (если возможно)
	// В реальной реализации это может потребовать дополнительного запроса

	for event := range eventsChan {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.stopChan:
			return nil
		default:
		}

		if err := r.projection.HandleEvent(ctx, event); err != nil {
			r.mu.Lock()
			r.status.ErrorCount++
			r.mu.Unlock()
			continue
		}

		if err := r.checkpointStore.SaveCheckpoint(ctx, r.projection.Name(), event.Position); err != nil {
			continue
		}

		processedEvents++
		totalEvents++

		r.mu.Lock()
		r.status.LastProcessedPosition = event.Position
		r.status.LastProcessedAt = time.Now()
		r.status.EventsProcessed = processedEvents
		if totalEvents > 0 {
			r.status.Progress = float64(processedEvents) / float64(totalEvents) * 100
		}
		r.mu.Unlock()
	}

	r.mu.Lock()
	r.status.State = "running"
	r.status.Progress = 100
	r.mu.Unlock()

	return nil
}

// GetStatus возвращает статус проекции
func (r *ProjectionRunner) GetStatus() *ProjectionStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := *r.status
	return &status
}

// ProjectionBuilder builder для создания проекций
type ProjectionBuilder struct {
	name            string
	eventHandlers   map[string]func(context.Context, StoredEvent) error
	checkpointStore CheckpointStore
	batchSize       int
}

// NewProjectionBuilder создает новый ProjectionBuilder
func NewProjectionBuilder(name string) *ProjectionBuilder {
	return &ProjectionBuilder{
		name:          name,
		eventHandlers: make(map[string]func(context.Context, StoredEvent) error),
		batchSize:     100,
	}
}

// OnEvent регистрирует обработчик события
func (b *ProjectionBuilder) OnEvent(eventType string, handler func(context.Context, StoredEvent) error) *ProjectionBuilder {
	b.eventHandlers[eventType] = handler
	return b
}

// WithCheckpointStore устанавливает checkpoint store
func (b *ProjectionBuilder) WithCheckpointStore(store CheckpointStore) *ProjectionBuilder {
	b.checkpointStore = store
	return b
}

// WithBatchSize устанавливает размер батча
func (b *ProjectionBuilder) WithBatchSize(size int) *ProjectionBuilder {
	b.batchSize = size
	return b
}

// Build создает проекцию
func (b *ProjectionBuilder) Build() Projection {
	return &BuilderProjection{
		name:          b.name,
		eventHandlers: b.eventHandlers,
	}
}

// BuilderProjection проекция созданная через builder
type BuilderProjection struct {
	name          string
	eventHandlers map[string]func(context.Context, StoredEvent) error
}

func (p *BuilderProjection) Name() string {
	return p.name
}

func (p *BuilderProjection) HandleEvent(ctx context.Context, event StoredEvent) error {
	handler, exists := p.eventHandlers[event.EventType]
	if !exists {
		return nil // Игнорируем неизвестные события
	}
	return handler(ctx, event)
}

func (p *BuilderProjection) Reset(ctx context.Context) error {
	// Для builder проекций reset не требуется
	return nil
}

