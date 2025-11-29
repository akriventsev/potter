// Package migrations предоставляет framework для управления миграциями схемы базы данных.
package migrations

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileMigrationSource загружает миграции из файлов
type FileMigrationSource struct {
	dir string
}

// NewFileMigrationSource создает новый FileMigrationSource
func NewFileMigrationSource(dir string) *FileMigrationSource {
	return &FileMigrationSource{dir: dir}
}

// LoadMigrations загружает миграции из директории
func (s *FileMigrationSource) LoadMigrations() ([]Migration, error) {
	if _, err := os.Stat(s.dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("migrations directory does not exist: %s", s.dir)
	}

	migrationMap := make(map[string]*FileMigration)

	err := filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		filename := d.Name()
		if !strings.HasSuffix(filename, ".up.sql") && !strings.HasSuffix(filename, ".down.sql") {
			return nil
		}

		// Извлекаем версию и имя из имени файла
		// Формат: {version}_{name}.up.sql или {version}_{name}.down.sql
		baseName := strings.TrimSuffix(filename, ".up.sql")
		baseName = strings.TrimSuffix(baseName, ".down.sql")

		parts := strings.SplitN(baseName, "_", 2)
		if len(parts) < 2 {
			return fmt.Errorf("invalid migration filename format: %s (expected: {version}_{name}.up.sql)", filename)
		}

		version := parts[0]
		name := parts[1]

		migration, exists := migrationMap[version]
		if !exists {
			migration = &FileMigration{
				version: version,
				name:    name,
			}
			migrationMap[version] = migration
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		if strings.HasSuffix(filename, ".up.sql") {
			migration.UpSQL = string(content)
		} else if strings.HasSuffix(filename, ".down.sql") {
			migration.DownSQL = string(content)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk migrations directory: %w", err)
	}

	// Проверяем, что все миграции имеют up и down файлы
	var migrations []Migration
	for version, migration := range migrationMap {
		if migration.UpSQL == "" {
			return nil, fmt.Errorf("migration %s is missing .up.sql file", version)
		}
		if migration.DownSQL == "" {
			return nil, fmt.Errorf("migration %s is missing .down.sql file", version)
		}
		migrations = append(migrations, migration)
	}

	// Сортируем по версии
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version() < migrations[j].Version()
	})

	return migrations, nil
}

// FileMigration миграция из файла
type FileMigration struct {
	version string
	name    string
	UpSQL   string
	DownSQL string
}

func (f *FileMigration) Version() string {
	return f.version
}

func (f *FileMigration) Name() string {
	return f.name
}

func (f *FileMigration) Up(ctx context.Context, db MigrationDB) error {
	// Разделяем SQL на отдельные statements
	statements := splitSQL(f.UpSQL)
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if err := db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute up migration: %w", err)
		}
	}
	return nil
}

func (f *FileMigration) Down(ctx context.Context, db MigrationDB) error {
	// Разделяем SQL на отдельные statements
	statements := splitSQL(f.DownSQL)
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if err := db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute down migration: %w", err)
		}
	}
	return nil
}

// splitSQL разделяет SQL на отдельные statements
func splitSQL(sql string) []string {
	// Удаляем комментарии
	sql = removeComments(sql)

	// Разделяем по точке с запятой
	parts := strings.Split(sql, ";")
	var statements []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			statements = append(statements, trimmed)
		}
	}
	return statements
}

// removeComments удаляет комментарии из SQL
func removeComments(sql string) string {
	// Удаляем однострочные комментарии (--)
	lines := strings.Split(sql, "\n")
	var cleaned []string
	for _, line := range lines {
		if idx := strings.Index(line, "--"); idx != -1 {
			line = line[:idx]
		}
		cleaned = append(cleaned, line)
	}
	sql = strings.Join(cleaned, "\n")

	// Удаляем многострочные комментарии (/* */)
	for {
		start := strings.Index(sql, "/*")
		if start == -1 {
			break
		}
		end := strings.Index(sql[start:], "*/")
		if end == -1 {
			break
		}
		sql = sql[:start] + sql[start+end+2:]
	}

	return sql
}
