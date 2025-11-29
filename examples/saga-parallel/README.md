# Saga Parallel Example

Пример демонстрации параллельного выполнения шагов в Saga Pattern.

## Описание

Этот пример показывает, как использовать `ParallelStep` для одновременного выполнения нескольких независимых операций в рамках одной саги. При ошибке в любом из параллельных шагов выполняется компенсация всех успешных шагов.

## Use Case

Обработка заказа с параллельным выполнением:
1. Проверка кредитного лимита клиента
2. Резервирование товара на складе
3. Расчет стоимости доставки

Все три операции выполняются параллельно для ускорения обработки заказа.

## Архитектура

- `application/parallel_saga.go` - определение саги с ParallelStep
- `application/steps.go` - реализация параллельных шагов
- `domain/events.go` - события для параллельных операций

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
  "items": [
    {"product_id": "product-1", "quantity": 2}
  ]
}
```

## Документация

См. [framework/saga/README.md](../../../framework/saga/README.md) для подробной документации по Saga Pattern.

