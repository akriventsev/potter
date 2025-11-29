// Package examples содержит примеры использования адаптеров.
package examples

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/akriventsev/potter/framework/adapters/messagebus"
	"github.com/akriventsev/potter/framework/transport"
)

// ExampleNATSMessageBus демонстрирует использование NATS MessageBus адаптера
func ExampleNATSMessageBus() {
	// Создание адаптера с builder pattern
	builder := messagebus.NewNATSAdapterBuilder().
		WithURL("nats://localhost:4222").
		WithMaxReconnects(10).
		WithReconnectWait(2 * time.Second).
		WithDrainTimeout(30 * time.Second).
		WithMetrics(true)

	adapter, err := builder.Build()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Запуск адаптера
	if err := adapter.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := adapter.Stop(ctx); err != nil {
			log.Printf("Failed to stop adapter: %v", err)
		}
	}()

	// Подписка на сообщения
	err = adapter.Subscribe(ctx, "users.*", func(ctx context.Context, msg *transport.Message) error {
		fmt.Printf("Received message on %s: %s\n", msg.Subject, string(msg.Data))
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// Публикация сообщения
	err = adapter.Publish(ctx, "users.created", []byte("user-123"), map[string]string{
		"user_id": "123",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Request-Reply пример
	reply, err := adapter.Request(ctx, "users.get", []byte("user-123"), 5*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Reply: %s\n", string(reply.Data))

	// Respond пример
	err = adapter.Respond(ctx, "users.get", func(ctx context.Context, request *transport.Message) (*transport.Message, error) {
		return &transport.Message{
			Subject: request.Subject,
			Data:    []byte("user data"),
			Headers: nil,
		}, nil
	})
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(1 * time.Second)
}

