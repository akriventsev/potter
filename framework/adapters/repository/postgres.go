// Package repository предоставляет generic адаптеры для работы с различными storage backends.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"potter/framework/core"
)

// Mapper интерфейс для преобразования между entity и database rows
type Mapper[T Entity] interface {
	ToRow(entity T) (map[string]interface{}, error)
	FromRow(row map[string]interface{}) (T, error)
}

// PostgresConfig конфигурация для PostgreSQL репозитория
type PostgresConfig struct {
	DSN          string
	TableName    string
	SchemaName   string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLifetime int // в секундах
}

// Validate проверяет корректность конфигурации
func (c PostgresConfig) Validate() error {
	if c.DSN == "" {
		return fmt.Errorf("DSN cannot be empty")
	}
	if c.TableName == "" {
		return fmt.Errorf("TableName cannot be empty")
	}
	if c.MaxOpenConns <= 0 {
		return fmt.Errorf("MaxOpenConns must be greater than 0")
	}
	if c.MaxIdleConns <= 0 {
		return fmt.Errorf("MaxIdleConns must be greater than 0")
	}
	return nil
}

// DefaultPostgresConfig возвращает конфигурацию PostgreSQL по умолчанию
func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		SchemaName:      "public",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 300,
	}
}

// PostgresRepository[T Entity] generic PostgreSQL репозиторий.
// 
// Current implementation provides basic CRUD operations. Advanced features
// (query builder, schema migrations, automatic indexing, TTL) are planned for
// future releases. For complex queries, use raw SQL via GetByID with custom filters.
// 
// См. ROADMAP.md для отслеживания прогресса по планируемым улучшениям.
type PostgresRepository[T Entity] struct {
	config PostgresConfig
	db     *pgx.Conn
	mapper Mapper[T]
}

// NewPostgresRepository создает новый PostgreSQL репозиторий
func NewPostgresRepository[T Entity](config PostgresConfig, mapper Mapper[T]) (*PostgresRepository[T], error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid postgres config: %w", err)
	}

	conn, err := pgx.Connect(context.Background(), config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	repo := &PostgresRepository[T]{
		config: config,
		db:     conn,
		mapper: mapper,
	}

	return repo, nil
}

// Start запускает адаптер (реализация core.Lifecycle)
func (p *PostgresRepository[T]) Start(ctx context.Context) error {
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (p *PostgresRepository[T]) Stop(ctx context.Context) error {
	if p.db != nil {
		return p.db.Close(ctx)
	}
	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (p *PostgresRepository[T]) IsRunning() bool {
	return p.db != nil
}

// Name возвращает имя компонента (реализация core.Component)
func (p *PostgresRepository[T]) Name() string {
	return "postgres-repository"
}

// Type возвращает тип компонента (реализация core.Component)
func (p *PostgresRepository[T]) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// Save сохраняет entity (INSERT/UPDATE)
func (p *PostgresRepository[T]) Save(ctx context.Context, entity T) error {
	row, err := p.mapper.ToRow(entity)
	if err != nil {
		return fmt.Errorf("failed to convert entity to row: %w", err)
	}

	// Простая реализация INSERT ON CONFLICT UPDATE
	tableName := fmt.Sprintf("%s.%s", p.config.SchemaName, p.config.TableName)
	query := fmt.Sprintf(`
		INSERT INTO %s (id, data) 
		VALUES ($1, $2)
		ON CONFLICT (id) 
		DO UPDATE SET data = $2, updated_at = NOW()
	`, tableName)

	id := entity.ID()
	dataJSON, _ := json.Marshal(row)

	_, err = p.db.Exec(ctx, query, id, dataJSON)
	if err != nil {
		return fmt.Errorf("failed to save entity: %w", err)
	}

	return nil
}

// FindByID находит entity по ID
func (p *PostgresRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	var zero T

	tableName := fmt.Sprintf("%s.%s", p.config.SchemaName, p.config.TableName)
	query := fmt.Sprintf("SELECT data FROM %s WHERE id = $1", tableName)

	var dataJSON []byte
	err := p.db.QueryRow(ctx, query, id).Scan(&dataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return zero, fmt.Errorf("entity not found: %s", id)
		}
		return zero, fmt.Errorf("failed to find entity: %w", err)
	}

	var row map[string]interface{}
	if err := json.Unmarshal(dataJSON, &row); err != nil {
		return zero, fmt.Errorf("failed to unmarshal entity: %w", err)
	}

	entity, err := p.mapper.FromRow(row)
	if err != nil {
		return zero, fmt.Errorf("failed to convert row to entity: %w", err)
	}

	return entity, nil
}

// FindAll возвращает все entities
func (p *PostgresRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	tableName := fmt.Sprintf("%s.%s", p.config.SchemaName, p.config.TableName)
	query := fmt.Sprintf("SELECT data FROM %s", tableName)

	rows, err := p.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		var dataJSON []byte
		if err := rows.Scan(&dataJSON); err != nil {
			continue
		}

		var row map[string]interface{}
		if err := json.Unmarshal(dataJSON, &row); err != nil {
			continue
		}

		entity, err := p.mapper.FromRow(row)
		if err != nil {
			continue
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

// Delete удаляет entity
func (p *PostgresRepository[T]) Delete(ctx context.Context, id string) error {
	tableName := fmt.Sprintf("%s.%s", p.config.SchemaName, p.config.TableName)
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", tableName)

	result, err := p.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("entity not found: %s", id)
	}

	return nil
}

