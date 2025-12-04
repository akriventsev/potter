// Package container предоставляет DI контейнер для управления зависимостями.
package container

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Container контейнер зависимостей
type Container struct {
	// Конфигурация
	Config *Config

	// Реестр модулей
	registry *ModuleRegistry

	// Хранилище зависимостей
	dependencies map[string]interface{}
	mu           sync.RWMutex

	// Активные транспорты (для управления жизненным циклом)
	activeTransports []Transport

	// Scoped dependencies
	scopes map[string]map[string]interface{}
}

// DependencyScope область видимости зависимостей
type DependencyScope string

const (
	ScopeSingleton DependencyScope = "singleton"
	ScopeTransient DependencyScope = "transient"
	ScopeScoped    DependencyScope = "scoped"
)

// Config конфигурация контейнера
type Config struct {
	ShutdownTimeout time.Duration
}

// NewContainer создает новый контейнер
func NewContainer(config *Config) *Container {
	if config == nil {
		config = &Config{
			ShutdownTimeout: 30 * time.Second,
		}
	}

	return &Container{
		Config:       config,
		registry:     NewModuleRegistry(),
		dependencies: make(map[string]interface{}),
		scopes:       make(map[string]map[string]interface{}),
	}
}

// GetRegistry возвращает реестр модулей
func (c *Container) GetRegistry() *ModuleRegistry {
	return c.registry
}

// Get[T] получает зависимость по типу
func Get[T any](c *Container, key string) (T, error) {
	var zero T
	c.mu.RLock()
	defer c.mu.RUnlock()

	dep, exists := c.dependencies[key]
	if !exists {
		return zero, fmt.Errorf("dependency %s not found", key)
	}

	typed, ok := dep.(T)
	if !ok {
		return zero, fmt.Errorf("dependency %s has wrong type", key)
	}

	return typed, nil
}

// Set[T] устанавливает зависимость
func Set[T any](c *Container, key string, value T) error {
	return SetWithScope[T](c, key, value, ScopeSingleton)
}

// SetWithScope устанавливает зависимость с указанной областью видимости
// Для ScopeScoped использует "default" scopeID. Для явного указания scopeID используйте SetInScope.
func SetWithScope[T any](c *Container, key string, value T, scope DependencyScope) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch scope {
	case ScopeSingleton:
		if _, exists := c.dependencies[key]; exists {
			return fmt.Errorf("dependency %s already registered", key)
		}
		c.dependencies[key] = value
	case ScopeScoped:
		// Используем "default" scopeID для обратной совместимости
		// Для явного указания scopeID используйте SetInScope
		scopeID := "default"
		if c.scopes[scopeID] == nil {
			c.scopes[scopeID] = make(map[string]interface{})
		}
		if _, exists := c.scopes[scopeID][key]; exists {
			return fmt.Errorf("dependency %s already registered in scope %s", key, scopeID)
		}
		c.scopes[scopeID][key] = value
	case ScopeTransient:
		// Transient зависимости не сохраняются, создаются каждый раз заново
		return fmt.Errorf("transient dependencies must be registered with factory function")
	}
	return nil
}

// SetInScope устанавливает зависимость в указанную область видимости (scope)
func SetInScope[T any](c *Container, key string, value T, scopeID string) error {
	if scopeID == "" {
		return fmt.Errorf("scopeID cannot be empty")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.scopes[scopeID] == nil {
		c.scopes[scopeID] = make(map[string]interface{})
	}
	if _, exists := c.scopes[scopeID][key]; exists {
		return fmt.Errorf("dependency %s already registered in scope %s", key, scopeID)
	}
	c.scopes[scopeID][key] = value
	return nil
}

// GetFromScope получает зависимость из указанной области видимости
func GetFromScope[T any](c *Container, key string, scopeID string) (T, error) {
	var zero T
	c.mu.RLock()
	defer c.mu.RUnlock()

	if scopeID == "" {
		scopeID = "default"
	}

	if c.scopes[scopeID] != nil {
		if dep, exists := c.scopes[scopeID][key]; exists {
			typed, ok := dep.(T)
			if !ok {
				return zero, fmt.Errorf("dependency %s has wrong type", key)
			}
			return typed, nil
		}
	}

	return zero, fmt.Errorf("dependency %s not found in scope %s", key, scopeID)
}

// CreateScope создает новую область видимости
func (c *Container) CreateScope(scopeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.scopes[scopeID] == nil {
		c.scopes[scopeID] = make(map[string]interface{})
	}
}

// ClearScope очищает область видимости
func (c *Container) ClearScope(scopeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.scopes, scopeID)
}

// GetActiveTransports возвращает активные транспорты
func (c *Container) GetActiveTransports() []Transport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeTransports
}

// AddActiveTransport добавляет активный транспорт
func (c *Container) AddActiveTransport(transport Transport) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeTransports = append(c.activeTransports, transport)
}

// Shutdown корректно завершает работу всех зависимостей
func (c *Container) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.Config.ShutdownTimeout)
	defer cancel()

	// Останавливаем все транспорты
	for _, transport := range c.activeTransports {
		if err := transport.Stop(ctx); err != nil {
			// Логируем ошибку, но продолжаем остановку остальных
			_ = err
		}
	}

	// Закрываем зависимости, реализующие Disposable
	c.mu.RLock()
	for _, dep := range c.dependencies {
		if disposable, ok := dep.(interface{ Dispose(context.Context) error }); ok {
			_ = disposable.Dispose(ctx)
		}
	}
	c.mu.RUnlock()

	return nil
}

// DetectCircularDependencies обнаруживает циклические зависимости
func (c *Container) DetectCircularDependencies() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Строим граф зависимостей для модулей
	modules := c.registry.GetAllModules()
	graph := make(map[string][]string)
	for name, module := range modules {
		graph[name] = module.Dependencies()
	}

	// Строим граф для адаптеров
	adapters := c.registry.GetAllAdapters()
	for name, adapter := range adapters {
		if _, exists := graph[name]; !exists {
			graph[name] = []string{}
		}
		graph[name] = append(graph[name], adapter.Dependencies()...)
	}

	// Используем DFS для обнаружения циклов
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(node string) error
	dfs = func(node string) error {
		visited[node] = true
		recStack[node] = true

		for _, dep := range graph[node] {
			if !visited[dep] {
				if err := dfs(dep); err != nil {
					return err
				}
			} else if recStack[dep] {
				return fmt.Errorf("circular dependency detected: %s -> %s", node, dep)
			}
		}

		recStack[node] = false
		return nil
	}

	for node := range graph {
		if !visited[node] {
			if err := dfs(node); err != nil {
				return err
			}
		}
	}

	return nil
}

