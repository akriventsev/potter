package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"potter/examples/eventsourcing-replay/application"
	"potter/examples/eventsourcing-replay/infrastructure"
	"potter/examples/eventsourcing-replay/projections"
	"potter/framework/eventsourcing"
)

func main() {
	var (
		dsn          = flag.String("dsn", "postgres://postgres:postgres@localhost:5432/eventsourcing_replay?sslmode=disable", "Database connection string")
		command      = flag.String("command", "", "Command: replay-all, replay-aggregate, replay-projection, replay-from")
		aggregateID  = flag.String("aggregate-id", "", "Aggregate ID for replay-aggregate")
		projection   = flag.String("projection", "", "Projection name for replay-projection")
		fromTime     = flag.String("from-time", "", "Start time for replay-from (RFC3339 format)")
		batchSize    = flag.Int("batch-size", 1000, "Batch size for replay")
		parallel     = flag.Bool("parallel", false, "Enable parallel processing")
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
	defer eventStore.Stop(ctx)

	snapshotStore, err := eventsourcing.NewPostgresSnapshotStore(eventStoreConfig)
	if err != nil {
		log.Fatalf("Failed to create snapshot store: %v", err)
	}

	// Создаем replayer
	replayer := eventsourcing.NewDefaultEventReplayer(eventStore, snapshotStore)

	// Создаем проекции
	orderSummaryProjection := projections.NewOrderSummaryProjection()
	customerStatsProjection := projections.NewCustomerStatsProjection()

	// Опции для replay
	options := eventsourcing.DefaultReplayOptions()
	options.BatchSize = *batchSize
	options.Parallel = *parallel

	// Callback для отслеживания прогресса
	progressCallback := func(progress eventsourcing.ReplayProgress) {
		percent := float64(progress.ProcessedEvents) / float64(progress.TotalEvents) * 100
		fmt.Printf("\rProgress: %d/%d (%.2f%%) | Position: %d | Elapsed: %v",
			progress.ProcessedEvents,
			progress.TotalEvents,
			percent,
			progress.CurrentPosition,
			progress.ElapsedTime,
		)
	}

	switch *command {
	case "replay-all":
		fmt.Println("Starting full replay of all events...")
		handler := &application.ProjectionHandler{
			OrderSummary:   orderSummaryProjection,
			CustomerStats:  customerStatsProjection,
		}
		err = replayer.ReplayWithProgress(ctx, handler, 0, options, progressCallback)
		if err != nil {
			log.Fatalf("Replay failed: %v", err)
		}
		fmt.Println("\n✓ Full replay completed successfully")

	case "replay-aggregate":
		if *aggregateID == "" {
			log.Fatal("aggregate-id is required for replay-aggregate")
		}
		fmt.Printf("Starting replay for aggregate: %s\n", *aggregateID)
		err = replayer.ReplayAggregate(ctx, *aggregateID, 0)
		if err != nil {
			log.Fatalf("Replay failed: %v", err)
		}
		fmt.Println("✓ Aggregate replay completed successfully")

	case "replay-projection":
		if *projection == "" {
			log.Fatal("projection is required for replay-projection")
		}
		fmt.Printf("Starting replay for projection: %s\n", *projection)
		var handler eventsourcing.ReplayHandler
		switch *projection {
		case "order_summary":
			handler = &application.ProjectionHandler{
				OrderSummary: orderSummaryProjection,
			}
		case "customer_stats":
			handler = &application.ProjectionHandler{
				CustomerStats: customerStatsProjection,
			}
		default:
			log.Fatalf("Unknown projection: %s", *projection)
		}
		err = replayer.ReplayWithProgress(ctx, handler, 0, options, progressCallback)
		if err != nil {
			log.Fatalf("Replay failed: %v", err)
		}
		fmt.Println("\n✓ Projection replay completed successfully")

	case "replay-from":
		if *fromTime == "" {
			log.Fatal("from-time is required for replay-from")
		}
		fromTimestamp, err := time.Parse(time.RFC3339, *fromTime)
		if err != nil {
			log.Fatalf("Invalid time format: %v (expected RFC3339)", err)
		}
		fmt.Printf("Starting replay from: %s\n", fromTimestamp.Format(time.RFC3339))
		handler := &application.ProjectionHandler{
			OrderSummary:  orderSummaryProjection,
			CustomerStats: customerStatsProjection,
		}
		// Используем GetAllEvents с фильтрацией по времени через канал
		eventChan, err := eventStore.GetAllEvents(ctx, 0)
		if err != nil {
			log.Fatalf("Failed to get events: %v", err)
		}
		count := 0
		for event := range eventChan {
			if event.OccurredAt.After(fromTimestamp) || event.OccurredAt.Equal(fromTimestamp) {
				if err := handler.HandleEvent(ctx, event); err != nil {
					log.Printf("Error processing event: %v", err)
				}
				count++
			}
		}
		fmt.Printf("✓ Replay from time completed successfully (processed %d events)\n", count)

	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}

