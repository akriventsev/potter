.PHONY: up run build test clean help

help: ## Показать справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

up: ## Запустить Docker Compose сервисы
	docker-compose up -d
	@sleep 5

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

