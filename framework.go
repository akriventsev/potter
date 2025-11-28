// Package framework предоставляет универсальные компоненты для построения
// асинхронных CQRS сервисов с гексагональной архитектурой.
//
// Основные возможности:
//   - CQRS паттерн с разделением команд и запросов
//   - Система событий для асинхронной обработки
//   - DI контейнер с модульной архитектурой
//   - Транспортный слой (REST, gRPC, MessageBus)
//   - Метрики на основе OpenTelemetry
//   - Конечный автомат для саг и оркестрации
//
// Пример использования:
//
//	fw := framework.New()
//	if err := fw.Initialize(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer fw.Shutdown(ctx)
package framework

import (
	"context"
	"fmt"

	"potter/framework/core"
)

// Version представляет версию фреймворка
const (
	Version = "1.0.0"
	Major   = 1
	Minor   = 0
	Patch   = 0
)

// Metadata содержит метаданные о фреймворке
type Metadata struct {
	Name        string
	Version     string
	Description string
	Author      string
	License     string
}

// GetMetadata возвращает метаданные фреймворка
func GetMetadata() Metadata {
	return Metadata{
		Name:        "Potter Framework",
		Version:     Version,
		Description: "Framework for building async CQRS services with hexagonal architecture",
		Author:      "Potter Team",
		License:     "MIT",
	}
}


// Framework основной интерфейс фреймворка
type Framework interface {
	// Initialize инициализирует фреймворк
	Initialize(ctx context.Context) error
	// Shutdown корректно завершает работу фреймворка
	Shutdown(ctx context.Context) error
	// GetComponent возвращает компонент по имени
	GetComponent(name string) (core.Component, error)
	// RegisterComponent регистрирует компонент
	RegisterComponent(component core.Component) error
}

// BaseFramework базовая реализация фреймворка
type BaseFramework struct {
	components map[string]core.Component
	metadata   Metadata
}

// New создает новый экземпляр фреймворка
func New() *BaseFramework {
	return &BaseFramework{
		components: make(map[string]core.Component),
		metadata:   GetMetadata(),
	}
}

// Initialize инициализирует фреймворк
func (f *BaseFramework) Initialize(ctx context.Context) error {
	// Инициализация компонентов будет реализована в будущих версиях
	return nil
}

// Shutdown корректно завершает работу фреймворка
func (f *BaseFramework) Shutdown(ctx context.Context) error {
	// Остановка компонентов будет реализована в будущих версиях
	return nil
}

// GetComponent возвращает компонент по имени
func (f *BaseFramework) GetComponent(name string) (core.Component, error) {
	component, exists := f.components[name]
	if !exists {
		return nil, fmt.Errorf("component %s not found", name)
	}
	return component, nil
}

// RegisterComponent регистрирует компонент
func (f *BaseFramework) RegisterComponent(component core.Component) error {
	if _, exists := f.components[component.Name()]; exists {
		return fmt.Errorf("component %s already registered", component.Name())
	}
	f.components[component.Name()] = component
	return nil
}

// FrameworkVersion возвращает версию фреймворка
func FrameworkVersion() string {
	return Version
}

