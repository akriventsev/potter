.PHONY: run docker-up docker-down migrate test clean help

# Запуск приложения
run:
	@echo "Running Event Sourcing basic example..."
	@go run ./cmd/server/main.go

# Запуск Docker инфраструктуры
docker-up:
	@echo "Starting Docker infrastructure..."
	@docker-compose up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3

# Остановка Docker инфраструктуры
docker-down:
	@echo "Stopping Docker infrastructure..."
	@docker-compose down

# Применение миграций
migrate:
	@echo "Running migrations..."
	@PGPASSWORD=postgres psql -h localhost -U postgres -d potter -f migrations/001_create_event_store.sql || true

# Тестирование
test:
	@echo "Running tests..."
	@go test -v ./...

# Очистка
clean:
	@echo "Cleaning..."
	@go clean

# Справка
help:
	@echo "Available commands:"
	@echo "  make run         - Run the application"
	@echo "  make docker-up   - Start Docker infrastructure"
	@echo "  make docker-down - Stop Docker infrastructure"
	@echo "  make migrate     - Run database migrations"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make help        - Show this help message"

