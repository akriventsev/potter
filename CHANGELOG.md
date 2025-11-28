# Changelog

Все значимые изменения в этом проекте будут документироваться в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.0.0/),
и этот проект придерживается [Semantic Versioning](https://semver.org/lang/ru/).

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

