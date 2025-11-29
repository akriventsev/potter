package infrastructure

import (
	"fmt"

	"potter/framework/eventsourcing"
	"potter/framework/saga"
)

// NewSagaEventStorePersistence создает EventStore persistence для саг
func NewSagaEventStorePersistence(dsn string) (saga.SagaPersistence, error) {
	eventStoreConfig := eventsourcing.DefaultPostgresEventStoreConfig()
	eventStoreConfig.DSN = dsn

	eventStore, err := eventsourcing.NewPostgresEventStore(eventStoreConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create event store: %w", err)
	}

	snapshotStore, err := eventsourcing.NewPostgresSnapshotStore(eventStoreConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	persistence := saga.NewEventStorePersistence(eventStore, snapshotStore).
		WithSnapshotFrequency(5)

	return persistence, nil
}

// NewPostgresCheckpointStore создает checkpoint store для проекций
func NewPostgresCheckpointStore(dsn string) (eventsourcing.CheckpointStore, error) {
	return eventsourcing.NewPostgresCheckpointStore(dsn)
}

// NewPostgresSagaReadModelStore создает read model store для саг
func NewPostgresSagaReadModelStore(dsn string) (saga.SagaReadModelStore, error) {
	return saga.NewPostgresSagaReadModelStore(dsn)
}

