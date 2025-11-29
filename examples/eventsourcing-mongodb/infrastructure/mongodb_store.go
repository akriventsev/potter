package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/akriventsev/potter/examples/eventsourcing-mongodb/domain"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/eventsourcing"
)

// EventDeserializer десериализатор событий для MongoDB
type EventDeserializer struct{}

// DeserializeEvent десериализует событие из JSON/BSON (реализация EventDeserializer)
func (d *EventDeserializer) DeserializeEvent(eventType string, data []byte) (events.Event, error) {
	switch eventType {
	case "inventory.item.added":
		event := &domain.ItemAddedEvent{}
		if err := json.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to deserialize ItemAddedEvent: %w", err)
		}
		return event, nil
	case "inventory.item.reserved":
		event := &domain.ItemReservedEvent{}
		if err := json.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to deserialize ItemReservedEvent: %w", err)
		}
		return event, nil
	default:
		// Для неизвестных типов создаем базовое событие
		var baseEvent events.BaseEvent
		if err := json.Unmarshal(data, &baseEvent); err != nil {
			return nil, fmt.Errorf("failed to deserialize event: %w", err)
		}
		return &baseEvent, nil
	}
}

// NewMongoDBStores создает MongoDB event store и snapshot store
func NewMongoDBStores(uri, database string) (eventsourcing.EventStore, eventsourcing.SnapshotStore, error) {
	config := eventsourcing.DefaultMongoDBEventStoreConfig()
	config.URI = uri
	config.Database = database
	config.Collection = "events"

	// Создаем десериализатор
	deserializer := &EventDeserializer{}

	// Создаем event store с десериализатором
	eventStore, err := eventsourcing.NewMongoDBEventStoreWithDeserializer(config, deserializer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MongoDB event store: %w", err)
	}

	// Создаем snapshot store
	snapshotStore, err := eventsourcing.NewMongoDBSnapshotStore(config)
	if err != nil {
		eventStore.Stop(context.Background())
		return nil, nil, fmt.Errorf("failed to create MongoDB snapshot store: %w", err)
	}

	return eventStore, snapshotStore, nil
}

