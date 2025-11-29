// Package migrations предоставляет framework для управления миграциями схемы базы данных.
package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresMigrationDB реализация MigrationDB для PostgreSQL
type PostgresMigrationDB struct {
	pool *pgxpool.Pool
	conn *pgx.Conn
}

// NewPostgresMigrationDB создает новый PostgresMigrationDB
func NewPostgresMigrationDB(dsn string) (*PostgresMigrationDB, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	conn, err := pool.Acquire(context.Background())
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}

	return &PostgresMigrationDB{
		pool: pool,
		conn: conn.Conn(),
	}, nil
}

// Close закрывает соединение
func (p *PostgresMigrationDB) Close() error {
	if p.conn != nil {
		// Connection будет возвращена в pool при закрытии pool
	}
	if p.pool != nil {
		p.pool.Close()
	}
	return nil
}

// NewPostgresMigrationDBFromConn создает PostgresMigrationDB из существующего соединения
func NewPostgresMigrationDBFromConn(conn *pgx.Conn) *PostgresMigrationDB {
	return &PostgresMigrationDB{
		conn: conn,
	}
}

// Exec выполняет SQL команду
func (p *PostgresMigrationDB) Exec(ctx context.Context, query string, args ...interface{}) error {
	if p.conn != nil {
		_, err := p.conn.Exec(ctx, query, args...)
		return err
	}
	_, err := p.pool.Exec(ctx, query, args...)
	return err
}

// Query выполняет SQL запрос
func (p *PostgresMigrationDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	if p.conn != nil {
		rows, err := p.conn.Query(ctx, query, args...)
		return &PostgresRows{rows: rows}, err
	}
	rows, err := p.pool.Query(ctx, query, args...)
	return &PostgresRows{rows: rows}, err
}

// Begin начинает транзакцию
func (p *PostgresMigrationDB) Begin(ctx context.Context) (Tx, error) {
	if p.conn != nil {
		tx, err := p.conn.Begin(ctx)
		if err != nil {
			return nil, err
		}
		return &PostgresTx{tx: tx}, nil
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &PostgresTx{tx: tx}, nil
}

// PostgresRows обертка для pgx.Rows
type PostgresRows struct {
	rows pgx.Rows
}

func (r *PostgresRows) Next() bool {
	return r.rows.Next()
}

func (r *PostgresRows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

func (r *PostgresRows) Close() error {
	r.rows.Close()
	return nil
}

// PostgresTx обертка для pgx.Tx
type PostgresTx struct {
	tx pgx.Tx
}

func (t *PostgresTx) Commit() error {
	return t.tx.Commit(context.Background())
}

func (t *PostgresTx) Rollback() error {
	return t.tx.Rollback(context.Background())
}

func (t *PostgresTx) Exec(ctx context.Context, query string, args ...interface{}) error {
	_, err := t.tx.Exec(ctx, query, args...)
	return err
}

// PostgresMigrationHistory реализация MigrationHistory для PostgreSQL
type PostgresMigrationHistory struct {
	db *PostgresMigrationDB
}

// NewPostgresMigrationHistory создает новый PostgresMigrationHistory
func NewPostgresMigrationHistory(db *PostgresMigrationDB) *PostgresMigrationHistory {
	return &PostgresMigrationHistory{db: db}
}

// EnsureTable создает таблицу schema_migrations
func (h *PostgresMigrationHistory) EnsureTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW(),
			execution_time_ms INTEGER NOT NULL,
			checksum VARCHAR(64) NOT NULL
		);
	`
	return h.db.Exec(ctx, query)
}

// Lock блокирует concurrent migrations через advisory lock
func (h *PostgresMigrationHistory) Lock(ctx context.Context) error {
	query := `SELECT pg_advisory_lock(hashtext('schema_migrations'));`
	return h.db.Exec(ctx, query)
}

// Unlock снимает блокировку
func (h *PostgresMigrationHistory) Unlock(ctx context.Context) error {
	query := `SELECT pg_advisory_unlock(hashtext('schema_migrations'));`
	return h.db.Exec(ctx, query)
}

// GetApplied возвращает список примененных версий
func (h *PostgresMigrationHistory) GetApplied(ctx context.Context) ([]string, error) {
	query := `SELECT version FROM schema_migrations ORDER BY version`
	rows, err := h.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			continue
		}
		versions = append(versions, version)
	}

	return versions, nil
}

// GetAll возвращает все записи истории
func (h *PostgresMigrationHistory) GetAll(ctx context.Context) ([]*MigrationRecord, error) {
	query := `
		SELECT version, name, applied_at, execution_time_ms, checksum
		FROM schema_migrations
		ORDER BY version
	`
	rows, err := h.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*MigrationRecord
	for rows.Next() {
		record := &MigrationRecord{}
		if err := rows.Scan(
			&record.Version,
			&record.Name,
			&record.AppliedAt,
			&record.ExecutionTime,
			&record.Checksum,
		); err != nil {
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

// RecordApplied записывает примененную миграцию
func (h *PostgresMigrationHistory) RecordApplied(ctx context.Context, tx Tx, version, name string, executionTime int64, checksum string) error {
	query := `
		INSERT INTO schema_migrations (version, name, applied_at, execution_time_ms, checksum)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (version) DO NOTHING
	`
	pgxTx, ok := tx.(*PostgresTx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}
	return pgxTx.Exec(ctx, query, version, name, time.Now(), executionTime, checksum)
}

// RecordRollback записывает откат миграции
func (h *PostgresMigrationHistory) RecordRollback(ctx context.Context, tx Tx, version string) error {
	query := `DELETE FROM schema_migrations WHERE version = $1`
	pgxTx, ok := tx.(*PostgresTx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}
	return pgxTx.Exec(ctx, query, version)
}
