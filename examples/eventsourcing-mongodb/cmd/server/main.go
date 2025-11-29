package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/akriventsev/potter/examples/eventsourcing-mongodb/application"
	"github.com/akriventsev/potter/examples/eventsourcing-mongodb/domain"
	"github.com/akriventsev/potter/examples/eventsourcing-mongodb/infrastructure"
)

type Config struct {
	Server struct {
		Port string
	}
	MongoDB struct {
		URI      string
		Database string
	}
}

func loadConfig() *Config {
	cfg := &Config{}
	cfg.Server.Port = getEnv("SERVER_PORT", "8080")
	cfg.MongoDB.URI = getEnv("MONGODB_URI", "mongodb://localhost:27017")
	cfg.MongoDB.Database = getEnv("MONGODB_DATABASE", "eventsourcing_mongodb")
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

	// Создаем MongoDB event store и snapshot store
	eventStore, snapshotStore, err := infrastructure.NewMongoDBStores(cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		log.Fatalf("Failed to create MongoDB stores: %v", err)
	}
	defer func() {
		if stop, ok := eventStore.(interface{ Stop(context.Context) error }); ok {
			_ = stop.Stop(ctx)
		}
	}()

	// Создаем репозиторий для Inventory
	inventoryRepo := application.NewInventoryRepository(eventStore, snapshotStore)

	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Добавление товара на склад
	router.POST("/inventory/items", func(c *gin.Context) {
		var req struct {
			ProductID  string `json:"product_id" binding:"required"`
			WarehouseID string `json:"warehouse_id" binding:"required"`
			Quantity   int    `json:"quantity" binding:"required,min=1"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		inventoryID := uuid.New().String()
		inventory := domain.NewInventory(inventoryID, req.ProductID, req.WarehouseID, req.Quantity)

		if err := inventoryRepo.Save(ctx, inventory); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":          inventoryID,
			"product_id":  req.ProductID,
			"warehouse_id": req.WarehouseID,
			"quantity":    inventory.GetQuantity(),
		})
	})

	// Резервирование товара
	router.POST("/inventory/reserve", func(c *gin.Context) {
		var req struct {
			InventoryID string `json:"inventory_id" binding:"required"`
			Quantity    int    `json:"quantity" binding:"required,min=1"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		inventory, err := inventoryRepo.GetByID(ctx, req.InventoryID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		if err := inventory.Reserve(req.Quantity); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := inventoryRepo.Save(ctx, inventory); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"inventory_id": req.InventoryID,
			"quantity":     inventory.GetQuantity(),
		})
	})

	// Получение информации о товаре
	router.GET("/inventory/:id", func(c *gin.Context) {
		inventoryID := c.Param("id")
		inventory, err := inventoryRepo.GetByID(ctx, inventoryID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":          inventoryID,
			"quantity":    inventory.GetQuantity(),
			"version":     inventory.Version(),
		})
	})

	// Получение событий по типу (демонстрация BSON запросов)
	router.GET("/events/by-type", func(c *gin.Context) {
		eventType := c.Query("type")
		if eventType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "type parameter is required"})
			return
		}

		fromTime := time.Now().Add(-24 * time.Hour) // Последние 24 часа
		events, err := eventStore.GetEventsByType(ctx, eventType, fromTime)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"event_type": eventType,
			"count":      len(events),
			"events":     events,
		})
	})

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

