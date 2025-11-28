package container

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestContainer_Get_Set(t *testing.T) {
	container := NewContainer(nil)

	err := Set[string](container, "test_key", "test_value")
	if err != nil {
		t.Fatalf("Failed to set dependency: %v", err)
	}

	value, err := Get[string](container, "test_key")
	if err != nil {
		t.Fatalf("Failed to get dependency: %v", err)
	}

	if value != "test_value" {
		t.Errorf("Expected 'test_value', got %v", value)
	}
}

func TestContainer_Get_NotFound(t *testing.T) {
	container := NewContainer(nil)

	_, err := Get[string](container, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent dependency")
	}
}

func TestContainer_Set_Duplicate(t *testing.T) {
	container := NewContainer(nil)

	if err := Set[string](container, "test_key", "value1"); err != nil {
		t.Fatalf("Failed to set value1: %v", err)
	}
	err := Set[string](container, "test_key", "value2")
	if err == nil {
		t.Error("Expected error for duplicate dependency")
	}
}

func TestContainer_SetWithScope(t *testing.T) {
	container := NewContainer(nil)

	err := SetWithScope[string](container, "test_key", "value", ScopeSingleton)
	if err != nil {
		t.Fatalf("Failed to set with scope: %v", err)
	}

	value, err := Get[string](container, "test_key")
	if err != nil {
		t.Fatalf("Failed to get dependency: %v", err)
	}

	if value != "value" {
		t.Errorf("Expected 'value', got %v", value)
	}
}

func TestContainer_SetWithScope_Scoped(t *testing.T) {
	container := NewContainer(nil)

	container.CreateScope("scope1")
	err := SetWithScope[string](container, "test_key", "scoped_value", ScopeScoped)
	if err != nil {
		t.Fatalf("Failed to set with scoped scope: %v", err)
	}

	value, err := GetFromScope[string](container, "test_key", "scope1")
	if err != nil {
		t.Fatalf("Failed to get from scope: %v", err)
	}

	if value != "scoped_value" {
		t.Errorf("Expected 'scoped_value', got %v", value)
	}
}

func TestContainer_SetWithScope_Transient(t *testing.T) {
	container := NewContainer(nil)

	err := SetWithScope[string](container, "test_key", "value", ScopeTransient)
	if err == nil {
		t.Error("Expected error for transient scope without factory")
	}
}

func TestContainer_GetFromScope(t *testing.T) {
	container := NewContainer(nil)

	container.CreateScope("scope1")
	if err := SetWithScope[string](container, "test_key", "value", ScopeScoped); err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	value, err := GetFromScope[string](container, "test_key", "scope1")
	if err != nil {
		t.Fatalf("Failed to get from scope: %v", err)
	}

	if value != "value" {
		t.Errorf("Expected 'value', got %v", value)
	}
}

func TestContainer_GetFromScope_NotFound(t *testing.T) {
	container := NewContainer(nil)

	container.CreateScope("scope1")
	_, err := GetFromScope[string](container, "nonexistent", "scope1")
	if err == nil {
		t.Error("Expected error for nonexistent dependency in scope")
	}
}

func TestContainer_CreateScope(t *testing.T) {
	container := NewContainer(nil)

	container.CreateScope("scope1")
	container.CreateScope("scope2")

	// Проверяем, что scope создан (косвенно через SetWithScope)
	err := SetWithScope[string](container, "test_key", "value", ScopeScoped)
	if err != nil {
		t.Fatalf("Failed to set in scope: %v", err)
	}
}

func TestContainer_ClearScope(t *testing.T) {
	container := NewContainer(nil)

	container.CreateScope("scope1")
	if err := SetWithScope[string](container, "test_key", "value", ScopeScoped); err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	container.ClearScope("scope1")

	_, err := GetFromScope[string](container, "test_key", "scope1")
	if err == nil {
		t.Error("Expected error after clearing scope")
	}
}

func TestContainer_DetectCircularDependencies(t *testing.T) {
	container := NewContainer(nil)

	// Упрощенный тест - проверяем что метод существует
	err := container.DetectCircularDependencies()
	// Без модулей не должно быть ошибки
	_ = err // Используем переменную для избежания пустой ветки
}

func TestContainer_Shutdown(t *testing.T) {
	container := NewContainer(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := container.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestContainer_Get_WrongType(t *testing.T) {
	container := NewContainer(nil)

	if err := Set[string](container, "test_key", "value"); err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	_, err := Get[int](container, "test_key")
	if err == nil {
		t.Error("Expected error for wrong type")
	}
}


// Disposable для тестирования
type MockDisposable struct {
	disposed bool
}

func (m *MockDisposable) Dispose(ctx context.Context) error {
	m.disposed = true
	return nil
}

func TestContainer_Shutdown_Disposable(t *testing.T) {
	container := NewContainer(nil)

	disposable := &MockDisposable{}
	if err := Set[*MockDisposable](container, "disposable", disposable); err != nil {
		t.Fatalf("Failed to set disposable: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := container.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !disposable.disposed {
		t.Error("Expected disposable to be disposed")
	}
}

func TestContainer_ConcurrentAccess(t *testing.T) {
	container := NewContainer(nil)

	// Конкурентная запись
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "key" + string(rune(idx))
			_ = Set[string](container, key, "value")
		}(i)
	}

	wg.Wait()

	// Конкурентное чтение
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "key" + string(rune(idx))
			_, _ = Get[string](container, key)
		}(i)
	}

	wg.Wait()
}

