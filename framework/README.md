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

```bash
go get github.com/akriventsev/potter/framework
```

**Примечание для локальной разработки**: Если вы работаете с форком или локальной версией фреймворка, используйте `replace` директиву в `go.mod`:

```go
replace github.com/akriventsev/potter => /path/to/local/potter
```

### Базовый пример

**Рекомендуемый способ**: Использование DI-контейнера для сборки приложений.

```go
package main

import (
    "context"
    "log"
    "github.com/akriventsev/potter/framework/container"
    "github.com/akriventsev/potter/framework/cqrs"
    "github.com/akriventsev/potter/framework/transport"
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
    ctx := context.Background()
    
    // Создаем контейнер через builder
    builder := container.NewContainerBuilder(&container.Config{}).
        WithDefaults()
    
    c, err := builder.Build(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer c.Shutdown(ctx)
    
    // Получаем компоненты из контейнера
    registry := cqrs.NewRegistry()
    commandBus := transport.NewInMemoryCommandBus()
    
    // Регистрируем обработчик
    handler := &CreateUserHandler{}
    cqrs.RegisterCommandHandler(registry, commandBus, handler)
    
    // Отправляем команду
    cmd := CreateUserCommand{Name: "John", Email: "john@example.com"}
    _ = commandBus.Send(ctx, cmd)
}
```

**Примечание**: `framework.New()` и `BaseFramework` помечены как deprecated и будут удалены в версии 2.0.0. Используйте `framework/container` для инициализации приложений.

## Пакеты фреймворка

### framework/core

Базовые интерфейсы и типы для всех компонентов фреймворка.

**Основные компоненты:**
- `Component` - базовый интерфейс для всех компонентов
- `Lifecycle` - управление жизненным циклом
- `Configurable` - конфигурируемые компоненты
- `FrameworkError` - система ошибок фреймворка (см. `framework/core/errors.go`)
- `Result[T]` - generic тип для результатов
- `Option[T]` - generic тип для опциональных значений

**Работа с ошибками:**
```go
// Создание новой ошибки
err := core.NewError(core.ErrNotFound, "resource not found")

// Оборачивание существующей ошибки
err := core.Wrap(originalErr, core.ErrInvalidConfig, "invalid configuration")

// Оборачивание с кодом
err := core.WrapWithCode(originalErr, core.ErrInitializationFailed)
```

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

### framework/eventsourcing

Полная поддержка Event Sourcing паттерна для построения систем с полной историей изменений.

**Возможности:**
- 📦 **EventStore** - хранилище событий с версионированием
- 🔄 **Event Replay** - восстановление состояния из событий
- 📸 **Snapshots** - оптимизация через снапшоты
- 🗄️ **Multiple Adapters** - PostgreSQL, MongoDB, EventStore DB, InMemory
- 🔐 **Optimistic Concurrency** - безопасная конкурентность через версионирование
- 🎯 **Type-Safe** - generic репозитории и агрегаты
- 📊 **Projections Framework** - централизованная инфраструктура для проекций с checkpoint management

**Пример использования:**
```go
// Event Sourced агрегат
type BankAccount struct {
    eventsourcing.EventSourcedAggregate
    balance int64
}

func (a *BankAccount) Deposit(amount int64) {
    a.RaiseEvent(&MoneyDepositedEvent{Amount: amount})
}

func (a *BankAccount) Apply(event events.Event) error {
    switch e := event.(type) {
    case *MoneyDepositedEvent:
        a.balance += e.Amount
    }
    return nil
}

// Использование
eventStore := eventsourcing.NewPostgresEventStore(config)
snapshotStore := eventsourcing.NewPostgresSnapshotStore(config)
repo := eventsourcing.NewEventSourcedRepository[*BankAccount](
    eventStore, snapshotStore,
)

account := NewBankAccount("ACC001")
account.Deposit(1000)
repo.Save(ctx, account)

// Загрузка с replay
loaded, _ := repo.GetByID(ctx, "ACC001")
```

**Документация:** [`framework/eventsourcing/README.md`](eventsourcing/README.md)

**Примеры:**
- [`examples/eventsourcing-basic`](../../examples/eventsourcing-basic) - базовый пример
- [`examples/warehouse`](../../examples/warehouse) - продвинутый пример

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

### Testing Applications

Фреймворк предоставляет пакет `framework/testing` с готовыми утилитами для тестирования приложений.

#### Использование InMemoryTestEnvironment

`InMemoryTestEnvironment` предоставляет готовую тестовую среду со всеми необходимыми компонентами:

```go
import "github.com/akriventsev/potter/framework/testing"

func TestCreateUserHandler(t *testing.T) {
    // Создаем тестовую среду
    env := testing.NewInMemoryTestEnvironment()
    defer env.Shutdown(context.Background())
    
    // Используем готовые компоненты
    repo := repository.NewInMemoryRepository[User](repository.DefaultInMemoryConfig())
    handler := command.NewCreateUserHandler(repo, env.EventBus)
    
    // Регистрируем handler
    env.CommandBus.Register(handler)
    
    // Выполняем команду
    cmd := CreateUserCommand{Name: "John", Email: "john@example.com"}
    err := env.CommandBus.Send(context.Background(), cmd)
    // assertions...
}
```

#### Использование NewTestContainer

Для тестирования с DI контейнером:

```go
import "github.com/akriventsev/potter/framework/testing"

func TestApplicationWithContainer(t *testing.T) {
    container := testing.NewTestContainer()
    defer container.Shutdown(context.Background())
    
    // Получаем компоненты из контейнера
    // ...
}
```

#### Ручное создание компонентов

Для более тонкого контроля можно создавать компоненты вручную:

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

### framework/adapters/repository

Generic репозитории для работы с различными storage backends.

**Возможности:**
- 🔍 **Query Builder** - fluent API для построения сложных запросов
- 📊 **Advanced Indexing** - автоматическое управление индексами
- ⏱️ **TTL Support** - автоматическая очистка для MongoDB
- 🔄 **Change Streams** - реактивные обновления для MongoDB

**Query Builder пример:**
```go
results, err := repo.Query().
    Where("status", Eq, "active").
    Where("created_at", Gte, time.Now().AddDate(0, -1, 0)).
    OrderBy("created_at", Desc).
    Limit(10).
    Execute(ctx)
```

**Index Management пример:**
```go
indexMgr := repo.IndexManager()
indexMgr.CreateIndex(ctx, IndexSpec{
    Name: "idx_status_created_at",
    Fields: []string{"status", "created_at"},
})
recommendations, _ := indexMgr.AnalyzeQueries(ctx)
```

**MongoDB TTL пример:**
```go
repo.EnableTTL("expires_at", 24*time.Hour)
```

**Change Streams пример:**
```go
watcher := repo.WatchChanges()
changes, _ := watcher.WatchCollection(ctx)
for change := range changes {
    handleChange(change)
}
```

### framework/migrations

Версионированные миграции схемы базы данных с rollback поддержкой.

**Возможности:**
- 📝 **SQL и Go миграции** - поддержка SQL миграций для PostgreSQL, MySQL, SQLite и Go миграций для MongoDB
- 🔄 **Up/Down Support** - полная поддержка rollback
- 🔒 **Concurrent Safety** - блокировки для предотвращения concurrent migrations
- ✅ **Out-of-order миграции** - поддержка применения миграций вне порядка
- 🌍 **Environment Variable Substitution** - подстановка переменных окружения в миграциях
- 🛠️ **CLI Tool** - potter-migrate (обертка над goose) и прямой доступ к goose CLI
- 📚 **Индустриальный стандарт** - основано на [goose](https://github.com/pressly/goose)

**Пример использования CLI:**
```bash
# Через potter-migrate
potter-migrate up --database-url postgres://localhost/db
potter-migrate down 1 --database-url postgres://localhost/db
potter-migrate status --database-url postgres://localhost/db
potter-migrate create add_user_roles

# Или напрямую через goose
goose -dir migrations postgres "postgres://localhost/db" up
goose -dir migrations postgres "postgres://localhost/db" down
goose -dir migrations postgres "postgres://localhost/db" status
goose -dir migrations create add_user_roles sql
```

**Программное использование:**
```go
import (
    "database/sql"
    "github.com/akriventsev/potter/framework/migrations"
    _ "github.com/jackc/pgx/v5/stdlib"
)

db, _ := sql.Open("pgx", dsn)
err := migrations.RunMigrations(db, "./migrations")

// Получить статус
statuses, _ := migrations.GetMigrationStatus(db, "./migrations")
```

**Подробная документация:** [framework/migrations/README.md](migrations/README.md)

### framework/saga

Механизмы для работы с сагами и оркестрацией распределенных транзакций.

**Возможности:**
- 🎯 **Saga Query Handler** - CQRS query handler для получения статуса и истории саг
- 📊 **Read Models** - оптимизированные read models для быстрых запросов
- 🔄 **Projections** - автоматическое обновление read models из saga events

**Saga Query Handler пример:**
```go
queryHandler := saga.NewSagaQueryHandler(persistence, readModelStore)
queryBus.RegisterHandler("GetSagaStatus", queryHandler)

query := &saga.GetSagaStatusQuery{SagaID: "saga-123"}
result, _ := queryHandler.Handle(ctx, query)
status := result.(*saga.SagaStatusResponse)
```

**Read Model Store пример:**
```go
readModelStore, _ := saga.NewPostgresSagaReadModelStore(dsn)
status, _ := readModelStore.GetSagaStatus(ctx, "saga-123")
sagas, _ := readModelStore.ListSagas(ctx, saga.SagaFilter{
    Status: &saga.SagaStatusRunning,
    Limit: 10,
})
```

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

См. [ROADMAP.md](../../ROADMAP.md) для детального плана развития фреймворка.

## Версионирование

Проект следует [Semantic Versioning](https://semver.org/).

Текущая версия: см. файл [`VERSION`](../../VERSION) в корне проекта.

## Лицензия

MIT

## Авторы

Potter Team

## Дополнительная документация

- [CHANGELOG](../CHANGELOG.md) - история изменений

