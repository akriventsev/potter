package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/akriventsev/potter/examples/eventsourcing-replay/infrastructure"
	"github.com/akriventsev/potter/examples/eventsourcing-replay/projections"
	"github.com/akriventsev/potter/framework/eventsourcing"
)

func main() {
	var (
		dsn         = flag.String("dsn", "postgres://postgres:postgres@localhost:5432/eventsourcing_replay?sslmode=disable", "Database connection string")
		command     = flag.String("command", "", "Command: replay-all, replay-aggregate, replay-projection, replay-from")
		aggregateID = flag.String("aggregate-id", "", "Aggregate ID for replay-aggregate")
		projection  = flag.String("projection", "", "Projection name for replay-projection")
		fromTime    = flag.String("from-time", "", "Start time for replay-from (RFC3339 format)")
	)
	flag.Parse()

	if *command == "" {
		fmt.Println("Usage: replay [options]")
		fmt.Println("\nCommands:")
		fmt.Println("  replay-all          - Replay all events")
		fmt.Println("  replay-aggregate    - Replay events for specific aggregate")
		fmt.Println("  replay-projection   - Rebuild specific projection")
		fmt.Println("  replay-from         - Replay events from specific time")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	ctx := context.Background()

	// Применяем миграции
	if err := infrastructure.RunMigrations(*dsn); err != nil {
		log.Printf("Warning: Failed to apply migrations: %v", err)
	}

	// Создаем event store
	eventStoreConfig := eventsourcing.DefaultPostgresEventStoreConfig()
	eventStoreConfig.DSN = *dsn

	eventStore, err := eventsourcing.NewPostgresEventStore(eventStoreConfig)
	if err != nil {
		log.Fatalf("Failed to create event store: %v", err)
	}

	// Создаем checkpoint store
	checkpointStore, err := eventsourcing.NewPostgresCheckpointStore(*dsn)
	if err != nil {
		log.Fatalf("Failed to create checkpoint store: %v", err)
	}

	// Создаем проекции
	orderSummaryProjection := projections.NewOrderSummaryProjection()
	customerStatsProjection := projections.NewCustomerStatsProjection()

	// Создаем ProjectionManager
	projectionManager := eventsourcing.NewProjectionManager(eventStore, checkpointStore)
	if err := projectionManager.Register(orderSummaryProjection); err != nil {
		log.Fatalf("Failed to register order summary projection: %v", err)
	}
	if err := projectionManager.Register(customerStatsProjection); err != nil {
		log.Fatalf("Failed to register customer stats projection: %v", err)
	}

	switch *command {
	case "replay-all":
		fmt.Println("Starting full replay of all events using ProjectionManager...")
		// Запускаем ProjectionManager для обработки всех событий
		if err := projectionManager.Start(ctx); err != nil {
			log.Fatalf("Failed to start projection manager: %v", err)
		}
		// Ждем завершения обработки всех событий
		// В реальном приложении это будет работать в фоне
		fmt.Println("ProjectionManager started. Processing events...")
		time.Sleep(5 * time.Second) // Даем время на обработку
		if err := projectionManager.Stop(ctx); err != nil {
			log.Printf("Warning: Failed to stop projection manager: %v", err)
		}
		fmt.Println("\n✓ Full replay completed successfully")

	case "replay-aggregate":
		if *aggregateID == "" {
			log.Fatal("aggregate-id is required for replay-aggregate")
		}
		fmt.Printf("Starting replay for aggregate: %s\n", *aggregateID)
		// Получаем события агрегата и обрабатываем через проекции
		events, err := eventStore.GetEvents(ctx, *aggregateID, 0)
		if err != nil {
			log.Fatalf("Failed to get events: %v", err)
		}
		for _, event := range events {
			if err := orderSummaryProjection.HandleEvent(ctx, event); err != nil {
				log.Printf("Error processing event in order summary: %v", err)
			}
			if err := customerStatsProjection.HandleEvent(ctx, event); err != nil {
				log.Printf("Error processing event in customer stats: %v", err)
			}
		}
		fmt.Println("✓ Aggregate replay completed successfully")

	case "replay-projection":
		if *projection == "" {
			log.Fatal("projection is required for replay-projection")
		}
		fmt.Printf("Starting rebuild for projection: %s\n", *projection)
		var projectionName string
		switch *projection {
		case "order_summary":
			projectionName = orderSummaryProjection.Name()
		case "customer_stats":
			projectionName = customerStatsProjection.Name()
		default:
			log.Fatalf("Unknown projection: %s", *projection)
		}
		// Используем Rebuild для пересоздания проекции
		if err := projectionManager.Rebuild(ctx, projectionName); err != nil {
			log.Fatalf("Rebuild failed: %v", err)
		}
		fmt.Println("\n✓ Projection rebuild completed successfully")

	case "replay-from":
		if *fromTime == "" {
			log.Fatal("from-time is required for replay-from")
		}
		fromTimestamp, err := time.Parse(time.RFC3339, *fromTime)
		if err != nil {
			log.Fatalf("Invalid time format: %v (expected RFC3339)", err)
		}
		fmt.Printf("Starting replay from: %s\n", fromTimestamp.Format(time.RFC3339))
		// Получаем все события и фильтруем по времени
		eventChan, err := eventStore.GetAllEvents(ctx, 0)
		if err != nil {
			log.Fatalf("Failed to get events: %v", err)
		}
		count := 0
		for event := range eventChan {
			if event.OccurredAt.After(fromTimestamp) || event.OccurredAt.Equal(fromTimestamp) {
				if err := orderSummaryProjection.HandleEvent(ctx, event); err != nil {
					log.Printf("Error processing event in order summary: %v", err)
				}
				if err := customerStatsProjection.HandleEvent(ctx, event); err != nil {
					log.Printf("Error processing event in customer stats: %v", err)
				}
				count++
			}
		}
		fmt.Printf("✓ Replay from time completed successfully (processed %d events)\n", count)

	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}
