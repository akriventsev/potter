# Changelog

Все значимые изменения в этом проекте будут документироваться в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.0.0/),
и этот проект придерживается [Semantic Versioning](https://semver.org/lang/ru/).

## [Unreleased]

### Fixed

- Исправлена ошибка "Input is shadowed in the --proto_path" при генерации кода из proto файлов
- Улучшена логика вызова protoc для корректной обработки путей к proto файлам

### Added

- Автоматическая генерация GraphQL транспорта из proto спецификаций
- Поддержка множественных транспортов через поле `transport` в proto options
- Интеграция GraphQL с Potter CQRS (CommandBus, QueryBus, EventBus)
- Автоматическая генерация GraphQL схемы, резолверов и subscriptions
- Генерация GraphQL адаптера с настройкой Playground и Introspection

### Changed

- Удален флаг `--with-graphql` из potter-gen CLI (GraphQL теперь активируется через proto options)
- Рефакторинг PresentationGenerator для поддержки множественных транспортов
- Обновлена генерация main.go для автоматической инициализации указанных транспортов

### Added (v1.6.0 - Development)

> **Примечание:** Текущая ветка представляет собой development snapshot версии 1.6.0, не предназначенный для production использования. Функциональность частично реализована и требует доработки и тестирования.

## [1.6.0] - TBD (Planned)

### Added

#### OpenAPI Integration

- **OpenAPIGenerator**: Автоматическая генерация OpenAPI 3.0 спецификаций из proto файлов
  - `framework/codegen/openapi_generator.go` - генератор с TypeMapper и SchemaBuilder
  - Маппинг proto типов → OpenAPI типов (string, integer, number, boolean, array, object)
  - Генерация paths из Commands/Queries с REST conventions
  - Генерация components/schemas из агрегатов
  - Поддержка Potter annotations → OpenAPI extensions (x-async, x-cacheable)
  - Выходные файлы: `api/openapi/openapi.yaml`

- **SwaggerUIAdapter**: Интеграция Swagger UI в REST транспорт
  - `framework/adapters/transport/swagger.go` - адаптер с lifecycle management
  - SwaggerUIConfig для настройки (Path, SpecPath, DeepLinking, ValidateSpec)
  - Endpoints: GET /swagger/openapi.yaml, GET /swagger/ (UI)
  - Использует Swagger UI CDN для assets
  - Опциональная валидация OpenAPI спецификации при загрузке
  - Интеграция с metrics для мониторинга

- **OpenAPIValidator**: Middleware для валидации HTTP запросов по OpenAPI схеме
  - `framework/adapters/transport/openapi_validation.go` - validator с kin-openapi
  - Валидация request body, query params, headers, path params по schema
  - ValidationOptions для настройки (ValidateRequest, ValidateResponse, MultiError)
  - Детальные сообщения об ошибках валидации с указанием поля и ожидаемой schema
  - Gin middleware для автоматической валидации
  - Кеширование загруженной спецификации для производительности

- **Proto Options Extensions**: Расширение `api/proto/potter/options.proto`
  - OpenAPIInfo message с title, version, description, contact, license
  - OpenAPIContact и OpenAPILicense для метаданных
  - Поля tags, summary, description, deprecated в CommandOptions и QueryOptions
  - enable_openapi, openapi_version, openapi_info в ServiceOptions
  - Все поля опциональны для обратной совместимости

- **Codegen Integration**: Интеграция OpenAPI в presentation generator
  - Обновление `PresentationGenerator.Generate()` для поддержки OpenAPI транспорта
  - `generateOpenAPIIntegration()` метод для генерации openapi.yaml
  - `generateSwaggerUIAdapter()` для генерации Swagger UI registration кода
  - Обновление `generateAPIExamples()` с секцией OpenAPI
  - Автоматическая генерация при наличии REST транспорта

- **Examples**: Comprehensive пример `examples/openapi-service/`
  - Демонстрация генерации OpenAPI спецификации из proto
  - Интеграция Swagger UI
  - Валидация запросов по OpenAPI схеме
  - REST API с CRUD операциями
  - Docker Compose для локального запуска
  - Подробный README с примерами использования

#### Observability Module

- **Distributed Tracing**: OpenTelemetry интеграция для distributed tracing
  - `framework/observability/tracing.go` - TracingManager с lifecycle management
  - TracingConfig для настройки (ServiceName, Exporter, SamplingRate, Environment)
  - Поддержка exporters: Jaeger, Zipkin, OTLP, Stdout
  - HTTPTracingMiddleware для автоматической инструментации HTTP requests
  - GRPCTracingInterceptor для gRPC calls
  - TraceCommand, TraceQuery, TraceEvent для интеграции с CQRS
  - Автоматическая propagation trace context через W3C Trace Context
  - Resource attributes (service.name, service.version, deployment.environment)

- **Correlation ID Propagation**: Сквозная propagation через все слои
  - ExtractCorrelationID, InjectCorrelationID, PropagateCorrelationID утилиты
  - CorrelationIDMiddleware для автоматической генерации/propagation
  - Propagation через HTTP headers (X-Correlation-ID)
  - Propagation через gRPC metadata
  - Интеграция с trace context

- **Debugging Utilities**: Инструменты для production debugging
  - `framework/observability/debugging.go` - DebugManager с pprof endpoints
  - DebugConfig для настройки (EnablePprof, PprofPort, EnableHealthCheck)
  - Pprof endpoints: /debug/pprof/* (heap, goroutine, profile, trace)
  - RequestDumpMiddleware для логирования HTTP requests/responses
  - ProfileCommand для профилирования команд
  - DetectBottlenecks для автоматического обнаружения bottlenecks
  - Sanitization sensitive data в логах

- **Health Checks**: Built-in health checks для Kubernetes integration
  - HealthCheck interface и HealthCheckResult struct
  - DatabaseHealthCheck - проверка подключения к БД
  - MessageBusHealthCheck - проверка message bus
  - DiskSpaceHealthCheck - проверка свободного места
  - MemoryHealthCheck - проверка использования памяти
  - Endpoints: GET /health (liveness), GET /ready (readiness)
  - JSON response с детальным статусом всех проверок
  - Интеграция с Kubernetes probes

- **Documentation**: Comprehensive документация
  - `framework/observability/README.md` - полное руководство по observability
  - Quick start guide с примерами кода
  - Интеграция с Jaeger, Zipkin, Prometheus
  - Best practices для production deployment
  - Troubleshooting guide

- **Examples**: Comprehensive пример `examples/observability-demo/`
  - Демонстрация distributed tracing с Jaeger
  - Correlation ID propagation
  - Health checks и readiness probes
  - Pprof profiling
  - Metrics с Prometheus и Grafana dashboards
  - Docker Compose с полным observability stack
  - Load testing и performance analysis
  - Подробный README с примерами использования

#### Production Best Practices

- **Production Deployment Guide**: Comprehensive руководство
  - `docs/PRODUCTION_BEST_PRACTICES.md` - полное руководство по production deployment
  - Configuration management (environment variables, secrets)
  - Database migrations best practices (zero-downtime, expand-contract pattern)
  - Security guidelines (authentication, authorization, TLS, input validation)
  - Performance optimization (connection pooling, caching, resource limits)
  - High availability setup (horizontal scaling, load balancing, database HA)
  - Disaster recovery (backups, recovery testing, multi-region)
  - Monitoring and alerting (key metrics, Prometheus alerts, on-call rotation)
  - CI/CD pipeline examples (GitHub Actions, deployment strategies)
  - Troubleshooting guide (common issues, debugging techniques)
  - Production deployment checklist

### Changed

- **VERSION**: Обновлена версия с 1.5.0 на 1.6.0-dev для обозначения development версии
- **ROADMAP.md**: Детализированы планы v1.6.0 с разбивкой задач по компонентам
  - Добавлена секция "Планируемые компоненты v1.6.0" в метрики качества
  - Обновлена таблица "Статус по версиям" с прогрессом v1.6.0 (40% завершено)
  - Детализированы приоритеты с указанием файлов и зависимостей
  - Отмечены реализованные компоненты (OpenAPI Generator, Swagger UI, Validation, Observability)

### Fixed

- Исправлено использование baggage API в tracing.go (заменено на go.opentelemetry.io/otel/baggage)
- Исправлено использование pprof API в debugging.go (заменено runtime/pprof на net/http/pprof)
- Реализовано использование всех полей ValidationOptions в openapi_validation.go
- Исправлена генерация correlation ID (используется UUID вместо неправильного кода)

### Known Issues

- Генерация schemas для request/response messages в OpenAPIGenerator требует доработки
- Парсинг OpenAPIInfo из ServiceOptions требует реализации
- Unit и integration тесты для новых компонентов требуются

### Dependencies

- Added `github.com/getkin/kin-openapi/openapi3` - OpenAPI 3.0 parser and validator
- Added `go.opentelemetry.io/otel` - OpenTelemetry SDK
- Added `go.opentelemetry.io/otel/exporters/jaeger` - Jaeger exporter
- Added `go.opentelemetry.io/otel/exporters/zipkin` - Zipkin exporter
- Added `go.opentelemetry.io/otel/exporters/otlp/otlptrace` - OTLP exporter
- Added `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` - HTTP instrumentation
- Added `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` - gRPC instrumentation

### Notes

- EventStoreDB Adapter остается в experimental статусе, ожидая stable Go client v21.2+
- Все новые компоненты полностью обратно совместимы с v1.5.0
- OpenAPI генерация автоматически активируется при наличии REST транспорта
- Observability компоненты опциональны и настраиваются через config
- Production best practices основаны на реальном production опыте

## [1.5.0] - 2025-XX-XX

### Added

#### GraphQL Transport

- **GraphQLAdapter**: Новый транспортный адаптер для GraphQL API
  - Интеграция с CommandBus и QueryBus для CQRS
  - Поддержка GraphQL Playground для разработки
  - Query complexity analysis и depth limiting для безопасности
  - Метрики через framework/metrics
  - Graceful shutdown и lifecycle management

- **GraphQL Resolvers**: Автоматические resolvers для CQRS
  - CommandResolver для GraphQL mutations → CQRS commands
  - QueryResolver для GraphQL queries → CQRS queries
  - SubscriptionResolver для GraphQL subscriptions → EventBus
  - ResolverRegistry для управления resolvers

- **GraphQL Subscriptions**: Real-time обновления через WebSocket
  - SubscriptionManager с интеграцией EventBus
  - Фильтрация событий (correlation ID, aggregate ID)
  - Поддержка graphql-ws protocol (Apollo subscriptions)
  - Автоматическая очистка при disconnect

- **Schema Generation**: Автоматическая генерация GraphQL схем из proto
  - GraphQLSchemaGenerator в framework/codegen
  - Маппинг Potter annotations → GraphQL directives
  - Генерация Query, Mutation, Subscription types
  - Генерация Input types и custom types
  - Интеграция с gqlgen для code generation

- **CLI**: Расширение potter-gen
  - Новый флаг `--with-graphql` для генерации GraphQL схем
  - Автоматический вызов gqlgen generate

- **Documentation**:
  - Подробная документация в framework/adapters/transport/GRAPHQL.md
  - Обновлен README с GraphQL примерами
  - API reference для GraphQL компонентов

- **Examples**:
  - Новый пример `examples/graphql-service/`
  - Демонстрация queries, mutations, subscriptions
  - Интеграция с Event Sourcing
  - Docker Compose setup
  - GraphQL Playground queries

- **Tests**:
  - Unit тесты для GraphQLAdapter, Resolvers, SubscriptionManager
  - Integration тесты для end-to-end flow
  - Subscription testing
  - Schema generation tests

### Dependencies

- Added `github.com/99designs/gqlgen v0.17.49` - GraphQL server library
- Added `github.com/vektah/gqlparser/v2 v2.5.16` - GraphQL parser

### Changed

- **BREAKING**: Заменена самописная система миграций на интеграцию с goose
- Формат файлов миграций изменен на goose формат (единый файл с аннотациями `-- +goose Up/Down`)
- CLI инструмент `potter-migrate` переписан на использование goose
- Все примеры обновлены для использования goose

### Added

- Интеграция с goose v3 для управления миграциями
- Обертка `framework/migrations/goose_wrapper.go` для программного использования
- Поддержка Go-миграций для MongoDB
- Поддержка out-of-order миграций
- Поддержка environment variable substitution в миграциях
- Документация по миграции с Potter v1.3.x
- Скрипты для автоматической конвертации старых миграций

### Removed

- Удалены файлы самописной системы миграций (~1000 строк кода):
  - `framework/migrations/migrator.go`
  - `framework/migrations/file_source.go`
  - `framework/migrations/postgres_adapter.go`
  - `framework/migrations/mongodb_adapter.go`
  - `framework/migrations/migrator_test.go`

### Migration Guide

Для миграции существующих проектов:

1. Установите goose: `go install github.com/pressly/goose/v3/cmd/goose@latest`
2. Конвертируйте миграции: объедините `.up.sql` и `.down.sql` файлы в один с аннотациями
3. Обновите скрипты сборки для использования goose вместо potter-migrate
4. См. полную документацию в `framework/migrations/README.md` и `framework/migrations/MIGRATION_GUIDE.md`

## [1.3.1] - 2025-XX-XX

### Removed

- Удален экспериментальный EventStoreDBAdapter из framework/eventsourcing (не был реализован)
- Удален неполный SagaQueryHandler из framework/saga/integration.go

### Changed

- Очищены TODO комментарии в production коде (adapters/repository, codegen)
- Улучшена документация для PostgresRepository и MongoRepository с четким описанием текущих возможностей
- Обновлен codegen для генерации более информативных комментариев при изменении сигнатур

### Documentation

- Обновлен README для eventsourcing с актуальным списком поддерживаемых адаптеров
- Добавлен roadmap для будущих улучшений (EventStoreDB, query builders, migrations)

## [1.3.0] - 2025-XX-XX

### Added

#### Saga Pattern Module

- **Core Components**
  - `framework/saga/` - полная реализация Saga Pattern через FSM
  - `Saga` интерфейс с методами Execute, Compensate, Resume
  - `SagaDefinition` для декларативного создания саг
  - `SagaContext` для передачи данных между шагами
  - `SagaStatus` enum (Pending, Running, Completed, Compensating, Compensated, Failed)

- **SagaStep**
  - `SagaStep` интерфейс с forward/compensate actions
  - `BaseStep` базовая реализация с timeout и retry
  - `CommandStep` для выполнения команд через CommandBus
  - `EventStep` для публикации событий
  - `TwoPhaseCommitStep` для интеграции с 2PC
  - `ParallelStep` для параллельного выполнения
  - `ConditionalStep` для условного выполнения
  - `RetryPolicy` с NoRetry, SimpleRetry, ExponentialBackoff

- **SagaOrchestrator**
  - `DefaultOrchestrator` для координации выполнения
  - Автоматическая компенсация при ошибках в обратном порядке
  - Поддержка параллельного выполнения шагов
  - Публикация saga events через EventBus
  - Recovery механизм для возобновления после сбоя
  - Timeout и cancellation support

- **Persistence**
  - `SagaPersistence` интерфейс
  - `EventStorePersistence` через Event Sourcing
  - `PostgresPersistence` с таблицами saga_instances, saga_history, saga_snapshots
  - `InMemoryPersistence` для тестирования
  - Snapshot механизм для оптимизации
  - SQL миграции для PostgreSQL

- **Builder API**
  - `SagaBuilder` для fluent API создания саг
  - `StepBuilder` для создания шагов
  - Валидация при построении

- **Events**
  - Saga lifecycle events: SagaStarted, SagaCompleted, SagaFailed, SagaCompensating, SagaCompensated
  - Step events: StepStarted, StepCompleted, StepFailed, StepCompensating, StepCompensated
  - Интеграция с EventBus для публикации

- **Integration**
  - Адаптеры для CommandBus, EventBus, 2PC
  - `SagaCommandHandler` для запуска саг через CommandBus
  - `SagaQueryHandler` для получения статуса
  - Type-safe интеграция через generics

- **Factory**
  - `OrchestratorFactory` для создания orchestrator
  - `PersistenceFactory` для создания persistence
  - `StepFactory` для создания различных типов шагов
  - `SagaRegistry` для регистрации saga definitions

- **Examples**
  - `examples/saga-order/` - Order Saga с резервированием, оплатой, доставкой
  - `examples/saga-warehouse-integration/` - интеграция с warehouse 2PC
  - Docker Compose для всех зависимостей
  - API examples и curl запросы

- **Documentation**
  - `framework/saga/README.md` - comprehensive документация
  - Архитектурные диаграммы (Mermaid)
  - Quick Start guide
  - Best practices
  - API reference
  - Troubleshooting guide

- **Tests**
  - Unit тесты для всех компонентов (100% покрытие критических путей)
  - Integration тесты с CQRS, EventBus, 2PC
  - Scenario тесты для Order Saga
  - E2E тесты с Docker Compose
  - Performance benchmarks
  - Concurrency тесты

### Changed

- Обновлен `framework/fsm/` для поддержки saga use cases
- Расширен `framework/events/` для saga events
- Обновлен ROADMAP.md с завершенными задачами

### Integration

- Полная интеграция с существующими модулями:
  - `framework/fsm/` - базовый state machine
  - `framework/eventsourcing/` - persistence через EventStore
  - `framework/cqrs/` - выполнение команд и запросов
  - `framework/events/` - публикация saga events
  - `framework/invoke/` - type-safe команды
  - `examples/warehouse/infrastructure/twopc/` - 2PC координатор

#### Event Sourcing Module

- **framework/eventsourcing** - полная реализация Event Sourcing паттерна
  - `EventStore` интерфейс для хранения событий с версионированием
  - `EventSourcedAggregate` базовый класс для агрегатов с replay механизмом
  - `EventSourcedRepository` generic репозиторий для Event Sourced агрегатов
  - `SnapshotStore` интерфейс для оптимизации через снапшоты
  - `EventReplayer` механизм для replay событий и rebuilding проекций

- **Event Store Adapters**:
  - `InMemoryEventStore` - для тестирования и разработки
  - `PostgresEventStore` - production-ready адаптер с оптимизациями
  - `MongoDBEventStore` - NoSQL вариант для гибкого хранения
  - `EventStoreDBAdapter` - ⚠️ EXPERIMENTAL/PLACEHOLDER: интеграция с EventStore DB (не реализована, планируется в будущих версиях)
  - `InMemorySnapshotStore`, `PostgresSnapshotStore`, `MongoDBSnapshotStore`

- **Snapshot Strategies**:
  - `FrequencySnapshotStrategy` - создание каждые N событий
  - `TimeBasedSnapshotStrategy` - создание по времени
  - `HybridSnapshotStrategy` - комбинированная стратегия

- **Features**:
  - Оптимистичная конкурентность через версионирование событий
  - Автоматическое создание снапшотов по настраиваемым стратегиям
  - Event replay для восстановления состояния и rebuilding проекций
  - Batch processing для производительности
  - Progress tracking для длительных replay операций
  - SQL миграции для PostgreSQL Event Store
  - Comprehensive unit и integration тесты
  - Benchmark тесты для производительности

- **Documentation**:
  - `framework/eventsourcing/README.md` - полная документация модуля
  - Архитектурные диаграммы и best practices
  - API reference для всех компонентов
  - Migration guide для перехода на Event Sourcing

- **Examples**:
  - `examples/eventsourcing-basic` - базовый пример с банковским счетом
  - `examples/warehouse/domain/product_eventsourced.go` - Event Sourced версия Product
  - Сравнение обычных агрегатов и Event Sourced
  - Docker Compose для запуска примеров

- **Factory and Builders**:
  - `EventStoreFactory` для создания различных адаптеров
  - `SnapshotStoreFactory` для snapshot stores
  - `EventSourcingBuilder` с fluent API для конфигурации

### Changed

- Обновлен `framework/README.md` с секцией о Event Sourcing
- Обновлен `examples/warehouse/README.md` с примерами Event Sourcing
- Обновлен `ROADMAP.md` - Event Sourcing задачи отмечены как выполненные

### Performance

- Snapshots обеспечивают быструю загрузку агрегатов с большой историей
- Batch processing в EventReplayer для эффективного replay
- Оптимизированные индексы в PostgreSQL для быстрых запросов
- Connection pooling в адаптерах БД

### Testing

- 100+ unit тестов для всех компонентов Event Sourcing
- Integration тесты для PostgreSQL и MongoDB адаптеров
- Benchmark тесты для измерения производительности
- Mock компоненты для тестирования приложений

## [1.2.0] - 2025-XX-XX

> **Примечание**: Для планируемых, но ещё не реализованных фич см. [ROADMAP.md](ROADMAP.md).

### Added

#### Warehouse Example

- **Warehouse Example** теперь считается частью официальной демонстрационной витрины и обновляется при изменении ядра фреймворка. Пример синхронизирован с версиями фреймворка и демонстрирует best practices использования всех компонентов.

#### Code Generator

- **Potter Custom Options** - protobuf extensions для аннотаций Commands, Queries, Events, Aggregates
- **potter-gen CLI** - инструмент для генерации и обновления кода из proto файлов
  - `potter-gen init` - инициализация нового проекта
  - `potter-gen generate` - генерация кода
  - `potter-gen update` - обновление с сохранением пользовательского кода
  - `potter-gen sdk` - генерация SDK
- **protoc-gen-potter** - protoc плагин для интеграции в стандартный workflow
- **framework/codegen** - библиотека генераторов:
  - `DomainGenerator` - генерация агрегатов, событий, репозиториев
  - `ApplicationGenerator` - генерация команд, запросов, handlers
  - `InfrastructureGenerator` - генерация репозиториев, cache, миграций
  - `PresentationGenerator` - генерация REST handlers
  - `MainGenerator` - генерация main.go, Makefile, docker-compose
  - `SDKGenerator` - генерация type-safe SDK на базе invoke
- **Code Updater** - система обновления с сохранением пользовательского кода:
  - Парсинг существующего кода через go/ast
  - Извлечение пользовательских блоков по маркерам
  - Merge с новым сгенерированным кодом
  - Интерактивный режим с diff preview
- **Examples** - пример использования кодогенератора (`examples/codegen/`)

#### Features

- Генерация полной структуры CQRS приложения из proto файлов
- Incremental updates с сохранением бизнес-логики
- SDK generation для интеграции в другие сервисы
- Поддержка всех Potter транспортов (REST, NATS, Kafka, gRPC)
- Автоматическая генерация SQL миграций
- Docker Compose для локальной разработки
- Comprehensive документация и примеры

- **Invoke Module Enhancements**: Расширение модуля `framework/invoke/` с новыми возможностями
  - `SubjectResolver` интерфейс и реализации (`DefaultSubjectResolver`, `FunctionSubjectResolver`, `StaticSubjectResolver`) для гибкой настройки subjects команд и событий
  - `ErrorEvent` интерфейс и `BaseErrorEvent` для стандартизации событий об ошибках с методами `Error()`, `ErrorCode()`, `ErrorMessage()`, `IsRetryable()`, `OriginalCommand()`
  - `EventSource` интерфейс и адаптеры (`EventBusAdapter`, `TransportSubscriberAdapter`) для унификации источников событий
  - Поддержка `transport.Subscriber` в `EventAwaiter` через `NewEventAwaiterFromTransport()` для работы с NATS, Kafka, Redis
  - Методы `AwaitAny()` и `AwaitSuccessOrError()` в `EventAwaiter` для ожидания любого из нескольких событий
  - Трехпараметровый `CommandInvoker[TCommand, TSuccessEvent, TErrorEvent]` для обработки успешных и ошибочных событий
  - Метод `InvokeWithBothResults()` в `CommandInvoker` для получения обоих типов событий
  - Конструктор `NewCommandInvokerWithoutError()` для обратной совместимости
  - Интеграция `SubjectResolver` в `AsyncCommandBus` с методами `WithSubjectResolver()`, `WithCommandSubjectFunc()`
  - Расширение `InvokeOptions` с полями `SubjectResolver`, `EventSource`, `SuccessEventType`, `ErrorEventType`
  - Новые опции: `WithSubjectResolver()`, `WithEventSource()`, `WithTransportSubscriber()`, `WithEventBus()`, `WithSuccessEventType()`, `WithErrorEventType()`
  - Новые типы ошибок: `ErrInvalidSubjectResolver`, `ErrEventSourceNotConfigured`, `ErrErrorEventReceived`
  - Примеры событий об ошибках в warehouse domain: `ProductCreationFailedEvent`, `StockAdjustmentFailedEvent`, `ReservationFailedEvent`
- Comprehensive examples for Invoke module demonstrating integration with different transports:
  - Command examples: NATS (with EventBus), Kafka (with Kafka Events)
  - Query examples: NATS Request-Reply, Kafka Request-Reply, REST HTTP, gRPC
  - Advanced example: Mixed transports usage in single application
- Docker Compose setup for running example dependencies (NATS, Kafka, Redis, PostgreSQL)
- Makefile for simplified example execution and infrastructure management
- Detailed README in `framework/invoke/examples/` with setup instructions

### Changed

- **CommandInvoker**: Изменена generic-сигнатура с `CommandInvoker[TCommand, TEvent]` на `CommandInvoker[TCommand, TSuccessEvent, TErrorEvent]` (breaking change)
- **EventAwaiter**: Изменен конструктор с `NewEventAwaiter(eventBus)` на `NewEventAwaiter(eventSource)` (breaking change)
- **AsyncCommandBus**: Заменено поле `subjectPrefix` на `subjectResolver SubjectResolver` для гибкой настройки subjects
- **Warehouse Example**: Обновлен для демонстрации новых возможностей (SubjectResolver, TransportSubscriber, ErrorEvents)
- Updated `framework/invoke/README.md` with links to practical examples
- Enhanced documentation with real-world usage patterns

### Deprecated

- **CommandInvoker**: Двухпараметровый `NewCommandInvoker[TCommand, TEvent]` помечен как deprecated, используйте `NewCommandInvokerWithoutError()` для обратной совместимости
- **EventAwaiter**: Прямое использование `events.EventBus` в конструкторе помечено как deprecated, используйте `NewEventAwaiterFromEventBus()`

### Migration

Для миграции с версии 1.1.x:

1. **CommandInvoker**: Замените `NewCommandInvoker[TCmd, TEvent](...)` на `NewCommandInvokerWithoutError[TCmd, TEvent](...)` или обновите до трехпараметрового варианта с поддержкой ошибок
2. **EventAwaiter**: Замените `NewEventAwaiter(eventBus)` на `NewEventAwaiterFromEventBus(eventBus)` или используйте `NewEventAwaiterFromTransport()` для работы с транспортами
3. **AsyncCommandBus**: Метод `WithSubjectPrefix()` все еще работает, но рекомендуется использовать `WithSubjectResolver()` для большей гибкости

Подробные инструкции по миграции см. в `framework/invoke/README.md#migration-guide`

## [1.1.0] - 2024-XX-XX

### Added

- **Invoke Module**: Новый модуль `framework/invoke/` для type-safe работы с CQRS командами и запросами
  - `CommandInvoker[TCmd, TEvent]` - generic invoker для команд с ожиданием событий по correlation ID
  - `QueryInvoker[TQuery, TResult]` - generic type-safe обертка над QueryBus
  - `EventAwaiter` - consumer событий по correlation ID с timeout
  - `AsyncCommandBus` - чистый producer команд для NATS pub/sub (produce command)
  - Утилиты для работы с correlation ID и causation ID
  - Сериализаторы: JSON, Protobuf, MessagePack (опционально)
  - Comprehensive unit тесты для всех компонентов
  - Полная документация с примерами использования

- **Pure Produce/Consume Pattern**: Реализация чистого produce/consume паттерна без request-reply overhead
  - Команды публикуются в NATS через AsyncCommandBus (produce)
  - События ожидаются через EventAwaiter по correlation ID (consume)
  - Автоматический матчинг событий с командами через correlation ID

- **Type Safety**: Generic-based API для compile-time проверки типов
  - Type-safe команды и запросы через generics
  - Автоматическое приведение типов результатов
  - Валидация результатов запросов

- **Integration Examples**: Примеры интеграции в warehouse приложении
  - Обновлен `CreateProductHandler` для propagation correlation ID в события
  - Добавлены примеры использования CommandInvoker и QueryInvoker в main.go

### Changed

- **Warehouse Example**: Обновлен handler создания товара для поддержки correlation ID
  - Извлечение correlation ID из контекста
  - Добавление correlation ID и causation ID в метаданные событий

## [1.0.3] - 2024-XX-XX

### Removed

- **internal/ директория**: Полностью удалена директория `internal/` из проекта
  - Удалены deprecated адаптеры из `internal/adapters/event/` (in_memory_event_publisher.go, framework_event_publisher_adapter.go, messagebus_event_publisher.go, nats_event_publisher.go)
  - Удалены deprecated адаптеры из `internal/adapters/messagebus/` (nats_adapter.go)
  - Удалены модули DI контейнера из `internal/container/modules/` (event_module.go, messagebus_module.go)
  - Причина: все компоненты перенесены в `framework/adapters/` (версия 1.1.0), код не используется в проекте

### Fixed

- **Ошибки компиляции**: Устранены ошибки компиляции, вызванные ссылками на несуществующий `internal/domain` в deprecated адаптерах
- **Архитектурная чистота**: Проект теперь полностью использует только `framework/` компоненты без legacy кода

### Changed

- Проект теперь содержит только актуальный код фреймворка и примеры
- Упрощена структура проекта - удалены неиспользуемые компоненты

## [1.0.2] - 2024-XX-XX

### Added

- **Warehouse Example Infrastructure**: Полная инфраструктура для examples/warehouse
  - docker-compose.yml с PostgreSQL, Redis, NATS, Prometheus
  - SQL миграции (001_create_tables.sql) для всех таблиц
  - prometheus.yml для сбора метрик приложения
  - Domain entities: Product и Warehouse
  - Comprehensive README.md с инструкциями и архитектурой
  - api_examples.md с curl примерами для всех endpoints
  - .gitignore для warehouse директории

### Fixed

- **Warehouse Example**: Устранены все блокирующие проблемы запуска
  - Добавлены отсутствующие domain файлы (product.go, warehouse.go)
  - Создана полная схема БД для всех сущностей (products, warehouses, stocks, reservations, transaction_log)
  - Обновлен Makefile с командами docker-logs, docker-clean, улучшенной migrate

### Changed

- **Documentation**: Обновлен главный README с полным описанием warehouse примера
- **Makefile**: Улучшена команда migrate с проверкой доступности PostgreSQL

## [1.0.1] - 2024-XX-XX

### Fixed

- Исправлен путь модуля с `github.com/potter/v1` на `potter` для корректной работы локальной разработки
- Создана точка входа `examples/warehouse/cmd/server/main.go` с полной инициализацией приложения
- Обновлены все импорты в проекте для использования нового пути модуля
- Добавлен `.gitignore` для игнорирования временных файлов и артефактов сборки

### Changed

- Warehouse example теперь полностью функционален и может быть запущен через `make run`


## [1.0.0] - 2024-01-XX

### Added

- **Framework Core**: Создана базовая структура фреймворка с интерфейсами и типами
  - `framework/core/interfaces.go` - базовые интерфейсы (Component, Lifecycle, Configurable, etc.)
  - `framework/core/types.go` - базовые типы (Context, Error, Result, Option, etc.)
  - `framework/core/errors.go` - система ошибок фреймворка

- **CQRS Framework**: Полная реализация CQRS паттерна
  - `framework/cqrs/registry.go` - реестр обработчиков команд и запросов
  - `framework/cqrs/builder.go` - построители для конфигурации обработчиков
  - `framework/cqrs/middleware.go` - middleware для обработчиков (logging, validation, recovery, timeout, retry, circuit breaker, rate limit, tracing, authorization, caching)
  - `framework/cqrs/helpers.go` - вспомогательные функции для работы с CQRS
  - `framework/cqrs/factory.go` - фабрика для создания обработчиков

- **Transport Layer**: Транспортный слой для команд, запросов и message bus
  - `framework/transport/command.go` - интерфейсы и типы для команд
  - `framework/transport/query.go` - интерфейсы и типы для запросов
  - `framework/transport/bus.go` - реализации шин команд и запросов
  - `framework/transport/messagebus.go` - абстракции для message bus

- **Events System**: Система событий для асинхронной обработки
  - `framework/events/event.go` - базовые интерфейсы для событий
  - `framework/events/publisher.go` - реализации публикаторов событий (InMemory, Async, Batch)
  - `framework/events/subscriber.go` - реализации подписчиков на события
  - `framework/events/bus.go` - шина событий для pub/sub паттерна

- **Container**: DI контейнер с модульной архитектурой
  - `framework/container/module.go` - система модулей, адаптеров и транспортов
  - `framework/container/container.go` - DI контейнер с поддержкой generic типов
  - `framework/container/builder.go` - построитель контейнера
  - `framework/container/initializer.go` - инициализатор с разрешением зависимостей

- **Metrics**: Система метрик на основе OpenTelemetry
  - `framework/metrics/metrics.go` - сборщик метрик приложения
  - `framework/metrics/setup.go` - функции для настройки системы метрик

- **FSM**: Конечный автомат для саг и оркестрации
  - `framework/fsm/fsm.go` - реализация конечного автомата
  - `framework/fsm/state.go` - определения состояний
  - `framework/fsm/transition.go` - определения переходов
  - `framework/fsm/event.go` - определения событий
  - `framework/fsm/action.go` - определения действий

- **Documentation**: Полная документация фреймворка
  - `framework/README.md` - главная документация фреймворка
  - `framework.go` - корневой файл с основными интерфейсами

### Changed

- Обновлен `go.mod` для поддержки использования как библиотеки с версионированием
  - Изменен module path с `github.com/potter` на `github.com/potter/v1`

- Обновлены файлы в `pkg/cqrs/` для обратной совместимости
  - Добавлены комментарии о deprecation
  - Обновлены импорты на использование `framework/transport`

### Deprecated

- Пакет `pkg/cqrs` помечен как deprecated и будет удален в будущих версиях
  - Используйте `github.com/potter/v1/framework/cqrs` вместо этого

### Security

- Добавлена поддержка валидации и авторизации через middleware
- Добавлена поддержка безопасной обработки ошибок с stack trace

