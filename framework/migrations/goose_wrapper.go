// Package migrations предоставляет обертку над goose для управления миграциями схемы базы данных.
package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
)

// MigrationStatus представляет статус миграции
type MigrationStatus struct {
	Version   int64
	Name      string
	AppliedAt *time.Time
	Status    string // "pending", "applied"
}

// RunMigrations применяет все pending миграции из указанной директории
func RunMigrations(db *sql.DB, dir string) error {
	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// RunMigrationsLimited применяет ограниченное количество pending миграций
func RunMigrationsLimited(db *sql.DB, dir string, steps int64) error {
	if steps <= 0 {
		return RunMigrations(db, dir)
	}

	// Получаем текущую версию БД
	currentVersion, err := GetCurrentVersion(db)
	if err != nil {
		// Если таблица не существует, начинаем с 0
		currentVersion = 0
	}

	// Собираем все доступные миграции
	migrations, err := goose.CollectMigrations(dir, 0, goose.MaxVersion)
	if err != nil {
		return fmt.Errorf("failed to collect migrations: %w", err)
	}

	// Находим следующие pending миграции
	var pendingMigrations []*goose.Migration
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			pendingMigrations = append(pendingMigrations, migration)
		}
	}

	if len(pendingMigrations) == 0 {
		// Нет pending миграций
		return nil
	}

	// Определяем целевую версию (текущая + steps миграций)
	var targetVersion int64
	if int64(len(pendingMigrations)) < steps {
		// Применяем все доступные миграции
		targetVersion = pendingMigrations[len(pendingMigrations)-1].Version
	} else {
		// Применяем только указанное количество
		targetVersion = pendingMigrations[steps-1].Version
	}

	// Применяем миграции до целевой версии
	if err := goose.UpTo(db, dir, targetVersion); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// RollbackMigration откатывает последнюю миграцию
func RollbackMigration(db *sql.DB, dir string) error {
	if err := goose.Down(db, dir); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	return nil
}

// RollbackMigrations откатывает N миграций
func RollbackMigrations(db *sql.DB, dir string, steps int64) error {
	// Получаем текущую версию
	currentVersion, err := GetCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Вычисляем целевую версию
	targetVersion := currentVersion - steps
	if targetVersion < 0 {
		targetVersion = 0
	}

	// Откатываем до целевой версии
	if err := goose.DownTo(db, dir, targetVersion); err != nil {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	return nil
}

// GetMigrationStatus возвращает статус всех миграций
func GetMigrationStatus(db *sql.DB, dir string) ([]MigrationStatus, error) {
	// Получаем список всех миграций из файлов
	migrations, err := goose.CollectMigrations(dir, 0, goose.MaxVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to collect migrations: %w", err)
	}

	// Получаем текущую версию из БД
	currentVersion, err := goose.GetDBVersion(db)
	if err != nil {
		// Если таблица не существует, все миграции pending
		currentVersion = 0
	}

	var statuses []MigrationStatus
	for _, migration := range migrations {
		status := MigrationStatus{
			Version: migration.Version,
			Name:    migration.Source,
			Status:  "pending",
		}

		if migration.Version <= currentVersion {
			// Получаем время применения из таблицы goose_db_version
			var appliedAt time.Time
			err := db.QueryRow(
				"SELECT tstamp FROM goose_db_version WHERE version_id = $1 AND is_applied = true ORDER BY tstamp DESC LIMIT 1",
				migration.Version,
			).Scan(&appliedAt)

			if err == nil {
				status.AppliedAt = &appliedAt
				status.Status = "applied"
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// GetCurrentVersion возвращает текущую версию БД
func GetCurrentVersion(db *sql.DB) (int64, error) {
	version, err := goose.GetDBVersion(db)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}

	return version, nil
}

// CreateMigration создает новый файл миграции
func CreateMigration(dir, name string) error {
	// Создаем директорию если не существует
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Генерируем timestamp для имени файла
	timestamp := time.Now().Format("20060102150405")
	
	// Формируем имя файла в формате goose: YYYYMMDDHHMMSS_name.sql
	filename := fmt.Sprintf("%s_%s.sql", timestamp, name)
	filepath := filepath.Join(dir, filename)

	// Создаем файл с базовым шаблоном
	content := fmt.Sprintf(`-- +goose Up
-- Migration: %s
-- Created: %s

-- Add your migration SQL here


-- +goose Down
-- Rollback migration: %s

-- Add your rollback SQL here

`, name, time.Now().Format("2006-01-02 15:04:05"), name)

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create migration file: %w", err)
	}

	fmt.Printf("Created migration: %s\n", filename)
	return nil
}

// SetDialect устанавливает диалект БД.
// Если dialect пустой, устанавливается значение по умолчанию "postgres".
func SetDialect(dialect string) error {
	if dialect == "" {
		dialect = "postgres"
	}
	return goose.SetDialect(dialect)
}

// SetVersion устанавливает версию миграции в БД без выполнения SQL миграций.
// Это полезно для пометки миграции как примененной или для исправления состояния БД.
func SetVersion(db *sql.DB, version int64) error {
	// Проверяем, что таблица goose_db_version существует, создаем если нет
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS goose_db_version (
			id serial PRIMARY KEY,
			version_id bigint NOT NULL,
			is_applied boolean NOT NULL,
			tstamp timestamp DEFAULT now()
		);
	`); err != nil {
		return fmt.Errorf("failed to ensure goose_db_version table exists: %w", err)
	}

	// Создаем уникальный индекс если не существует
	if _, err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_goose_db_version_version_id 
		ON goose_db_version(version_id);
	`); err != nil {
		return fmt.Errorf("failed to ensure index exists: %w", err)
	}

	// Удаляем старую запись для этой версии (если есть)
	if _, err := db.Exec(`DELETE FROM goose_db_version WHERE version_id = $1`, version); err != nil {
		return fmt.Errorf("failed to delete existing version record: %w", err)
	}

	// Вставляем новую запись
	if _, err := db.Exec(`
		INSERT INTO goose_db_version (version_id, is_applied, tstamp) 
		VALUES ($1, true, now())
	`, version); err != nil {
		return fmt.Errorf("failed to set version: %w", err)
	}

	return nil
}

