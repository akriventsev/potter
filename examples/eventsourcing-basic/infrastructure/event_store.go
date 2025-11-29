package infrastructure

import (
	"context"
	"log"

	"github.com/akriventsev/potter/framework/eventsourcing"
)

// NewEventStore создает новый Event Store
func NewEventStore(ctx context.Context) (eventsourcing.EventStore, error) {
	// Используем InMemory для простоты примера
	// В production используйте PostgresEventStore
	config := eventsourcing.DefaultInMemoryEventStoreConfig()
	store := eventsourcing.NewInMemoryEventStore(config)
	
	log.Println("Using InMemoryEventStore for example")
	return store, nil
}

// NewPostgresEventStore создает PostgreSQL Event Store (для production)
func NewPostgresEventStore(ctx context.Context) (eventsourcing.EventStore, error) {
	config := eventsourcing.DefaultPostgresEventStoreConfig()
	config.DSN = "postgres://postgres:postgres@localhost:5432/potter?sslmode=disable"
	config.SchemaName = "public"
	config.TableName = "event_store"

	store, err := eventsourcing.NewPostgresEventStore(config)
	if err != nil {
		return nil, err
	}

	if err := store.Start(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

