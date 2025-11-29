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
	"github.com/akriventsev/potter/examples/eventsourcing-snapshots/domain"
	"github.com/akriventsev/potter/examples/eventsourcing-snapshots/infrastructure"
	"github.com/akriventsev/potter/framework/eventsourcing"
)

type Config struct {
	Server struct {
		Port string
	}
	Database struct {
		DSN string
	}
	SnapshotStrategy string
	SnapshotFrequency int
	SnapshotInterval  time.Duration
}

func loadConfig() *Config {
	cfg := &Config{}
	cfg.Server.Port = getEnv("SERVER_PORT", "8080")
	cfg.Database.DSN = getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/eventsourcing_snapshots?sslmode=disable")
	cfg.SnapshotStrategy = getEnv("SNAPSHOT_STRATEGY", "frequency") // frequency, timebased, hybrid
	cfg.SnapshotFrequency = 10 // Снапшот каждые 10 событий
	cfg.SnapshotInterval = 1 * time.Hour // Снапшот каждый час
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
		log.Printf("Warning: Failed to apply migrations: %v", err)
	}

	eventStoreConfig := eventsourcing.DefaultPostgresEventStoreConfig()
	eventStoreConfig.DSN = cfg.Database.DSN

	eventStore, err := eventsourcing.NewPostgresEventStore(eventStoreConfig)
	if err != nil {
		log.Fatalf("Failed to create event store: %v", err)
	}

	snapshotStore, err := eventsourcing.NewPostgresSnapshotStore(eventStoreConfig)
	if err != nil {
		log.Fatalf("Failed to create snapshot store: %v", err)
	}

	// Создаем репозиторий с выбранной стратегией снапшотов
	var productRepo *eventsourcing.EventSourcedRepository[*domain.Product]
	switch cfg.SnapshotStrategy {
	case "frequency":
		productRepo = infrastructure.NewProductRepositoryWithFrequencyStrategy(
			eventStore, snapshotStore, int64(cfg.SnapshotFrequency))
		log.Printf("Using FrequencySnapshotStrategy (every %d events)", cfg.SnapshotFrequency)
	case "timebased":
		productRepo = infrastructure.NewProductRepositoryWithTimeBasedStrategy(
			eventStore, snapshotStore, cfg.SnapshotInterval)
		log.Printf("Using TimeBasedSnapshotStrategy (every %v)", cfg.SnapshotInterval)
	case "hybrid":
		productRepo = infrastructure.NewProductRepositoryWithHybridStrategy(
			eventStore, snapshotStore, int64(cfg.SnapshotFrequency), cfg.SnapshotInterval)
		log.Printf("Using HybridSnapshotStrategy (every %d events OR every %v)", cfg.SnapshotFrequency, cfg.SnapshotInterval)
	default:
		productRepo = infrastructure.NewProductRepositoryWithFrequencyStrategy(
			eventStore, snapshotStore, int64(cfg.SnapshotFrequency))
		log.Printf("Using default FrequencySnapshotStrategy (every %d events)", cfg.SnapshotFrequency)
	}

	router := gin.Default()

	router.POST("/products", func(c *gin.Context) {
		var req struct {
			Name  string  `json:"name"`
			Price float64 `json:"price"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		productID := uuid.New().String()
		product := domain.NewProduct(productID, req.Name, req.Price)

		if err := productRepo.Save(ctx, product); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":    productID,
			"name":  product.GetName(),
			"price": product.GetPrice(),
		})
	})

	router.PUT("/products/:id/price", func(c *gin.Context) {
		productID := c.Param("id")
		var req struct {
			Price float64 `json:"price"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		product, err := productRepo.GetByID(ctx, productID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		if err := product.UpdatePrice(req.Price); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := productRepo.Save(ctx, product); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":    productID,
			"price": product.GetPrice(),
		})
	})

	router.GET("/products/:id", func(c *gin.Context) {
		productID := c.Param("id")
		product, err := productRepo.GetByID(ctx, productID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":          productID,
			"name":        product.GetName(),
			"price":       product.GetPrice(),
			"stock":       product.GetStock(),
			"version":     product.Version(),
		})
	})

	// Статистика снапшотов
	router.GET("/snapshots/stats", func(c *gin.Context) {
		productID := c.Query("product_id")
		if productID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "product_id is required"})
			return
		}

		version, err := productRepo.GetVersion(ctx, productID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		// Проверяем наличие снапшота
		snapshot, err := snapshotStore.GetSnapshot(ctx, productID)
		hasSnapshot := err == nil && snapshot != nil

		c.JSON(http.StatusOK, gin.H{
			"product_id":   productID,
			"version":      version,
			"has_snapshot": hasSnapshot,
			"strategy":     cfg.SnapshotStrategy,
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

