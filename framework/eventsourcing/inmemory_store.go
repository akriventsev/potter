package eventsourcing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"potter/framework/events"
)

// InMemoryEventStoreConfig конфигурация для InMemory Event Store
type InMemoryEventStoreConfig struct {
	MaxEventsPerStream int64
}

// DefaultInMemoryEventStoreConfig возвращает конфигурацию по умолчанию
func DefaultInMemoryEventStoreConfig() InMemoryEventStoreConfig {
	return InMemoryEventStoreConfig{
		MaxEventsPerStream: 10000,
	}
}

// InMemoryEventStore реализация EventStore в памяти для тестирования и разработки
type InMemoryEventStore struct {
	mu          sync.RWMutex
	streams     map[string][]StoredEvent
	allEvents   []StoredEvent
	position    int64
	config      InMemoryEventStoreConfig
}

// NewInMemoryEventStore создает новый InMemory Event Store
func NewInMemoryEventStore(config InMemoryEventStoreConfig) *InMemoryEventStore {
	return &InMemoryEventStore{
		streams:   make(map[string][]StoredEvent),
		allEvents: make([]StoredEvent, 0),
		position:  0,
		config:    config,
	}
}

// AppendEvents добавляет события в поток агрегата
func (s *InMemoryEventStore) AppendEvents(ctx context.Context, aggregateID string, expectedVersion int64, events []events.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Получаем текущий поток
	stream, exists := s.streams[aggregateID]
	currentVersion := int64(0)
	if exists {
		if len(stream) > 0 {
			currentVersion = stream[len(stream)-1].Version
		}
	}

	// Проверяем версию для оптимистичной конкурентности
	if expectedVersion != currentVersion {
		return fmt.Errorf("%w: expected %d, got %d", ErrConcurrencyConflict, expectedVersion, currentVersion)
	}

	// Проверяем лимит MaxEventsPerStream
	if s.config.MaxEventsPerStream > 0 {
		newEventCount := int64(len(stream)) + int64(len(events))
		if newEventCount > s.config.MaxEventsPerStream {
			return fmt.Errorf("max events per stream exceeded: %d (limit: %d)", newEventCount, s.config.MaxEventsPerStream)
		}
	}

	// Добавляем события
	for i, event := range events {
		s.position++
		storedEvent := StoredEvent{
			ID:           event.EventID(),
			AggregateID:  aggregateID,
			AggregateType: getAggregateType(event),
			EventType:    event.EventType(),
			EventData:    event,
			Metadata:     convertMetadata(event.Metadata()),
			Version:      expectedVersion + int64(i) + 1,
			Position:     s.position,
			OccurredAt:   event.OccurredAt(),
			CreatedAt:    time.Now(),
		}
		stream = append(stream, storedEvent)
		s.allEvents = append(s.allEvents, storedEvent)
	}

	s.streams[aggregateID] = stream
	return nil
}

// GetEvents возвращает события агрегата начиная с указанной версии
func (s *InMemoryEventStore) GetEvents(ctx context.Context, aggregateID string, fromVersion int64) ([]StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stream, exists := s.streams[aggregateID]
	if !exists {
		return nil, ErrStreamNotFound
	}

	var result []StoredEvent
	for _, event := range stream {
		if event.Version >= fromVersion {
			result = append(result, event)
		}
	}

	return result, nil
}

// GetEventsByType возвращает события определенного типа
func (s *InMemoryEventStore) GetEventsByType(ctx context.Context, eventType string, fromTimestamp time.Time) ([]StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []StoredEvent
	for _, event := range s.allEvents {
		if event.EventType == eventType && event.OccurredAt.After(fromTimestamp) {
			result = append(result, event)
		}
	}

	return result, nil
}

// GetAllEvents возвращает все события начиная с указанной позиции
func (s *InMemoryEventStore) GetAllEvents(ctx context.Context, fromPosition int64) (<-chan StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch := make(chan StoredEvent, 100)

	go func() {
		defer close(ch)
		s.mu.RLock()
		defer s.mu.RUnlock()

		for _, event := range s.allEvents {
			if event.Position >= fromPosition {
				select {
				case ch <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// Clear очищает все события (для тестов)
func (s *InMemoryEventStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streams = make(map[string][]StoredEvent)
	s.allEvents = make([]StoredEvent, 0)
	s.position = 0
}

// InMemorySnapshotStore реализация SnapshotStore в памяти
type InMemorySnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[string]*Snapshot
}

// NewInMemorySnapshotStore создает новый InMemory Snapshot Store
func NewInMemorySnapshotStore() *InMemorySnapshotStore {
	return &InMemorySnapshotStore{
		snapshots: make(map[string]*Snapshot),
	}
}

// SaveSnapshot сохраняет снапшот
func (s *InMemorySnapshotStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.AggregateID] = &snapshot
	return nil
}

// GetSnapshot возвращает последний снапшот
func (s *InMemorySnapshotStore) GetSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, exists := s.snapshots[aggregateID]
	if !exists {
		return nil, nil
	}
	return snapshot, nil
}

// DeleteSnapshots удаляет старые снапшоты
func (s *InMemorySnapshotStore) DeleteSnapshots(ctx context.Context, aggregateID string, beforeVersion int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, exists := s.snapshots[aggregateID]
	if exists && snapshot.Version < beforeVersion {
		delete(s.snapshots, aggregateID)
	}
	return nil
}

// Clear очищает все снапшоты (для тестов)
func (s *InMemorySnapshotStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = make(map[string]*Snapshot)
}

// Вспомогательные функции
func getAggregateType(event events.Event) string {
	// Можно извлечь из метаданных или использовать рефлексию
	if metadata := event.Metadata(); metadata != nil {
		if aggType, ok := metadata.Get("aggregate_type"); ok {
			if str, ok := aggType.(string); ok {
				return str
			}
		}
	}
	return "unknown"
}

func convertMetadata(metadata events.EventMetadata) map[string]interface{} {
	result := make(map[string]interface{})
	if metadata != nil {
		// Простое преобразование, в реальности может быть сложнее
		for k, v := range metadata {
			result[k] = v
		}
	}
	return result
}

