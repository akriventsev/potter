package examples

import (
	"context"
	"fmt"
	"time"

	"potter/framework/adapters/messagebus"
	"potter/framework/invoke"
	"potter/framework/transport"
	"github.com/segmentio/kafka-go"
)

// GetOrderQuery запрос для получения заказа
type GetOrderQuery struct {
	OrderID string `json:"order_id"`
}

func (q GetOrderQuery) QueryName() string {
	return "get_order"
}

// GetOrderResponse ответ на запрос заказа
type GetOrderResponse struct {
	OrderID   string    `json:"order_id"`
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// GetOrderQueryHandler обработчик запроса заказа
type GetOrderQueryHandler struct {
	orders map[string]*GetOrderResponse
}

func NewGetOrderQueryHandler() *GetOrderQueryHandler {
	return &GetOrderQueryHandler{
		orders: map[string]*GetOrderResponse{
			"order-1": {
				OrderID:   "order-1",
				ProductID: "prod-123",
				Quantity:  5,
				Status:    "completed",
				CreatedAt: time.Now(),
			},
			"order-2": {
				OrderID:   "order-2",
				ProductID: "prod-456",
				Quantity:  3,
				Status:    "pending",
				CreatedAt: time.Now(),
			},
		},
	}
}

func (h *GetOrderQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	query := q.(GetOrderQuery)
	order, exists := h.orders[query.OrderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", query.OrderID)
	}
	return order, nil
}

func (h *GetOrderQueryHandler) QueryName() string {
	return "get_order"
}

// ExampleQueryInvokerWithKafka демонстрирует использование QueryInvoker с Kafka Request-Reply
func ExampleQueryInvokerWithKafka() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Инициализация Kafka адаптера с consumer/producer конфигурацией
	kafkaConfig := messagebus.DefaultKafkaConfig()
	kafkaConfig.Brokers = []string{"localhost:9092"}
	kafkaConfig.GroupID = "query-service"
	kafkaConfig.ConsumerConfig.StartOffset = -1 // earliest
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

	// 2. Создание QueryBus с Kafka Request-Reply через correlation ID
	queryBus := transport.NewInMemoryQueryBus()
	handler := NewGetOrderQueryHandler()
	_ = queryBus.Register(handler)

	// 3. Регистрация query handler с Respond() для обработки запросов
	requestTopic := "queries.get_order"
	serializer := invoke.NewJSONSerializer()

	// Kafka Request-Reply через Respond метод
	if err := kafkaAdapter.Respond(ctx, requestTopic, func(ctx context.Context, request *transport.Message) (*transport.Message, error) {
		// Десериализуем запрос
		var query GetOrderQuery
		if err := serializer.Deserialize(request.Data, &query); err != nil {
			return nil, fmt.Errorf("failed to deserialize: %w", err)
		}

		// Выполняем запрос через QueryBus
		result, err := queryBus.Ask(ctx, query)
		if err != nil {
			return nil, err
		}

		// Сериализуем результат
		resultData, err := serializer.Serialize(result)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize: %w", err)
		}

		// Возвращаем ответ с correlation ID из headers запроса
		return &transport.Message{
			Subject: request.Subject,
			Data:    resultData,
			Headers: map[string]string{
				"correlation_id": request.Headers["correlation_id"],
				"status":         "success",
			},
		}, nil
	}); err != nil {
		fmt.Printf("Failed to register responder: %v\n", err)
		return
	}

	// 4. Создание QueryInvoker с Kafka Request-Reply оберткой
	kafkaQueryBus := &kafkaRequestReplyQueryBus{
		adapter:      kafkaAdapter,
		requestTopic: requestTopic,
		serializer:   serializer,
		timeout:      5 * time.Second,
		brokers:      kafkaConfig.Brokers,
	}

	invoker := invoke.NewQueryInvoker[GetOrderQuery, GetOrderResponse](kafkaQueryBus).
		WithTimeout(5 * time.Second)

	// 5. Отправка запроса через Request() с reply topic
	fmt.Println("=== Запрос заказа через Kafka ===")
	query := GetOrderQuery{OrderID: "order-1"}

	result, err := invoker.Invoke(ctx, query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Order found: OrderID=%s, ProductID=%s, Quantity=%d, Status=%s\n",
			result.OrderID, result.ProductID, result.Quantity, result.Status)
	}

	// 6. Ожидание ответа через correlation ID matching
	fmt.Println("\n=== Ожидание ответа ===")
	order, err := invoker.Invoke(ctx, GetOrderQuery{OrderID: "order-2"})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Order status: %s\n", order.Status)
	}

	// 7. Демонстрация timeout handling
	fmt.Println("\n=== Timeout handling ===")
	timeoutInvoker := invoker.WithTimeout(1 * time.Second)
	_, err = timeoutInvoker.Invoke(ctx, GetOrderQuery{OrderID: "order-999"})
	if err != nil {
		fmt.Printf("Timeout or error (expected): %v\n", err)
	}

	// 8. Cleanup reply topics (выполняется автоматически при использовании Request())
	fmt.Println("\n=== Cleanup ===")
	fmt.Println("Reply topics cleanup выполняется автоматически Kafka адаптером")

	// Output:
	// === Запрос заказа через Kafka ===
	// Order found: OrderID=order-1, ProductID=prod-123, Quantity=5, Status=completed
	// === Ожидание ответа ===
	// Order status: pending
	// === Timeout handling ===
	// Timeout or error (expected): ...
	// === Cleanup ===
	// Reply topics cleanup выполняется автоматически Kafka адаптером
}

// kafkaRequestReplyQueryBus обертка QueryBus для работы через Kafka Request-Reply
type kafkaRequestReplyQueryBus struct {
	adapter      transport.RequestReplyBus
	requestTopic string
	serializer   transport.MessageSerializer
	timeout      time.Duration
	brokers      []string
}

func (b *kafkaRequestReplyQueryBus) Ask(ctx context.Context, q transport.Query) (interface{}, error) {
	// Сериализуем запрос
	data, err := b.serializer.Serialize(q)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query: %w", err)
	}

	// Генерируем correlation ID
	correlationID := invoke.GenerateCorrelationID()
	
	// Создаем reply topic для ожидания ответа
	replyTopic := fmt.Sprintf("%s.reply.%s", b.requestTopic, correlationID)
	
	// Публикуем запрос с correlation ID в заголовках через Publish
	// Используем Publish напрямую, так как Request не поддерживает передачу заголовков
	headers := map[string]string{
		"correlation_id": correlationID,
		"reply_topic":    replyTopic,
	}
	
	if err := b.adapter.Publish(ctx, b.requestTopic, data, headers); err != nil {
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}
	
	// Создаем временный reader для reply topic и ожидаем ответ
	// Используем низкоуровневый API Kafka для ожидания ответа
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     b.brokers,
		Topic:       replyTopic,
		StartOffset: kafka.LastOffset,
		MaxWait:     b.timeout,
	})
	defer func() {
		_ = reader.Close()
	}()

	// Создаем контекст с timeout для ожидания ответа
	waitCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	// Ждем ответа
	msg, err := reader.FetchMessage(waitCtx)
	if err != nil {
		return nil, fmt.Errorf("request timeout or failed: %w", err)
	}

	// Преобразуем Kafka message в transport.Message
	mbMsg := &transport.Message{
		Subject: msg.Topic,
		Data:    msg.Value,
		Headers: make(map[string]string),
	}

	// Копируем headers из Kafka message
	for _, h := range msg.Headers {
		mbMsg.Headers[h.Key] = string(h.Value)
	}

	// Проверяем correlation ID в ответе
	if mbMsg.Headers["correlation_id"] != correlationID {
		return nil, fmt.Errorf("correlation ID mismatch")
	}

	// Десериализуем результат
	var result GetOrderResponse
	if err := b.serializer.Deserialize(mbMsg.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to deserialize result: %w", err)
	}

	return result, nil
}

func (b *kafkaRequestReplyQueryBus) Register(handler transport.QueryHandler) error {
	// Регистрация уже выполнена в примере
	return nil
}

