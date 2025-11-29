package examples

import (
	"context"
	"fmt"
	"reflect"
	"time"

	eventsadapters "github.com/akriventsev/potter/framework/adapters/events"
	"github.com/akriventsev/potter/framework/adapters/messagebus"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/transport"
)

// CreateOrderCommand команда для создания заказа
type CreateOrderCommand struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	UserID    string `json:"user_id"`
}

func (c CreateOrderCommand) CommandName() string {
	return "create_order"
}

// OrderCreatedEvent событие успешного создания заказа
type OrderCreatedEvent struct {
	*events.BaseEvent
	OrderID   string `json:"order_id"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

func NewOrderCreatedEvent(orderID, productID string, quantity int) *OrderCreatedEvent {
	return &OrderCreatedEvent{
		BaseEvent: events.NewBaseEvent("order.created", orderID),
		OrderID:   orderID,
		ProductID: productID,
		Quantity:  quantity,
	}
}

// OrderCreationFailedEvent событие ошибки при создании заказа
type OrderCreationFailedEvent struct {
	*invoke.BaseErrorEvent
	ProductID string `json:"product_id"`
	Reason    string `json:"reason"`
}

func NewOrderCreationFailedEvent(productID, reason string, err error) *OrderCreationFailedEvent {
	return &OrderCreationFailedEvent{
		BaseErrorEvent: invoke.NewBaseErrorEvent(
			"order.creation_failed",
			"",
			"VALIDATION_ERROR",
			reason,
			err,
			false,
		),
		ProductID: productID,
		Reason:    reason,
	}
}

// OrderCommandHandler обработчик команд заказа
type OrderCommandHandler struct {
	eventPublisher events.EventPublisher
}

func NewOrderCommandHandler(eventPublisher events.EventPublisher) *OrderCommandHandler {
	return &OrderCommandHandler{
		eventPublisher: eventPublisher,
	}
}

func (h *OrderCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	createCmd := cmd.(CreateOrderCommand)

	correlationID := invoke.ExtractCorrelationID(ctx)
	if correlationID == "" {
		return fmt.Errorf("correlation ID not found")
	}

	// Валидация
	if createCmd.Quantity <= 0 {
		errorEvent := NewOrderCreationFailedEvent(
			createCmd.ProductID,
			"Quantity must be positive",
			fmt.Errorf("invalid quantity"),
		)
		errorEvent.WithCorrelationID(correlationID)
		_ = h.eventPublisher.Publish(ctx, errorEvent)
		return nil
	}

	// Имитация создания заказа
	orderID := fmt.Sprintf("order-%d", time.Now().UnixNano())

	successEvent := NewOrderCreatedEvent(orderID, createCmd.ProductID, createCmd.Quantity)
	successEvent.WithCorrelationID(correlationID)

	return h.eventPublisher.Publish(ctx, successEvent)
}

func (h *OrderCommandHandler) CommandName() string {
	return "create_order"
}

// ExampleCommandInvokerWithKafka демонстрирует использование CommandInvoker с Kafka транспортом
func ExampleCommandInvokerWithKafka() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Инициализация Kafka адаптера с конфигурацией
	kafkaConfig := messagebus.DefaultKafkaConfig()
	kafkaConfig.Brokers = []string{"localhost:9092"}
	kafkaConfig.GroupID = "order-service"
	kafkaConfig.Compression = "snappy"
	kafkaConfig.ProducerConfig.Idempotent = true
	kafkaConfig.EnableMetrics = true

	kafkaAdapter, err := messagebus.NewKafkaAdapter(kafkaConfig)
	if err != nil {
		fmt.Printf("Failed to create Kafka adapter: %v\n", err)
		return
	}

	if err := kafkaAdapter.Start(ctx); err != nil {
		fmt.Printf("Failed to start Kafka adapter: %v\n", err)
		return
	}
	defer func() {
		_ = kafkaAdapter.Stop(ctx)
	}()

	// 2. Создание Kafka Event Publisher
	kafkaEventConfig := eventsadapters.DefaultKafkaEventConfig()
	kafkaEventConfig.Brokers = []string{"localhost:9092"}
	kafkaEventConfig.TopicPrefix = "events"
	kafkaEventConfig.Compression = "snappy"
	kafkaEventConfig.IdempotentWrites = true

	kafkaEventPublisher, err := eventsadapters.NewKafkaEventAdapter(kafkaEventConfig)
	if err != nil {
		fmt.Printf("Failed to create Kafka event adapter: %v\n", err)
		return
	}

	if err := kafkaEventPublisher.Start(ctx); err != nil {
		fmt.Printf("Failed to start Kafka event adapter: %v\n", err)
		return
	}
	defer func() {
		_ = kafkaEventPublisher.Stop(ctx)
	}()

	// 3. Создание AsyncCommandBus с Kafka publisher
	asyncBus := invoke.NewAsyncCommandBus(kafkaAdapter)

	// 4. Настройка SubjectResolver для маппинга команд/событий на Kafka topics
	subjectResolver := invoke.NewFunctionSubjectResolver(
		func(cmd transport.Command) string {
			return fmt.Sprintf("commands.%s", cmd.CommandName())
		},
		func(eventType string) string {
			// Для Kafka события публикуются через KafkaEventAdapter
			// Формат темы: events.{aggregate_type}.{event_type}
			// Для упрощения используем формат events.{event_type}
			// В реальном приложении aggregate_type извлекается из aggregate_id события
			return fmt.Sprintf("events.%s", eventType)
		},
	)
	asyncBus.WithSubjectResolver(subjectResolver)

	// 5. Создание EventAwaiter из Kafka subscriber через NewEventAwaiterFromTransport
	serializer := invoke.NewJSONSerializer()
	eventAwaiter := invoke.NewEventAwaiterFromTransport(
		kafkaAdapter,
		serializer,
		subjectResolver,
	)

	// 6. Создание CommandInvoker с поддержкой error events
	invoker := invoke.NewCommandInvoker[CreateOrderCommand, OrderCreatedEvent, OrderCreationFailedEvent](
		asyncBus,
		eventAwaiter,
		"order.created",
		"order.creation_failed",
	).WithTimeout(15 * time.Second)

	// 7. Регистрация handler для подписки на Kafka topic
	handler := NewOrderCommandHandler(kafkaEventPublisher)

	commandTopic := "commands.create_order"
	if err := kafkaAdapter.Subscribe(ctx, commandTopic, func(ctx context.Context, msg *transport.Message) error {
		var cmd CreateOrderCommand
		if err := serializer.Deserialize(msg.Data, &cmd); err != nil {
			return fmt.Errorf("failed to deserialize: %w", err)
		}

		if correlationID, ok := msg.Headers["correlation_id"]; ok {
			ctx = invoke.WithCorrelationID(ctx, correlationID)
		}

		return handler.Handle(ctx, cmd)
	}); err != nil {
		fmt.Printf("Failed to subscribe to commands: %v\n", err)
		return
	}

	// Подписка на события через Kafka (для EventAwaiter)
	// EventAwaiter автоматически подписывается на события через TransportSubscriberAdapter
	// Используем реальные имена тем без wildcard
	eventTopics := []string{"events.order.created", "events.order.creation_failed"}
	for _, topic := range eventTopics {
		_ = kafkaAdapter.Subscribe(ctx, topic, func(ctx context.Context, msg *transport.Message) error {
			// EventAwaiter обрабатывает события автоматически
			return nil
		})
	}

	// 8. Отправка команды и ожидание события через Kafka
	fmt.Println("=== Успешный сценарий ===")
	cmd := CreateOrderCommand{
		ProductID: "prod-123",
		Quantity:  5,
		UserID:    "user-456",
	}

	event, err := invoker.Invoke(ctx, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Order created: %+v\n", event)
	}

	// 9. Демонстрация InvokeWithBothResults для детального анализа
	fmt.Println("\n=== Детальный анализ с InvokeWithBothResults ===")
	successEvent, errorEvent, err := invoker.InvokeWithBothResults(ctx, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		var zeroError OrderCreationFailedEvent
		var zeroSuccess OrderCreatedEvent
		if !reflect.DeepEqual(errorEvent, zeroError) {
			fmt.Printf("Error event: %s (retryable: %v)\n", errorEvent.ErrorMessage(), errorEvent.IsRetryable())
		} else if !reflect.DeepEqual(successEvent, zeroSuccess) {
			fmt.Printf("Success event: OrderID=%s, Quantity=%d\n", successEvent.OrderID, successEvent.Quantity)
		}
	}

	// 10. Graceful shutdown с drain timeout
	fmt.Println("\n=== Graceful shutdown ===")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := eventAwaiter.Stop(shutdownCtx); err != nil {
		fmt.Printf("Error stopping awaiter: %v\n", err)
	}

	// Output:
	// === Успешный сценарий ===
	// Order created: &{BaseEvent:... OrderID:order-... ProductID:prod-123 Quantity:5}
	// === Детальный анализ с InvokeWithBothResults ===
	// Success event: OrderID=order-..., Quantity=5
	// === Graceful shutdown ===
}

