.PHONY: up run test clean help

DB_NAME ?= saga_parallel

help:
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

up: ## Запустить Docker Compose сервисы
	docker-compose up -d
	@sleep 5

run: ## Запустить сервер
	go run cmd/server/main.go

test: ## Запустить тесты
	go test -v ./...

clean: ## Очистить временные файлы
	go clean -cache -testcache

