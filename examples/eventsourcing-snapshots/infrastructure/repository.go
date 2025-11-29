package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/akriventsev/potter/examples/eventsourcing-snapshots/domain"
	"github.com/akriventsev/potter/framework/eventsourcing"
)

// NewProductRepositoryWithFrequencyStrategy создает репозиторий с FrequencySnapshotStrategy
func NewProductRepositoryWithFrequencyStrategy(
	eventStore eventsourcing.EventStore,
	snapshotStore eventsourcing.SnapshotStore,
	frequency int64,
) *eventsourcing.EventSourcedRepository[*domain.Product] {
	config := eventsourcing.DefaultRepositoryConfig()
	config.UseSnapshots = true
	config.SnapshotFrequency = int(frequency)
	config.SnapshotStrategy = eventsourcing.NewFrequencySnapshotStrategy(frequency)

	return eventsourcing.NewEventSourcedRepository[*domain.Product](
		eventStore,
		snapshotStore,
		config,
		func(id string) *domain.Product {
			product := &domain.Product{
				EventSourcedAggregate: eventsourcing.NewEventSourcedAggregate(id),
			}
			product.SetApplier(product)
			return product
		},
	)
}

// NewProductRepositoryWithTimeBasedStrategy создает репозиторий с TimeBasedSnapshotStrategy
func NewProductRepositoryWithTimeBasedStrategy(
	eventStore eventsourcing.EventStore,
	snapshotStore eventsourcing.SnapshotStore,
	interval time.Duration,
) *eventsourcing.EventSourcedRepository[*domain.Product] {
	config := eventsourcing.DefaultRepositoryConfig()
	config.UseSnapshots = true
	config.SnapshotStrategy = eventsourcing.NewTimeBasedSnapshotStrategy(interval)

	return eventsourcing.NewEventSourcedRepository[*domain.Product](
		eventStore,
		snapshotStore,
		config,
		func(id string) *domain.Product {
			product := &domain.Product{
				EventSourcedAggregate: eventsourcing.NewEventSourcedAggregate(id),
			}
			product.SetApplier(product)
			return product
		},
	)
}

// NewProductRepositoryWithHybridStrategy создает репозиторий с HybridSnapshotStrategy
func NewProductRepositoryWithHybridStrategy(
	eventStore eventsourcing.EventStore,
	snapshotStore eventsourcing.SnapshotStore,
	frequency int64,
	interval time.Duration,
) *eventsourcing.EventSourcedRepository[*domain.Product] {
	config := eventsourcing.DefaultRepositoryConfig()
	config.UseSnapshots = true
	config.SnapshotFrequency = int(frequency)
	config.SnapshotStrategy = eventsourcing.NewHybridSnapshotStrategy(frequency, interval)

	return eventsourcing.NewEventSourcedRepository[*domain.Product](
		eventStore,
		snapshotStore,
		config,
		func(id string) *domain.Product {
			product := &domain.Product{
				EventSourcedAggregate: eventsourcing.NewEventSourcedAggregate(id),
			}
			product.SetApplier(product)
			return product
		},
	)
}

// RunMigrations применяет миграции из SQL файлов
func RunMigrations(dsn string) error {
	ctx := context.Background()
	
	if _, err := exec.LookPath("psql"); err == nil {
		migrationFile := filepath.Join("migrations", "001_create_event_store.sql")
		if _, err := os.Stat(migrationFile); err == nil {
			cmd := exec.Command("psql", dsn, "-f", migrationFile)
			if err := cmd.Run(); err != nil {
				return ApplyMigrations(ctx, dsn)
			}
			return nil
		}
	}
	
	return ApplyMigrations(ctx, dsn)
}

// ApplyMigrations применяет миграции программно
func ApplyMigrations(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	migrationSQL := `
		CREATE SCHEMA IF NOT EXISTS public;

		CREATE TABLE IF NOT EXISTS public.event_store (
			id VARCHAR(255) PRIMARY KEY,
			aggregate_id VARCHAR(255) NOT NULL,
			aggregate_type VARCHAR(255) NOT NULL,
			event_type VARCHAR(255) NOT NULL,
			event_data JSONB NOT NULL,
			metadata JSONB,
			version BIGINT NOT NULL,
			position BIGSERIAL,
			occurred_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(aggregate_id, version)
		);

		CREATE INDEX IF NOT EXISTS idx_event_store_aggregate_id ON public.event_store(aggregate_id);
		CREATE INDEX IF NOT EXISTS idx_event_store_event_type ON public.event_store(event_type);
		CREATE INDEX IF NOT EXISTS idx_event_store_occurred_at ON public.event_store(occurred_at);
		CREATE INDEX IF NOT EXISTS idx_event_store_position ON public.event_store(position);

		CREATE TABLE IF NOT EXISTS public.snapshots (
			aggregate_id VARCHAR(255) PRIMARY KEY,
			aggregate_type VARCHAR(255) NOT NULL,
			version BIGINT NOT NULL,
			state BYTEA NOT NULL,
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_snapshots_aggregate_id ON public.snapshots(aggregate_id);
	`

	if _, err := db.ExecContext(ctx, migrationSQL); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

