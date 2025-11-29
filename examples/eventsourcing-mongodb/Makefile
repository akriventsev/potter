.PHONY: up down run test clean help migrate

help: ## Показать справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

up: ## Запустить Docker Compose сервисы
	docker-compose up -d
	@echo "Ожидание готовности MongoDB..."
	@sleep 5

down: ## Остановить Docker Compose сервисы
	docker-compose down -v

run: ## Запустить сервер
	go run cmd/server/main.go

test: ## Запустить тесты
	go test -v ./...

clean: ## Очистить временные файлы
	go clean -cache -testcache

migrate: ## Создать индексы в MongoDB (выполняется автоматически при запуске)
	@echo "Индексы создаются автоматически при подключении к MongoDB"

