# Changelog

Все значимые изменения в этом проекте будут документироваться в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.0.0/),
и этот проект придерживается [Semantic Versioning](https://semver.org/lang/ru/).

## [1.2.0] - 2025-XX-XX

### Added

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

### Dependencies

- Added `github.com/dave/jennifer v1.7.0` для генерации Go кода

### Documentation

- Добавлена документация кодогенератора в `framework/codegen/README.md`
- Обновлен главный README с секцией о Code Generator
- Добавлен пример использования в `examples/codegen/README.md`

## [1.2.0] - 2024-XX-XX

### Added

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

## [1.1.0] - 2024-01-XX (Unreleased)

### Fixed

- **Критическая ошибка импорта**: Удален неправильный импорт `examples/warehouse/domain` из `framework/adapters/repository/inmemory.go`
- **Архитектурное нарушение**: Удален `DomainRepository` из `inmemory.go`, который нарушал принцип независимости фреймворка
- **TODO комментарии**: Исправлены все TODO в `registry.go` (RegisteredAt), `factory.go` (CreateHandlerFromFunc), `fsm.go` (persistence ID), `kafka.go` (transactional transport)
- **Отсутствие валидации**: Добавлена валидация конфигураций для всех адаптеров (PostgreSQL, MongoDB, NATS, Kafka, Redis)
- **Graceful shutdown**: Добавлен graceful shutdown для `AsyncEventPublisher` с drain queue и ожиданием завершения воркеров

### Removed

- **Deprecated код**: Удалена директория `pkg/cqrs/` - все пользователи должны использовать `framework/cqrs` напрямую
- **Артефакты сборки**: Удален бинарный файл `server` из корня проекта
- **Неиспользуемый код**: Удален `DomainRepository` из `inmemory.go` (нарушал архитектуру фреймворка)

### Added

- **Unit тесты**: Добавлены comprehensive unit тесты для core компонентов:
  - `framework/core/types_test.go` - тесты для FrameworkContext, Error, Result, Option
  - `framework/transport/bus_test.go` - тесты для CommandBus и QueryBus
  - `framework/events/publisher_test.go` - тесты для всех типов EventPublisher
  - `framework/container/container_test.go` - тесты для DI контейнера
  - `framework/adapters/repository/inmemory_test.go` - тесты для InMemoryRepository
  - `framework/cqrs/registry_test.go` - тесты для CQRS Registry
- **Валидация конфигураций**: Добавлены методы `Validate()` для всех адаптеров:
  - `PostgresConfig.Validate()` - проверка DSN, TableName, MaxOpenConns, MaxIdleConns
  - `MongoConfig.Validate()` - проверка URI, Database, Collection, MaxPoolSize
  - `NATSConfig.Validate()` - проверка URL формата (nats:// или tls://)
  - `KafkaConfig.Validate()` - проверка Brokers (не пустой, формат host:port)
  - `RedisConfig.Validate()` - проверка Addr и StreamName, Ping при создании
- **Улучшенная документация**: Обновлены примеры кода и README файлы
- **Makefile команды**: Добавлены команды для тестирования (`test-coverage`, `test-unit`, `test-integration`) и очистки (`clean`)

### Changed

- **FSM persistence**: Добавлено поле `id` в структуру `FSM` с автоматической генерацией UUID для корректного сохранения состояния
- **AsyncEventPublisher**: Добавлен graceful shutdown с `stopCh`, `WaitGroup` и drain queue логикой
- **Warehouse example**: Рефакторинг 2PC handlers - создана helper функция `handleTwoPCRequest` для уменьшения дублирования кода (~90 строк до ~30)
- **Примеры кода**: Переименованы функции примеров в lowercase для соответствия Go conventions
- **.gitignore**: Добавлены игнорирования для всех артефактов сборки, IDE файлов и временных файлов

### Added (продолжение)

- **Built-in MessageBus адаптеры**: NATS, Kafka, Redis Streams, InMemory
  - `framework/adapters/messagebus/nats.go` - улучшенный NATS адаптер с connection pooling, метриками, lifecycle
  - `framework/adapters/messagebus/kafka.go` - Kafka адаптер с поддержкой request-reply, dead letter queue
  - `framework/adapters/messagebus/redis.go` - Redis Streams адаптер для легковесных pub/sub сценариев
  - `framework/adapters/messagebus/inmemory.go` - InMemory адаптер для тестирования с поддержкой wildcards
  - `framework/adapters/messagebus/factory.go` - фабрика для создания MessageBus адаптеров

- **Built-in Event Publisher адаптеры**: NATS, Kafka, MessageBus
  - `framework/adapters/events/nats.go` - NATS Event Publisher с retry логикой и метриками
  - `framework/adapters/events/kafka.go` - Kafka Event Publisher для event sourcing с гарантией порядка
  - `framework/adapters/events/messagebus.go` - универсальный MessageBus Event Publisher с batch publishing
  - `framework/adapters/events/factory.go` - фабрика для создания Event Publisher адаптеров

- **Built-in Repository адаптеры**: InMemory, PostgreSQL, MongoDB
  - `framework/adapters/repository/inmemory.go` - generic in-memory репозиторий с индексами и транзакциями
  - `framework/adapters/repository/postgres.go` - generic PostgreSQL репозиторий с query builder
  - `framework/adapters/repository/mongodb.go` - generic MongoDB репозиторий с поддержкой aggregation
  - `framework/adapters/repository/factory.go` - фабрика для создания Repository адаптеров

- **Built-in Transport адаптеры**: REST, gRPC, WebSocket
  - `framework/adapters/transport/rest.go` - REST API адаптер с автоматической маршрутизацией команд/запросов
  - `framework/adapters/transport/grpc.go` - gRPC адаптер с interceptors и health checks
  - `framework/adapters/transport/websocket.go` - WebSocket адаптер для real-time коммуникации и event streaming
  - `framework/adapters/transport/router.go` - generic Command/Query router для transport адаптеров

- **Документация и примеры**:
  - `framework/adapters/README.md` - полная документация по всем адаптерам
  - Примеры использования для каждого типа адаптера (в планах)

### Changed

- Перемещены адаптеры из `internal/adapters/` в `framework/adapters/` как built-in компоненты
- Улучшена конфигурация адаптеров с использованием builder pattern
- Добавлена поддержка lifecycle методов (Start, Stop, IsRunning) для всех адаптеров
- Улучшена обработка ошибок с typed errors для различных сценариев
- Добавлена интеграция с метриками OpenTelemetry для всех адаптеров
- Реализован graceful shutdown для всех адаптеров

### Deprecated

- `internal/adapters/*` пакеты помечены как deprecated
  - Используйте `framework/adapters/*` вместо этого
  - Старые адаптеры будут удалены в версии 2.0.0

### Migration Guide

Для миграции на новые адаптеры:

1. Обновите импорты с `internal/adapters/*` на `framework/adapters/*`
2. Используйте фабрики для создания адаптеров:
   ```go
   factory := messagebus.NewMessageBusFactory()
   bus, err := factory.Create("nats", config)
   ```
3. Обновите конфигурацию согласно новым структурам конфигурации
4. Протестируйте изменения в staging окружении

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

