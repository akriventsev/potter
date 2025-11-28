// Package messagebus предоставляет адаптеры для различных message brokers.
package messagebus

import (
	"fmt"
	"sync"

	"potter/framework/transport"
)

// MessageBusFactory интерфейс фабрики для создания MessageBus адаптеров
type MessageBusFactory interface {
	Create(busType string, config interface{}) (transport.RequestReplyBus, error)
	Register(name string, creator func(config interface{}) (transport.RequestReplyBus, error)) error
}

// DefaultMessageBusFactory реализация фабрики MessageBus
type DefaultMessageBusFactory struct {
	creators map[string]func(config interface{}) (transport.RequestReplyBus, error)
	mu       sync.RWMutex
}

// NewMessageBusFactory создает новую фабрику MessageBus
func NewMessageBusFactory() *DefaultMessageBusFactory {
	factory := &DefaultMessageBusFactory{
		creators: make(map[string]func(config interface{}) (transport.RequestReplyBus, error)),
	}

	// Регистрируем built-in адаптеры
	_ = factory.Register("nats", func(config interface{}) (transport.RequestReplyBus, error) {
		cfg, ok := config.(NATSConfig)
		if !ok {
			// Пытаемся преобразовать из map или других типов
			if url, ok := config.(string); ok {
				return NewNATSAdapter(url)
			}
			return nil, fmt.Errorf("invalid NATS config type: %T", config)
		}
		builder := NewNATSAdapterBuilder().
			WithURL(cfg.URL).
			WithMaxReconnects(cfg.MaxReconnects).
			WithReconnectWait(cfg.ReconnectWait).
			WithDrainTimeout(cfg.DrainTimeout).
			WithConnectionTimeout(cfg.ConnectionTimeout).
			WithMetrics(cfg.EnableMetrics).
			WithConnectionPool(cfg.ConnectionPoolSize)
		if cfg.TLS != nil {
			builder.WithTLS(cfg.TLS)
		}
		if cfg.Token != "" {
			builder.WithToken(cfg.Token)
		}
		if cfg.Username != "" && cfg.Password != "" {
			builder.WithCredentials(cfg.Username, cfg.Password)
		}
		return builder.Build()
	})

	_ = factory.Register("kafka", func(config interface{}) (transport.RequestReplyBus, error) {
		cfg, ok := config.(KafkaConfig)
		if !ok {
			return nil, fmt.Errorf("invalid Kafka config type: %T", config)
		}
		adapter, err := NewKafkaAdapter(cfg)
		if err != nil {
			return nil, err
		}
		return adapter, nil
	})

	_ = factory.Register("redis", func(config interface{}) (transport.RequestReplyBus, error) {
		cfg, ok := config.(RedisConfig)
		if !ok {
			return nil, fmt.Errorf("invalid Redis config type: %T", config)
		}
		return NewRedisAdapter(cfg)
	})

	_ = factory.Register("inmemory", func(config interface{}) (transport.RequestReplyBus, error) {
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
		return NewInMemoryAdapter(cfg), nil
	})

	return factory
}

// Create создает MessageBus адаптер указанного типа
func (f *DefaultMessageBusFactory) Create(busType string, config interface{}) (transport.RequestReplyBus, error) {
	f.mu.RLock()
	creator, exists := f.creators[busType]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown message bus type: %s", busType)
	}

	adapter, err := creator(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s adapter: %w", busType, err)
	}

	return adapter, nil
}

// Register регистрирует custom адаптер
func (f *DefaultMessageBusFactory) Register(name string, creator func(config interface{}) (transport.RequestReplyBus, error)) error {
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
func (f *DefaultMessageBusFactory) Unregister(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.creators[name]; !exists {
		return fmt.Errorf("adapter %s not registered", name)
	}

	delete(f.creators, name)
	return nil
}

// ListRegistered возвращает список зарегистрированных адаптеров
func (f *DefaultMessageBusFactory) ListRegistered() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.creators))
	for name := range f.creators {
		names = append(names, name)
	}
	return names
}

// ValidateConfig валидирует конфигурацию для указанного типа адаптера
func (f *DefaultMessageBusFactory) ValidateConfig(busType string, config interface{}) error {
	switch busType {
	case "nats":
		cfg, ok := config.(NATSConfig)
		if !ok {
			return fmt.Errorf("invalid NATS config type")
		}
		if cfg.URL == "" {
			return fmt.Errorf("NATS URL is required")
		}
	case "kafka":
		cfg, ok := config.(KafkaConfig)
		if !ok {
			return fmt.Errorf("invalid Kafka config type")
		}
		if len(cfg.Brokers) == 0 {
			return fmt.Errorf("kafka brokers are required")
		}
		if cfg.GroupID == "" {
			return fmt.Errorf("kafka GroupID is required")
		}
	case "redis":
		cfg, ok := config.(RedisConfig)
		if !ok {
			return fmt.Errorf("invalid Redis config type")
		}
		if cfg.Addr == "" {
			return fmt.Errorf("redis address is required")
		}
	case "inmemory":
		// InMemory не требует валидации
	default:
		return fmt.Errorf("unknown message bus type: %s", busType)
	}

	return nil
}

