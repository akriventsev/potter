package eventsourcing

import (
	"fmt"
	"time"
)

// EventStoreFactory фабрика для создания Event Store адаптеров.
// Доступные адаптеры:
//   - InMemory (для тестов)
//   - Postgres (production-ready)
//   - MongoDB (production-ready)
// Поддержка EventStore DB планируется в будущих версиях при наличии стабильного Go клиента.
type EventStoreFactory struct{}

// NewEventStoreFactory создает новую фабрику Event Store
func NewEventStoreFactory() *EventStoreFactory {
	return &EventStoreFactory{}
}

// CreateInMemory создает InMemory Event Store
func (f *EventStoreFactory) CreateInMemory(config InMemoryEventStoreConfig) *InMemoryEventStore {
	return NewInMemoryEventStore(config)
}

// CreatePostgres создает PostgreSQL Event Store
func (f *EventStoreFactory) CreatePostgres(config PostgresEventStoreConfig) (*PostgresEventStore, error) {
	return NewPostgresEventStore(config)
}

// CreateMongoDB создает MongoDB Event Store
func (f *EventStoreFactory) CreateMongoDB(config MongoDBEventStoreConfig) (*MongoDBEventStore, error) {
	return NewMongoDBEventStore(config)
}

// SnapshotStoreFactory фабрика для создания Snapshot Store адаптеров
type SnapshotStoreFactory struct{}

// NewSnapshotStoreFactory создает новую фабрику Snapshot Store
func NewSnapshotStoreFactory() *SnapshotStoreFactory {
	return &SnapshotStoreFactory{}
}

// CreateInMemory создает InMemory Snapshot Store
func (f *SnapshotStoreFactory) CreateInMemory() *InMemorySnapshotStore {
	return NewInMemorySnapshotStore()
}

// CreatePostgres создает PostgreSQL Snapshot Store
func (f *SnapshotStoreFactory) CreatePostgres(config PostgresEventStoreConfig) (*PostgresSnapshotStore, error) {
	return NewPostgresSnapshotStore(config)
}

// CreateMongoDB создает MongoDB Snapshot Store
func (f *SnapshotStoreFactory) CreateMongoDB(config MongoDBEventStoreConfig) (*MongoDBSnapshotStore, error) {
	return NewMongoDBSnapshotStore(config)
}

// RepositoryFactory фабрика для создания Event Sourced репозиториев
type RepositoryFactory struct{}

// NewRepositoryFactory создает новую фабрику репозиториев
func NewRepositoryFactory() *RepositoryFactory {
	return &RepositoryFactory{}
}

// CreateEventSourcedRepository создает Event Sourced репозиторий
func CreateEventSourcedRepository[T AggregateInterface](
	f *RepositoryFactory,
	eventStore EventStore,
	snapshotStore SnapshotStore,
	config RepositoryConfig,
	factory AggregateFactory[T],
) *EventSourcedRepository[T] {
	return NewEventSourcedRepository[T](eventStore, snapshotStore, config, factory)
}

// EventSourcingBuilder builder для создания настроенного Event Sourcing репозитория
type EventSourcingBuilder struct {
	eventStore       EventStore
	snapshotStore    SnapshotStore
	snapshotStrategy SnapshotStrategy
	serializer       SnapshotSerializer
	config           RepositoryConfig
}

// NewEventSourcingBuilder создает новый builder
func NewEventSourcingBuilder() *EventSourcingBuilder {
	return &EventSourcingBuilder{
		config: DefaultRepositoryConfig(),
	}
}

// WithEventStore устанавливает Event Store
func (b *EventSourcingBuilder) WithEventStore(store EventStore) *EventSourcingBuilder {
	b.eventStore = store
	return b
}

// WithSnapshotStore устанавливает Snapshot Store
func (b *EventSourcingBuilder) WithSnapshotStore(store SnapshotStore) *EventSourcingBuilder {
	b.snapshotStore = store
	return b
}

// WithSnapshotStrategy устанавливает стратегию снапшотов
func (b *EventSourcingBuilder) WithSnapshotStrategy(strategy SnapshotStrategy) *EventSourcingBuilder {
	b.snapshotStrategy = strategy
	b.config.SnapshotStrategy = strategy
	return b
}

// WithSerializer устанавливает сериализатор
func (b *EventSourcingBuilder) WithSerializer(serializer SnapshotSerializer) *EventSourcingBuilder {
	b.serializer = serializer
	b.config.Serializer = serializer
	return b
}

// WithSnapshotFrequency устанавливает частоту создания снапшотов
func (b *EventSourcingBuilder) WithSnapshotFrequency(frequency int) *EventSourcingBuilder {
	b.config.SnapshotFrequency = frequency
	if b.snapshotStrategy == nil {
		b.snapshotStrategy = NewFrequencySnapshotStrategy(int64(frequency))
		b.config.SnapshotStrategy = b.snapshotStrategy
	}
	return b
}

// WithSnapshotsEnabled включает/выключает использование снапшотов
func (b *EventSourcingBuilder) WithSnapshotsEnabled(enabled bool) *EventSourcingBuilder {
	b.config.UseSnapshots = enabled
	return b
}

// Build создает настроенный Event Sourced репозиторий
func Build[T AggregateInterface](b *EventSourcingBuilder, factory AggregateFactory[T]) (*EventSourcedRepository[T], error) {
	if b.eventStore == nil {
		return nil, fmt.Errorf("event store is required")
	}
	if factory == nil {
		return nil, fmt.Errorf("aggregate factory is required")
	}

	return NewEventSourcedRepository[T](b.eventStore, b.snapshotStore, b.config, factory), nil
}

// FrequencyStrategy создает стратегию по частоте (helper функция)
func FrequencyStrategy(frequency int64) SnapshotStrategy {
	return NewFrequencySnapshotStrategy(frequency)
}

// TimeBasedStrategy создает стратегию по времени (helper функция)
func TimeBasedStrategy(interval interface{}) SnapshotStrategy {
	// Принимаем time.Duration или int (секунды)
	switch v := interval.(type) {
	case int:
		return NewTimeBasedSnapshotStrategy(time.Duration(v) * time.Second)
	case time.Duration:
		return NewTimeBasedSnapshotStrategy(v)
	default:
		return NewTimeBasedSnapshotStrategy(1 * time.Hour)
	}
}

