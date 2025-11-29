# Saga Query Handler Example

Полный пример использования Saga Query Handler с проекциями для read model и REST API для запросов.

## Архитектура

```
┌─────────────────┐
│   REST API      │
│   (Gin)         │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   QueryBus      │
│   (InMemory)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐      ┌──────────────────┐
│ QueryHandler    │─────▶│ ReadModelStore  │
│                 │      │ (Postgres)       │
└─────────────────┘      └──────────────────┘
         │
         │ (fallback)
         ▼
┌─────────────────┐
│ SagaPersistence │
│ (EventStore)    │
└─────────────────┘

┌─────────────────┐
│ EventStore      │─────▶┌──────────────────┐
│ (Postgres)      │      │ ProjectionManager│
└─────────────────┘      └────────┬─────────┘
                                  │
                                  ▼
                         ┌──────────────────┐
                         │ SagaReadModel    │
                         │ Projection       │
                         └────────┬─────────┘
                                  │
                                  ▼
                         ┌──────────────────┐
                         │ ReadModelStore   │
                         │ (Postgres)       │
                         └──────────────────┘
```

## Компоненты

1. **SagaReadModelProjection** - проекция, которая подписывается на события саг из EventStore и обновляет read model
2. **ProjectionManager** - управляет жизненным циклом проекций, checkpoint'ами
3. **SagaQueryHandler** - обработчик запросов, использует read model для быстрых ответов
4. **REST API** - предоставляет HTTP endpoints для работы с сагами

## Запуск

### 1. Запустить PostgreSQL

```bash
make docker-up
```

### 2. Применить миграции

```bash
make migrate
```

### 3. Запустить сервер

```bash
make run
```

## API Endpoints

### Создать сагу

```bash
curl -X POST http://localhost:8080/api/v1/sagas \
  -H "Content-Type: application/json" \
  -d '{
    "definition_name": "simple_saga",
    "correlation_id": "corr-123",
    "context": {
      "user_id": "user-456",
      "order_id": "order-789"
    }
  }'
```

Ответ:
```json
{
  "saga_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "started"
}
```

### Получить статус саги

```bash
curl http://localhost:8080/api/v1/sagas/550e8400-e29b-41d4-a716-446655440000
```

Ответ:
```json
{
  "saga_id": "550e8400-e29b-41d4-a716-446655440000",
  "definition_name": "simple_saga",
  "status": "running",
  "current_step": "step2",
  "total_steps": 3,
  "completed_steps": 1,
  "failed_steps": 0,
  "started_at": "2024-01-15T10:30:00Z",
  "completed_at": null,
  "duration": null,
  "correlation_id": "corr-123",
  "context": {
    "user_id": "user-456",
    "order_id": "order-789"
  },
  "last_error": null,
  "retry_count": 0
}
```

### Получить историю саги

```bash
curl http://localhost:8080/api/v1/sagas/550e8400-e29b-41d4-a716-446655440000/history
```

Ответ:
```json
{
  "saga_id": "550e8400-e29b-41d4-a716-446655440000",
  "history": [
    {
      "step_name": "step1",
      "status": "completed",
      "started_at": "2024-01-15T10:30:00Z",
      "completed_at": "2024-01-15T10:30:01Z",
      "duration": "100ms",
      "retry_attempt": 0,
      "error": null
    },
    {
      "step_name": "step2",
      "status": "running",
      "started_at": "2024-01-15T10:30:01Z",
      "completed_at": null,
      "duration": null,
      "retry_attempt": 0,
      "error": null
    }
  ]
}
```

### Список саг с фильтрацией

```bash
# Все запущенные саги
curl "http://localhost:8080/api/v1/sagas?status=running&limit=10"

# Саги по определению
curl "http://localhost:8080/api/v1/sagas?definition_name=simple_saga&limit=20&offset=0"

# Саги по correlation_id
curl "http://localhost:8080/api/v1/sagas?correlation_id=corr-123"
```

Ответ:
```json
{
  "sagas": [
    {
      "saga_id": "550e8400-e29b-41d4-a716-446655440000",
      "definition_name": "simple_saga",
      "status": "running",
      "current_step": "step2",
      "started_at": "2024-01-15T10:30:00Z",
      "completed_at": null,
      "correlation_id": "corr-123"
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

### Получить метрики саг

```bash
curl "http://localhost:8080/api/v1/sagas/metrics?definition_name=simple_saga"
```

Ответ:
```json
{
  "total_sagas": 100,
  "completed_sagas": 85,
  "failed_sagas": 10,
  "compensated_sagas": 5,
  "success_rate": 85.0,
  "avg_duration": "2m30s",
  "throughput": 0
}
```

## Особенности

1. **Проекции** - автоматическое обновление read model из событий EventStore
2. **Checkpoint'ы** - проекции сохраняют позицию обработки для восстановления после перезапуска
3. **Idempotency** - проекции обрабатывают события идемпотентно
4. **Fallback** - QueryHandler использует read model, но может fallback на persistence при необходимости
5. **Фильтрация** - поддержка фильтрации по статусу, определению, correlation_id, датам

## Тестирование

```bash
# Unit тесты
make test

# Запустить сервер и протестировать API
make run
# В другом терминале:
curl -X POST http://localhost:8080/api/v1/sagas \
  -H "Content-Type: application/json" \
  -d '{"definition_name": "simple_saga", "correlation_id": "test-1"}'

# Получить созданную сагу
SAGA_ID=$(curl -s -X POST http://localhost:8080/api/v1/sagas \
  -H "Content-Type: application/json" \
  -d '{"definition_name": "simple_saga", "correlation_id": "test-2"}' | jq -r '.saga_id')

curl http://localhost:8080/api/v1/sagas/$SAGA_ID
```

## Структура проекта

```
saga-query-handler/
├── cmd/
│   └── server/
│       └── main.go          # Основной сервер с REST API
├── application/
│   └── simple_saga.go       # Определение простой саги
├── infrastructure/
│   └── persistence.go      # Инициализация persistence и stores
├── migrations/
│   └── 001_create_tables.sql # SQL миграции
├── docker-compose.yml        # Docker Compose конфигурация
├── Makefile                  # Команды для запуска
└── README.md                 # Документация
```

