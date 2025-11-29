# Roadmap

План развития Potter Framework.

> **Текущая версия:** 1.3.1  
> **Следующий релиз:** 1.4.0 (все функции реализованы, ожидается публикация)

## Версия 1.0.0 ✅

- [x] Базовая структура фреймворка
- [x] CQRS Framework
- [x] Transport Layer
- [x] Events System
- [x] Container (DI)
- [x] Metrics
- [x] FSM

## Версия 1.1.0 ✅

- [x] Invoke Module для type-safe работы с CQRS
- [x] Event Sourcing базовая поддержка
- [x] Метрики OpenTelemetry

## Версия 1.2.0 ✅

- [x] Интеграция с популярными message brokers (Kafka, Redis)
- [x] Поддержка WebSocket транспорта
- [x] Интеграция с популярными базами данных (PostgreSQL, MongoDB)
- [x] Code Generator с incremental updates
- [x] Invoke Module с type-safe API
- [x] Testing utilities для приложений

## Версия 1.3.0 ✅

### Event Sourcing
- [x] Полная поддержка Event Sourcing паттерна
- [x] Event Store адаптеры (PostgreSQL, MongoDB, InMemory)
- [x] Snapshot механизм для оптимизации восстановления состояния
- [x] Event replay и projection rebuilding
- [x] Оптимистичная конкурентность через версионирование
- [x] Comprehensive документация и примеры
- [x] Integration с существующими компонентами фреймворка

### Cleanup (v1.3.1)
- [x] Удален нереализованный EventStoreDBAdapter
- [x] Удален неполный SagaQueryHandler
- [x] Очищены TODO комментарии в production коде

## Версия 1.3.x ✅

### Saga Pattern
- [x] Saga Pattern полная реализация
- [x] SagaOrchestrator с автоматической компенсацией
- [x] SagaStep с forward/compensate actions
- [x] Persistence через EventStore и PostgreSQL
- [x] Интеграция с CQRS (CommandBus, QueryBus)
- [x] Интеграция с EventBus для saga events
- [x] Интеграция с 2PC координатором
- [x] Retry механизм с exponential backoff
- [x] Timeout и cancellation support
- [x] Comprehensive документация и примеры
- [x] Order Saga example
- [x] Warehouse 2PC integration example
- [x] Полное покрытие unit и integration тестами

## Версия 1.4.0 ✅

### Repository Enhancements
- ✅ Query Builder для Postgres и MongoDB с fluent API
- ✅ Schema Migrations с версионированием и rollback (PostgreSQL only, MongoDB migrations experimental)
- ✅ Advanced Indexing с автоматическим управлением и рекомендациями
- ✅ TTL поддержка для MongoDB репозиториев
- ✅ Change Streams для реактивных обновлений в MongoDB

### Saga Pattern Enhancements
- ✅ Saga Query Handler с CQRS read models
- ✅ Read model infrastructure для оптимизированных запросов
- ✅ Projection для обновления read models из saga events

### Event Sourcing Enhancements
- ✅ Projection framework с checkpoint management
- ✅ Automatic projection registration и lifecycle management
- ✅ Rebuild support для проекций
- ✅ PostgreSQL и MongoDB checkpoint stores

### Tooling
- ✅ potter-migrate CLI для управления миграциями
- ✅ Интеграция миграций с codegen

### Покрытие примерами (v1.4.0)

✅ **Saga Pattern** - полное покрытие всех типов шагов:

  - Базовые шаги: `saga-order`, `saga-warehouse-integration`

  - Параллельные шаги: `saga-parallel`

  - Условные шаги: `saga-conditional`

  - 2PC интеграция: `saga-warehouse-integration`

  - Query Handler с read models: `saga-query-handler`



✅ **Event Sourcing** - полное покрытие возможностей:

  - Базовые операции: `eventsourcing-basic`

  - Snapshot стратегии: `eventsourcing-snapshots` (Frequency, TimeBased, Hybrid)

  - Event Replay: `eventsourcing-replay` (full, aggregate-specific, filtered)

  - MongoDB persistence: `eventsourcing-mongodb`

  - Projections: все примеры используют projection framework



✅ **Repository** - демонстрация Query Builder и индексов:

  - `repository-query-builder` - сложные запросы, joins, агрегация, full-text search

## Planned (v1.2.x+)

- ⏳ EventStoreDB Adapter (pending stable Go client v21.2+)
  - Базовая реализация готова в `framework/eventsourcing/eventstoredb_store.go` (249 строк кода с полной структурой), ожидает интеграции со стабильным официальным Go client
  - Требуется стабильная версия официального Go client
  - Comprehensive тесты с testcontainers

### GraphQL Transport
- [ ] GraphQL транспорт для запросов
- [ ] Автоматическая генерация GraphQL схем из proto
- [ ] GraphQL subscriptions для real-time обновлений
- [ ] Интеграция с существующими GraphQL серверами

## Версия 1.4 (Планируется)

### Автоматическая генерация OpenAPI спецификаций
- [ ] Генерация OpenAPI 3.0 спецификаций из proto файлов
- [ ] Интеграция с Swagger UI
- [ ] Автоматическая документация REST endpoints
- [ ] Валидация запросов по OpenAPI схеме

### Расширенная поддержка distributed tracing
- [ ] Интеграция с Jaeger, Zipkin, Datadog
- [ ] Автоматическая инструментация всех компонентов
- [ ] Correlation ID propagation через все слои
- [ ] Distributed tracing для 2PC транзакций
- [ ] Performance profiling и bottleneck detection

### Улучшения производительности
- [ ] Connection pooling оптимизация
- [ ] Batch processing для событий
- [ ] Кэширование на уровне фреймворка
- [ ] Оптимизация сериализации (Protobuf, MessagePack)

## Версия 2.0 (Планируется)

### Breaking Changes
- [ ] Удаление deprecated типов (`core.Error`)
- [ ] Рефакторинг API для улучшения консистентности
- [ ] Упрощение конфигурации и инициализации
- [ ] Миграция на новые версии зависимостей

### Новые возможности
- [ ] Поддержка WebAssembly для edge computing
- [ ] Multi-tenancy на уровне фреймворка
- [ ] Автоматическое масштабирование через Kubernetes
- [ ] Serverless deployment support (AWS Lambda, Google Cloud Functions)

### Улучшения разработки
- [ ] Расширенная документация с примерами
- [ ] Интерактивные туториалы
- [ ] Видео-курсы и вебинары
- [ ] Best practices guide для production deployments

## Завершенные версии

Все завершённые версии (v1.0.0 - v1.4.0) детально описаны в разделах выше. Основные достижения:

- **v1.0.0**: Базовая структура фреймворка, CQRS, Transport, Events, Container, Metrics, FSM
- **v1.1.0**: Invoke Module, базовая поддержка Event Sourcing, OpenTelemetry метрики
- **v1.2.0**: Интеграция с message brokers и базами данных, Code Generator, Testing utilities
- **v1.3.0**: Полная реализация Event Sourcing с адаптерами, snapshots, replay
- **v1.3.x**: Полная реализация Saga Pattern с FSM, компенсацией, интеграциями
- **v1.4.0**: Repository enhancements, Saga Query Handler, Projection framework, Tooling

## Приоритеты

1. **Высокий приоритет**: GraphQL Transport, OpenAPI генерация
   - Event Sourcing и Saga Pattern полностью реализованы в v1.4.0
2. **Средний приоритет**: Расширенный distributed tracing
3. **Низкий приоритет**: WebAssembly, multi-tenancy, serverless support

## Обратная связь

Если у вас есть предложения по улучшению фреймворка или новые фичи, которые вы хотели бы видеть, пожалуйста, создайте issue в репозитории проекта.

## Примечания

- Roadmap может изменяться в зависимости от обратной связи сообщества
- Приоритеты могут быть пересмотрены на основе реальных потребностей пользователей
- Breaking changes будут объявлены заранее в CHANGELOG.md

