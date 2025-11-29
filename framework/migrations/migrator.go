// Package migrations предоставляет framework для управления миграциями схемы базы данных.
package migrations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// Migration интерфейс для миграции
type Migration interface {
	Version() string
	Name() string
	Up(ctx context.Context, db MigrationDB) error
	Down(ctx context.Context, db MigrationDB) error
}

// MigrationDB интерфейс для абстракции database operations
type MigrationDB interface {
	Exec(ctx context.Context, query string, args ...interface{}) error
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	Begin(ctx context.Context) (Tx, error)
}

// Rows интерфейс для результатов запроса
type Rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close() error
}

// Tx интерфейс для транзакции
type Tx interface {
	Commit() error
	Rollback() error
	Exec(ctx context.Context, query string, args ...interface{}) error
}

// MigrationStatus статус миграции
type MigrationStatus struct {
	Version       string
	Name          string
	AppliedAt     *time.Time
	Status        string // "pending", "applied", "failed"
	ExecutionTime int64  // в миллисекундах
	Checksum      string
}

// Migrator управляет миграциями
type Migrator struct {
	db         MigrationDB
	migrations []Migration
	history    MigrationHistoryInterface
}

// MigrationHistoryInterface интерфейс для истории миграций
type MigrationHistoryInterface interface {
	EnsureTable(ctx context.Context) error
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
	GetApplied(ctx context.Context) ([]string, error)
	GetAll(ctx context.Context) ([]*MigrationRecord, error)
	RecordApplied(ctx context.Context, tx Tx, version, name string, executionTime int64, checksum string) error
	RecordRollback(ctx context.Context, tx Tx, version string) error
}

// NewMigrator создает новый Migrator
func NewMigrator(db MigrationDB) *Migrator {
	var history MigrationHistoryInterface

	// Определяем тип адаптера и создаем соответствующую историю
	switch db := db.(type) {
	case *PostgresMigrationDB:
		history = NewPostgresMigrationHistory(db)
	case *MongoMigrationDB:
		history = NewMongoMigrationHistory(db)
	default:
		history = NewMigrationHistory(db)
	}

	return &Migrator{
		db:         db,
		migrations: make([]Migration, 0),
		history:    history,
	}
}

// Register регистрирует миграцию
func (m *Migrator) Register(migration Migration) error {
	// Проверяем дубликаты версий
	for _, existing := range m.migrations {
		if existing.Version() == migration.Version() {
			return fmt.Errorf("migration with version %s already registered", migration.Version())
		}
	}

	m.migrations = append(m.migrations, migration)
	return nil
}

// RegisterFromFiles регистрирует миграции из файлов
func (m *Migrator) RegisterFromFiles(dir string) error {
	source := NewFileMigrationSource(dir)
	migrations, err := source.LoadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	for _, migration := range migrations {
		if err := m.Register(migration); err != nil {
			return fmt.Errorf("failed to register migration %s: %w", migration.Version(), err)
		}
	}

	return nil
}

// Up применяет все pending миграции
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.history.EnsureTable(ctx); err != nil {
		return fmt.Errorf("failed to ensure history table: %w", err)
	}

	// Блокируем concurrent migrations
	if err := m.history.Lock(ctx); err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	defer m.history.Unlock(ctx)

	// Сортируем миграции по версии
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version() < m.migrations[j].Version()
	})

	// Получаем список примененных миграций
	applied, err := m.history.GetApplied(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	appliedMap := make(map[string]bool)
	for _, v := range applied {
		appliedMap[v] = true
	}

	// Применяем pending миграции
	for _, migration := range m.migrations {
		if appliedMap[migration.Version()] {
			continue
		}

		if err := m.applyMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version(), err)
		}
	}

	return nil
}

// Down откатывает N миграций
func (m *Migrator) Down(ctx context.Context, steps int) error {
	if err := m.history.EnsureTable(ctx); err != nil {
		return fmt.Errorf("failed to ensure history table: %w", err)
	}

	if err := m.history.Lock(ctx); err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	defer m.history.Unlock(ctx)

	// Сортируем миграции по версии (обратный порядок)
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version() > m.migrations[j].Version()
	})

	// Получаем список примененных миграций
	applied, err := m.history.GetApplied(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	appliedMap := make(map[string]bool)
	for _, v := range applied {
		appliedMap[v] = true
	}

	// Откатываем миграции
	count := 0
	for _, migration := range m.migrations {
		if !appliedMap[migration.Version()] {
			continue
		}

		if count >= steps {
			break
		}

		if err := m.rollbackMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", migration.Version(), err)
		}

		count++
	}

	return nil
}

// Status возвращает статус всех миграций
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.history.EnsureTable(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure history table: %w", err)
	}

	applied, err := m.history.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get migration history: %w", err)
	}

	appliedMap := make(map[string]*MigrationRecord)
	for _, record := range applied {
		appliedMap[record.Version] = record
	}

	// Сортируем миграции по версии
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version() < m.migrations[j].Version()
	})

	var statuses []MigrationStatus
	for _, migration := range m.migrations {
		status := MigrationStatus{
			Version: migration.Version(),
			Name:    migration.Name(),
			Status:  "pending",
		}

		if record, ok := appliedMap[migration.Version()]; ok {
			status.AppliedAt = &record.AppliedAt
			status.Status = "applied"
			status.ExecutionTime = record.ExecutionTime
			status.Checksum = record.Checksum
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// Version возвращает текущую версию (последнюю примененную)
func (m *Migrator) Version(ctx context.Context) (string, error) {
	if err := m.history.EnsureTable(ctx); err != nil {
		return "", fmt.Errorf("failed to ensure history table: %w", err)
	}

	applied, err := m.history.GetApplied(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get applied migrations: %w", err)
	}

	if len(applied) == 0 {
		return "", nil
	}

	// Сортируем и возвращаем последнюю версию
	sort.Strings(applied)
	return applied[len(applied)-1], nil
}

// applyMigration применяет миграцию
func (m *Migrator) applyMigration(ctx context.Context, migration Migration) error {
	startTime := time.Now()

	// Вычисляем checksum для Up миграции
	upSQL := ""
	if fileMigration, ok := migration.(*FileMigration); ok {
		upSQL = fileMigration.UpSQL
	}
	checksum := calculateChecksum(upSQL)

	// Начинаем транзакцию
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Применяем миграцию
	if err := migration.Up(ctx, &TxMigrationDB{tx: tx}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migration failed: %w", err)
	}

	// Сохраняем в историю
	executionTime := time.Since(startTime).Milliseconds()
	if err := m.history.RecordApplied(ctx, tx, migration.Version(), migration.Name(), executionTime, checksum); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// rollbackMigration откатывает миграцию
func (m *Migrator) rollbackMigration(ctx context.Context, migration Migration) error {
	// Начинаем транзакцию
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Откатываем миграцию
	if err := migration.Down(ctx, &TxMigrationDB{tx: tx}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("rollback failed: %w", err)
	}

	// Удаляем из истории
	if err := m.history.RecordRollback(ctx, tx, migration.Version()); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to record rollback: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// calculateChecksum вычисляет SHA-256 checksum
func calculateChecksum(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// MigrationRecord запись о примененной миграции
type MigrationRecord struct {
	Version       string
	Name          string
	AppliedAt     time.Time
	ExecutionTime int64
	Checksum      string
}

// MigrationHistory управляет историей миграций
type MigrationHistory struct {
	db MigrationDB
}

// NewMigrationHistory создает новый MigrationHistory
func NewMigrationHistory(db MigrationDB) *MigrationHistory {
	return &MigrationHistory{db: db}
}

// EnsureTable создает таблицу истории миграций (должна быть реализована в адаптерах)
func (h *MigrationHistory) EnsureTable(ctx context.Context) error {
	// Должна быть реализована в адаптерах (PostgresMigrationDB, MongoMigrationDB)
	return fmt.Errorf("EnsureTable must be implemented by adapter")
}

// Lock блокирует concurrent migrations (должна быть реализована в адаптерах)
func (h *MigrationHistory) Lock(ctx context.Context) error {
	// Должна быть реализована в адаптерах
	return fmt.Errorf("Lock must be implemented by adapter")
}

// Unlock снимает блокировку (должна быть реализована в адаптерах)
func (h *MigrationHistory) Unlock(ctx context.Context) error {
	// Должна быть реализована в адаптерах
	return fmt.Errorf("Unlock must be implemented by adapter")
}

// GetApplied возвращает список примененных версий
func (h *MigrationHistory) GetApplied(ctx context.Context) ([]string, error) {
	// Должна быть реализована в адаптерах
	return nil, fmt.Errorf("GetApplied must be implemented by adapter")
}

// GetAll возвращает все записи истории
func (h *MigrationHistory) GetAll(ctx context.Context) ([]*MigrationRecord, error) {
	// Должна быть реализована в адаптерах
	return nil, fmt.Errorf("GetAll must be implemented by adapter")
}

// RecordApplied записывает примененную миграцию
func (h *MigrationHistory) RecordApplied(ctx context.Context, tx Tx, version, name string, executionTime int64, checksum string) error {
	// Должна быть реализована в адаптерах
	return fmt.Errorf("RecordApplied must be implemented by adapter")
}

// RecordRollback записывает откат миграции
func (h *MigrationHistory) RecordRollback(ctx context.Context, tx Tx, version string) error {
	// Должна быть реализована в адаптерах
	return fmt.Errorf("RecordRollback must be implemented by adapter")
}

// TxMigrationDB обертка для использования Tx как MigrationDB
type TxMigrationDB struct {
	tx Tx
}

func (t *TxMigrationDB) Exec(ctx context.Context, query string, args ...interface{}) error {
	return t.tx.Exec(ctx, query, args...)
}

func (t *TxMigrationDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	return nil, fmt.Errorf("Query not supported in transaction context")
}

func (t *TxMigrationDB) Begin(ctx context.Context) (Tx, error) {
	return t.tx, nil
}
