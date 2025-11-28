// Package repository предоставляет generic адаптеры для работы с различными storage backends.
package repository

import (
	"fmt"
	"sync"
)

// RepositoryFactory интерфейс фабрики для создания Repository адаптеров
type RepositoryFactory interface {
	Register(name string, creator func(config interface{}) (interface{}, error)) error
}

// RepositoryFactoryImpl реализация фабрики с generic методом
type RepositoryFactoryImpl interface {
	Register(name string, creator func(config interface{}) (interface{}, error)) error
}

// DefaultRepositoryFactory реализация фабрики Repository
type DefaultRepositoryFactory struct {
	creators map[string]func(config interface{}) (interface{}, error)
	mu       sync.RWMutex
}

// NewRepositoryFactory создает новую фабрику Repository
func NewRepositoryFactory() *DefaultRepositoryFactory {
	factory := &DefaultRepositoryFactory{
		creators: make(map[string]func(config interface{}) (interface{}, error)),
	}

	// Регистрируем built-in адаптеры
	_ = factory.Register("inmemory", func(config interface{}) (interface{}, error) {
		var cfg InMemoryConfig
		if config != nil {
			if c, ok := config.(InMemoryConfig); ok {
				cfg = c
			} else {
				cfg = DefaultInMemoryConfig()
			}
		} else {
			cfg = DefaultInMemoryConfig()
		}
		// Возвращаем как interface{}, вызывающий код должен выполнить type assertion
		return NewInMemoryRepository[Entity](cfg), nil
	})

	_ = factory.Register("postgres", func(config interface{}) (interface{}, error) {
		// Требуется mapper, поэтому возвращаем ошибку
		return nil, fmt.Errorf("postgres repository requires a mapper, use NewPostgresRepository directly")
	})

	_ = factory.Register("mongodb", func(config interface{}) (interface{}, error) {
		cfg, ok := config.(MongoConfig)
		if !ok {
			return nil, fmt.Errorf("invalid Mongo config type: %T", config)
		}
		return NewMongoRepository[Entity](cfg)
	})

	return factory
}

// Create создает Repository адаптер указанного типа (generic метод)
func CreateRepository[T Entity](factory *DefaultRepositoryFactory, repoType string, config interface{}) (Repository[T], error) {
	factory.mu.RLock()
	creator, exists := factory.creators[repoType]
	factory.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown repository type: %s", repoType)
	}

	repo, err := creator(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s repository: %w", repoType, err)
	}

	// Type assertion
	typedRepo, ok := repo.(Repository[T])
	if !ok {
		return nil, fmt.Errorf("repository type mismatch")
	}

	return typedRepo, nil
}

// Register регистрирует custom адаптер
func (f *DefaultRepositoryFactory) Register(name string, creator func(config interface{}) (interface{}, error)) error {
	if name == "" {
		return fmt.Errorf("adapter name cannot be empty")
	}
	if creator == nil {
		return fmt.Errorf("creator function cannot be nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.creators[name]; exists {
		return fmt.Errorf("adapter %s already registered", name)
	}

	f.creators[name] = creator
	return nil
}

