// Package repository предоставляет generic адаптеры для работы с различными storage backends.
package repository

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryConfig конфигурация для InMemory репозитория
type InMemoryConfig struct {
	// MaxEntities максимальное количество сущностей (0 = без ограничений)
	// При достижении лимита Save вернет ошибку
	MaxEntities int
}

// DefaultInMemoryConfig возвращает конфигурацию InMemory по умолчанию
func DefaultInMemoryConfig() InMemoryConfig {
	return InMemoryConfig{
		MaxEntities: 0, // Без ограничений по умолчанию
	}
}

// InMemoryRepository[T Entity] generic in-memory репозиторий
type InMemoryRepository[T Entity] struct {
	config   InMemoryConfig
	entities map[string]T
	indexes  map[string]map[string][]string // index name -> key -> entity IDs
	keyFuncs map[string]func(T) string      // index name -> key function
	mu       sync.RWMutex
}

// NewInMemoryRepository создает новый in-memory репозиторий
func NewInMemoryRepository[T Entity](config InMemoryConfig) *InMemoryRepository[T] {
	return &InMemoryRepository[T]{
		config:   config,
		entities: make(map[string]T),
		indexes:  make(map[string]map[string][]string),
		keyFuncs: make(map[string]func(T) string),
	}
}

// Save сохраняет entity
func (r *InMemoryRepository[T]) Save(ctx context.Context, entity T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := entity.ID()
	if id == "" {
		return fmt.Errorf("entity ID cannot be empty")
	}

	// Проверяем лимит, если он установлен
	if r.config.MaxEntities > 0 {
		// Если entity уже существует, не считаем его как новую запись
		if _, exists := r.entities[id]; !exists {
			if len(r.entities) >= r.config.MaxEntities {
				return fmt.Errorf("repository limit reached: max %d entities", r.config.MaxEntities)
			}
		}
	}

	// Удаляем старую сущность из индексов, если она существует
	if oldEntity, exists := r.entities[id]; exists {
		for indexName, keyFunc := range r.keyFuncs {
			oldKey := keyFunc(oldEntity)
			if index, ok := r.indexes[indexName]; ok {
				if ids, ok := index[oldKey]; ok {
					// Удаляем ID из списка
					newIds := make([]string, 0, len(ids))
					for _, existingID := range ids {
						if existingID != id {
							newIds = append(newIds, existingID)
						}
					}
					if len(newIds) == 0 {
						delete(index, oldKey)
					} else {
						index[oldKey] = newIds
					}
				}
			}
		}
	}

	// Сохраняем сущность
	r.entities[id] = entity

	// Обновляем индексы для новой сущности
	for indexName, keyFunc := range r.keyFuncs {
		key := keyFunc(entity)
		if r.indexes[indexName] == nil {
			r.indexes[indexName] = make(map[string][]string)
		}
		ids := r.indexes[indexName][key]
		// Проверяем, что ID еще не добавлен
		found := false
		for _, existingID := range ids {
			if existingID == id {
				found = true
				break
			}
		}
		if !found {
			r.indexes[indexName][key] = append(ids, id)
		}
	}

	return nil
}

// FindByID находит entity по ID
func (r *InMemoryRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var zero T
	entity, exists := r.entities[id]
	if !exists {
		return zero, fmt.Errorf("entity not found: %s", id)
	}

	return entity, nil
}

// FindAll возвращает все entities
func (r *InMemoryRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entities := make([]T, 0, len(r.entities))
	for _, entity := range r.entities {
		entities = append(entities, entity)
	}

	return entities, nil
}

// Delete удаляет entity
func (r *InMemoryRepository[T]) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entity, exists := r.entities[id]
	if !exists {
		return fmt.Errorf("entity not found: %s", id)
	}

	// Удаляем из индексов
	for indexName, keyFunc := range r.keyFuncs {
		key := keyFunc(entity)
		if index, ok := r.indexes[indexName]; ok {
			if ids, ok := index[key]; ok {
				// Удаляем ID из списка
				newIds := make([]string, 0, len(ids))
				for _, existingID := range ids {
					if existingID != id {
						newIds = append(newIds, existingID)
					}
				}
				if len(newIds) == 0 {
					delete(index, key)
				} else {
					index[key] = newIds
				}
			}
		}
	}

	delete(r.entities, id)
	return nil
}

// Find находит entities по предикату
func (r *InMemoryRepository[T]) Find(ctx context.Context, predicate func(T) bool) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []T
	for _, entity := range r.entities {
		if predicate(entity) {
			results = append(results, entity)
		}
	}

	return results, nil
}

// AddIndex добавляет secondary index
func (r *InMemoryRepository[T]) AddIndex(name string, keyFunc func(T) string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Сохраняем функцию построения ключей
	r.keyFuncs[name] = keyFunc

	if r.indexes[name] == nil {
		r.indexes[name] = make(map[string][]string)
	}

	// Переиндексируем все entities
	for id, entity := range r.entities {
		key := keyFunc(entity)
		ids := r.indexes[name][key]
		// Проверяем, что ID еще не добавлен
		found := false
		for _, existingID := range ids {
			if existingID == id {
				found = true
				break
			}
		}
		if !found {
			r.indexes[name][key] = append(ids, id)
		}
	}
}

// FindByIndex находит entities по index key
func (r *InMemoryRepository[T]) FindByIndex(ctx context.Context, indexName, key string) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	index, exists := r.indexes[indexName]
	if !exists {
		return nil, fmt.Errorf("index not found: %s", indexName)
	}

	ids, exists := index[key]
	if !exists {
		return []T{}, nil
	}

	var results []T
	for _, id := range ids {
		if entity, exists := r.entities[id]; exists {
			results = append(results, entity)
		}
	}

	return results, nil
}

// Count возвращает количество entities
func (r *InMemoryRepository[T]) Count(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entities), nil
}

// Clear очищает репозиторий (для тестирования)
func (r *InMemoryRepository[T]) Clear(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entities = make(map[string]T)
	r.indexes = make(map[string]map[string][]string)
	r.keyFuncs = make(map[string]func(T) string)
	return nil
}

// Пример использования InMemoryRepository в приложении:
//
//	type User struct {
//		IDField string
//		Name    string
//	}
//
//	func (u User) ID() string {
//		return u.IDField
//	}
//
//	func main() {
//		config := repository.DefaultInMemoryConfig()
//		repo := repository.NewInMemoryRepository[User](config)
//
//		ctx := context.Background()
//		user := User{IDField: "user-1", Name: "John"}
//		_ = repo.Save(ctx, user)
//		found, _ := repo.FindByID(ctx, "user-1")
//	}
