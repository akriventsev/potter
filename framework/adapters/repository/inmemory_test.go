package repository

import (
	"context"
	"testing"
)

// TestEntity для тестирования
type TestEntity struct {
	IDField string
	Name    string
}

func (e TestEntity) ID() string {
	return e.IDField
}

func TestInMemoryRepository_Save(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	entity := TestEntity{IDField: "test-1", Name: "Test"}
	err := repo.Save(ctx, entity)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestInMemoryRepository_Save_EmptyID(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	entity := TestEntity{IDField: "", Name: "Test"}
	err := repo.Save(ctx, entity)
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestInMemoryRepository_FindByID(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	entity := TestEntity{IDField: "test-1", Name: "Test"}
	if err := repo.Save(ctx, entity); err != nil {
		t.Fatalf("Failed to save entity: %v", err)
	}

	found, err := repo.FindByID(ctx, "test-1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if found.ID() != "test-1" {
		t.Errorf("Expected ID 'test-1', got %s", found.ID())
	}

	if found.Name != "Test" {
		t.Errorf("Expected Name 'Test', got %s", found.Name)
	}
}

func TestInMemoryRepository_FindByID_NotFound(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent entity")
	}
}

func TestInMemoryRepository_FindAll(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	entity1 := TestEntity{IDField: "test-1", Name: "Test1"}
	entity2 := TestEntity{IDField: "test-2", Name: "Test2"}

	if err := repo.Save(ctx, entity1); err != nil {
		t.Fatalf("Failed to save entity1: %v", err)
	}
	if err := repo.Save(ctx, entity2); err != nil {
		t.Fatalf("Failed to save entity2: %v", err)
	}

	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(all) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(all))
	}
}

func TestInMemoryRepository_Delete(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	entity := TestEntity{IDField: "test-1", Name: "Test"}
	if err := repo.Save(ctx, entity); err != nil {
		t.Fatalf("Failed to save entity: %v", err)
	}

	err := repo.Delete(ctx, "test-1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	_, err = repo.FindByID(ctx, "test-1")
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestInMemoryRepository_Delete_NotFound(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent entity")
	}
}

func TestInMemoryRepository_Find(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	entity1 := TestEntity{IDField: "test-1", Name: "Test1"}
	entity2 := TestEntity{IDField: "test-2", Name: "Test2"}

	if err := repo.Save(ctx, entity1); err != nil {
		t.Fatalf("Failed to save entity1: %v", err)
	}
	if err := repo.Save(ctx, entity2); err != nil {
		t.Fatalf("Failed to save entity2: %v", err)
	}

	results, err := repo.Find(ctx, func(e TestEntity) bool {
		return e.Name == "Test1"
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].ID() != "test-1" {
		t.Errorf("Expected ID 'test-1', got %s", results[0].ID())
	}
}

func TestInMemoryRepository_AddIndex(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	entity1 := TestEntity{IDField: "test-1", Name: "Test1"}
	entity2 := TestEntity{IDField: "test-2", Name: "Test1"}

	if err := repo.Save(ctx, entity1); err != nil {
		t.Fatalf("Failed to save entity1: %v", err)
	}
	if err := repo.Save(ctx, entity2); err != nil {
		t.Fatalf("Failed to save entity2: %v", err)
	}

	repo.AddIndex("name", func(e TestEntity) string {
		return e.Name
	})

	results, err := repo.FindByIndex(ctx, "name", "Test1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestInMemoryRepository_FindByIndex_NotFound(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	repo.AddIndex("name", func(e TestEntity) string {
		return e.Name
	})

	_, err := repo.FindByIndex(ctx, "nonexistent", "key")
	if err == nil {
		t.Error("Expected error for nonexistent index")
	}
}

func TestInMemoryRepository_Count(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	count, err := repo.Count(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	entity := TestEntity{IDField: "test-1", Name: "Test"}
	if err := repo.Save(ctx, entity); err != nil {
		t.Fatalf("Failed to save entity: %v", err)
	}

	count, err = repo.Count(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
}

func TestInMemoryRepository_Clear(t *testing.T) {
	repo := NewInMemoryRepository[TestEntity](DefaultInMemoryConfig())
	ctx := context.Background()

	entity := TestEntity{IDField: "test-1", Name: "Test"}
	if err := repo.Save(ctx, entity); err != nil {
		t.Fatalf("Failed to save entity: %v", err)
	}

	err := repo.Clear(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	count, _ := repo.Count(ctx)
	if count != 0 {
		t.Errorf("Expected count 0 after clear, got %d", count)
	}
}

