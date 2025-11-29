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
	@which goose > /dev/null || (echo "$(YELLOW)goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest$(NC)" && exit 1)
	@goose -dir migrations postgres "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable" up
	@echo "$(GREEN)Миграции применены$(NC)"

migrate: migrate-up ## Алиас для migrate-up

migrate-down: ## Откатить миграции
	@echo "$(YELLOW)Откат миграций...$(NC)"
	@which goose > /dev/null || (echo "$(YELLOW)goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest$(NC)" && exit 1)
	@goose -dir migrations postgres "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable" down
	@echo "$(GREEN)Миграции откачены$(NC)"

migrate-status: ## Показать статус миграций
	@which goose > /dev/null || (echo "$(YELLOW)goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest$(NC)" && exit 1)
	@goose -dir migrations postgres "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable" status

migrate-create: ## Создать новую миграцию (использование: make migrate-create NAME=migration_name)
	@if [ -z "$(NAME)" ]; then \
		echo "$(YELLOW)Error: NAME is required. Usage: make migrate-create NAME=migration_name$(NC)"; \
		exit 1; \
	fi
	@which goose > /dev/null || (echo "$(YELLOW)goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest$(NC)" && exit 1)
	@goose -dir migrations create $(NAME) sql

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
