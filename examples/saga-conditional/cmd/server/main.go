package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/akriventsev/potter/examples/saga-conditional/application"
	"github.com/akriventsev/potter/examples/saga-conditional/infrastructure"
	adapterevents "github.com/akriventsev/potter/framework/adapters/events"
	"github.com/akriventsev/potter/framework/adapters/messagebus"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/saga"
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
}

func loadConfig() *Config {
	cfg := &Config{}
	cfg.Server.Port = getEnv("SERVER_PORT", "8080")
	cfg.Database.DSN = getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/saga_conditional?sslmode=disable")
	cfg.NATS.URL = getEnv("NATS_URL", "nats://localhost:4222")
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

	if err := infrastructure.RunMigrations(cfg.Database.DSN); err != nil {
		log.Fatalf("Failed to apply migrations: %v", err)
	}

	sagaPersistence, err := infrastructure.NewSagaEventStorePersistence(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("Failed to create saga persistence: %v", err)
	}

	natsAdapter, err := messagebus.NewNATSAdapter(cfg.NATS.URL)
	if err != nil {
		log.Fatalf("Failed to create NATS adapter: %v", err)
	}

	if err := natsAdapter.Start(ctx); err != nil {
		log.Fatalf("Failed to start NATS adapter: %v", err)
	}
	defer natsAdapter.Stop(ctx)

	eventConfig := adapterevents.NATSEventConfig{
		Conn:          natsAdapter.Conn(),
		SubjectPrefix: "events",
	}
	eventPublisher, err := adapterevents.NewNATSEventAdapter(eventConfig)
	if err != nil {
		log.Fatalf("Failed to create event publisher: %v", err)
	}

	if err := eventPublisher.Start(ctx); err != nil {
		log.Fatalf("Failed to start event publisher: %v", err)
	}
	defer eventPublisher.Stop(ctx)

	eventBus := events.NewInMemoryEventBus()

	asyncCommandBus := invoke.NewAsyncCommandBus(natsAdapter)
	eventAwaiter := invoke.NewEventAwaiterFromEventBus(eventBus)

	registry := saga.NewSagaRegistry()
	conditionalSagaDef := application.NewConditionalSagaDefinition(asyncCommandBus, eventAwaiter)

	if err := registry.RegisterSaga("conditional_order_saga", conditionalSagaDef); err != nil {
		log.Fatalf("Failed to register conditional saga: %v", err)
	}

	if eventStorePersistence, ok := sagaPersistence.(*saga.EventStorePersistence); ok {
		eventStorePersistence.WithRegistry(registry)
	}

	orchestrator := saga.NewDefaultOrchestrator(sagaPersistence, eventBus)
	orchestrator.WithRegistry(registry)

	router := gin.Default()

	router.POST("/orders", func(c *gin.Context) {
		var req struct {
			CustomerID  string  `json:"customer_id"`
			Amount      float64 `json:"amount"`
			CustomerType string `json:"customer_type"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		sagaCtx := saga.NewSagaContext()
		orderID := uuid.New().String()
		sagaCtx.Set("order_id", orderID)
		sagaCtx.Set("customer_id", req.CustomerID)
		sagaCtx.Set("amount", req.Amount)
		sagaCtx.Set("customer_type", req.CustomerType)
		sagaCtx.SetCorrelationID(uuid.New().String())

		sagaInstance, err := orchestrator.StartSaga(ctx, "conditional_order_saga", sagaCtx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"saga_id":  sagaInstance.ID(),
			"order_id": orderID,
			"status":   sagaInstance.Status(),
		})
	})

	router.GET("/orders/:saga_id", func(c *gin.Context) {
		sagaID := c.Param("saga_id")
		sagaInstance, err := sagaPersistence.Load(ctx, sagaID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		history := sagaInstance.GetHistory()
		historyJSON, _ := json.Marshal(history)

		c.JSON(http.StatusOK, gin.H{
			"saga_id": sagaInstance.ID(),
			"status":  sagaInstance.Status(),
			"history": json.RawMessage(historyJSON),
		})
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}

