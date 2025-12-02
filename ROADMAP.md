# Roadmap

План развития Potter Framework.

> **Текущая версия:** 1.5.0  
> **Следующий релиз:** 1.6.0 (планируется)

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
- ✅ Интеграция с goose для управления миграциями
- ✅ Обертка над goose для программного использования
- ✅ Поддержка Go-миграций для MongoDB
- ✅ Интеграция миграций с codegen

## Версия 1.5.0 ✅

### Migrations
- ✅ Goose integration для версионированных миграций
- ✅ CLI инструмент potter-migrate для управления миграциями
- ✅ Программный API для запуска миграций

### Покрытие примерами (v1.4.0)

✅ **Saga Pattern** - полное покрытие всех типов шагов:

  - Базовые шаги: `saga-order`

  - Параллельные шаги: `saga-parallel`

  - Условные шаги: `saga-conditional`

  - Query Handler с read models: `saga-query-handler`



✅ **Event Sourcing** - полное покрытие возможностей:

  - Базовые операции: `eventsourcing-basic`

  - Snapshot стратегии: `eventsourcing-snapshots` (Frequency, TimeBased, Hybrid)

  - Event Replay: `eventsourcing-replay` (full, aggregate-specific, filtered)

  - MongoDB persistence: `eventsourcing-mongodb`

  - Projections: все примеры используют projection framework

## Planned (v1.6.0+)

- ⏳ **EventStoreDB Adapter** (экспериментальный, не готов к production)
  - ⚠️ **СТАТУС**: Экспериментальный плейсхолдер, не готов к production использованию
  - Базовая структура готова в `framework/eventsourcing/eventstoredb_store.go`, но требует интеграции со стабильным официальным Go client
  - Все методы возвращают ошибку "EventStoreDB adapter not fully implemented - requires stable Go client"
  - Блокирующий фактор: отсутствие стабильной версии официального Go client для EventStoreDB
  - После появления стабильного клиента потребуется:
    - Интеграция с официальным Go client
    - Comprehensive тесты с testcontainers
    - Обновление документации и примеров

### GraphQL Transport ✅ (v1.4.0)
- ✅ GraphQL транспорт для запросов
- ✅ Автоматическая генерация GraphQL схем из proto
- ✅ GraphQL subscriptions для real-time обновлений
- ✅ Интеграция с существующими GraphQL серверами
- ⚠️ Требуется стабилизация и улучшение DX для production использования

## Версия 1.6.0 (Планируется)

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

Все завершённые версии (v1.0.0 - v1.5.0) детально описаны в разделах выше. Основные достижения:

- **v1.0.0**: Базовая структура фреймворка, CQRS, Transport, Events, Container, Metrics, FSM
- **v1.1.0**: Invoke Module, базовая поддержка Event Sourcing, OpenTelemetry метрики
- **v1.2.0**: Интеграция с message brokers и базами данных, Code Generator, Testing utilities
- **v1.3.0**: Полная реализация Event Sourcing с адаптерами, snapshots, replay
- **v1.3.x**: Полная реализация Saga Pattern с FSM, компенсацией, интеграциями
- **v1.4.0**: Repository enhancements, Saga Query Handler, Projection framework, Tooling
- **v1.5.0**: Goose integration для версионированных миграций

## Приоритеты

1. **Высокий приоритет** (v1.6.0):
   - Стабилизация GraphQL транспорта и доведение до production-ready состояния
   - Доведение кодогенератора до действительно production-ready состояния
   - Улучшение observability и DX (developer experience)
   - OpenAPI генерация из proto файлов

2. **Средний приоритет** (v1.7.0+):
   - Расширенный distributed tracing с интеграцией Jaeger, Zipkin, Datadog
   - Улучшения производительности (connection pooling, batch processing, оптимизация сериализации)

3. **Низкий приоритет** (v2.0+):
   - WebAssembly поддержка для edge computing
   - Multi-tenancy на уровне фреймворка
   - Serverless deployment support (AWS Lambda, Google Cloud Functions)

## Обратная связь

Если у вас есть предложения по улучшению фреймворка или новые фичи, которые вы хотели бы видеть, пожалуйста, создайте issue в репозитории проекта.

## Примечания

- Roadmap может изменяться в зависимости от обратной связи сообщества
- Приоритеты могут быть пересмотрены на основе реальных потребностей пользователей
- Breaking changes будут объявлены заранее в CHANGELOG.md

