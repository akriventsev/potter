# Potter - Hexagonal Architecture with CQRS

Проект демонстрирует реализацию гексагональной архитектуры с паттерном CQRS в Go.

## Архитектура

Проект построен по принципам гексагональной архитектуры (Ports & Adapters) с модульным DI контейнером:

- **Domain Layer** - доменные сущности, value objects и события
- **Application Layer** - use cases (command/query handlers)
- **Ports** - интерфейсы для взаимодействия с внешним миром
- **Adapters** - реализации портов (REST, gRPC, Message Queue)
- **Modules** - модули для инициализации компонентов (метрики, репозитории, события, CQRS)
- **Transports** - транспорты для взаимодействия (REST, gRPC, MessageBus)

## CQRS

- **Commands** - операции записи (изменение состояния)
- **Queries** - операции чтения (получение данных)
- **Events** - доменные события, публикуемые после выполнения команд

## Features

- 🔄 **Saga Pattern** - Orchestration долгоживущих транзакций с автоматической компенсацией
  - Forward и compensating actions
  - Интеграция с CQRS и EventBus
  - Persistence через EventStore и PostgreSQL
  - Retry механизм и timeout support
  - Интеграция с 2PC для distributed transactions

## Транспорты

Поддерживаются следующие транспорты:
- REST API (Gin)
- gRPC
- Message Queue (NATS)

## Метрики

Отдельный пакет `pkg/metrics` для сбора метрик через OpenTelemetry и Prometheus.

## Production Readiness

| Компонент | Статус | Описание |
|-----------|--------|----------|
| Event Sourcing (Postgres/MongoDB) | ✅ Production Ready | Полнофункциональные адаптеры с поддержкой снапшотов и replay |
| Saga Pattern | ✅ Production Ready | Полная реализация с FSM, компенсацией и persistence |
| CQRS Invoke | ✅ Production Ready | Type-safe invokers для команд и запросов |
| Code Generator | ⚠️ Beta | Стабильный API, активная разработка |

Подробнее о планах развития см. [ROADMAP.md](ROADMAP.md).

## Структура проекта

```
.
├── framework/           # Основной фреймворк
│   ├── adapters/       # Built-in адаптеры (repository, messagebus, events, transport)
│   ├── container/      # DI контейнер
│   ├── core/           # Базовые интерфейсы и типы
│   ├── cqrs/           # CQRS компоненты
│   ├── events/         # Система событий
│   ├── fsm/            # Конечный автомат для саг
│   ├── invoke/         # Invoke module (type-safe CQRS invokers)
│   │   └── examples/   # Практические примеры для всех транспортов
│   ├── metrics/        # Метрики OpenTelemetry
│   └── transport/      # Транспортный слой (CommandBus, QueryBus, MessageBus)
├── examples/           # Примеры приложений
│   └── warehouse/      # Warehouse example (2PC, Redis, PostgreSQL, NATS)
└── api/                # API определения (proto)
```

**Примечание**: Директория `internal/` была удалена в версии 1.0.3. Все компоненты перенесены в `framework/adapters/` как built-in адаптеры фреймворка.

## Testing

Фреймворк включает comprehensive unit тесты для всех основных компонентов. Для запуска тестов:

```bash
# Все тесты
make test

# С покрытием кода
make test-coverage

# Только unit тесты
make test-unit
```

См. `framework/README.md` для подробной информации о тестировании и примеров использования тестов как документации API.

## Установка и запуск

### Предварительные требования

- Go 1.21 или выше
- Protocol Buffers compiler (protoc)
- (Опционально) NATS Server для использования Message Queue

### Установка зависимостей

```bash
go mod download
go mod tidy
```

### Генерация proto файлов (опционально)

Если у вас установлен protoc и плагины:

```bash
make install-tools  # Установить protoc-gen-go и protoc-gen-go-grpc
make proto          # Сгенерировать proto файлы
```

**Примечание:** Проект включает заглушки proto файлов, поэтому компиляция работает без protoc.

## Examples

Фреймворк включает примеры приложений, демонстрирующие различные возможности. Также см. тесты в каждом пакете как примеры использования API.

### Saga Pattern Examples

- [Order Saga](examples/saga-order/) - Пример Order Saga с резервированием товара, оплатой, доставкой
- [Warehouse 2PC Integration](examples/saga-warehouse-integration/) - Интеграция Saga с 2PC координатором

### Quick Start: Saga Pattern

```go
import "potter/framework/saga"

// Определение саги
sagaDef := saga.NewSagaBuilder("order_saga").
    AddStep(
        saga.NewCommandStep(
            "reserve_inventory",
            commandBus,
            ReserveInventoryCommand{...},
            ReleaseInventoryCommand{...},
        ),
    ).
    AddStep(
        saga.NewCommandStep(
            "process_payment",
            commandBus,
            ProcessPaymentCommand{...},
            RefundPaymentCommand{...},
        ),
    ).
    WithPersistence(persistence).
    WithEventBus(eventBus).
    Build()

// Создание и выполнение саги
orchestrator := saga.NewDefaultOrchestrator(persistence, eventBus)
instance := sagaDef.CreateInstance(sagaContext)
err := orchestrator.Execute(ctx, instance)
```

См. [Saga Pattern Documentation](framework/saga/README.md) для подробной информации.

### Warehouse Example

Полноценное приложение для управления складскими остатками с реализацией Two-Phase Commit (2PC) через NATS, кешированием в Redis и snapshot в PostgreSQL.

**Статус:** Поддерживаемый showcase — пример обновляется при изменении ядра фреймворка и синхронизируется с версиями фреймворка.

**Основные возможности:**
- Управление товарами и складами
- Изменение количества товаров на складах
- Резервирование товаров с 2PC транзакциями
- Кеширование данных в Redis
- Event sourcing и публикация событий

**What's Included:**
- Full hexagonal architecture с четким разделением слоев
- CQRS с CommandBus/QueryBus для разделения read/write моделей
- Two-Phase Commit координация через NATS для распределенных транзакций
- Redis кеширование для read models
- PostgreSQL для persistence и transaction log
- Prometheus метрики интеграция
- REST API с Gin фреймворком
- Docker Compose для локальной разработки

**Запуск:**
```bash
cd examples/warehouse
make docker-up    # Запуск PostgreSQL, Redis, NATS, Prometheus
make migrate      # Применение SQL миграций
make run          # Запуск приложения на порту 8080
```

**Мониторинг:**
- Prometheus dashboard: http://localhost:9090
- NATS monitoring: http://localhost:8222
- Metrics endpoint: http://localhost:2112/metrics

Подробнее см. [examples/warehouse/README.md](examples/warehouse/README.md)

Для быстрого тестирования API см. [examples/warehouse/api_examples.md](examples/warehouse/api_examples.md)

### Переменные окружения (Warehouse Example)

Warehouse example использует следующие переменные окружения:

- `SERVER_PORT` - порт для HTTP сервера (по умолчанию: 8080)
- `DATABASE_DSN` - строка подключения к PostgreSQL (по умолчанию: postgres://postgres:postgres@localhost:5432/warehouse?sslmode=disable)
- `REDIS_ADDR` - адрес Redis сервера (по умолчанию: localhost:6379)
- `REDIS_PASSWORD` - пароль Redis (по умолчанию: пусто)
- `REDIS_DB` - номер базы данных Redis (по умолчанию: 0)
- `NATS_URL` - URL NATS сервера (по умолчанию: nats://localhost:4222)
- `METRICS_ENABLED` - включить метрики (по умолчанию: true)
- `METRICS_PORT` - порт для метрик (по умолчанию: 2112)

### Пример запуска

Для запуска warehouse example:

```bash
cd examples/warehouse
make docker-up    # Запуск инфраструктуры (PostgreSQL, Redis, NATS, Prometheus)
make migrate      # Применение SQL миграций
make run          # Запуск приложения на порту 8080
```

После запуска приложение будет доступно на `http://localhost:8080`.

Или напрямую:

```bash
go run examples/warehouse/cmd/server/main.go
```

С настройкой переменных окружения:

```bash
SERVER_PORT=8080 \
DATABASE_DSN="postgres://postgres:postgres@localhost:5432/warehouse?sslmode=disable" \
REDIS_ADDR=localhost:6379 \
NATS_URL=nats://localhost:4222 \
METRICS_ENABLED=true \
METRICS_PORT=2112 \
go run examples/warehouse/cmd/server/main.go
```

## Quick Start

### Запуск примеров

Для быстрого старта используйте warehouse example:

```bash
# Запуск warehouse примера
make example-warehouse

# Или вручную
cd examples/warehouse
make docker-up
make migrate
make run
```

### Использование фреймворка

Фреймворк предоставляет готовые компоненты для построения CQRS приложений:

- **CommandBus/QueryBus**: Шины для команд и запросов
- **Invoke Module**: Type-safe invokers для команд и запросов с ожиданием событий
- **Invoke Examples**: Полная коллекция практических примеров для всех транспортов (NATS, Kafka, REST, gRPC)
- **EventPublisher**: Публикация доменных событий
- **Repository адаптеры**: PostgreSQL, MongoDB, InMemory
- **MessageBus адаптеры**: NATS, Kafka, Redis
- **Metrics**: OpenTelemetry интеграция

## Code Generator

Potter Framework включает мощный кодогенератор для создания CQRS приложений из protobuf спецификаций.

### Возможности

- **Генерация из protobuf** - декларативное описание сервисов с Potter custom options
- **Полная структура проекта** - domain, application, infrastructure, presentation слои
- **Incremental updates** - обновление кода с сохранением пользовательской логики
- **SDK generation** - type-safe SDK на базе framework/invoke
- **Protoc integration** - работа как protoc плагин

### Установка

```bash
# Установка CLI инструмента
make install-potter-gen

# Установка protoc плагина
make install-protoc-gen-potter

# Или все сразу
make install-codegen-tools
```

### Быстрый старт

1. Создайте proto файл с Potter аннотациями:

```protobuf
syntax = "proto3";
import "potter/options.proto";

service ProductService {
  option (potter.service) = {
    module_name: "product"
    transport: ["REST", "NATS"]
  };

  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse) {
    option (potter.command) = {
      aggregate: "Product"
      async: true
    };
  }
}
```

2. Сгенерируйте приложение:

```bash
potter-gen init --proto api/service.proto --module myapp --output ./myapp
```

3. Проверка синхронности (для CI):

```bash
# Проверка расхождений между proto и кодом
potter-gen check --proto api/service.proto --output ./myapp

# Команда завершится с ненулевым кодом, если есть расхождения
# Рекомендуется запускать в CI перед merge для гарантии синхронности
```

4. Запустите приложение:

```bash
cd myapp
make docker-up
make migrate
make run
```

### Документация

- [Code Generator Guide](framework/codegen/README.md) - полное руководство
- [Potter Custom Options](api/proto/potter/options.proto) - описание аннотаций
- [Codegen Example](examples/codegen/README.md) - пример использования

### Примеры

```bash
# Запуск примера кодогенерации
make example-codegen

# Просмотр сгенерированного кода
ls examples/codegen/generated/
```

#### Invoke Module - Type-safe CQRS Invokers

Модуль `framework/invoke/` предоставляет generic-based API для type-safe работы с командами и запросами:

```go
// CommandInvoker - асинхронная отправка команд с ожиданием событий
asyncBus := invoke.NewAsyncCommandBus(natsAdapter)
awaiter := invoke.NewEventAwaiterFromEventBus(eventBus)
invoker := invoke.NewCommandInvoker[CreateProductCommand, ProductCreatedEvent, ProductCreationFailedEvent](
    asyncBus, awaiter, "product.created", "product.creation_failed",
)

cmd := CreateProductCommand{Name: "Laptop", SKU: "LAP-001"}
event, err := invoker.Invoke(ctx, cmd)

// QueryInvoker - type-safe запросы
queryInvoker := invoke.NewQueryInvoker[GetProductQuery, GetProductResponse](queryBus)
result, err := queryInvoker.Invoke(ctx, GetProductQuery{ID: "product-123"})
```

Подробнее см. [framework/invoke/README.md](framework/invoke/README.md)

##### Invoke Module Examples

Модуль Invoke предоставляет type-safe CQRS операции. См. практические примеры:

```bash
cd framework/invoke/examples
make start-infra  # Запустить NATS, Kafka, Redis, PostgreSQL
make test-all     # Запустить все примеры
```

Доступные примеры:
- Commands: NATS, Kafka
- Queries: NATS, Kafka, REST, gRPC
- Advanced: Mixed transports

См. [framework/invoke/examples/README.md](framework/invoke/examples/README.md) для деталей.

**Примечание**: Для локальной разработки используется путь модуля `potter`. При использовании в собственных проектах импортируйте пакеты как `potter/framework/...`. При публикации на GitHub путь модуля можно изменить на `github.com/username/potter`.

См. примеры в `examples/warehouse/` для детальной демонстрации использования.

## Архитектурные решения

### Гексагональная архитектура

- **Domain Layer** - чистый бизнес-логика, без зависимостей
- **Application Layer** - use cases, обработчики команд и запросов
- **Ports** (`framework/transport`) - интерфейсы для транспортов
- **Adapters** (`framework/adapters`) - реализации портов (REST, gRPC, NATS, репозитории)

### Two-Phase Commit (2PC)

Warehouse example демонстрирует реализацию распределенных транзакций через 2PC:
- Координатор управляет транзакциями через NATS
- Участники обрабатывают prepare/commit/abort фазы
- Все транзакции логируются в PostgreSQL для восстановления

### CQRS

- **Commands** - изменяют состояние, проходят через `CommandBus`
- **Queries** - читают данные, проходят через `QueryBus`
- **Events** - публикуются после выполнения команд, обрабатываются асинхронно

### Метрики

Все операции автоматически инструментируются через пакет `pkg/metrics`:
- Счетчики команд/запросов/событий
- Длительность выполнения
- Активные операции
- Ошибки

## Зависимости

- **Gin** - REST API фреймворк
- **gRPC** - RPC транспорт
- **NATS** - Message Queue
- **OpenTelemetry** - метрики и трейсинг
- **Prometheus** - экспорт метрик

