// Package testing предоставляет утилиты для тестирования приложений на базе фреймворка.
package testing

import (
	"context"
	"testing"

	"potter/framework/adapters/messagebus"
	"potter/framework/container"
	"potter/framework/events"
	"potter/framework/transport"
)

// InMemoryTestEnvironment тестовая среда с готовыми in-memory компонентами
type InMemoryTestEnvironment struct {
	CommandBus  transport.CommandBus
	QueryBus    transport.QueryBus
	EventBus    events.EventBus
	MessageBus  transport.MessageBus
	Container   *container.Container
}

// NewInMemoryTestEnvironment создает новую тестовую среду с готовыми компонентами
// Если сборка контейнера завершается с ошибкой, тест завершается с t.Fatalf
func NewInMemoryTestEnvironment(t *testing.T) *InMemoryTestEnvironment {
	// Создаем in-memory компоненты
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()
	messageBusAdapter := messagebus.NewInMemoryAdapter(messagebus.DefaultInMemoryConfig())

	// Создаем контейнер с дефолтными настройками
	builder := container.NewContainerBuilder(&container.Config{}).
		WithDefaults()

	cnt, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("failed to build test container: %v", err)
	}

	return &InMemoryTestEnvironment{
		CommandBus: commandBus,
		QueryBus:   queryBus,
		EventBus:   eventBus,
		MessageBus: messageBusAdapter,
		Container:  cnt,
	}
}

// Shutdown корректно завершает работу тестовой среды
func (e *InMemoryTestEnvironment) Shutdown(ctx context.Context) error {
	if e.Container != nil {
		return e.Container.Shutdown(ctx)
	}
	return nil
}

