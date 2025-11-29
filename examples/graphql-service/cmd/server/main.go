package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/vektah/gqlparser/v2/ast"
	pottertransport "github.com/akriventsev/potter/framework/adapters/transport"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/transport"
)

func main() {
	// Загрузка конфигурации из переменных окружения
	config := loadConfig()

	// Инициализация подключений
	ctx := context.Background()

	// PostgreSQL подключение
	dbPool, err := initPostgreSQL(ctx, config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer dbPool.Close()

	// NATS подключение
	nc, err := initNATS(config.NATSURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	// Создание buses
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()
	eventBus := events.NewInMemoryEventBus()

	// TODO: Регистрация command handlers
	// TODO: Регистрация query handlers

	// Создание базовой GraphQL схемы
	// В реальной реализации схема должна быть сгенерирована из proto файлов
	// с помощью potter-gen и gqlgen
	baseSchema := createMinimalSchema()

	// Создание GraphQL адаптера с интеграцией CQRS
	graphQLConfig := pottertransport.DefaultGraphQLConfig()
	graphQLConfig.Port = config.GraphQLPort
	graphQLConfig.EnablePlayground = config.EnablePlayground
	graphQLConfig.EnableIntrospection = config.EnableIntrospection
	graphQLConfig.ComplexityLimit = 1000
	graphQLConfig.MaxDepth = 15

	adapter, err := pottertransport.NewGraphQLAdapterWithCQRS(
		graphQLConfig,
		commandBus,
		queryBus,
		eventBus,
		baseSchema,
	)
	if err != nil {
		log.Fatalf("Failed to create GraphQL adapter: %v", err)
	}

	// Запуск GraphQL сервера
	if err := adapter.Start(ctx); err != nil {
		log.Fatalf("Failed to start GraphQL adapter: %v", err)
	}
	defer adapter.Stop(ctx)

	log.Printf("GraphQL server started on port %d", graphQLConfig.Port)
	log.Printf("GraphQL Playground: http://localhost:%d%s", graphQLConfig.Port, graphQLConfig.PlaygroundPath)
	log.Printf("GraphQL endpoint: http://localhost:%d%s", graphQLConfig.Port, graphQLConfig.Path)

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := adapter.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping adapter: %v", err)
	}

	log.Println("Server stopped")
}

// Config конфигурация приложения
type Config struct {
	DatabaseURL          string
	NATSURL              string
	GraphQLPort          int
	ServerPort           int
	EnablePlayground    bool
	EnableIntrospection  bool
}

// loadConfig загружает конфигурацию из переменных окружения
func loadConfig() Config {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/graphql_service?sslmode=disable"
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	enablePlayground := os.Getenv("GRAPHQL_PLAYGROUND_ENABLED") != "false"
	enableIntrospection := os.Getenv("GRAPHQL_INTROSPECTION_ENABLED") != "false"

	return Config{
		DatabaseURL:         databaseURL,
		NATSURL:             natsURL,
		GraphQLPort:         8082,
		ServerPort:          8080,
		EnablePlayground:    enablePlayground,
		EnableIntrospection: enableIntrospection,
	}
}

// initPostgreSQL инициализирует подключение к PostgreSQL
func initPostgreSQL(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Проверка подключения
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// initNATS инициализирует подключение к NATS
func initNATS(url string) (*nats.Conn, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return nc, nil
}

// createMinimalSchema создает минимальную GraphQL схему для примера
// В реальной реализации схема должна быть сгенерирована из proto файлов
func createMinimalSchema() graphql.ExecutableSchema {
	return &minimalSchema{}
}

// minimalSchema минимальная реализация ExecutableSchema для примера
type minimalSchema struct{}

func (s *minimalSchema) Schema() *ast.Schema {
	return &ast.Schema{}
}

func (s *minimalSchema) Complexity(typeName, fieldName string, childComplexity int, args map[string]any) (int, bool) {
	return childComplexity, true
}

func (s *minimalSchema) Exec(ctx context.Context) graphql.ResponseHandler {
	return graphql.OneShot(graphql.ErrorResponse(ctx, "operations not implemented in example"))
}

