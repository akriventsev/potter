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
migrate: migrate-up ## Алиас для migrate-up

migrate-up: ## Применить миграции
	@echo "Running migrations..."
	@which goose > /dev/null || (echo "goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest" && exit 1)
	@goose -dir migrations postgres "postgres://postgres:postgres@localhost:5432/potter?sslmode=disable" up

migrate-down: ## Откатить миграции
	@echo "Rolling back migrations..."
	@which goose > /dev/null || (echo "goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest" && exit 1)
	@goose -dir migrations postgres "postgres://postgres:postgres@localhost:5432/potter?sslmode=disable" down

migrate-status: ## Показать статус миграций
	@which goose > /dev/null || (echo "goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest" && exit 1)
	@goose -dir migrations postgres "postgres://postgres:postgres@localhost:5432/potter?sslmode=disable" status

migrate-create: ## Создать новую миграцию (использование: make migrate-create NAME=migration_name)
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Usage: make migrate-create NAME=migration_name"; \
		exit 1; \
	fi
	@which goose > /dev/null || (echo "goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest" && exit 1)
	@goose -dir migrations create $(NAME) sql

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

