// Package container предоставляет построитель для создания контейнера.
package container

import (
	"context"
	"fmt"
	"time"
)

// ContainerBuilder построитель контейнера
type ContainerBuilder struct {
	registry   *ModuleRegistry
	config     *Config
	initConfig *InitializationConfig
	profiles   []string
}

// NewContainerBuilder создает новый построитель контейнера
func NewContainerBuilder(cfg *Config) *ContainerBuilder {
	return &ContainerBuilder{
		registry: NewModuleRegistry(),
		config:   cfg,
		initConfig: &InitializationConfig{
			Modules:    []string{},
			Adapters:   []string{},
			Transports: []string{},
		},
		profiles: []string{},
	}
}

// WithConfig загружает конфигурацию из разных источников
func (b *ContainerBuilder) WithConfig(config *Config) *ContainerBuilder {
	b.config = config
	return b
}

// WithDefaults устанавливает значения по умолчанию
func (b *ContainerBuilder) WithDefaults() *ContainerBuilder {
	if b.config == nil {
		b.config = &Config{
			ShutdownTimeout: 30 * time.Second,
		}
	}
	return b
}

// WithProfile добавляет профиль (dev, prod, test)
func (b *ContainerBuilder) WithProfile(profile string) *ContainerBuilder {
	b.profiles = append(b.profiles, profile)
	return b
}

// WithModule добавляет модуль в реестр
func (b *ContainerBuilder) WithModule(module Module) *ContainerBuilder {
	_ = b.registry.RegisterModule(module)
	return b
}

// WithAdapter добавляет адаптер в реестр
func (b *ContainerBuilder) WithAdapter(adapter Adapter) *ContainerBuilder {
	_ = b.registry.RegisterAdapter(adapter)
	return b
}

// WithTransport добавляет транспорт в реестр
func (b *ContainerBuilder) WithTransport(transport Transport) *ContainerBuilder {
	_ = b.registry.RegisterTransport(transport)
	return b
}

// WithModules указывает какие модули инициализировать (пустой список = все)
func (b *ContainerBuilder) WithModules(moduleNames ...string) *ContainerBuilder {
	b.initConfig.Modules = moduleNames
	return b
}

// WithAdapters указывает какие адаптеры инициализировать (пустой список = все)
func (b *ContainerBuilder) WithAdapters(adapterNames ...string) *ContainerBuilder {
	b.initConfig.Adapters = adapterNames
	return b
}

// WithTransports указывает какие транспорты инициализировать (пустой список = все)
func (b *ContainerBuilder) WithTransports(transportNames ...string) *ContainerBuilder {
	b.initConfig.Transports = transportNames
	return b
}

// WithConditionalModule добавляет модуль с условием
func (b *ContainerBuilder) WithConditionalModule(module Module, condition func(ctx context.Context, container *Container) bool) *ContainerBuilder {
	conditional := NewConditionalModule(module, condition)
	_ = b.registry.RegisterModule(conditional)
	return b
}

// IgnoreDependencyErrors указывает игнорировать ошибки зависимостей
func (b *ContainerBuilder) IgnoreDependencyErrors(ignore bool) *ContainerBuilder {
	b.initConfig.IgnoreDependencyErrors = ignore
	return b
}

// Validate валидирует конфигурацию перед сборкой
func (b *ContainerBuilder) Validate() error {
	// Создаем временный контейнер для проверки циклических зависимостей
	container := NewContainer(b.config)
	container.registry = b.registry

	// Проверка циклических зависимостей
	if err := container.DetectCircularDependencies(); err != nil {
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	// Проверка наличия всех зависимостей модулей
	modules := b.registry.GetAllModules()
	availableModules := make(map[string]Module)
	for name, module := range modules {
		availableModules[name] = module
	}

	for name, module := range modules {
		if err := b.registry.validateDependencies(module.Dependencies(), availableModules); err != nil {
			return fmt.Errorf("module %s has invalid dependencies: %w", name, err)
		}
	}

	// Проверка наличия всех зависимостей адаптеров
	adapters := b.registry.GetAllAdapters()
	for name, adapter := range adapters {
		deps := adapter.Dependencies()
		for _, dep := range deps {
			// Проверяем, существует ли зависимость в модулях или адаптерах
			if _, exists := availableModules[dep]; !exists {
				if _, exists := adapters[dep]; !exists {
					return fmt.Errorf("adapter %s depends on non-existent component: %s", name, dep)
				}
			}
		}
	}

	// Проверка наличия всех зависимостей транспортов
	transports := b.registry.GetAllTransports()
	for name, transport := range transports {
		deps := transport.Dependencies()
		for _, dep := range deps {
			// Проверяем, существует ли зависимость в модулях, адаптерах или транспортах
			if _, exists := availableModules[dep]; !exists {
				if _, exists := adapters[dep]; !exists {
					if _, exists := transports[dep]; !exists {
						return fmt.Errorf("transport %s depends on non-existent component: %s", name, dep)
					}
				}
			}
		}
	}

	return nil
}

// Build создает и инициализирует контейнер
func (b *ContainerBuilder) Build(ctx context.Context) (*Container, error) {
	// Валидация конфигурации перед сборкой (fail-fast)
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	container := NewContainer(b.config)
	container.registry = b.registry

	// Создаем инициализатор
	initializer := NewInitializer(b.registry, b.initConfig)

	// Инициализируем контейнер
	if err := initializer.Initialize(ctx, container); err != nil {
		return nil, fmt.Errorf("failed to initialize container: %w", err)
	}

	// Добавляем инициализированные транспорты в активные
	transports := b.registry.GetAllTransports()
	for _, transport := range transports {
		container.AddActiveTransport(transport)
	}

	return container, nil
}

// GetRegistry возвращает реестр модулей (для расширенного использования)
func (b *ContainerBuilder) GetRegistry() *ModuleRegistry {
	return b.registry
}

