package container

import (
	"fmt"

	mbfactory "potter/framework/adapters/messagebus"
	"potter/framework/transport"
)

// Deprecated: Используйте framework/adapters/messagebus.NewMessageBusFactory напрямую.
// Этот файл будет удален в следующей версии.

// NewNATSMessageBus создает новый NATS MessageBus адаптер
// Deprecated: Используйте framework/adapters/messagebus.NewNATSAdapter
func NewNATSMessageBus(url string) (transport.RequestReplyBus, error) {
	return mbfactory.NewNATSAdapter(url)
}

// MessageBusFactory фабрика для создания MessageBus
type MessageBusFactory interface {
	Create(busType, url string) (transport.RequestReplyBus, error)
}

// DefaultMessageBusFactory реализация фабрики MessageBus
// Deprecated: Используйте framework/adapters/messagebus.NewMessageBusFactory
type DefaultMessageBusFactory struct {
	factory *mbfactory.DefaultMessageBusFactory
}

// NewMessageBusFactory создает новую фабрику MessageBus
func NewMessageBusFactory() *DefaultMessageBusFactory {
	return &DefaultMessageBusFactory{
		factory: mbfactory.NewMessageBusFactory(),
	}
}

// Create создает MessageBus указанного типа
func (f *DefaultMessageBusFactory) Create(busType, url string) (transport.RequestReplyBus, error) {
	// Конвертируем url в конфигурацию
	var config interface{}
	switch busType {
	case "nats":
		config = mbfactory.NATSConfig{URL: url}
	case "kafka":
		config = mbfactory.KafkaConfig{Brokers: []string{url}}
	case "redis":
		config = mbfactory.RedisConfig{Addr: url}
	case "inmemory":
		config = mbfactory.DefaultInMemoryConfig()
	default:
		return nil, fmt.Errorf("unknown message bus type: %s", busType)
	}
	
	return f.factory.Create(busType, config)
}
