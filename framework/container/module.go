// Package container предоставляет систему модулей для DI контейнера.
package container

import (
	"context"
	"fmt"
	"sync"

	"github.com/akriventsev/potter/framework/core"
)

// Module представляет модуль приложения
type Module interface {
	core.Component
	// Initialize инициализирует модуль
	Initialize(ctx context.Context, container *Container) error
	// Dependencies возвращает список зависимостей модуля
	Dependencies() []string
	// Priority возвращает приоритет инициализации (меньше = выше приоритет)
	Priority() core.Priority
}

// BaseModule базовая реализация модуля
type BaseModule struct {
	name         string
	dependencies []string
	priority     core.Priority
	metadata     ModuleMetadata
}

// ModuleMetadata метаданные модуля
type ModuleMetadata struct {
	Version     string
	Author      string
	Description string
}

// NewBaseModule создает новый базовый модуль
func NewBaseModule(name string, dependencies []string, priority core.Priority) *BaseModule {
	return &BaseModule{
		name:         name,
		dependencies: dependencies,
		priority:     priority,
	}
}

// WithMetadata добавляет метаданные к модулю
func (m *BaseModule) WithMetadata(metadata ModuleMetadata) *BaseModule {
	m.metadata = metadata
	return m
}

func (m *BaseModule) Name() string {
	return m.name
}

func (m *BaseModule) Type() core.ComponentType {
	return core.ComponentTypeModule
}

func (m *BaseModule) Dependencies() []string {
	return m.dependencies
}

func (m *BaseModule) Priority() core.Priority {
	return m.priority
}

// ModuleLifecycleHooks хуки жизненного цикла модуля
type ModuleLifecycleHooks struct {
	OnBeforeInit func(ctx context.Context, container *Container) error
	OnAfterInit  func(ctx context.Context, container *Container) error
	OnShutdown   func(ctx context.Context, container *Container) error
}

// ModuleWithHooks модуль с хуками жизненного цикла
type ModuleWithHooks struct {
	Module
	hooks ModuleLifecycleHooks
}

// NewModuleWithHooks создает модуль с хуками
func NewModuleWithHooks(module Module, hooks ModuleLifecycleHooks) *ModuleWithHooks {
	return &ModuleWithHooks{
		Module: module,
		hooks:  hooks,
	}
}

// Initialize инициализирует модуль с хуками
func (m *ModuleWithHooks) Initialize(ctx context.Context, container *Container) error {
	if m.hooks.OnBeforeInit != nil {
		if err := m.hooks.OnBeforeInit(ctx, container); err != nil {
			return fmt.Errorf("onBeforeInit failed: %w", err)
		}
	}

	if err := m.Module.Initialize(ctx, container); err != nil {
		return err
	}

	if m.hooks.OnAfterInit != nil {
		if err := m.hooks.OnAfterInit(ctx, container); err != nil {
			return fmt.Errorf("onAfterInit failed: %w", err)
		}
	}

	return nil
}

// Hooks возвращает хуки модуля
func (m *ModuleWithHooks) Hooks() ModuleLifecycleHooks {
	return m.hooks
}

// ConditionalModule модуль, загружаемый по условию
type ConditionalModule struct {
	Module
	condition func(ctx context.Context, container *Container) bool
}

// NewConditionalModule создает условный модуль
func NewConditionalModule(module Module, condition func(ctx context.Context, container *Container) bool) *ConditionalModule {
	return &ConditionalModule{
		Module:    module,
		condition: condition,
	}
}

// Initialize инициализирует модуль только если условие выполнено
func (m *ConditionalModule) Initialize(ctx context.Context, container *Container) error {
	if !m.ShouldLoad(ctx, container) {
		return nil
	}
	return m.Module.Initialize(ctx, container)
}

// ShouldLoad проверяет, нужно ли загружать модуль
func (m *ConditionalModule) ShouldLoad(ctx context.Context, container *Container) bool {
	return m.condition(ctx, container)
}

// Adapter представляет адаптер (репозиторий, публикатор событий и т.д.)
type Adapter interface {
	core.Component
	// Initialize инициализирует адаптер
	Initialize(ctx context.Context, container *Container) error
	// Dependencies возвращает список зависимостей адаптера
	Dependencies() []string
}

// BaseAdapter базовая реализация адаптера
type BaseAdapter struct {
	name         string
	dependencies []string
}

// NewBaseAdapter создает новый базовый адаптер
func NewBaseAdapter(name string, dependencies []string) *BaseAdapter {
	return &BaseAdapter{
		name:         name,
		dependencies: dependencies,
	}
}

func (a *BaseAdapter) Name() string {
	return a.name
}

func (a *BaseAdapter) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

func (a *BaseAdapter) Dependencies() []string {
	return a.dependencies
}

// Transport представляет транспорт (REST, gRPC, MessageBus и т.д.)
type Transport interface {
	core.Component
	// Initialize инициализирует транспорт
	Initialize(ctx context.Context, container *Container) error
	// Start запускает транспорт
	Start(ctx context.Context) error
	// Stop останавливает транспорт
	Stop(ctx context.Context) error
	// Dependencies возвращает список зависимостей транспорта
	Dependencies() []string
}

// BaseTransport базовая реализация транспорта
type BaseTransport struct {
	name         string
	dependencies []string
}

// NewBaseTransport создает новый базовый транспорт
func NewBaseTransport(name string, dependencies []string) *BaseTransport {
	return &BaseTransport{
		name:         name,
		dependencies: dependencies,
	}
}

func (t *BaseTransport) Name() string {
	return t.name
}

func (t *BaseTransport) Type() core.ComponentType {
	return core.ComponentTypeTransport
}

func (t *BaseTransport) Dependencies() []string {
	return t.dependencies
}

// ModuleRegistry реестр модулей
type ModuleRegistry struct {
	modules    map[string]Module
	adapters   map[string]Adapter
	transports map[string]Transport
	mu         sync.RWMutex
}

// NewModuleRegistry создает новый реестр модулей
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules:    make(map[string]Module),
		adapters:   make(map[string]Adapter),
		transports: make(map[string]Transport),
	}
}

// RegisterModule регистрирует модуль
func (r *ModuleRegistry) RegisterModule(module Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[module.Name()]; exists {
		return fmt.Errorf("module %s already registered", module.Name())
	}

	// Валидация зависимостей
	if err := r.validateDependencies(module.Dependencies(), r.modules); err != nil {
		return fmt.Errorf("module %s has invalid dependencies: %w", module.Name(), err)
	}

	r.modules[module.Name()] = module
	return nil
}

// RegisterAdapter регистрирует адаптер
func (r *ModuleRegistry) RegisterAdapter(adapter Adapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[adapter.Name()]; exists {
		return fmt.Errorf("adapter %s already registered", adapter.Name())
	}

	r.adapters[adapter.Name()] = adapter
	return nil
}

// RegisterTransport регистрирует транспорт
func (r *ModuleRegistry) RegisterTransport(transport Transport) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transports[transport.Name()]; exists {
		return fmt.Errorf("transport %s already registered", transport.Name())
	}

	r.transports[transport.Name()] = transport
	return nil
}

// validateDependencies проверяет наличие зависимостей
func (r *ModuleRegistry) validateDependencies(deps []string, available map[string]Module) error {
	for _, dep := range deps {
		if _, exists := available[dep]; !exists {
			return fmt.Errorf("dependency %s not found", dep)
		}
	}
	return nil
}

// GetModule возвращает модуль по имени
func (r *ModuleRegistry) GetModule(name string) (Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	module, exists := r.modules[name]
	return module, exists
}

// GetAdapter возвращает адаптер по имени
func (r *ModuleRegistry) GetAdapter(name string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, exists := r.adapters[name]
	return adapter, exists
}

// GetTransport возвращает транспорт по имени
func (r *ModuleRegistry) GetTransport(name string) (Transport, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	transport, exists := r.transports[name]
	return transport, exists
}

// GetAllModules возвращает все модули
func (r *ModuleRegistry) GetAllModules() map[string]Module {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.modules
}

// GetAllAdapters возвращает все адаптеры
func (r *ModuleRegistry) GetAllAdapters() map[string]Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters
}

// GetAllTransports возвращает все транспорты
func (r *ModuleRegistry) GetAllTransports() map[string]Transport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.transports
}

