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
  - CQRS Query Handler с read models для мониторинга
- 🔍 **Query Builder** - Fluent API для сложных запросов (Postgres, MongoDB)
- 🗄️ **Schema Migrations** - Версионированные миграции через goose
- 📊 **Projections Framework** - Централизованная инфраструктура для проекций с checkpoint management
- 🎨 **GraphQL Transport** - Автогенерация схем из proto, queries/mutations/subscriptions
- 📈 **Advanced Indexing** - Автоматическое управление индексами и рекомендации
- 🔄 **Change Streams** - Реактивные обновления для MongoDB
- ⏱️ **TTL Support** - Автоматическая очистка данных в MongoDB

## Транспорты

Поддерживаются следующие транспорты:
- REST API (Gin)
- gRPC
- WebSocket
- GraphQL (gqlgen) с автогенерацией схем из proto
- Subscriptions для real-time обновлений
- Message Queue (NATS, Kafka, Redis)

## Метрики

Отдельный пакет `framework/metrics` для сбора метрик через OpenTelemetry и Prometheus.

## Production Readiness

| Компонент | Статус | Описание |
|-----------|--------|----------|
| Event Sourcing | ✅ Production Ready | Postgres/MongoDB адаптеры, snapshots, replay, projections |
| Saga Pattern | ✅ Production Ready | FSM, компенсация, persistence, query handler с read models |
| CQRS Invoke | ✅ Production Ready | Type-safe invokers для команд и запросов |
| GraphQL Transport | ✅ Production Ready | Автогенерация схем, queries/mutations/subscriptions |
| Query Builder | ✅ Production Ready | Fluent API для Postgres и MongoDB |
| Schema Migrations | ✅ Production Ready | Goose integration, SQL и Go миграции |
| Projections Framework | ✅ Production Ready | Checkpoint management, rebuild support |
| Code Generator | ✅ Production Ready | Proto-first codegen с incremental updates |
| EventStoreDB Adapter | ⏳ Pending | Структура готова, ожидает stable Go client v21.2+ |

Подробнее о планах развития см. [ROADMAP.md](ROADMAP.md).

## Структура проекта

```
.
├── framework/              # Основной фреймворк
│   ├── adapters/          # Built-in адаптеры
│   │   ├── events/        # Event publishers (NATS, Kafka, MessageBus)
│   │   ├── messagebus/    # Message bus адаптеры (NATS, Kafka, Redis)
│   │   ├── repository/    # Репозитории (Postgres, MongoDB, InMemory)
│   │   └── transport/     # Транспорты (REST, gRPC, WebSocket, GraphQL)
│   ├── codegen/           # Code generator из proto файлов
│   ├── container/         # DI контейнер
│   ├── core/              # Базовые интерфейсы и типы
│   ├── cqrs/              # CQRS компоненты
│   ├── events/            # Система событий
│   ├── eventsourcing/     # Event Sourcing (stores, snapshots, replay, projections)
│   ├── fsm/               # Конечный автомат для саг
│   ├── invoke/            # Type-safe CQRS invokers
│   ├── metrics/           # Метрики OpenTelemetry
│   ├── migrations/        # Goose wrapper для миграций
│   ├── saga/              # Saga Pattern (orchestrator, query handler, read models)
│   ├── testing/           # Testing utilities
│   └── transport/         # Транспортный слой (CommandBus, QueryBus, MessageBus)
├── examples/              # Примеры приложений
│   ├── codegen/           # Пример кодогенерации
│   ├── eventsourcing-basic/        # Базовый Event Sourcing
│   ├── eventsourcing-snapshots/    # Стратегии снапшотов
│   ├── eventsourcing-replay/       # Event replay и projections
│   ├── eventsourcing-mongodb/      # Event Sourcing с MongoDB
│   ├── graphql-service/            # GraphQL Transport
│   ├── saga-order/                 # Базовая Saga
│   ├── saga-parallel/              # Параллельные шаги
│   ├── saga-conditional/           # Условные шаги
│   └── saga-query-handler/         # Saga Query Handler с read models
├── cmd/                   # CLI инструменты
│   ├── potter-gen/        # Code generator CLI
│   ├── potter-migrate/    # Migration CLI (goose wrapper)
│   └── protoc-gen-potter/ # Protoc плагин
└── api/                   # API определения (proto)
```

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

- Go 1.25.0 или выше
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

Фреймворк включает comprehensive примеры для всех основных паттернов. Полная документация: [`examples/README.md`](examples/README.md)

### Saga Pattern

- **saga-order** - Базовая Saga с последовательными шагами и компенсацией
- **saga-parallel** - Параллельное выполнение независимых операций
- **saga-conditional** - Условное выполнение шагов на основе контекста
- **saga-query-handler** - CQRS query handler с read models для мониторинга саг

### Event Sourcing

- **eventsourcing-basic** - Базовые операции с Event Sourced агрегатами
- **eventsourcing-snapshots** - Три стратегии снапшотов (Frequency, TimeBased, Hybrid)
- **eventsourcing-replay** - Event replay и rebuilding проекций
- **eventsourcing-mongodb** - Event Sourcing с MongoDB вместо PostgreSQL

### GraphQL Transport

- **graphql-service** - Product Catalog с автогенерацией схем, queries/mutations/subscriptions

### Code Generation

- **codegen** - Генерация CQRS приложений из proto файлов

Подробнее см. [`examples/README.md`](examples/README.md) и [`framework/saga/README.md`](framework/saga/README.md)

## Quick Start

### Установка

```bash
go get github.com/akriventsev/potter/framework
```

### Установка инструментов

```bash
# Установка всех CLI инструментов
make install-codegen-tools

# Или по отдельности:
make install-potter-gen      # Code generator
make install-potter-migrate  # Migration tool
make install-goose           # Goose CLI
```

### Запуск примеров

**Saga Pattern:**

```bash
cd examples/saga-order
make docker-up && make migrate && make run
```

**Event Sourcing:**

```bash
cd examples/eventsourcing-basic
make docker-up && make migrate && make run
```

**GraphQL Transport:**

```bash
cd examples/graphql-service
make docker-up && make migrate-up && make generate && make run

make playground  # Открыть GraphQL Playground
```

### Создание нового проекта

1. Создайте proto файл с Potter аннотациями:

```protobuf
syntax = "proto3";
import "github.com/akriventsev/potter/options.proto";

service ProductService {
  option (potter.service) = {
    module_name: "product"
    transport: ["REST", "GRAPHQL"]
  };

  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse) {
    option (potter.command) = {
      aggregate: "Product"
      async: true
    };
  }

  rpc GetProduct(GetProductRequest) returns (GetProductResponse) {
    option (potter.query) = {
      cacheable: true
      cache_ttl_seconds: 300
    };
  }
}
```

2. Сгенерируйте приложение:

```bash
potter-gen init --proto api/service.proto --module myapp --output ./myapp --with-graphql
```

3. Запустите приложение:

```bash
cd myapp
make docker-up
make migrate
make run
```

Подробнее см. [Code Generator Guide](framework/codegen/README.md)

## Key Features

### GraphQL Transport

Автоматическая генерация GraphQL API из proto файлов:
- Queries → CQRS QueryBus
- Mutations → CQRS CommandBus  
- Subscriptions → EventBus (real-time updates)
- Query complexity limits и security
- GraphQL Playground для разработки

Подробнее: [`framework/adapters/transport/GRAPHQL.md`](framework/adapters/transport/GRAPHQL.md)

### Query Builder

Fluent API для построения сложных запросов:

```go
results, err := repo.Query().
    Where("status", Eq, "active").
    Where("created_at", Gte, time.Now().AddDate(0, -1, 0)).
    OrderBy("created_at", Desc).
    Limit(10).
    Execute(ctx)
```

Поддержка: Postgres, MongoDB, joins, агрегация, full-text search, geo queries

### Schema Migrations

Версионированные миграции через goose:

```bash
# CLI
potter-migrate up --database-url postgres://localhost/db
potter-migrate down 1 --database-url postgres://localhost/db
potter-migrate status --database-url postgres://localhost/db

# Или напрямую через goose
goose -dir migrations postgres "postgres://localhost/db" up
```

```go
// Программное использование
import "github.com/akriventsev/potter/framework/migrations"

db, _ := sql.Open("pgx", dsn)
err := migrations.RunMigrations(db, "./migrations")
```

Поддержка: SQL миграции (Postgres, MySQL, SQLite), Go миграции (MongoDB), rollback, out-of-order миграции

Подробнее: [`framework/migrations/README.md`](framework/migrations/README.md)

### Projections Framework

Централизованная инфраструктура для проекций:

```go
projectionMgr := eventsourcing.NewProjectionManager(checkpointStore)
projectionMgr.RegisterProjection("order_summary", orderSummaryProjection)
projectionMgr.Start(ctx)

// Rebuild проекций
projectionMgr.RebuildProjection(ctx, "order_summary")
```

Возможности: checkpoint management, automatic registration, rebuild support, batch processing

### Saga Query Handler

CQRS query handler для мониторинга саг:

```go
queryHandler := saga.NewSagaQueryHandler(persistence, readModelStore)
queryBus.RegisterHandler("GetSagaStatus", queryHandler)

query := &saga.GetSagaStatusQuery{SagaID: "saga-123"}
result, _ := queryHandler.Handle(ctx, query)
```

Возможности: read models, оптимизированные запросы, фильтрация, пагинация, метрики

### Использование фреймворка

Фреймворк предоставляет готовые компоненты для построения CQRS приложений:

- **CommandBus/QueryBus**: Шины для команд и запросов
- **Invoke Module**: Type-safe invokers с ожиданием событий
- **EventPublisher/EventBus**: Публикация и подписка на события
- **GraphQL Transport**: Автогенерация GraphQL API из proto
- **Query Builder**: Fluent API для сложных запросов
- **Schema Migrations**: Goose integration для версионирования БД
- **Projections Framework**: Централизованное управление проекциями
- **Repository адаптеры**: PostgreSQL, MongoDB, InMemory с advanced indexing
- **MessageBus адаптеры**: NATS, Kafka, Redis
- **Event Store адаптеры**: PostgreSQL, MongoDB, EventStoreDB (pending), InMemory
- **Metrics**: OpenTelemetry интеграция
- **Code Generator**: Proto-first генерация приложений

Подробнее: [`framework/README.md`](framework/README.md)

## Code Generator

Potter Framework включает мощный кодогенератор для создания CQRS приложений из protobuf спецификаций.

### Возможности

- **Генерация из protobuf** - декларативное описание сервисов с Potter custom options
- **Полная структура проекта** - domain, application, infrastructure, presentation слои
- **Incremental updates** - обновление кода с сохранением пользовательской логики
- **SDK generation** - type-safe SDK на базе framework/invoke
- **GraphQL support** - автогенерация GraphQL схем с флагом `--with-graphql`
- **Protoc integration** - работа как protoc плагин

### Установка

```bash
make install-codegen-tools  # potter-gen, protoc-gen-potter, potter-migrate, goose
```

### Быстрый старт

```bash
# Создание нового проекта
potter-gen init --proto api/service.proto --module myapp --output ./myapp --with-graphql

# Обновление существующего проекта
potter-gen update --proto api/service.proto --output ./myapp

# Проверка синхронности (для CI)
potter-gen check --proto api/service.proto --output ./myapp
```

### Документация

- [Code Generator Guide](framework/codegen/README.md) - полное руководство
- [Potter Custom Options](api/proto/potter/options.proto) - описание аннотаций
- [Codegen Example](examples/codegen/README.md) - пример использования

Подробнее: [`framework/codegen/README.md`](framework/codegen/README.md)

## Invoke Module - Type-safe CQRS Invokers

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

Подробнее: [`framework/invoke/README.md`](framework/invoke/README.md) и [`framework/invoke/examples/README.md`](framework/invoke/examples/README.md)

## Архитектурные решения

### Гексагональная архитектура

- **Domain Layer** - чистый бизнес-логика, без зависимостей
- **Application Layer** - use cases, обработчики команд и запросов
- **Ports** (`framework/transport`) - интерфейсы для транспортов
- **Adapters** (`framework/adapters`) - реализации портов (REST, gRPC, NATS, репозитории)

### CQRS

- **Commands** - изменяют состояние, проходят через `CommandBus`
- **Queries** - читают данные, проходят через `QueryBus`
- **Events** - публикуются после выполнения команд, обрабатываются асинхронно

### Метрики

Все операции автоматически инструментируются через пакет `framework/metrics`:
- Счетчики команд/запросов/событий
- Длительность выполнения
- Активные операции
- Ошибки

## Зависимости

- **Gin** - REST API фреймворк
- **gRPC** - RPC транспорт
- **gqlgen** - GraphQL сервер
- **NATS** - Message Queue
- **Kafka** - Event streaming
- **OpenTelemetry** - метрики и трейсинг
- **Prometheus** - экспорт метрик
- **goose** - Schema migrations
- **PostgreSQL** - Primary database
- **MongoDB** - NoSQL database

## Версионирование

Проект следует [Semantic Versioning](https://semver.org/).

**Текущая версия:** 1.5.0 (см. [`VERSION`](VERSION))

**История изменений:**

- **v1.5.0** - Goose integration для миграций
- **v1.4.0** - GraphQL Transport, Query Builder, Projections Framework, Saga Query Handler
- **v1.3.x** - Saga Pattern, Event Sourcing enhancements
- **v1.2.0** - Code Generator, Invoke Module, Testing utilities
- **v1.1.0** - Event Sourcing базовая поддержка
- **v1.0.0** - Базовая структура фреймворка

Подробнее: [`ROADMAP.md`](ROADMAP.md)

## Документация

- **Framework Overview**: [`framework/README.md`](framework/README.md)
- **Examples**: [`examples/README.md`](examples/README.md)
- **Roadmap**: [`ROADMAP.md`](ROADMAP.md)
- **Code Generator**: [`framework/codegen/README.md`](framework/codegen/README.md)
- **GraphQL Transport**: [`framework/adapters/transport/GRAPHQL.md`](framework/adapters/transport/GRAPHQL.md)
- **Migrations**: [`framework/migrations/README.md`](framework/migrations/README.md)
- **Saga Pattern**: [`framework/saga/README.md`](framework/saga/README.md)
- **Event Sourcing**: [`framework/eventsourcing/README.md`](framework/eventsourcing/README.md)
- **Invoke Module**: [`framework/invoke/README.md`](framework/invoke/README.md)

## Лицензия

MIT

## Авторы

Potter Team

