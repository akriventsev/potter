// Package events предоставляет адаптеры для публикации доменных событий.
package events

import (
	"context"
	"fmt"
	"sync"

	"potter/framework/events"
)

// EventPublisherFactory интерфейс фабрики для создания Event Publisher адаптеров
type EventPublisherFactory interface {
	Create(publisherType string, config interface{}) (events.EventPublisher, error)
	Register(name string, creator func(config interface{}) (events.EventPublisher, error)) error
}

// DefaultEventPublisherFactory реализация фабрики Event Publisher
type DefaultEventPublisherFactory struct {
	creators map[string]func(config interface{}) (events.EventPublisher, error)
	mu       sync.RWMutex
}

// NewEventPublisherFactory создает новую фабрику Event Publisher
func NewEventPublisherFactory() *DefaultEventPublisherFactory {
	factory := &DefaultEventPublisherFactory{
		creators: make(map[string]func(config interface{}) (events.EventPublisher, error)),
	}

	// Регистрируем built-in адаптеры
	_ = factory.Register("nats", func(config interface{}) (events.EventPublisher, error) {
		cfg, ok := config.(NATSEventConfig)
		if !ok {
			return nil, fmt.Errorf("invalid NATS event config type: %T", config)
		}
		return NewNATSEventAdapter(cfg)
	})

	_ = factory.Register("kafka", func(config interface{}) (events.EventPublisher, error) {
		cfg, ok := config.(KafkaEventConfig)
		if !ok {
			return nil, fmt.Errorf("invalid Kafka event config type: %T", config)
		}
		return NewKafkaEventAdapter(cfg)
	})

	_ = factory.Register("messagebus", func(config interface{}) (events.EventPublisher, error) {
		cfg, ok := config.(MessageBusEventConfig)
		if !ok {
			return nil, fmt.Errorf("invalid MessageBus event config type: %T", config)
		}
		return NewMessageBusEventAdapter(cfg)
	})

	_ = factory.Register("inmemory", func(config interface{}) (events.EventPublisher, error) {
		// Используем framework/events InMemoryEventPublisher
		return events.NewInMemoryEventPublisher(), nil
	})

	return factory
}

// Create создает Event Publisher адаптер указанного типа
func (f *DefaultEventPublisherFactory) Create(publisherType string, config interface{}) (events.EventPublisher, error) {
	f.mu.RLock()
	creator, exists := f.creators[publisherType]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown event publisher type: %s", publisherType)
	}

	publisher, err := creator(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s event publisher: %w", publisherType, err)
	}

	return publisher, nil
}

// Register регистрирует custom адаптер
func (f *DefaultEventPublisherFactory) Register(name string, creator func(config interface{}) (events.EventPublisher, error)) error {
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

// Unregister удаляет регистрацию адаптера
func (f *DefaultEventPublisherFactory) Unregister(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.creators[name]; !exists {
		return fmt.Errorf("adapter %s not registered", name)
	}

	delete(f.creators, name)
	return nil
}

// ListRegistered возвращает список зарегистрированных адаптеров
func (f *DefaultEventPublisherFactory) ListRegistered() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.creators))
	for name := range f.creators {
		names = append(names, name)
	}
	return names
}

// CreateComposite создает composite publisher для публикации в несколько destinations
func (f *DefaultEventPublisherFactory) CreateComposite(publishers ...events.EventPublisher) events.EventPublisher {
	return &CompositeEventPublisher{
		publishers: publishers,
	}
}

// CompositeEventPublisher публикует события в несколько publishers
type CompositeEventPublisher struct {
	publishers []events.EventPublisher
}

// Publish публикует событие во все publishers
func (c *CompositeEventPublisher) Publish(ctx context.Context, event events.Event) error {
	var lastErr error
	for _, publisher := range c.publishers {
		if err := publisher.Publish(ctx, event); err != nil {
			lastErr = err
			// Продолжаем публикацию в другие publishers
		}
	}
	return lastErr
}

// CreateWithFallback создает publisher с fallback на in-memory при недоступности внешних систем
func (f *DefaultEventPublisherFactory) CreateWithFallback(publisherType string, config interface{}) (events.EventPublisher, error) {
	primary, err := f.Create(publisherType, config)
	if err != nil {
		// Fallback на in-memory
		return events.NewInMemoryEventPublisher(), nil
	}

	fallback := events.NewInMemoryEventPublisher()
	return &FallbackEventPublisher{
		primary:  primary,
		fallback: fallback,
	}, nil
}

// FallbackEventPublisher использует fallback при ошибках primary publisher
type FallbackEventPublisher struct {
	primary  events.EventPublisher
	fallback events.EventPublisher
}

// Publish публикует событие, используя fallback при ошибках
func (f *FallbackEventPublisher) Publish(ctx context.Context, event events.Event) error {
	err := f.primary.Publish(ctx, event)
	if err != nil {
		// Fallback на in-memory
		return f.fallback.Publish(ctx, event)
	}
	return nil
}

