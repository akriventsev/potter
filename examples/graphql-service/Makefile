.PHONY: generate build run docker-up docker-down migrate-up migrate-down test test-integration lint clean playground help

# Переменные
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/graphql_service?sslmode=disable
NATS_URL ?= nats://localhost:4222

# Генерация кода из proto
generate:
	@echo "Generating code from proto files..."
	potter-gen generate --proto api/proto/product_service.proto --with-graphql --output .
	@echo "Running gqlgen generate..."
	cd api/graphql && gqlgen generate

# Сборка приложения
build:
	@echo "Building application..."
	go build -o bin/server cmd/server/main.go

# Запуск приложения
run:
	@echo "Running application..."
	go run cmd/server/main.go

# Запуск Docker Compose
docker-up:
	@echo "Starting Docker Compose..."
	docker-compose up -d

# Остановка Docker Compose
docker-down:
	@echo "Stopping Docker Compose..."
	docker-compose down

# Применение миграций
migrate-up:
	@echo "Applying migrations..."
	goose -dir migrations postgres "$(DATABASE_URL)" up

# Откат миграций
migrate-down:
	@echo "Rolling back migrations..."
	goose -dir migrations postgres "$(DATABASE_URL)" down

# Запуск тестов
test:
	@echo "Running tests..."
	go test -v ./...

# Integration тесты
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./...

# Линтинг кода
lint:
	@echo "Running linter..."
	golangci-lint run

# Очистка сгенерированных файлов
clean:
	@echo "Cleaning generated files..."
	rm -rf bin/
	rm -rf api/graphql/generated.go
	rm -rf api/graphql/models_gen.go

# Открыть GraphQL Playground
playground:
	@echo "Opening GraphQL Playground..."
	open http://localhost:8082/playground

# Справка по командам
help:
	@echo "Available commands:"
	@echo "  make generate        - Generate code from proto files"
	@echo "  make build          - Build application"
	@echo "  make run            - Run application"
	@echo "  make docker-up      - Start Docker Compose"
	@echo "  make docker-down    - Stop Docker Compose"
	@echo "  make migrate-up     - Apply migrations"
	@echo "  make migrate-down   - Rollback migrations"
	@echo "  make test           - Run tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make lint           - Run linter"
	@echo "  make clean          - Clean generated files"
	@echo "  make playground     - Open GraphQL Playground"
	@echo "  make help           - Show this help message"

