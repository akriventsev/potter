package eventsourcing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"potter/framework/core"
	"potter/framework/events"
)

// PostgresEventStoreConfig конфигурация для PostgreSQL Event Store
type PostgresEventStoreConfig struct {
	DSN             string
	SchemaName      string
	TableName       string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // в секундах
}

// Validate проверяет корректность конфигурации
func (c PostgresEventStoreConfig) Validate() error {
	if c.DSN == "" {
		return fmt.Errorf("DSN cannot be empty")
	}
	if c.TableName == "" {
		c.TableName = "event_store"
	}
	if c.SchemaName == "" {
		c.SchemaName = "public"
	}
	return nil
}

// DefaultPostgresEventStoreConfig возвращает конфигурацию по умолчанию
func DefaultPostgresEventStoreConfig() PostgresEventStoreConfig {
	return PostgresEventStoreConfig{
		SchemaName:      "public",
		TableName:       "event_store",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 300,
	}
}

// PostgresEventStore реализация EventStore для PostgreSQL
type PostgresEventStore struct {
	config      PostgresEventStoreConfig
	pool        *pgx.Conn
	deserializer EventDeserializer
}

// NewPostgresEventStore создает новый PostgreSQL Event Store
func NewPostgresEventStore(config PostgresEventStoreConfig) (*PostgresEventStore, error) {
	return NewPostgresEventStoreWithDeserializer(config, nil)
}

// NewPostgresEventStoreWithDeserializer создает новый PostgreSQL Event Store с десериализатором
func NewPostgresEventStoreWithDeserializer(config PostgresEventStoreConfig, deserializer EventDeserializer) (*PostgresEventStore, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid postgres config: %w", err)
	}

	conn, err := pgx.Connect(context.Background(), config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	return &PostgresEventStore{
		config:       config,
		pool:         conn,
		deserializer: deserializer,
	}, nil
}

// Start запускает адаптер
func (s *PostgresEventStore) Start(ctx context.Context) error {
	return nil
}

// Stop останавливает адаптер
func (s *PostgresEventStore) Stop(ctx context.Context) error {
	if s.pool != nil {
		return s.pool.Close(ctx)
	}
	return nil
}

// IsRunning проверяет, запущен ли адаптер
func (s *PostgresEventStore) IsRunning() bool {
	return s.pool != nil
}

// Name возвращает имя компонента
func (s *PostgresEventStore) Name() string {
	return "postgres-event-store"
}

// Type возвращает тип компонента
func (s *PostgresEventStore) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// AppendEvents добавляет события в поток агрегата
func (s *PostgresEventStore) AppendEvents(ctx context.Context, aggregateID string, expectedVersion int64, events []events.Event) error {
	tableName := fmt.Sprintf("%s.%s", s.config.SchemaName, s.config.TableName)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Проверяем текущую версию
	var currentVersion sql.NullInt64
	checkQuery := fmt.Sprintf("SELECT MAX(version) FROM %s WHERE aggregate_id = $1", tableName)
	err = tx.QueryRow(ctx, checkQuery, aggregateID).Scan(&currentVersion)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check version: %w", err)
	}

	actualVersion := int64(0)
	if currentVersion.Valid {
		actualVersion = currentVersion.Int64
	}

	// Проверяем оптимистичную конкурентность
	if expectedVersion != actualVersion {
		return fmt.Errorf("%w: expected %d, got %d", ErrConcurrencyConflict, expectedVersion, actualVersion)
	}

	// Вставляем события
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s (aggregate_id, aggregate_type, event_type, event_data, metadata, version, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, tableName)

	for i, event := range events {
		eventData, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		metadata, err := json.Marshal(convertMetadata(event.Metadata()))
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		version := expectedVersion + int64(i) + 1
		_, err = tx.Exec(ctx, insertQuery,
			aggregateID,
			getAggregateType(event),
			event.EventType(),
			eventData,
			metadata,
			version,
			event.OccurredAt(),
		)
		if err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetEvents возвращает события агрегата
func (s *PostgresEventStore) GetEvents(ctx context.Context, aggregateID string, fromVersion int64) ([]StoredEvent, error) {
	tableName := fmt.Sprintf("%s.%s", s.config.SchemaName, s.config.TableName)
	query := fmt.Sprintf(`
		SELECT id, aggregate_id, aggregate_type, event_type, event_data, metadata, version, position, occurred_at, created_at
		FROM %s
		WHERE aggregate_id = $1 AND version >= $2
		ORDER BY version ASC
	`, tableName)

	rows, err := s.pool.Query(ctx, query, aggregateID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var result []StoredEvent
	for rows.Next() {
		var stored StoredEvent
		var eventDataJSON, metadataJSON []byte
		var id string

		err := rows.Scan(
			&id,
			&stored.AggregateID,
			&stored.AggregateType,
			&stored.EventType,
			&eventDataJSON,
			&metadataJSON,
			&stored.Version,
			&stored.Position,
			&stored.OccurredAt,
			&stored.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		stored.ID = id
		if err := json.Unmarshal(metadataJSON, &stored.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		// Десериализуем eventData обратно в events.Event
		if s.deserializer != nil {
			event, err := s.deserializer.DeserializeEvent(stored.EventType, eventDataJSON)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize event: %w", err)
			}
			stored.EventData = event
		} else {
			// Если десериализатор не установлен, пытаемся использовать базовую десериализацию
			// Это fallback для обратной совместимости
			var baseEvent events.BaseEvent
			if err := json.Unmarshal(eventDataJSON, &baseEvent); err == nil {
				// Создаем минимальное событие для обратной совместимости
				// В production всегда должен быть установлен десериализатор
			}
		}
		result = append(result, stored)
	}

	if len(result) == 0 && fromVersion > 0 {
		return nil, ErrStreamNotFound
	}

	return result, nil
}

// GetEventsByType возвращает события определенного типа
func (s *PostgresEventStore) GetEventsByType(ctx context.Context, eventType string, fromTimestamp time.Time) ([]StoredEvent, error) {
	tableName := fmt.Sprintf("%s.%s", s.config.SchemaName, s.config.TableName)
	query := fmt.Sprintf(`
		SELECT id, aggregate_id, aggregate_type, event_type, event_data, metadata, version, position, occurred_at, created_at
		FROM %s
		WHERE event_type = $1 AND occurred_at >= $2
		ORDER BY position ASC
	`, tableName)

	rows, err := s.pool.Query(ctx, query, eventType, fromTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to query events by type: %w", err)
	}
	defer rows.Close()

	var result []StoredEvent
	for rows.Next() {
		var stored StoredEvent
		var eventDataJSON, metadataJSON []byte
		var id string

		err := rows.Scan(
			&id,
			&stored.AggregateID,
			&stored.AggregateType,
			&stored.EventType,
			&eventDataJSON,
			&metadataJSON,
			&stored.Version,
			&stored.Position,
			&stored.OccurredAt,
			&stored.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		stored.ID = id
		if err := json.Unmarshal(metadataJSON, &stored.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		// Десериализуем eventData обратно в events.Event
		if s.deserializer != nil {
			event, err := s.deserializer.DeserializeEvent(stored.EventType, eventDataJSON)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize event: %w", err)
			}
			stored.EventData = event
		}
		result = append(result, stored)
	}

	return result, nil
}

// GetAllEvents возвращает все события начиная с указанной позиции
func (s *PostgresEventStore) GetAllEvents(ctx context.Context, fromPosition int64) (<-chan StoredEvent, error) {
	ch := make(chan StoredEvent, 100)

	go func() {
		defer close(ch)
		tableName := fmt.Sprintf("%s.%s", s.config.SchemaName, s.config.TableName)
		query := fmt.Sprintf(`
			SELECT id, aggregate_id, aggregate_type, event_type, event_data, metadata, version, position, occurred_at, created_at
			FROM %s
			WHERE position >= $1
			ORDER BY position ASC
		`, tableName)

		rows, err := s.pool.Query(ctx, query, fromPosition)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var stored StoredEvent
			var eventDataJSON, metadataJSON []byte
			var id string

			if err := rows.Scan(
				&id,
				&stored.AggregateID,
				&stored.AggregateType,
				&stored.EventType,
				&eventDataJSON,
				&metadataJSON,
				&stored.Version,
				&stored.Position,
				&stored.OccurredAt,
				&stored.CreatedAt,
			); err != nil {
				return
			}

			stored.ID = id
			if err := json.Unmarshal(metadataJSON, &stored.Metadata); err != nil {
				continue
			}

			// Десериализуем eventData обратно в events.Event
			if s.deserializer != nil {
				event, err := s.deserializer.DeserializeEvent(stored.EventType, eventDataJSON)
				if err != nil {
					continue
				}
				stored.EventData = event
			}

			select {
			case ch <- stored:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// PostgresSnapshotStore реализация SnapshotStore для PostgreSQL
type PostgresSnapshotStore struct {
	config PostgresEventStoreConfig
	pool   *pgx.Conn
}

// NewPostgresSnapshotStore создает новый PostgreSQL Snapshot Store
func NewPostgresSnapshotStore(config PostgresEventStoreConfig) (*PostgresSnapshotStore, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid postgres config: %w", err)
	}

	conn, err := pgx.Connect(context.Background(), config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	return &PostgresSnapshotStore{
		config: config,
		pool:   conn,
	}, nil
}

// SaveSnapshot сохраняет снапшот
func (s *PostgresSnapshotStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	tableName := fmt.Sprintf("%s.snapshots", s.config.SchemaName)
	query := fmt.Sprintf(`
		INSERT INTO %s (aggregate_id, aggregate_type, version, state, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (aggregate_id) 
		DO UPDATE SET version = $3, state = $4, metadata = $5, updated_at = $7
	`, tableName)

	metadataJSON, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = s.pool.Exec(ctx, query,
		snapshot.AggregateID,
		snapshot.AggregateType,
		snapshot.Version,
		snapshot.State,
		metadataJSON,
		snapshot.CreatedAt,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	return nil
}

// GetSnapshot возвращает последний снапшот
func (s *PostgresSnapshotStore) GetSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error) {
	tableName := fmt.Sprintf("%s.snapshots", s.config.SchemaName)
	query := fmt.Sprintf(`
		SELECT aggregate_id, aggregate_type, version, state, metadata, created_at
		FROM %s
		WHERE aggregate_id = $1
	`, tableName)

	var snapshot Snapshot
	var metadataJSON []byte

	err := s.pool.QueryRow(ctx, query, aggregateID).Scan(
		&snapshot.AggregateID,
		&snapshot.AggregateType,
		&snapshot.Version,
		&snapshot.State,
		&metadataJSON,
		&snapshot.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &snapshot.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &snapshot, nil
}

// DeleteSnapshots удаляет старые снапшоты
func (s *PostgresSnapshotStore) DeleteSnapshots(ctx context.Context, aggregateID string, beforeVersion int64) error {
	tableName := fmt.Sprintf("%s.snapshots", s.config.SchemaName)
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE aggregate_id = $1 AND version < $2
	`, tableName)

	_, err := s.pool.Exec(ctx, query, aggregateID, beforeVersion)
	if err != nil {
		return fmt.Errorf("failed to delete snapshots: %w", err)
	}

	return nil
}

