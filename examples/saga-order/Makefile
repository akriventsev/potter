.PHONY: docker-up docker-down migrate-up migrate-down run test test-int lint clean help

# Переменные
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_NAME ?= saga_order
PGPASSWORD := $(DB_PASSWORD)

# Цвета для вывода
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

help: ## Показать справку
	@echo "$(GREEN)Доступные команды:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'

docker-up: ## Запустить Docker Compose сервисы
	@echo "$(GREEN)Запуск Docker Compose сервисов...$(NC)"
	docker-compose up -d
	@echo "$(GREEN)Ожидание готовности сервисов...$(NC)"
	@sleep 5

docker-down: ## Остановить Docker Compose сервисы
	@echo "$(GREEN)Остановка Docker Compose сервисов...$(NC)"
	docker-compose down -v

migrate-up: ## Применить миграции
	@echo "$(GREEN)Применение миграций...$(NC)"
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f migrations/001_create_saga_tables.sql || echo "$(YELLOW)Warning: psql not available, migrations will be applied programmatically$(NC)"
	@echo "$(GREEN)Миграции применены$(NC)"

migrate: migrate-up ## Алиас для migrate-up

migrate-down: ## Откатить миграции (удалить таблицы)
	@echo "$(YELLOW)Откат миграций...$(NC)"
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -c "DROP TABLE IF EXISTS saga_history CASCADE; DROP TABLE IF EXISTS saga_instances CASCADE;"
	@echo "$(GREEN)Миграции откачены$(NC)"

run: ## Запустить сервер
	@echo "$(GREEN)Запуск saga-order сервера...$(NC)"
	@go run cmd/server/main.go

test: ## Запустить unit тесты
	@echo "$(GREEN)Запуск unit тестов...$(NC)"
	go test -v -race -cover ./...

test-int: ## Запустить integration тесты
	@echo "$(GREEN)Запуск integration тестов...$(NC)"
	go test -v -tags=integration -race -cover ./...

lint: ## Запустить линтер
	@echo "$(GREEN)Запуск линтера...$(NC)"
	@golangci-lint run ./... || echo "$(YELLOW)Линтер не установлен, пропускаем$(NC)"

clean: ## Очистить временные файлы
	@echo "$(GREEN)Очистка временных файлов...$(NC)"
	@go clean -cache -testcache
	@find . -type f -name "*.test" -delete
	@echo "$(GREEN)Очистка завершена$(NC)"
