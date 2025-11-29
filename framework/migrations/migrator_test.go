package migrations

import (
	"context"
	"testing"
)

// TestMigration тестовая миграция
type TestMigration struct {
	version string
	name    string
	upSQL   string
	downSQL string
}

func (m *TestMigration) Version() string {
	return m.version
}

func (m *TestMigration) Name() string {
	return m.name
}

func (m *TestMigration) Up(ctx context.Context, db MigrationDB) error {
	if m.upSQL != "" {
		return db.Exec(ctx, m.upSQL)
	}
	return nil
}

func (m *TestMigration) Down(ctx context.Context, db MigrationDB) error {
	if m.downSQL != "" {
		return db.Exec(ctx, m.downSQL)
	}
	return nil
}

func TestMigrator_Register_DuplicateVersion(t *testing.T) {
	migrator := NewMigrator(nil) // nil DB для unit теста

	migration1 := &TestMigration{version: "001", name: "test1"}
	migration2 := &TestMigration{version: "001", name: "test2"}

	if err := migrator.Register(migration1); err != nil {
		t.Fatalf("Failed to register first migration: %v", err)
	}

	// Попытка зарегистрировать миграцию с тем же version должна вернуть ошибку
	err := migrator.Register(migration2)
	if err == nil {
		t.Error("Expected error when registering duplicate version")
	}
}

func TestMigrator_Register_Success(t *testing.T) {
	migrator := NewMigrator(nil)

	migration1 := &TestMigration{version: "001", name: "test1"}
	migration2 := &TestMigration{version: "002", name: "test2"}

	if err := migrator.Register(migration1); err != nil {
		t.Fatalf("Failed to register first migration: %v", err)
	}

	if err := migrator.Register(migration2); err != nil {
		t.Fatalf("Failed to register second migration: %v", err)
	}
}

func TestMigrator_Up(t *testing.T) {
	t.Skip("Requires testcontainers Postgres/Mongo - integration test")
}

func TestMigrator_Down(t *testing.T) {
	t.Skip("Requires testcontainers Postgres/Mongo - integration test")
}

func TestMigrator_Checksums(t *testing.T) {
	t.Skip("Requires testcontainers Postgres/Mongo - integration test")
}

func TestMigrator_Locks(t *testing.T) {
	t.Skip("Requires testcontainers Postgres/Mongo - integration test")
}
