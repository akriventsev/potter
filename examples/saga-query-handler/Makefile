.PHONY: docker-up docker-down migrate-up migrate-down run test lint clean help

# Переменные
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_NAME ?= saga_query_handler
PGPASSWORD := $(DB_PASSWORD)
DATABASE_URL := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

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

migrate-up: ## Применить миграции через potter-migrate
	@echo "$(GREEN)Применение миграций...$(NC)"
	@cd ../.. && go run cmd/potter-migrate/main.go up -d $(DATABASE_URL) -p examples/saga-query-handler/migrations || \
		(echo "$(YELLOW)Warning: potter-migrate not available, using psql$(NC)" && \
		PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f migrations/001_create_tables.sql)
	@echo "$(GREEN)Миграции применены$(NC)"

migrate: migrate-up ## Алиас для migrate-up

migrate-down: ## Откатить миграции
	@echo "$(YELLOW)Откат миграций...$(NC)"
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -c "DROP TABLE IF EXISTS saga_history CASCADE; DROP TABLE IF EXISTS saga_instances CASCADE; DROP TABLE IF EXISTS projection_checkpoints CASCADE; DROP TABLE IF EXISTS public.snapshots CASCADE; DROP TABLE IF EXISTS public.event_store CASCADE;"
	@echo "$(GREEN)Миграции откачены$(NC)"

run: ## Запустить сервер
	@echo "$(GREEN)Запуск saga-query-handler сервера...$(NC)"
	@DATABASE_URL=$(DATABASE_URL) go run cmd/server/main.go

test: ## Запустить unit тесты
	@echo "$(GREEN)Запуск unit тестов...$(NC)"
	go test -v -race -cover ./...

lint: ## Запустить линтер
	@echo "$(GREEN)Запуск линтера...$(NC)"
	@golangci-lint run ./... || echo "$(YELLOW)Линтер не установлен, пропускаем$(NC)"

clean: ## Очистить временные файлы
	@echo "$(GREEN)Очистка временных файлов...$(NC)"
	@go clean -cache -testcache
	@find . -type f -name "*.test" -delete
	@echo "$(GREEN)Очистка завершена$(NC)"

