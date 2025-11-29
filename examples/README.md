# Potter Framework Examples

Этот каталог содержит примеры использования различных паттернов и компонентов фреймворка Potter.

## Предварительные требования

- Docker и Docker Compose
- Go 1.25.0+
- goose CLI (для миграций): `go install github.com/pressly/goose/v3/cmd/goose@latest`

Подробнее о миграциях см. [framework/migrations/README.md](../../framework/migrations/README.md).

## Saga Pattern Examples

### saga-order

Базовый пример Order Saga, демонстрирующий последовательное выполнение шагов с компенсацией при ошибках.

**Ключевые особенности:**
- Последовательное выполнение шагов (ReserveInventory → ProcessPayment → CreateShipment → CompleteOrder)
- Автоматическая компенсация при ошибках
- Использование Event Sourcing для агрегата Order
- REST API для создания и мониторинга саг

**Quick Start:**
```bash
cd examples/saga-order
make up
make run
```

**Документация:** [saga-order/README.md](saga-order/README.md)

### saga-warehouse-integration

Пример интеграции Saga с Two-Phase Commit (2PC) для координации распределенных транзакций между несколькими складами.

**Ключевые особенности:**
- Использование TwoPhaseCommitStep для координации нескольких участников
- Резервирование товара на нескольких складах одновременно
- Интеграция с warehouse сервисом через 2PC

**Quick Start:**
```bash
cd examples/saga-warehouse-integration
make up
make run
```

**Документация:** [saga-warehouse-integration/README.md](saga-warehouse-integration/README.md)

### saga-parallel

Пример демонстрации параллельного выполнения шагов в Saga.

**Ключевые особенности:**
- Параллельное выполнение независимых операций (проверка кредита, резервирование товара, расчет доставки)
- Обработка ошибок в параллельных шагах
- Компенсация всех успешных параллельных шагов при ошибке

**Quick Start:**
```bash
cd examples/saga-parallel
make up
make run
```

**Документация:** [saga-parallel/README.md](saga-parallel/README.md)

### saga-conditional

Пример условного выполнения шагов в Saga на основе контекста.

**Ключевые особенности:**
- Условное выполнение шагов на основе данных в SagaContext
- Примеры различных условий (сумма заказа, тип клиента, регион)
- Пропуск шагов при невыполнении условий

**Quick Start:**
```bash
cd examples/saga-conditional
make up
make run
```

**Документация:** [saga-conditional/README.md](saga-conditional/README.md)

## Event Sourcing Examples

### eventsourcing-basic

Базовый пример Event Sourcing с банковским счетом.

**Ключевые особенности:**
- Event Sourced агрегат BankAccount
- Базовые операции (Deposit, Withdraw, Close)
- Восстановление состояния из событий
- PostgreSQL event store

**Quick Start:**
```bash
cd examples/eventsourcing-basic
make up
make run
```

**Документация:** [eventsourcing-basic/README.md](eventsourcing-basic/README.md)

### eventsourcing-snapshots

Пример работы со снапшотами в Event Sourcing для оптимизации производительности.

**Ключевые особенности:**
- Три стратегии снапшотов: FrequencyStrategy, TimeBasedStrategy, HybridStrategy
- Сравнение производительности загрузки с/без снапшотов
- Генерация большого количества событий для тестирования
- REST API для управления и просмотра статистики

**Quick Start:**
```bash
cd examples/eventsourcing-snapshots
make up
make run
```

**Документация:** [eventsourcing-snapshots/README.md](eventsourcing-snapshots/README.md)

### eventsourcing-replay

Пример Event Replay и rebuilding проекций из event store.

**Ключевые особенности:**
- Полный replay всех событий для rebuilding проекций
- Replay для конкретного агрегата
- Replay с фильтрацией по типу события
- Progress tracking во время replay
- Batch processing для оптимизации
- CLI команды для различных сценариев replay

**Quick Start:**
```bash
cd examples/eventsourcing-replay
make up
make run
```

**Документация:** [eventsourcing-replay/README.md](eventsourcing-replay/README.md)

### eventsourcing-mongodb

Пример Event Sourcing с использованием MongoDB вместо PostgreSQL.

**Ключевые особенности:**
- Использование MongoDBEventStore
- Настройка MongoDB специфичных параметров (коллекции, индексы)
- Работа с BSON типами и сериализацией
- Агрегат Inventory с операциями управления складом
- Сравнение с PostgreSQL подходом

**Quick Start:**
```bash
cd examples/eventsourcing-mongodb
make up
make run
```

**Документация:** [eventsourcing-mongodb/README.md](eventsourcing-mongodb/README.md)

## Repository Examples

### repository-query-builder

Демонстрация Query Builder для Postgres и MongoDB репозиториев.

**Ключевые особенности:**
- Сложные запросы с фильтрацией, сортировкой, пагинацией
- Joins между таблицами/коллекциями
- Агрегация (Count, Sum, Avg, Min, Max)
- Группировка с Having
- Full-text search (MongoDB)
- Geo queries (MongoDB)
- Index management и performance optimization

**Quick Start:**
```bash
cd examples/repository-query-builder
make docker-up
make migrate
make run
```

**Документация:** [repository-query-builder/README.md](repository-query-builder/README.md)

## GraphQL Transport Example

### graphql-service

Полнофункциональный Product Catalog Service с GraphQL API.

**Ключевые особенности:**
- Автогенерация GraphQL схем из proto файлов
- GraphQL queries для чтения данных (CQRS queries)
- GraphQL mutations для команд (CQRS commands)
- GraphQL subscriptions для real-time обновлений (EventBus)
- Интеграция с Event Sourcing
- Query complexity limits и security
- GraphQL Playground для разработки

**Quick Start:**
```bash
cd examples/graphql-service
make docker-up
make migrate-up
make generate
make run
make playground
```

**Документация:** [graphql-service/README.md](graphql-service/README.md)

## Saga Query Handler Example

### saga-query-handler

Демонстрация SagaQueryHandler с CQRS read models.

**Ключевые особенности:**
- Query handler для получения статуса и истории саг
- Read model store для оптимизированных запросов
- Projection для обновления read models из saga events
- REST API для запросов с фильтрацией и пагинацией
- Метрики выполнения саг

**Quick Start:**
```bash
cd examples/saga-query-handler
make docker-up
make migrate
make run
```

**Документация:** [saga-query-handler/README.md](saga-query-handler/README.md)

## Покрытие функциональности

### Saga Pattern

| Пример | Базовые шаги | Параллельные шаги | Условные шаги | 2PC интеграция | Компенсация |
|--------|-------------|-------------------|---------------|----------------|-------------|
| saga-order | ✅ | ❌ | ❌ | ❌ | ✅ |
| saga-warehouse-integration | ✅ | ❌ | ❌ | ✅ | ✅ |
| saga-parallel | ✅ | ✅ | ❌ | ❌ | ✅ |
| saga-conditional | ✅ | ❌ | ✅ | ❌ | ✅ |

### Event Sourcing

| Пример | Базовые операции | Снапшоты | Replay | PostgreSQL | MongoDB |
|--------|-----------------|----------|--------|------------|---------|
| eventsourcing-basic | ✅ | ❌ | ❌ | ✅ | ❌ |
| eventsourcing-snapshots | ✅ | ✅ | ❌ | ✅ | ❌ |
| eventsourcing-replay | ✅ | ❌ | ✅ | ✅ | ❌ |
| eventsourcing-mongodb | ✅ | ❌ | ❌ | ❌ | ✅ |

## Ссылки на документацию

- [Saga Pattern Framework Documentation](../../framework/saga/README.md)
- [Event Sourcing Framework Documentation](../../framework/eventsourcing/README.md)
- [Framework Overview](../../README.md)

