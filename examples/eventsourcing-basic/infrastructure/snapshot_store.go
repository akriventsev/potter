package infrastructure

import (
	"context"
	"log"

	"github.com/akriventsev/potter/framework/eventsourcing"
)

// NewSnapshotStore создает новый Snapshot Store
func NewSnapshotStore(ctx context.Context) (eventsourcing.SnapshotStore, error) {
	// Используем InMemory для простоты примера
	// В production используйте PostgresSnapshotStore
	store := eventsourcing.NewInMemorySnapshotStore()
	
	log.Println("Using InMemorySnapshotStore for example")
	return store, nil
}

// NewPostgresSnapshotStore создает PostgreSQL Snapshot Store (для production)
func NewPostgresSnapshotStore(ctx context.Context) (eventsourcing.SnapshotStore, error) {
	config := eventsourcing.DefaultPostgresEventStoreConfig()
	config.DSN = "postgres://postgres:postgres@localhost:5432/potter?sslmode=disable"
	config.SchemaName = "public"

	store, err := eventsourcing.NewPostgresSnapshotStore(config)
	if err != nil {
		return nil, err
	}

	return store, nil
}

