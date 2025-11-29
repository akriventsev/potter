package repository

import (
	"context"
	"testing"
)

func TestAutoIndexManager_RecordQueryPattern(t *testing.T) {
	policy := DefaultIndexPolicy()
	manager := NewAutoIndexManager(nil, policy) // nil IndexManager для unit теста

	manager.RecordQueryPattern("name")
	manager.RecordQueryPattern("name")
	manager.RecordQueryPattern("email")

	if manager.queryPatterns["name"] != 2 {
		t.Errorf("Expected name pattern count 2, got %d", manager.queryPatterns["name"])
	}

	if manager.queryPatterns["email"] != 1 {
		t.Errorf("Expected email pattern count 1, got %d", manager.queryPatterns["email"])
	}
}

func TestAutoIndexManager_AnalyzeAndOptimize_NoPolicy(t *testing.T) {
	policy := DefaultIndexPolicy()
	policy.AutoCreate = false
	policy.AutoDrop = false

	manager := NewAutoIndexManager(nil, policy)

	// Должен вернуть nil без ошибки когда политика отключена
	err := manager.AnalyzeAndOptimize(context.Background())
	if err != nil {
		t.Errorf("Expected no error when policy disabled, got %v", err)
	}
}

func TestIndexPolicy_Default(t *testing.T) {
	policy := DefaultIndexPolicy()

	if policy.AutoCreate {
		t.Error("Expected AutoCreate to be false by default")
	}

	if policy.AutoDrop {
		t.Error("Expected AutoDrop to be false by default")
	}

	if policy.MinUsageThreshold != 100 {
		t.Errorf("Expected MinUsageThreshold 100, got %d", policy.MinUsageThreshold)
	}

	if policy.MaxIndexes != 10 {
		t.Errorf("Expected MaxIndexes 10, got %d", policy.MaxIndexes)
	}
}

func TestPostgresIndexManager_CreateIndex(t *testing.T) {
	t.Skip("Requires testcontainers Postgres - integration test")
}

func TestPostgresIndexManager_DropIndex(t *testing.T) {
	t.Skip("Requires testcontainers Postgres - integration test")
}

func TestPostgresIndexManager_ListIndexes(t *testing.T) {
	t.Skip("Requires testcontainers Postgres - integration test")
}

func TestPostgresIndexManager_CreateBTreeIndex(t *testing.T) {
	t.Skip("Requires testcontainers Postgres - integration test")
}

func TestPostgresIndexManager_CreateGINIndex(t *testing.T) {
	t.Skip("Requires testcontainers Postgres - integration test")
}

func TestPostgresIndexManager_CreateUniqueIndex(t *testing.T) {
	t.Skip("Requires testcontainers Postgres - integration test")
}

func TestPostgresIndexManager_CreatePartialIndex(t *testing.T) {
	t.Skip("Requires testcontainers Postgres - integration test")
}

func TestPostgresIndexManager_AnalyzeQueries(t *testing.T) {
	// Тест рекомендаций по индексам
	t.Skip("Requires testcontainers Postgres with pg_stat_statements - integration test")
}

func TestMongoIndexManager_CreateIndex(t *testing.T) {
	t.Skip("Requires testcontainers MongoDB replica set - integration test")
}

func TestMongoIndexManager_DropIndex(t *testing.T) {
	t.Skip("Requires testcontainers MongoDB replica set - integration test")
}

func TestMongoIndexManager_ListIndexes(t *testing.T) {
	t.Skip("Requires testcontainers MongoDB replica set - integration test")
}

func TestMongoIndexManager_AnalyzeQueries(t *testing.T) {
	t.Skip("Requires testcontainers MongoDB replica set - integration test")
}
