// Package examples содержит примеры использования адаптеров.
package examples

import (
	"context"
	"fmt"
	"log"

	eventsadapters "potter/framework/adapters/events"
	frameworkevents "potter/framework/events"
)

// ExampleKafkaEventPublisher демонстрирует использование Kafka Event Publisher
func ExampleKafkaEventPublisher() {
	// Создание конфигурации
	config := eventsadapters.KafkaEventConfig{
		Brokers:         []string{"localhost:9092"},
		TopicPrefix:     "events",
		Compression:     "snappy",
		IdempotentWrites: true,
		EnableMetrics:   true,
	}

	// Создание адаптера
	publisher, err := eventsadapters.NewKafkaEventAdapter(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Запуск адаптера
	if err := publisher.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := publisher.Stop(ctx); err != nil {
			log.Printf("Failed to stop publisher: %v", err)
		}
	}()

	// Публикация события
	event := frameworkevents.NewBaseEvent("user.created", "user-123").
		WithCorrelationID("req-456").
		WithCausationID("cmd-789").
		WithUserID("user-123")

	err = publisher.Publish(ctx, event)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Event published successfully")
}

