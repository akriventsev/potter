.PHONY: up run build test clean help migrate-up migrate-down migrate-status migrate-create

DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_NAME ?= potter

help: ## Показать справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

up: ## Запустить Docker Compose сервисы
	docker-compose up -d
	@sleep 5

migrate-up: ## Применить миграции
	@which goose > /dev/null || (echo "goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest" && exit 1)
	@goose -dir migrations postgres "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable" up

migrate-down: ## Откатить миграции
	@which goose > /dev/null || (echo "goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest" && exit 1)
	@goose -dir migrations postgres "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable" down

migrate-status: ## Показать статус миграций
	@which goose > /dev/null || (echo "goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest" && exit 1)
	@goose -dir migrations postgres "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable" status

migrate-create: ## Создать новую миграцию (использование: make migrate-create NAME=migration_name)
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Usage: make migrate-create NAME=migration_name"; \
		exit 1; \
	fi
	@which goose > /dev/null || (echo "goose not found. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest" && exit 1)
	@goose -dir migrations create $(NAME) sql

run: build ## Запустить CLI
	./bin/replay -command=replay-all

build: ## Собрать CLI
	go build -o bin/replay ./cmd/replay

test: ## Запустить тесты
	go test -v ./...

clean: ## Очистить временные файлы
	go clean -cache -testcache
	rm -rf bin/

replay-all: build ## Полный replay всех событий
	./bin/replay -command=replay-all

replay-aggregate: build ## Replay для конкретного агрегата
	./bin/replay -command=replay-aggregate -aggregate-id=$(ID)

replay-projection: build ## Rebuild проекции
	./bin/replay -command=replay-projection -projection=$(PROJECTION)

replay-from: build ## Replay с определенного момента
	./bin/replay -command=replay-from -from-time=$(TIME)

