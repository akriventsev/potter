# Saga Conditional Example

Пример демонстрации условного выполнения шагов в Saga Pattern.

## Описание

Этот пример показывает, как использовать `ConditionalStep` для условного выполнения шагов на основе данных в SagaContext. Шаги могут быть пропущены, если условия не выполняются.

## Use Case

Обработка заказа с условными шагами:
- Верификация для крупных сумм (> 1000)
- Специальная обработка для VIP клиентов
- Дополнительные проверки для международной доставки

## Архитектура

- `application/conditional_saga.go` - определение саги с ConditionalStep
- `application/steps.go` - реализация условных шагов
- `domain/events.go` - события

## Quick Start

```bash
make up    # Запустить PostgreSQL и NATS
make run   # Запустить приложение
```

## API Examples

```http
POST /orders
Content-Type: application/json

{
  "customer_id": "customer-123",
  "amount": 1500.00,
  "customer_type": "vip"
}
```

## Документация

См. [framework/saga/README.md](../../../framework/saga/README.md) для подробной документации по Saga Pattern.

