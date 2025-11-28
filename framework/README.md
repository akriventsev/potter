# Potter Framework

Potter Framework - это универсальный фреймворк для построения асинхронных CQRS сервисов с гексагональной архитектурой на Go.

## Описание

Potter Framework предоставляет полный набор компонентов для создания масштабируемых микросервисов с поддержкой:
- **CQRS** паттерна для разделения команд и запросов
- **Event Sourcing** и асинхронной обработки событий
- **Гексагональной архитектуры** (Ports & Adapters)
- **DI контейнера** с модульной системой
- **Метрик** на основе OpenTelemetry
- **Конечных автоматов** для саг и оркестрации

## Основные возможности

- ✅ Полная реализация CQRS паттерна
- ✅ Система событий с поддержкой pub/sub
- ✅ Модульный DI контейнер
- ✅ Транспортный слой (REST, gRPC, MessageBus)
- ✅ Метрики и трейсинг через OpenTelemetry
- ✅ Конечные автоматы для сложных бизнес-процессов
- ✅ Middleware для обработчиков (logging, validation, recovery, retry, circuit breaker, rate limit, tracing, authorization, caching)
- ✅ Поддержка generic типов
- ✅ Thread-safe реализации

## Архитектурный обзор

```
┌─────────────────────────────────────────────────────────┐
│                    Application Layer                     │
│  (Command Handlers, Query Handlers, Event Handlers)     │
└─────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────────────────────────────────────┐
│                      Framework Layer                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐ │
│  │   CQRS   │  │  Events  │  │ Container│  │ Metrics│ │
│  └──────────┘  └──────────┘  └──────────┘  └────────┘ │
│  ┌──────────┐  ┌──────────┐                            │
│  │Transport │  │   FSM    │                            │
│  └──────────┘  └──────────┘                            │
└─────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────────────────────────────────────┐
│                      Domain Layer                        │
│           (Entities, Value Objects, Events)             │
└─────────────────────────────────────────────────────────┘
```

## Quick Start

### Установка

**Примечание**: Путь `potter` используется для локальной разработки. При публикации на GitHub путь модуля можно изменить на `github.com/username/potter`.

```bash
go get potter/framework
```

### Базовый пример

```go
package main

import (
    "context"
    "potter/framework/cqrs"
    "potter/framework/transport"
)

// Определяем команду
type CreateUserCommand struct {
    Name  string
    Email string
}

func (c CreateUserCommand) CommandName() string {
    return "create_user"
}

// Создаем обработчик
type CreateUserHandler struct{}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd transport.Command) error {
    createCmd := cmd.(CreateUserCommand)
    // Логика создания пользователя
    return nil
}

func (h *CreateUserHandler) CommandName() string {
    return "create_user"
}

func main() {
    // Создаем реестр и шину
    registry := cqrs.NewRegistry()
    commandBus := transport.NewInMemoryCommandBus()
    
    // Регистрируем обработчик
    handler := &CreateUserHandler{}
    cqrs.RegisterCommandHandler(registry, commandBus, handler)
    
    // Отправляем команду
    ctx := context.Background()
    cmd := CreateUserCommand{Name: "John", Email: "john@example.com"}
    _ = commandBus.Send(ctx, cmd)
}
```

## Пакеты фреймворка

### framework/core

Базовые интерфейсы и типы для всех компонентов фреймворка.

**Основные компоненты:**
- `Component` - базовый интерфейс для всех компонентов
- `Lifecycle` - управление жизненным циклом
- `Configurable` - конфигурируемые компоненты
- `Error` - система ошибок фреймворка
- `Result[T]` - generic тип для результатов
- `Option[T]` - generic тип для опциональных значений

### framework/cqrs

Полная реализация CQRS паттерна.

**Основные компоненты:**
- `Registry` - реестр обработчиков команд и запросов
- `CommandHandlerBuilder` / `QueryHandlerBuilder` - построители для настройки обработчиков
- Middleware: Logging, Validation, Recovery, Timeout, Retry, Circuit Breaker, Rate Limit, Tracing, Authorization, Caching
- `HandlerFactory` - фабрика для создания обработчиков

**Пример использования:**
```go
registry := cqrs.NewRegistry()
commandBus := transport.NewInMemoryCommandBus()

handler := &CreateUserHandler{}
builder := cqrs.NewCommandHandlerBuilder("create_user", handler).
    WithMetrics(metrics).
    WithMiddleware(cqrs.DefaultLoggingCommandMiddleware()).
    WithMiddleware(cqrs.RecoveryCommandMiddleware()).
    WithRetry(3, time.Second, time.Second).
    WithCircuitBreaker(5, 30*time.Second)

wrappedHandler := builder.Build()
cqrs.RegisterCommandHandler(registry, commandBus, wrappedHandler)
```

### framework/transport

Транспортный слой для команд, запросов и message bus.

**Основные компоненты:**
- `CommandBus` / `QueryBus` - шины команд и запросов
- `MessageBus` - абстракция для message bus
- `InMemoryCommandBus` / `InMemoryQueryBus` - реализации в памяти

### framework/events

Система событий для асинхронной обработки.

**Основные компоненты:**
- `Event` - интерфейс события
- `EventPublisher` - публикатор событий
- `EventSubscriber` - подписчик на события
- `EventBus` - шина событий

**Пример использования:**
```go
eventBus := events.NewInMemoryEventBus()

// Подписываемся на события
eventBus.Subscribe("user_created", &UserCreatedHandler{})

// Публикуем событие
event := events.NewBaseEvent("user_created", "user-123").
    WithCorrelationID("req-456").
    WithUserID("user-789")
eventBus.Publish(ctx, event)
```

### framework/container

DI контейнер с модульной архитектурой.

**Основные компоненты:**
- `Container` - DI контейнер
- `Module` / `Adapter` / `Transport` - типы компонентов
- `ContainerBuilder` - построитель контейнера
- `Initializer` - инициализатор с разрешением зависимостей

**Пример использования:**
```go
builder := container.NewContainerBuilder(&container.Config{}).
    WithModule(&CQRSModule{}).
    WithAdapter(&RepositoryAdapter{}).
    WithTransport(&RESTTransport{})

container, err := builder.Build(ctx)
```

### framework/metrics

Система метрик на основе OpenTelemetry.

**Основные компоненты:**
- `Metrics` - сборщик метрик
- `SetupMetrics` - настройка экспорта метрик

**Пример использования:**
```go
config := &metrics.MetricsConfig{
    ExporterType: "prometheus",
    SamplingRate: 1.0,
}
provider, _ := metrics.SetupMetrics(config)
defer metrics.ShutdownMetrics(ctx, provider)

m, _ := metrics.NewMetrics()
m.RecordCommand(ctx, "create_user", duration, true)
```

### framework/fsm

Конечный автомат для саг и оркестрации.

**Основные компоненты:**
- `FSM` - конечный автомат
- `State` - состояние
- `Transition` - переход
- `Event` - событие
- `Action` - действие

**Пример использования:**
```go
initialState := fsm.NewBaseState("initial")
finalState := fsm.NewBaseState("final")

fsm := fsm.NewFSM(initialState)
fsm.AddState(finalState)

transition := fsm.NewTransition(initialState, finalState, "complete").
    WithGuard(func(ctx context.Context, from, to fsm.State, event fsm.Event) (bool, error) {
        return true, nil
    })

fsm.AddTransition(transition)
fsm.Trigger(ctx, fsm.NewEvent("complete", nil))
```

## Testing

Фреймворк включает comprehensive unit тесты для всех основных компонентов. Тесты служат как примеры использования API и демонстрируют best practices.

### Запуск тестов

```bash
# Все тесты
make test

# С покрытием кода
make test-coverage

# Только unit тесты
make test-unit

# Integration тесты
make test-integration
```

### Примеры тестов

См. тестовые файлы в каждом пакете:
- `framework/core/types_test.go` - примеры работы с FrameworkContext, Result, Option
- `framework/transport/bus_test.go` - примеры использования CommandBus и QueryBus
- `framework/events/publisher_test.go` - примеры работы с EventPublisher
- `framework/container/container_test.go` - примеры использования DI контейнера
- `framework/adapters/repository/inmemory_test.go` - примеры работы с репозиториями
- `framework/cqrs/registry_test.go` - примеры регистрации обработчиков

### Написание тестов для приложений

При написании тестов для приложений на базе фреймворка:

1. Используйте `InMemoryRepository` для тестирования без внешних зависимостей
2. Используйте `InMemoryCommandBus` и `InMemoryQueryBus` для изоляции тестов
3. Используйте `InMemoryEventPublisher` для проверки публикации событий
4. Мокируйте внешние зависимости через интерфейсы

Пример:
```go
func TestCreateUserHandler(t *testing.T) {
    repo := repository.NewInMemoryRepository[User](repository.DefaultInMemoryConfig())
    publisher := events.NewInMemoryEventPublisher()
    handler := command.NewCreateUserHandler(repo, publisher)
    
    cmd := CreateUserCommand{Name: "John", Email: "john@example.com"}
    err := handler.Handle(context.Background(), cmd)
    // assertions...
}
```

## Configuration Validation

Все адаптеры фреймворка теперь включают валидацию конфигураций при создании. Это помогает обнаружить ошибки конфигурации на раннем этапе.

### Примеры валидации

**PostgreSQL Repository:**
```go
config := repository.PostgresConfig{
    DSN:        "postgres://user:pass@localhost/db",
    TableName:  "users",
    MaxOpenConns: 25,
    MaxIdleConns: 5,
}

if err := config.Validate(); err != nil {
    log.Fatal(err)
}

repo, err := repository.NewPostgresRepository[User](config, mapper)
```

**NATS MessageBus:**
```go
config := messagebus.NATSConfig{
    URL: "nats://localhost:4222", // Должен начинаться с nats:// или tls://
}

if err := config.Validate(); err != nil {
    log.Fatal(err)
}

adapter, err := messagebus.NewNATSAdapter(config.URL)
```

**Kafka MessageBus:**
```go
config := messagebus.KafkaConfig{
    Brokers: []string{"localhost:9092"}, // Каждый broker должен быть в формате host:port
}

if err := config.Validate(); err != nil {
    log.Fatal(err)
}

adapter, err := messagebus.NewKafkaAdapter(config)
```

**Redis MessageBus:**
```go
config := messagebus.RedisConfig{
    Addr:      "localhost:6379",
    StreamName: "events", // Обязательное поле
}

if err := config.Validate(); err != nil {
    log.Fatal(err)
}

adapter, err := messagebus.NewRedisAdapter(config)
```

Все адаптеры автоматически валидируют конфигурацию при создании через `New*` функции, возвращая понятные ошибки при некорректных значениях.

## Best Practices

1. **Используйте middleware** для общей функциональности (логирование, метрики, валидация)
2. **Применяйте circuit breaker** для защиты от каскадных сбоев
3. **Используйте retry** с exponential backoff для временных ошибок
4. **Кэшируйте результаты запросов** где это возможно
5. **Используйте типизированные обработчики** для типобезопасности
6. **Применяйте distributed tracing** для отладки в production
7. **Мониторьте метрики** для понимания производительности системы
8. **Валидируйте конфигурации** перед использованием адаптеров
9. **Пишите тесты** для всех компонентов приложения

## Built-in Adapters

Фреймворк предоставляет готовые адаптеры для интеграции с внешними системами:

### MessageBus Adapters

- **NATS** - высокопроизводительный pub/sub с connection pooling и метриками
- **Kafka** - event streaming с поддержкой request-reply и dead letter queue
- **Redis Streams** - легковесный pub/sub для кэширования и real-time сценариев
- **InMemory** - для тестирования и локальной разработки

### Event Publisher Adapters

- **NATS** - публикация событий через NATS с retry логикой
- **Kafka** - event sourcing с гарантией порядка событий для агрегата
- **MessageBus** - универсальный адаптер для любого message bus с batch publishing

### Repository Adapters

- **InMemory** - generic in-memory репозиторий с индексами
- **PostgreSQL** - generic PostgreSQL репозиторий с query builder
- **MongoDB** - generic MongoDB репозиторий с поддержкой aggregation

### Transport Adapters

- **REST** - автоматическая маршрутизация команд/запросов через HTTP
- **gRPC** - высокопроизводительные RPC сервисы с interceptors
- **WebSocket** - real-time коммуникация и event streaming

Подробная документация: [framework/adapters/README.md](adapters/README.md)

## Roadmap

- [ ] Поддержка Event Sourcing
- [ ] Поддержка Saga Pattern через FSM
- [x] Интеграция с популярными message brokers (Kafka, Redis)
- [ ] Поддержка GraphQL транспорта
- [ ] Автоматическая генерация OpenAPI спецификаций
- [x] Поддержка WebSocket транспорта
- [ ] Расширенная поддержка distributed tracing
- [x] Интеграция с популярными базами данных (PostgreSQL, MongoDB)

## Версионирование

Проект следует [Semantic Versioning](https://semver.org/).

Текущая версия: **1.1.0**

## Лицензия

MIT

## Авторы

Potter Team

## Дополнительная документация

- [CQRS README](../pkg/cqrs/README.md) - подробная документация по CQRS (deprecated, см. framework/cqrs)
- [CHANGELOG](../CHANGELOG.md) - история изменений

