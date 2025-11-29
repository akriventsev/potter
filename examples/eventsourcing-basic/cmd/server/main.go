package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"potter/examples/eventsourcing-basic/application"
	"potter/examples/eventsourcing-basic/infrastructure"
	"potter/framework/eventsourcing"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализация инфраструктуры
	eventStore, err := infrastructure.NewEventStore(ctx)
	if err != nil {
		log.Fatalf("Failed to create event store: %v", err)
	}

	snapshotStore, err := infrastructure.NewSnapshotStore(ctx)
	if err != nil {
		log.Fatalf("Failed to create snapshot store: %v", err)
	}

	// Создание репозитория
	config := eventsourcing.DefaultRepositoryConfig()
	config.UseSnapshots = true
	config.SnapshotFrequency = 100

	factory := func(id string) *application.BankAccountAggregate {
		return application.NewBankAccountAggregate(id)
	}

	repo := eventsourcing.NewEventSourcedRepository(
		eventStore,
		snapshotStore,
		config,
		factory,
	)

	// Создание handlers
	handler := application.NewHandler(repo)

	// Настройка HTTP сервера
	mux := http.NewServeMux()
	mux.HandleFunc("/accounts", handler.CreateAccount)
	mux.HandleFunc("/accounts/", handler.HandleAccount)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		cancel()
	}()

	log.Println("Server starting on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
