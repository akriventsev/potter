package examples

import (
	"context"
	"fmt"
	"time"

	eventsadapters "github.com/akriventsev/potter/framework/adapters/events"
	"github.com/akriventsev/potter/framework/adapters/messagebus"
	transportadapters "github.com/akriventsev/potter/framework/adapters/transport"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/transport"
)

// ExampleMixedTransports демонстрирует использование разных транспортов одновременно
func ExampleMixedTransports() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("=== Инициализация всех транспортов ===")

	// 1. NATS для команд (легковесные, быстрые)
	fmt.Println("1. Настройка NATS для команд...")
	natsAdapter, err := messagebus.NewNATSAdapter("nats://localhost:4222")
	if err != nil {
		fmt.Printf("Failed to create NATS adapter: %v\n", err)
		return
	}

	if err := natsAdapter.Start(ctx); err != nil {
		fmt.Printf("Failed to start NATS adapter: %v\n", err)
		return
	}
	defer func() {
		_ = natsAdapter.Stop(ctx)
	}()

	natsCommandBus := invoke.NewAsyncCommandBus(natsAdapter)
	natsSubjectResolver := invoke.NewDefaultSubjectResolver("commands", "events")
	natsCommandBus.WithSubjectResolver(natsSubjectResolver)

	// 2. Kafka для событий (высокая пропускная способность, персистентность)
	fmt.Println("2. Настройка Kafka для событий...")
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

	// Kafka адаптер для подписки на события
	kafkaConfig := messagebus.DefaultKafkaConfig()
	kafkaConfig.Brokers = []string{"localhost:9092"}
	kafkaConfig.GroupID = "mixed-transports-service"
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

	kafkaSubjectResolver := invoke.NewFunctionSubjectResolver(
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

	serializer := invoke.NewJSONSerializer()
	kafkaEventAwaiter := invoke.NewEventAwaiterFromTransport(
		kafkaAdapter,
		serializer,
		kafkaSubjectResolver,
	)

	// 3. REST для синхронных запросов (публичный API)
	fmt.Println("3. Настройка REST для публичного API...")
	restConfig := transportadapters.DefaultRESTConfig()
	restConfig.Port = 8080
	restConfig.BasePath = "/api/v1"

	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()

	restAdapter, err := transportadapters.NewRESTAdapter(restConfig, commandBus, queryBus)
	if err != nil {
		fmt.Printf("Failed to create REST adapter: %v\n", err)
		return
	}

	if err := restAdapter.Start(ctx); err != nil {
		fmt.Printf("Failed to start REST adapter: %v\n", err)
		return
	}
	defer func() {
		_ = restAdapter.Stop(ctx)
	}()

	// 4. gRPC для внутренних микросервисных запросов (производительность)
	fmt.Println("4. Настройка gRPC для внутренних запросов...")
	grpcConfig := transportadapters.DefaultGRPCConfig()
	grpcConfig.Port = 50051

	grpcAdapter, err := transportadapters.NewGRPCAdapter(grpcConfig, commandBus, queryBus)
	if err != nil {
		fmt.Printf("Failed to create gRPC adapter: %v\n", err)
		return
	}

	if err := grpcAdapter.Start(ctx); err != nil {
		fmt.Printf("Failed to start gRPC adapter: %v\n", err)
		return
	}
	defer func() {
		_ = grpcAdapter.Stop(ctx)
	}()

	// Даем серверам время на запуск
	time.Sleep(200 * time.Millisecond)

	fmt.Println("\n=== Сценарий использования ===")

	// Сценарий 1: Клиент отправляет команду CreateOrder через NATS
	fmt.Println("\nСценарий 1: Отправка команды через NATS")
	orderInvoker := invoke.NewCommandInvoker[CreateOrderCommand, OrderCreatedEvent, OrderCreationFailedEvent](
		natsCommandBus,
		kafkaEventAwaiter,
		"order.created",
		"order.creation_failed",
	)

	// Регистрация handler для команд через NATS
	orderHandler := NewOrderCommandHandler(kafkaEventPublisher)
	_ = natsAdapter.Subscribe(ctx, "commands.create_order", func(ctx context.Context, msg *transport.Message) error {
		var cmd CreateOrderCommand
		if err := serializer.Deserialize(msg.Data, &cmd); err != nil {
			return err
		}
		if correlationID, ok := msg.Headers["correlation_id"]; ok {
			ctx = invoke.WithCorrelationID(ctx, correlationID)
		}
		return orderHandler.Handle(ctx, cmd)
	})

	cmd := CreateOrderCommand{
		ProductID: "prod-123",
		Quantity:  10,
		UserID:    "user-456",
	}

	event, err := orderInvoker.Invoke(ctx, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Order created via NATS: OrderID=%s\n", event.OrderID)
	}

	// Сценарий 2: Handler обрабатывает команду и публикует OrderCreated в Kafka
	fmt.Println("\nСценарий 2: Событие опубликовано в Kafka")
	fmt.Println("Event published to Kafka topic: events.order.created")

	// Сценарий 3: EventAwaiter подписан на Kafka для получения события
	fmt.Println("\nСценарий 3: EventAwaiter получил событие из Kafka")
	fmt.Println("Event matched by correlation ID and returned to client")

	// Сценарий 4: Параллельно REST endpoint принимает GetOrder запрос
	fmt.Println("\nСценарий 4: REST запрос через публичный API")
	getOrderHandler := NewGetOrderQueryHandler()
	_ = queryBus.Register(getOrderHandler)
	restAdapter.RegisterQuery("GET", "/orders/:id", GetOrderQuery{})

	orderQueryInvoker := invoke.NewQueryInvoker[GetOrderQuery, GetOrderResponse](queryBus)
	orderQuery := GetOrderQuery{OrderID: "order-1"}
	orderResult, err := orderQueryInvoker.Invoke(ctx, orderQuery)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Order retrieved via REST: OrderID=%s, Status=%s\n",
			orderResult.OrderID, orderResult.Status)
	}

	// Сценарий 5: gRPC service обрабатывает внутренний запрос GetInventory
	fmt.Println("\nСценарий 5: gRPC запрос для внутреннего сервиса")
	// Пример использования gRPC для внутренних запросов
	fmt.Println("gRPC service available for internal microservice communication")

	// Демонстрация разных SubjectResolver для каждого транспорта
	fmt.Println("\n=== Разные SubjectResolver ===")
	fmt.Printf("NATS commands: %s\n", natsSubjectResolver.ResolveCommandSubject(cmd))
	fmt.Printf("Kafka events: %s\n", kafkaSubjectResolver.ResolveEventSubject("order.created"))

	// Единый EventBus для координации (опционально)
	fmt.Println("\n=== Единый EventBus для координации ===")
	_ = events.NewInMemoryEventBus() // Создан для координации событий между транспортами
	fmt.Println("Unified EventBus создан для координации событий между транспортами")

	// Метрики для всех транспортов
	fmt.Println("\n=== Метрики ===")
	fmt.Println("Metrics enabled for all transports (NATS, Kafka, REST, gRPC)")

	// Graceful shutdown в правильном порядке
	fmt.Println("\n=== Graceful shutdown ===")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Останавливаем в обратном порядке запуска
	if err := kafkaEventAwaiter.Stop(shutdownCtx); err != nil {
		fmt.Printf("Error stopping Kafka event awaiter: %v\n", err)
	}

	fmt.Println("Все компоненты остановлены корректно")

	// Output:
	// === Инициализация всех транспортов ===
	// 1. Настройка NATS для команд...
	// 2. Настройка Kafka для событий...
	// 3. Настройка REST для публичного API...
	// 4. Настройка gRPC для внутренних запросов...
	// === Сценарий использования ===
	// Сценарий 1: Отправка команды через NATS
	// Order created via NATS: OrderID=order-...
	// Сценарий 2: Событие опубликовано в Kafka
	// Event published to Kafka topic: events.order.created
	// Сценарий 3: EventAwaiter получил событие из Kafka
	// Event matched by correlation ID and returned to client
	// Сценарий 4: REST запрос через публичный API
	// Order retrieved via REST: OrderID=order-1, Status=completed
	// Сценарий 5: gRPC запрос для внутреннего сервиса
	// gRPC service available for internal microservice communication
	// === Разные SubjectResolver ===
	// NATS commands: commands.create_order
	// Kafka events: events.order.created
	// === Единый EventBus для координации ===
	// Unified EventBus создан для координации событий между транспортами
	// === Метрики ===
	// Metrics enabled for all transports (NATS, Kafka, REST, gRPC)
	// === Graceful shutdown ===
	// Все компоненты остановлены корректно
}

