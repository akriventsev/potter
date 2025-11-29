package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/akriventsev/potter/examples/saga-order/application"
	"github.com/akriventsev/potter/examples/saga-order/domain"
	"github.com/akriventsev/potter/examples/saga-order/infrastructure"
	"github.com/akriventsev/potter/framework/adapters/messagebus"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/eventsourcing"
	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/saga"
	"github.com/akriventsev/potter/framework/transport"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Config struct {
	Server struct {
		Port string
	}
	Database struct {
		DSN string
	}
	NATS struct {
		URL string
	}
	Redis struct {
		Addr     string
		Password string
		DB       int
	}
}

func loadConfig() *Config {
	cfg := &Config{}

	cfg.Server.Port = getEnv("SERVER_PORT", "8080")
	cfg.Database.DSN = getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/saga_order?sslmode=disable")
	cfg.NATS.URL = getEnv("NATS_URL", "nats://localhost:4222")
	cfg.Redis.Addr = getEnv("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", "")
	cfg.Redis.DB = 0

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	cfg := loadConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Применяем миграции
	if err := infrastructure.RunMigrations(cfg.Database.DSN); err != nil {
		log.Fatalf("Failed to apply migrations: %v", err)
	}

	// Создаем EventStore persistence для саг
	sagaPersistence, err := infrastructure.NewSagaEventStorePersistence(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("Failed to create saga persistence: %v", err)
	}

	// Создаем NATS adapter
	natsAdapter, err := messagebus.NewNATSAdapter(cfg.NATS.URL)
	if err != nil {
		log.Fatalf("Failed to create NATS adapter: %v", err)
	}

	if err := natsAdapter.Start(ctx); err != nil {
		log.Fatalf("Failed to start NATS adapter: %v", err)
	}
	defer func() {
		if err := natsAdapter.Stop(ctx); err != nil {
			log.Printf("Failed to stop NATS adapter: %v", err)
		}
	}()

	// Создаем InMemoryEventBus для событий
	eventBus := events.NewInMemoryEventBus()

	// Создаем AsyncCommandBus и EventAwaiter для CommandInvoker
	asyncCommandBus := invoke.NewAsyncCommandBus(natsAdapter)
	eventAwaiter := invoke.NewEventAwaiterFromEventBus(eventBus)

	// Создаем мост между NATS событиями и InMemoryEventBus для EventAwaiter
	// Подписываемся на все события, которые публикуются через AsyncCommandBus
	// По умолчанию используется префикс "events" для событий
	serializer := invoke.DefaultSerializer()
	eventSubjectPrefix := "events"

	// Подписываемся на все события с префиксом "events" (wildcard "events.>")
	eventSubjectPattern := eventSubjectPrefix + ".>"

	eventBridgeHandler := func(ctx context.Context, msg *transport.Message) error {
		// Извлекаем тип события из subject (формат: events.<eventType> или events.<aggregate>.<eventType>)
		subject := msg.Subject
		eventType := extractEventTypeFromSubject(subject, eventSubjectPrefix)
		if eventType == "" {
			// Пытаемся получить тип события из заголовков
			if eventTypeFromHeader, ok := msg.Headers["event_type"]; ok {
				eventType = eventTypeFromHeader
			} else {
				log.Printf("Failed to extract event type from subject: %s", subject)
				return nil
			}
		}

		// Десериализуем событие из JSON
		var eventData map[string]interface{}
		if err := serializer.Deserialize(msg.Data, &eventData); err != nil {
			log.Printf("Failed to deserialize event from NATS: %v", err)
			return nil
		}

		// Извлекаем базовые поля события
		eventID, _ := getStringFromMap(eventData, "event_id")
		aggregateID, _ := getStringFromMap(eventData, "aggregate_id")

		// Создаем BaseEvent
		baseEvent := events.NewBaseEvent(eventType, aggregateID)
		if eventID != "" {
			// Используем eventID из сообщения, если есть
			// BaseEvent генерирует свой ID, но мы можем переопределить через metadata
			baseEvent = baseEvent.WithMetadata("original_event_id", eventID)
		}

		// Переносим correlation ID из заголовков NATS в Metadata события
		if correlationID, ok := msg.Headers["correlation_id"]; ok && correlationID != "" {
			baseEvent = baseEvent.WithCorrelationID(correlationID)
		}

		// Переносим causation ID из заголовков NATS в Metadata события
		if causationID, ok := msg.Headers["causation_id"]; ok && causationID != "" {
			baseEvent = baseEvent.WithCausationID(causationID)
		}

		// Переносим другие метаданные из заголовков
		for key, value := range msg.Headers {
			if key != "correlation_id" && key != "causation_id" && key != "event_type" {
				baseEvent = baseEvent.WithMetadata(key, value)
			}
		}

		// Переносим метаданные из десериализованного события, если они есть
		if metadata, ok := eventData["metadata"].(map[string]interface{}); ok {
			for key, value := range metadata {
				baseEvent = baseEvent.WithMetadata(key, value)
			}
		}

		// Публикуем событие в InMemoryEventBus
		if err := eventBus.Publish(ctx, baseEvent); err != nil {
			log.Printf("Failed to publish event to InMemoryEventBus: %v", err)
			return nil
		}

		return nil
	}

	// Подписываемся на события через NATS adapter
	if err := natsAdapter.Subscribe(ctx, eventSubjectPattern, eventBridgeHandler); err != nil {
		log.Fatalf("Failed to subscribe to NATS events: %v", err)
	}
	log.Printf("Subscribed to NATS events with pattern: %s", eventSubjectPattern)

	// Создаем EventStore и SnapshotStore для Order агрегата
	eventStoreConfig := eventsourcing.DefaultPostgresEventStoreConfig()
	eventStoreConfig.DSN = cfg.Database.DSN

	orderEventStore, err := eventsourcing.NewPostgresEventStore(eventStoreConfig)
	if err != nil {
		log.Fatalf("Failed to create order event store: %v", err)
	}

	orderSnapshotStore, err := eventsourcing.NewPostgresSnapshotStore(eventStoreConfig)
	if err != nil {
		log.Fatalf("Failed to create order snapshot store: %v", err)
	}

	// Создаем репозиторий для Order агрегата
	orderRepoConfig := eventsourcing.DefaultRepositoryConfig()
	orderRepoConfig.SnapshotFrequency = 10
	orderRepo := eventsourcing.NewEventSourcedRepository(
		orderEventStore,
		orderSnapshotStore,
		orderRepoConfig,
		func(id string) *domain.Order {
			return domain.NewOrderWithID(id)
		},
	)

	// Создаем реестр саг
	registry := saga.NewSagaRegistry()

	// Создаем определение саги заказа
	orderSagaDef := application.NewOrderSagaDefinition(asyncCommandBus, eventAwaiter, eventBus, orderRepo)

	// Регистрируем сагу в реестре
	if err := registry.RegisterSaga("order_saga", orderSagaDef); err != nil {
		log.Fatalf("Failed to register order saga: %v", err)
	}

	// Настраиваем persistence с реестром
	if eventStorePersistence, ok := sagaPersistence.(*saga.EventStorePersistence); ok {
		eventStorePersistence.WithRegistry(registry)
	}

	// Создаем оркестратор саг
	orchestrator := saga.NewDefaultOrchestrator(sagaPersistence, eventBus)
	orchestrator.WithRegistry(registry)

	// Настраиваем Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API для работы с сагами
	api := router.Group("/api/v1")
	{
		// Создание заказа (запуск саги)
		api.POST("/orders", func(c *gin.Context) {
			var req struct {
				CustomerID string `json:"customer_id" binding:"required"`
				Items      []struct {
					ProductID string  `json:"product_id" binding:"required"`
					Quantity  int     `json:"quantity" binding:"required,min=1"`
					Price     float64 `json:"price" binding:"required,min=0"`
				} `json:"items" binding:"required,min=1"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Преобразуем items
			items := make([]domain.OrderItem, len(req.Items))
			for i, item := range req.Items {
				items[i] = domain.OrderItem{
					ProductID: item.ProductID,
					Quantity:  item.Quantity,
					Price:     item.Price,
				}
			}

			// Создаем контекст саги
			sagaCtx := saga.NewSagaContext()
			orderID := uuid.New().String()
			sagaCtx.Set("order_id", orderID)
			sagaCtx.Set("customer_id", req.CustomerID)
			sagaCtx.Set("items", items)

			// Вычисляем общую сумму
			totalAmount := 0.0
			for _, item := range items {
				totalAmount += item.Price * float64(item.Quantity)
			}
			sagaCtx.Set("total_amount", totalAmount)

			// Создаем экземпляр саги
			sagaInstance, err := registry.CreateInstanceWithPersistence(ctx, "order_saga", sagaCtx, sagaPersistence)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create saga instance: %v", err)})
				return
			}
			if sagaInstance == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create saga instance: returned nil"})
				return
			}

			// Запускаем сагу асинхронно
			go func() {
				if err := orchestrator.Execute(ctx, sagaInstance); err != nil {
					log.Printf("Saga execution failed: %v", err)
				}
			}()

			c.JSON(http.StatusAccepted, gin.H{
				"saga_id":  sagaInstance.ID(),
				"order_id": orderID,
				"status":   "pending",
				"message":  "Order creation started",
			})
		})

		// Получение статуса саги
		api.GET("/sagas/:id", func(c *gin.Context) {
			sagaID := c.Param("id")

			status, err := orchestrator.GetStatus(ctx, sagaID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}

			// Загружаем сагу для получения деталей
			sagaInstance, err := sagaPersistence.Load(ctx, sagaID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"saga_id":      sagaID,
				"status":       string(status),
				"current_step": sagaInstance.CurrentStep(),
				"context":      sagaInstance.Context().ToMap(),
			})
		})

		// Получение истории саги
		api.GET("/sagas/:id/history", func(c *gin.Context) {
			sagaID := c.Param("id")

			history, err := sagaPersistence.GetHistory(ctx, sagaID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}

			historyJSON := make([]map[string]interface{}, len(history))
			for i, h := range history {
				historyJSON[i] = map[string]interface{}{
					"step_name":     h.StepName,
					"status":        string(h.Status),
					"started_at":    h.StartedAt,
					"completed_at":  h.CompletedAt,
					"error":         nil,
					"retry_attempt": h.RetryAttempt,
				}
				if h.Error != nil {
					historyJSON[i]["error"] = h.Error.Error()
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"saga_id": sagaID,
				"history": historyJSON,
			})
		})

		// Отмена саги
		api.POST("/sagas/:id/cancel", func(c *gin.Context) {
			sagaID := c.Param("id")

			// Загружаем сагу
			sagaInstance, err := sagaPersistence.Load(ctx, sagaID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}

			// Запускаем компенсацию
			if err := orchestrator.Compensate(ctx, sagaInstance); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"saga_id": sagaID,
				"status":  "compensating",
				"message": "Saga cancellation started",
			})
		})

		// Возобновление саги
		api.POST("/sagas/:id/resume", func(c *gin.Context) {
			sagaID := c.Param("id")

			if err := orchestrator.Resume(ctx, sagaID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"saga_id": sagaID,
				"status":  "resumed",
				"message": "Saga resumed",
			})
		})
	}

	// Создаем HTTP сервер
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	// Запускаем сервер в горутине
	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Останавливаем EventAwaiter
	if err := eventAwaiter.Stop(shutdownCtx); err != nil {
		log.Printf("Failed to stop EventAwaiter: %v", err)
	}

	// Останавливаем InMemoryEventBus
	if err := eventBus.Shutdown(shutdownCtx); err != nil {
		log.Printf("Failed to shutdown InMemoryEventBus: %v", err)
	}

	log.Println("Server exited")
}

// extractEventTypeFromSubject извлекает тип события из NATS subject
// Формат subject: events.<eventType> или events.<aggregate>.<eventType>
func extractEventTypeFromSubject(subject, prefix string) string {
	if !strings.HasPrefix(subject, prefix+".") {
		return ""
	}

	// Убираем префикс
	withoutPrefix := strings.TrimPrefix(subject, prefix+".")

	// Если формат events.<eventType>, возвращаем eventType
	// Если формат events.<aggregate>.<eventType>, возвращаем eventType (последняя часть)
	parts := strings.Split(withoutPrefix, ".")
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) >= 2 {
		// Возвращаем последнюю часть как тип события
		return parts[len(parts)-1]
	}

	return ""
}

// getStringFromMap безопасно извлекает строковое значение из map
func getStringFromMap(m map[string]interface{}, key string) (string, bool) {
	val, ok := m[key]
	if !ok {
		return "", false
	}
	if str, ok := val.(string); ok {
		return str, true
	}
	return "", false
}
