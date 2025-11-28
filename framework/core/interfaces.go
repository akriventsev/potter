// Package core предоставляет базовые интерфейсы и типы для всех компонентов фреймворка.
package core

import "context"

// Component базовый интерфейс для всех компонентов фреймворка
type Component interface {
	// Name возвращает имя компонента
	Name() string
	// Type возвращает тип компонента
	Type() ComponentType
}

// Lifecycle интерфейс для управления жизненным циклом компонентов
type Lifecycle interface {
	// Start запускает компонент
	Start(ctx context.Context) error
	// Stop останавливает компонент
	Stop(ctx context.Context) error
	// IsRunning проверяет, запущен ли компонент
	IsRunning() bool
}

// Configurable интерфейс для конфигурируемых компонентов
type Configurable interface {
	// Configure настраивает компонент
	Configure(config interface{}) error
	// GetConfig возвращает конфигурацию компонента
	GetConfig() interface{}
}

// Initializable интерфейс для компонентов, требующих инициализации
type Initializable interface {
	// Initialize инициализирует компонент
	Initialize(ctx context.Context) error
}

// Disposable интерфейс для компонентов, требующих очистки ресурсов
type Disposable interface {
	// Dispose освобождает ресурсы компонента
	Dispose(ctx context.Context) error
}

// HealthCheckable интерфейс для проверки здоровья компонентов
type HealthCheckable interface {
	// HealthCheck проверяет здоровье компонента
	HealthCheck(ctx context.Context) error
}

