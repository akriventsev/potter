package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/akriventsev/potter/examples/saga-query-handler/application"
	"github.com/akriventsev/potter/examples/saga-query-handler/infrastructure"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/eventsourcing"
	"github.com/akriventsev/potter/framework/saga"
	"github.com/akriventsev/potter/framework/transport"

	"github.com/gin-gonic/gin"
)

type Config struct {
	Server struct {
		Port string
	}
	Database struct {
		DSN string
	}
}

func loadConfig() *Config {
	cfg := &Config{}
	cfg.Server.Port = getEnv("PORT", "8080")
	cfg.Database.DSN = getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/saga_query_handler?sslmode=disable")
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

	// Создаем EventStore
	eventStoreConfig := eventsourcing.DefaultPostgresEventStoreConfig()
	eventStoreConfig.DSN = cfg.Database.DSN
	eventStore, err := eventsourcing.NewPostgresEventStore(eventStoreConfig)
	if err != nil {
		log.Fatalf("Failed to create event store: %v", err)
	}

	// Создаем SagaPersistence
	sagaPersistence, err := infrastructure.NewSagaEventStorePersistence(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("Failed to create saga persistence: %v", err)
	}

	// Создаем CheckpointStore
	checkpointStore, err := infrastructure.NewPostgresCheckpointStore(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("Failed to create checkpoint store: %v", err)
	}

	// Создаем ReadModelStore
	readModelStore, err := infrastructure.NewPostgresSagaReadModelStore(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("Failed to create read model store: %v", err)
	}

	// Создаем EventBus
	eventBus := events.NewInMemoryEventBus()

	// Создаем SagaReadModelProjection
	projection := saga.NewSagaReadModelProjection(readModelStore)

	// Создаем ProjectionManager
	projectionManager := eventsourcing.NewProjectionManager(eventStore, checkpointStore)
	if err := projectionManager.Register(projection); err != nil {
		log.Fatalf("Failed to register projection: %v", err)
	}

	// Запускаем ProjectionManager
	if err := projectionManager.Start(ctx); err != nil {
		log.Fatalf("Failed to start projection manager: %v", err)
	}
	defer func() {
		if err := projectionManager.Stop(ctx); err != nil {
			log.Printf("Failed to stop projection manager: %v", err)
		}
	}()

	// Создаем SagaOrchestrator
	sagaDefinition := application.NewSimpleSagaDefinition()
	orchestrator := saga.NewDefaultOrchestrator(sagaPersistence, eventBus)
	if err := orchestrator.RegisterSaga(sagaDefinition.Name(), sagaDefinition); err != nil {
		log.Fatalf("Failed to register saga definition: %v", err)
	}

	// Создаем QueryBus и QueryHandler
	queryBus := transport.NewInMemoryQueryBus()
	queryHandler := saga.NewSagaQueryHandler(sagaPersistence, readModelStore)
	if err := queryBus.Register(queryHandler); err != nil {
		log.Fatalf("Failed to register query handler: %v", err)
	}

	// Настраиваем Gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// REST API endpoints
	api := router.Group("/api/v1")
	{
		// Создание саги
		api.POST("/sagas", createSagaHandler(ctx, orchestrator))

		// Получение статуса саги
		api.GET("/sagas/:id", getSagaStatusHandler(queryBus))

		// Получение истории саги
		api.GET("/sagas/:id/history", getSagaHistoryHandler(queryBus))

		// Список саг с фильтрацией
		api.GET("/sagas", listSagasHandler(queryBus))

		// Метрики саг
		api.GET("/sagas/metrics", getSagaMetricsHandler(queryBus))
	}

	// Запускаем сервер
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

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
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// createSagaHandler создает новую сагу
func createSagaHandler(appCtx context.Context, orchestrator *saga.DefaultOrchestrator) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			DefinitionName string                 `json:"definition_name"`
			CorrelationID  string                 `json:"correlation_id"`
			Context        map[string]interface{} `json:"context"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		sagaCtx := saga.NewSagaContext()
		if req.CorrelationID != "" {
			sagaCtx.SetCorrelationID(req.CorrelationID)
		}
		for k, v := range req.Context {
			sagaCtx.Set(k, v)
		}

		sagaInstance, err := orchestrator.StartSaga(appCtx, req.DefinitionName, sagaCtx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"saga_id": sagaInstance.ID(),
			"status":  sagaInstance.Status(),
		})
	}
}

// getSagaStatusHandler получает статус саги
func getSagaStatusHandler(queryBus transport.QueryBus) gin.HandlerFunc {
	return func(c *gin.Context) {
		sagaID := c.Param("id")
		query := &saga.GetSagaStatusQuery{SagaID: sagaID}

		result, err := queryBus.Ask(c.Request.Context(), query)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

// getSagaHistoryHandler получает историю саги
func getSagaHistoryHandler(queryBus transport.QueryBus) gin.HandlerFunc {
	return func(c *gin.Context) {
		sagaID := c.Param("id")
		query := &saga.GetSagaHistoryQuery{SagaID: sagaID}

		result, err := queryBus.Ask(c.Request.Context(), query)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

// listSagasHandler получает список саг с фильтрацией
func listSagasHandler(queryBus transport.QueryBus) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := &saga.ListSagasQuery{}

		// Парсим параметры запроса
		if status := c.Query("status"); status != "" {
			sagaStatus := saga.SagaStatus(status)
			query.Status = &sagaStatus
		}
		if definitionName := c.Query("definition_name"); definitionName != "" {
			query.DefinitionName = &definitionName
		}
		if correlationID := c.Query("correlation_id"); correlationID != "" {
			query.CorrelationID = &correlationID
		}
		if limitStr := c.Query("limit"); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil {
				query.Limit = limit
			}
		}
		if query.Limit == 0 {
			query.Limit = 10 // дефолт
		}
		if offsetStr := c.Query("offset"); offsetStr != "" {
			if offset, err := strconv.Atoi(offsetStr); err == nil {
				query.Offset = offset
			}
		}

		result, err := queryBus.Ask(c.Request.Context(), query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

// getSagaMetricsHandler получает метрики саг
func getSagaMetricsHandler(queryBus transport.QueryBus) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := &saga.GetSagaMetricsQuery{}

		if definitionName := c.Query("definition_name"); definitionName != "" {
			query.DefinitionName = &definitionName
		}

		result, err := queryBus.Ask(c.Request.Context(), query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}
