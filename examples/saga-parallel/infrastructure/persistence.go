package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	_ "github.com/jackc/pgx/v5/stdlib"
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

// ApplyMigrations применяет миграции для саг
func ApplyMigrations(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	sagaSQL := `
		CREATE TABLE IF NOT EXISTS saga_instances (
			id UUID PRIMARY KEY,
			definition_name VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			context JSONB NOT NULL DEFAULT '{}',
			correlation_id VARCHAR(255),
			current_step VARCHAR(255),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_saga_status ON saga_instances(status);
		CREATE INDEX IF NOT EXISTS idx_saga_correlation ON saga_instances(correlation_id);
		
		CREATE TABLE IF NOT EXISTS saga_history (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			saga_id UUID NOT NULL REFERENCES saga_instances(id) ON DELETE CASCADE,
			step_name VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			error TEXT,
			retry_attempt INT DEFAULT 0,
			started_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_history_saga ON saga_history(saga_id);
	`

	if _, err := db.ExecContext(ctx, sagaSQL); err != nil {
		return fmt.Errorf("failed to apply saga migrations: %w", err)
	}

	return nil
}

// RunMigrations применяет миграции из SQL файлов
func RunMigrations(dsn string) error {
	ctx := context.Background()
	
	if _, err := exec.LookPath("psql"); err == nil {
		migrationFile := filepath.Join("migrations", "001_create_saga_tables.sql")
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

