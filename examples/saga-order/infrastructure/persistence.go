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
	// Создаем конфигурацию для EventStore
	eventStoreConfig := eventsourcing.DefaultPostgresEventStoreConfig()
	eventStoreConfig.DSN = dsn

	// Создаем EventStore
	eventStore, err := eventsourcing.NewPostgresEventStore(eventStoreConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create event store: %w", err)
	}

	// Создаем SnapshotStore
	snapshotStore, err := eventsourcing.NewPostgresSnapshotStore(eventStoreConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Создаем EventStorePersistence с частотой snapshots каждые 5 шагов
	persistence := saga.NewEventStorePersistence(eventStore, snapshotStore).
		WithSnapshotFrequency(5)

	return persistence, nil
}

// NewPostgresPersistence создает PostgreSQL persistence (альтернативный вариант)
func NewPostgresPersistence(dsn string) (saga.SagaPersistence, error) {
	return saga.NewPostgresPersistence(dsn)
}

// ApplyMigrations применяет миграции для саг
func ApplyMigrations(ctx context.Context, dsn string) error {
	// Открываем соединение с БД
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Проверяем соединение
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Применяем миграции для EventStore и SnapshotStore
	eventStoreSQL := `
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

	if _, err := db.ExecContext(ctx, eventStoreSQL); err != nil {
		return fmt.Errorf("failed to apply event store migrations: %w", err)
	}

	// Применяем миграции для Saga
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
		CREATE INDEX IF NOT EXISTS idx_saga_created ON saga_instances(created_at);
		CREATE INDEX IF NOT EXISTS idx_saga_definition ON saga_instances(definition_name);
		
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
		CREATE INDEX IF NOT EXISTS idx_history_step ON saga_history(step_name);
		CREATE INDEX IF NOT EXISTS idx_history_started ON saga_history(started_at);
		
		CREATE OR REPLACE FUNCTION update_saga_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		DROP TRIGGER IF EXISTS trigger_update_saga_updated_at ON saga_instances;
		CREATE TRIGGER trigger_update_saga_updated_at
			BEFORE UPDATE ON saga_instances
			FOR EACH ROW
			EXECUTE FUNCTION update_saga_updated_at();
	`

	if _, err := db.ExecContext(ctx, sagaSQL); err != nil {
		return fmt.Errorf("failed to apply saga migrations: %w", err)
	}

	return nil
}

// RunMigrations применяет миграции из SQL файлов
func RunMigrations(dsn string) error {
	ctx := context.Background()
	
	// Пытаемся применить миграции через psql, если доступен
	if _, err := exec.LookPath("psql"); err == nil {
		// Получаем путь к файлу миграций
		migrationFile := filepath.Join("migrations", "001_create_saga_tables.sql")
		if _, err := os.Stat(migrationFile); err == nil {
			cmd := exec.Command("psql", dsn, "-f", migrationFile)
			if err := cmd.Run(); err != nil {
				// Если psql не сработал, используем программный способ
				return ApplyMigrations(ctx, dsn)
			}
			return nil
		}
	}
	
	// Используем программный способ
	return ApplyMigrations(ctx, dsn)
}
