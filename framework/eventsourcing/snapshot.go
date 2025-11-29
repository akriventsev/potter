package eventsourcing

import (
	"context"
	"encoding/json"
	"time"
)

// Snapshot представляет снапшот состояния агрегата
type Snapshot struct {
	AggregateID  string
	AggregateType string
	Version      int64
	State        []byte
	Metadata     map[string]interface{}
	CreatedAt    time.Time
}

// SnapshotStore интерфейс для хранения снапшотов
type SnapshotStore interface {
	// SaveSnapshot сохраняет снапшот агрегата
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error

	// GetSnapshot возвращает последний снапшот агрегата
	GetSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error)

	// DeleteSnapshots удаляет старые снапшоты до указанной версии
	DeleteSnapshots(ctx context.Context, aggregateID string, beforeVersion int64) error
}

// SnapshotSerializer интерфейс для сериализации состояния агрегата
type SnapshotSerializer interface {
	// Serialize сериализует агрегат в байты
	Serialize(aggregate interface{}) ([]byte, error)

	// Deserialize десериализует байты обратно в агрегат
	Deserialize(data []byte, aggregate interface{}) error
}

// JSONSnapshotSerializer реализация SnapshotSerializer с использованием JSON
type JSONSnapshotSerializer struct{}

// NewJSONSnapshotSerializer создает новый JSON сериализатор
func NewJSONSnapshotSerializer() *JSONSnapshotSerializer {
	return &JSONSnapshotSerializer{}
}

// Serialize сериализует агрегат в JSON
func (s *JSONSnapshotSerializer) Serialize(aggregate interface{}) ([]byte, error) {
	return json.Marshal(aggregate)
}

// Deserialize десериализует JSON обратно в агрегат
func (s *JSONSnapshotSerializer) Deserialize(data []byte, aggregate interface{}) error {
	return json.Unmarshal(data, aggregate)
}

// SnapshotStrategy интерфейс для стратегий создания снапшотов
type SnapshotStrategy interface {
	// ShouldCreateSnapshot определяет, нужно ли создать снапшот
	ShouldCreateSnapshot(aggregate AggregateInterface, eventCount int64) bool
}

// FrequencySnapshotStrategy создает снапшот каждые N событий
type FrequencySnapshotStrategy struct {
	Frequency int64
}

// NewFrequencySnapshotStrategy создает стратегию по частоте
func NewFrequencySnapshotStrategy(frequency int64) *FrequencySnapshotStrategy {
	return &FrequencySnapshotStrategy{
		Frequency: frequency,
	}
}

// ShouldCreateSnapshot проверяет, нужно ли создать снапшот
func (s *FrequencySnapshotStrategy) ShouldCreateSnapshot(aggregate AggregateInterface, eventCount int64) bool {
	if s.Frequency <= 0 {
		return false
	}
	return eventCount > 0 && eventCount%s.Frequency == 0
}

// TimeBasedSnapshotStrategy создает снапшот по времени
type TimeBasedSnapshotStrategy struct {
	Interval time.Duration
	lastSnapshot time.Time
}

// NewTimeBasedSnapshotStrategy создает стратегию по времени
func NewTimeBasedSnapshotStrategy(interval time.Duration) *TimeBasedSnapshotStrategy {
	return &TimeBasedSnapshotStrategy{
		Interval: interval,
		lastSnapshot: time.Now(),
	}
}

// ShouldCreateSnapshot проверяет, нужно ли создать снапшот
func (s *TimeBasedSnapshotStrategy) ShouldCreateSnapshot(aggregate AggregateInterface, eventCount int64) bool {
	if s.Interval <= 0 {
		return false
	}
	now := time.Now()
	if now.Sub(s.lastSnapshot) >= s.Interval {
		s.lastSnapshot = now
		return true
	}
	return false
}

// HybridSnapshotStrategy комбинирует частоту и время
type HybridSnapshotStrategy struct {
	FrequencyStrategy *FrequencySnapshotStrategy
	TimeStrategy      *TimeBasedSnapshotStrategy
}

// NewHybridSnapshotStrategy создает гибридную стратегию
func NewHybridSnapshotStrategy(frequency int64, interval time.Duration) *HybridSnapshotStrategy {
	return &HybridSnapshotStrategy{
		FrequencyStrategy: NewFrequencySnapshotStrategy(frequency),
		TimeStrategy:      NewTimeBasedSnapshotStrategy(interval),
	}
}

// ShouldCreateSnapshot проверяет, нужно ли создать снапшот
func (s *HybridSnapshotStrategy) ShouldCreateSnapshot(aggregate AggregateInterface, eventCount int64) bool {
	return s.FrequencyStrategy.ShouldCreateSnapshot(aggregate, eventCount) ||
		s.TimeStrategy.ShouldCreateSnapshot(aggregate, eventCount)
}

