# Order Saga Example

Полнофункциональный пример реализации Saga Pattern для оркестрации долгоживущей транзакции создания заказа с использованием фреймворка Potter.

## Описание

Этот пример демонстрирует использование Saga Pattern для координации распределенной транзакции создания заказа, которая включает:
1. **Резервирование товара на складе** (ReserveInventoryStep)
2. **Обработка оплаты** (ProcessPaymentStep)
3. **Создание доставки** (CreateShipmentStep)
4. **Завершение заказа** (CompleteOrderStep)

При ошибке на любом этапе выполняется автоматическая компенсация всех выполненных шагов в обратном порядке.

## Архитектура

### Компоненты

- **Domain Layer**: `Order` агрегат с Event Sourcing
- **Application Layer**: Определение саги и шаги выполнения
- **Infrastructure Layer**: Persistence (EventStore), адаптеры (NATS, Redis)
- **Presentation Layer**: REST API handlers

### Диаграмма выполнения саги

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant Orchestrator
    participant ReserveStep
    participant PaymentStep
    participant ShipmentStep
    participant CompleteStep

    Client->>API: POST /orders
    API->>Orchestrator: Create & Execute Saga
    
    Orchestrator->>ReserveStep: Execute
    ReserveStep->>ReserveStep: Send ReserveInventoryCommand
    ReserveStep-->>Orchestrator: InventoryReservedEvent
    
    Orchestrator->>PaymentStep: Execute
    PaymentStep->>PaymentStep: Send ProcessPaymentCommand
    PaymentStep-->>Orchestrator: PaymentProcessedEvent
    
    Orchestrator->>ShipmentStep: Execute
    ShipmentStep->>ShipmentStep: Send CreateShipmentCommand
    ShipmentStep-->>Orchestrator: ShipmentCreatedEvent
    
    Orchestrator->>CompleteStep: Execute
    CompleteStep->>CompleteStep: Publish OrderCompletedEvent
    CompleteStep-->>Orchestrator: Success
    
    Orchestrator-->>API: Saga Completed
    API-->>Client: 202 Accepted
```

### Диаграмма компенсации

```mermaid
sequenceDiagram
    participant Orchestrator
    participant PaymentStep
    participant ReserveStep
    participant InventoryService

    Note over Orchestrator: Payment Failed
    
    Orchestrator->>PaymentStep: Compensate
    PaymentStep->>PaymentStep: Send RefundPaymentCommand
    
    Orchestrator->>ReserveStep: Compensate
    ReserveStep->>InventoryService: Send ReleaseInventoryCommand
    InventoryService-->>ReserveStep: Success
    
    Orchestrator-->>Orchestrator: Saga Compensated
```

## Быстрый старт

### Предварительные требования

- Docker и Docker Compose
- Go 1.25.0+
- goose CLI (для миграций): `go install github.com/pressly/goose/v3/cmd/goose@latest`

### Запуск

```bash
# 1. Запуск инфраструктуры (PostgreSQL, NATS, Redis, Prometheus, Grafana)
make docker-up

# 2. Ожидание готовности сервисов (10 секунд)
sleep 10

# 3. Применение миграций
make migrate-up

# 4. Запуск приложения
make run
```

### Переменные окружения

```bash
export SERVER_PORT=8080
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/saga_order?sslmode=disable"
export NATS_URL="nats://localhost:4222"
export REDIS_ADDR="localhost:6379"
```

## API Endpoints

### Создание заказа

```http
POST /api/v1/orders
Content-Type: application/json

{
  "customer_id": "customer-123",
  "items": [
    {
      "product_id": "product-1",
      "quantity": 2,
      "price": 100.0
    }
  ]
}
```

**Ответ:**
```json
{
  "saga_id": "uuid",
  "order_id": "uuid",
  "status": "pending",
  "message": "Order creation started"
}
```

### Получение статуса саги

```http
GET /api/v1/sagas/{saga_id}
```

**Ответ:**
```json
{
  "saga_id": "uuid",
  "status": "running",
  "current_step": "process_payment",
  "context": {
    "order_id": "uuid",
    "customer_id": "customer-123",
    "items": [...]
  }
}
```

### Получение истории саги

```http
GET /api/v1/sagas/{saga_id}/history
```

**Ответ:**
```json
{
  "saga_id": "uuid",
  "history": [
    {
      "step_name": "reserve_inventory",
      "status": "completed",
      "started_at": "2024-01-01T10:00:00Z",
      "completed_at": "2024-01-01T10:00:05Z",
      "retry_attempt": 0
    },
    {
      "step_name": "process_payment",
      "status": "running",
      "started_at": "2024-01-01T10:00:05Z",
      "retry_attempt": 0
    }
  ]
}
```

### Отмена саги

```http
POST /api/v1/sagas/{saga_id}/cancel
```

### Возобновление саги

```http
POST /api/v1/sagas/{saga_id}/resume
```

## Тестирование

### Unit тесты

```bash
make test
```

### Integration тесты

```bash
make test-int
```

### Примеры запросов

См. файл `api_examples.http` для примеров использования REST Client в VSCode.

## Мониторинг

### Prometheus

Метрики доступны по адресу: http://localhost:9090

### Grafana

Дашборды доступны по адресу: http://localhost:3000
- Логин: `admin`
- Пароль: `admin`

### Ключевые метрики

- `saga.started` - количество запущенных саг
- `saga.completed` - количество завершенных саг
- `saga.failed` - количество неудачных саг
- `saga.compensated` - количество компенсированных саг
- `step.completed` - количество завершенных шагов
- `step.failed` - количество неудачных шагов

## Миграции

Этот пример использует [goose](https://github.com/pressly/goose) для управления миграциями базы данных.

### Установка goose

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Команды миграций

```bash
# Применить все pending миграции
make migrate-up

# Откатить последнюю миграцию
make migrate-down

# Показать статус миграций
make migrate-status

# Создать новую миграцию
make migrate-create NAME=add_new_table
```

### Использование goose напрямую

```bash
# Применить миграции
goose -dir migrations postgres "postgres://user:pass@localhost:5432/saga_order?sslmode=disable" up

# Откатить миграции
goose -dir migrations postgres "postgres://user:pass@localhost:5432/saga_order?sslmode=disable" down

# Показать статус
goose -dir migrations postgres "postgres://user:pass@localhost:5432/saga_order?sslmode=disable" status
```

Подробнее о goose см. [документацию goose](https://github.com/pressly/goose) и [framework/migrations/README.md](../../../framework/migrations/README.md).

## Структура проекта

```
examples/saga-order/
├── cmd/
│   └── server/
│       └── main.go              # Точка входа приложения
├── application/
│   ├── commands.go              # Команды CQRS
│   ├── order_saga.go           # Определение саги
│   └── steps.go                # Шаги саги
├── domain/
│   ├── events.go               # Доменные события
│   └── order.go                # Order агрегат (EventSourced)
├── infrastructure/
│   └── persistence.go          # Persistence для саг
├── migrations/
│   └── 001_create_saga_tables.sql  # Миграции в формате goose
├── docker-compose.yml          # Docker Compose конфигурация
├── Makefile                    # Make команды
├── api_examples.http           # Примеры API запросов
├── prometheus.yml              # Prometheus конфигурация
└── README.md                   # Документация
```

## Шаги саги

### 1. ReserveInventoryStep

- **Execute**: Отправляет `ReserveInventoryCommand`, ожидает `InventoryReservedEvent`
- **Compensate**: Отправляет `ReleaseInventoryCommand` для освобождения резерва
- **Timeout**: 30 секунд
- **Retry**: Exponential backoff (3 попытки)

### 2. ProcessPaymentStep

- **Execute**: Отправляет `ProcessPaymentCommand`, ожидает `PaymentProcessedEvent` или `PaymentFailedEvent`
- **Compensate**: Отправляет `RefundPaymentCommand` для возврата платежа
- **Guard**: Проверяет, что товар был зарезервирован
- **Timeout**: 30 секунд
- **Retry**: Simple retry (2 попытки)

### 3. CreateShipmentStep

- **Execute**: Отправляет `CreateShipmentCommand`, ожидает `ShipmentCreatedEvent`
- **Compensate**: Отправляет `CancelShipmentCommand` для отмены доставки
- **Guard**: Проверяет, что платеж был обработан
- **Timeout**: 30 секунд

### 4. CompleteOrderStep

- **Execute**: Публикует `OrderCompletedEvent`
- **Compensate**: Нет (финальный шаг)

## Persistence

Сага использует EventStore для персистентности состояния:
- Состояние саги сохраняется после каждого шага
- История выполнения шагов записывается в EventStore
- Snapshot создается каждые 5 шагов для оптимизации загрузки

## Обработка ошибок

При ошибке на любом шаге:
1. Выполняется компенсация всех выполненных шагов в обратном порядке
2. Состояние саги обновляется на `compensated`
3. События компенсации публикуются в EventBus

## Расширение

Для добавления новых шагов:

1. Создайте новый шаг в `application/steps.go`
2. Добавьте шаг в `NewOrderSagaDefinition` в `application/order_saga.go`
3. Реализуйте команды и события в соответствующих слоях

## Troubleshooting

### Проблема: Saga не запускается

- Проверьте подключение к NATS: `docker-compose logs nats`
- Проверьте подключение к PostgreSQL: `docker-compose logs postgres`
- Убедитесь, что миграции применены: `make migrate-up`
- Убедитесь, что goose установлен: `which goose` или установите через `go install github.com/pressly/goose/v3/cmd/goose@latest`

### Проблема: Saga застревает в статусе "running"

- Проверьте логи приложения
- Проверьте статус шагов через `/api/v1/sagas/{id}/history`
- Попробуйте возобновить сагу: `POST /api/v1/sagas/{id}/resume`

## Лицензия

Этот пример является частью фреймворка Potter и следует его лицензии.
