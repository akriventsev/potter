.PHONY: help start-infra stop-infra test-command-nats test-command-kafka test-query-nats test-query-kafka test-query-rest test-query-grpc test-mixed test-all clean check-infra

# Цвета для вывода
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

help: ## Показать справку по командам
	@echo "$(CYAN)Potter Invoke Examples - Makefile$(NC)"
	@echo ""
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

start-infra: ## Запустить docker-compose с NATS/Kafka/Redis/Postgres
	@echo "$(CYAN)Запуск инфраструктуры...$(NC)"
	docker-compose up -d
	@echo "$(YELLOW)Ожидание готовности сервисов...$(NC)"
	@sleep 5
	@echo "$(GREEN)Инфраструктура запущена$(NC)"
	@echo "  NATS: nats://localhost:4222 (monitoring: http://localhost:8222)"
	@echo "  Kafka: localhost:9092 (UI: http://localhost:8080)"
	@echo "  Redis: localhost:6379"
	@echo "  PostgreSQL: localhost:5432"

stop-infra: ## Остановить docker-compose
	@echo "$(CYAN)Остановка инфраструктуры...$(NC)"
	docker-compose down
	@echo "$(GREEN)Инфраструктура остановлена$(NC)"

check-infra: ## Проверить, что инфраструктура запущена
	@echo "$(CYAN)Проверка инфраструктуры...$(NC)"
	@docker ps | grep -q nats || (echo "$(RED)Ошибка: NATS не запущен. Запустите 'make start-infra'$(NC)" && exit 1)
	@docker ps | grep -q kafka || (echo "$(RED)Ошибка: Kafka не запущен. Запустите 'make start-infra'$(NC)" && exit 1)
	@docker ps | grep -q redis || (echo "$(YELLOW)Предупреждение: Redis не запущен (опционально)$(NC)")
	@docker ps | grep -q postgres || (echo "$(YELLOW)Предупреждение: PostgreSQL не запущен (опционально)$(NC)")
	@echo "$(GREEN)Инфраструктура доступна$(NC)"

test-command-nats: check-infra ## Запустить пример Command через NATS
	@echo "$(CYAN)Запуск примера: Command через NATS$(NC)"
	@cd .. && go test -v -run ExampleCommandInvokerWithNATS ./examples

test-command-kafka: check-infra ## Запустить пример Command через Kafka
	@echo "$(CYAN)Запуск примера: Command через Kafka$(NC)"
	@cd .. && go test -v -run ExampleCommandInvokerWithKafka ./examples

test-query-nats: check-infra ## Запустить пример Query через NATS
	@echo "$(CYAN)Запуск примера: Query через NATS$(NC)"
	@cd .. && go test -v -run ExampleQueryInvokerWithNATS ./examples

test-query-kafka: check-infra ## Запустить пример Query через Kafka
	@echo "$(CYAN)Запуск примера: Query через Kafka$(NC)"
	@cd .. && go test -v -run ExampleQueryInvokerWithKafka ./examples

test-query-rest: check-infra ## Запустить пример Query через REST
	@echo "$(CYAN)Запуск примера: Query через REST$(NC)"
	@cd .. && go test -v -run ExampleQueryInvokerWithREST ./examples

test-query-grpc: check-infra ## Запустить пример Query через gRPC
	@echo "$(CYAN)Запуск примера: Query через gRPC$(NC)"
	@cd .. && go test -v -run ExampleQueryInvokerWithGRPC ./examples

test-mixed: check-infra ## Запустить пример Mixed Transports
	@echo "$(CYAN)Запуск примера: Mixed Transports$(NC)"
	@cd .. && go test -v -run ExampleMixedTransports ./examples

test-all: check-infra ## Запустить все примеры последовательно
	@echo "$(CYAN)Запуск всех примеров...$(NC)"
	@cd .. && \
		go test -v -run ExampleCommandInvokerWithNATS ./examples && \
		go test -v -run ExampleCommandInvokerWithKafka ./examples && \
		go test -v -run ExampleQueryInvokerWithNATS ./examples && \
		go test -v -run ExampleQueryInvokerWithKafka ./examples && \
		go test -v -run ExampleQueryInvokerWithREST ./examples && \
		go test -v -run ExampleQueryInvokerWithGRPC ./examples && \
		go test -v -run ExampleMixedTransports ./examples
	@echo "$(GREEN)Все примеры выполнены успешно$(NC)"

clean: ## Очистить docker volumes и временные файлы
	@echo "$(CYAN)Очистка...$(NC)"
	docker-compose down -v
	@echo "$(GREEN)Очистка завершена$(NC)"

