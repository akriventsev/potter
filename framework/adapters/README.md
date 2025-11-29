# Framework Adapters

Пакет `framework/adapters` предоставляет built-in адаптеры для интеграции с внешними системами и инфраструктурой. Эти адаптеры реализуют паттерн Ports & Adapters (Hexagonal Architecture), позволяя приложению работать с различными технологиями без изменения бизнес-логики.

## Структура пакета

```
framework/adapters/
├── messagebus/     # MessageBus адаптеры (NATS, Kafka, Redis, InMemory)
├── events/         # Event Publisher адаптеры (NATS, Kafka, MessageBus)
├── repository/     # Repository адаптеры (InMemory, PostgreSQL, MongoDB)
├── transport/      # Transport адаптеры (REST, gRPC, WebSocket)
└── examples/       # Примеры использования адаптеров
```

## Доступные адаптеры

### MessageBus адаптеры

Адаптеры для работы с message brokers и очередями сообщений.

| Адаптер | Описание | Когда использовать |
|---------|----------|-------------------|
| **NATS** | Адаптер для NATS message broker | Высокопроизводительные pub/sub сценарии, request-reply паттерны |
| **Kafka** | Адаптер для Apache Kafka | Event streaming, high throughput, event sourcing |
| **Redis** | Адаптер для Redis Streams | Легковесные pub/sub сценарии, кэширование |
| **InMemory** | In-memory реализация с поддержкой воркеров и очередей | Тестирование, локальная разработка, нагрузочное тестирование |

**Пример использования:**
```go
import "github.com/akriventsev/potter/framework/adapters/messagebus"

// Создание NATS адаптера
builder := messagebus.NewNATSAdapterBuilder().
    WithURL("nats://localhost:4222").
    WithMaxReconnects(10).
    WithReconnectWait(2 * time.Second)

adapter, err := builder.Build()
if err != nil {
    log.Fatal(err)
}

// Публикация сообщения
err = adapter.Publish(ctx, "users.created", []byte("data"), nil)

// Подписка на сообщения
err = adapter.Subscribe(ctx, "users.*", func(ctx context.Context, msg *transport.Message) error {
    // Обработка сообщения
    return nil
})
```

### Event Publisher адаптеры

Адаптеры для публикации доменных событий в различные event stores и message brokers.

| Адаптер | Описание | Когда использовать |
|---------|----------|-------------------|
| **NATS** | Публикация событий через NATS | Event-driven архитектура с NATS |
| **Kafka** | Публикация событий через Kafka | Event sourcing, event streaming |
| **MessageBus** | Публикация через любой MessageBus | Универсальный адаптер для любого message bus |
| **InMemory** | In-memory публикация | Тестирование, локальная разработка |

**Пример использования:**
```go
import "github.com/akriventsev/potter/framework/adapters/events"

// Создание Kafka Event Publisher
config := events.KafkaEventConfig{
    Brokers:      []string{"localhost:9092"},
    TopicPrefix:  "events",
    Compression:  "snappy",
}

publisher, err := events.NewKafkaEventAdapter(config)
if err != nil {
    log.Fatal(err)
}

// Публикация события
event := events.NewBaseEvent("user.created", "user-123").
    WithCorrelationID("req-456")
err = publisher.Publish(ctx, event)
```

### Repository адаптеры

Generic адаптеры для работы с различными базами данных и storage backends.

| Адаптер | Описание | Когда использовать |
|---------|----------|-------------------|
| **InMemory** | In-memory хранилище с опциональным лимитом сущностей | Тестирование, прототипирование |
| **PostgreSQL** | PostgreSQL репозиторий | Реляционные данные, ACID транзакции |
| **MongoDB** | MongoDB репозиторий | Документно-ориентированные данные, гибкие схемы |

**Пример использования:**
```go
import "github.com/akriventsev/potter/framework/adapters/repository"

// Создание PostgreSQL репозитория
config := repository.PostgresConfig{
    DSN:        "postgres://user:pass@localhost/db",
    TableName:  "users",
    SchemaName: "public",
}

repo, err := repository.NewPostgresRepository[User](config)
if err != nil {
    log.Fatal(err)
}

// Использование репозитория
user := NewUser("John", "john@example.com")
err = repo.Save(ctx, user)

found, err := repo.FindByID(ctx, user.ID())
```

### Transport адаптеры

Адаптеры для различных транспортных протоколов (HTTP, gRPC, WebSocket, GraphQL).

| Адаптер | Описание | Когда использовать |
|---------|----------|-------------------|
| **REST** | REST API с поддержкой CommandBus/QueryBus | HTTP API, веб-приложения |
| **gRPC** | gRPC сервисы | Микросервисы, высокопроизводительные API |
| **WebSocket** | WebSocket сервер | Real-time коммуникация, event streaming |
| **GraphQL** | GraphQL API с автогенерацией схем | Гибкие API, клиенты с различными требованиями к данным |

**Пример использования:**
```go
import "github.com/akriventsev/potter/framework/adapters/transport"

// Создание REST адаптера
config := transport.RESTConfig{
    Port:     8080,
    BasePath: "/api/v1",
}

adapter, err := transport.NewRESTAdapter(config, commandBus, queryBus)
if err != nil {
    log.Fatal(err)
}

// Регистрация маршрутов
adapter.RegisterCommand("POST", "/users", CreateUserCommand{})
adapter.RegisterQuery("GET", "/users/:id", GetUserQuery{})

// Запуск сервера
err = adapter.Start(ctx)
```

**Пример использования GraphQL:**
```go
import "github.com/akriventsev/potter/framework/adapters/transport"

// Создание GraphQL адаптера
config := transport.DefaultGraphQLConfig()
config.Port = 8082
config.EnablePlayground = true

adapter, err := transport.NewGraphQLAdapter(
    config,
    commandBus,
    queryBus,
    eventBus,
    schema, // graphql.ExecutableSchema
)
if err != nil {
    log.Fatal(err)
}

// Запуск сервера
err = adapter.Start(ctx)
```

Подробнее см. [GraphQL Transport Documentation](transport/GRAPHQL.md).
```

## Фабрики адаптеров

Для удобного создания адаптеров предоставляются фабрики:

### MessageBus Factory

```go
import "github.com/akriventsev/potter/framework/adapters/messagebus"

factory := messagebus.NewMessageBusFactory()

// Создание NATS адаптера
natsConfig := messagebus.NATSConfig{URL: "nats://localhost:4222"}
natsBus, err := factory.Create("nats", natsConfig)

// Создание Kafka адаптера
kafkaConfig := messagebus.KafkaConfig{Brokers: []string{"localhost:9092"}}
kafkaBus, err := factory.Create("kafka", kafkaConfig)
```

### Event Publisher Factory

```go
import "github.com/akriventsev/potter/framework/adapters/events"

factory := events.NewEventPublisherFactory()

// Создание Kafka Event Publisher
kafkaConfig := events.KafkaEventConfig{Brokers: []string{"localhost:9092"}}
publisher, err := factory.Create("kafka", kafkaConfig)
```

### Repository Factory

```go
import "github.com/akriventsev/potter/framework/adapters/repository"

factory := repository.NewRepositoryFactory()

// Создание PostgreSQL репозитория
postgresConfig := repository.PostgresConfig{DSN: "postgres://..."}
repo, err := factory.Create[User]("postgres", postgresConfig)
```

## Регистрация custom адаптеров

Все фабрики поддерживают регистрацию custom адаптеров:

```go
// Регистрация custom MessageBus адаптера
messagebus.Register("custom", func(config interface{}) (transport.RequestReplyBus, error) {
    // Создание custom адаптера
    return customAdapter, nil
})

// Регистрация custom Event Publisher
events.Register("custom", func(config interface{}) (events.EventPublisher, error) {
    return customPublisher, nil
})
```

## Таблица совместимости

| Адаптер | NATS | Kafka | Redis | PostgreSQL | MongoDB | InMemory |
|---------|------|-------|-------|------------|---------|----------|
| MessageBus | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
| Event Publisher | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| Repository | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
| Transport | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

## Известные ограничения

### REST Transport

- **Только JSON body binding**: Текущая реализация поддерживает только JSON body binding. Query parameters и form data не поддерживаются. Для поддержки дополнительных источников данных необходимо расширить адаптер или использовать middleware.
- **Отсутствуют встроенные middleware**: CORS, rate limiting и аутентификация не реализованы. Эти функции должны быть реализованы на уровне приложения.

### gRPC Transport

- **Отсутствуют встроенные interceptors**: Текущая реализация не предоставляет встроенных server interceptors для логирования, recovery и метрик. Эти функции должны быть добавлены через опции `grpc.ServerOption` или реализованы на уровне приложения.
- **Отсутствуют health checking и reflection**: Встроенные health checking и reflection не реализованы. Необходимо добавить их вручную при необходимости.

### WebSocket Transport

- **Только управление соединениями**: Текущая реализация только управляет соединениями и не предоставляет встроенной маршрутизации сообщений к command/query handlers или broadcasting событий через eventPublisher. Высокоуровневая маршрутизация должна быть реализована на уровне приложения.

### Repository адаптеры (PostgreSQL и MongoDB)

- **Только базовый CRUD**: Текущая реализация покрывает только базовые CRUD операции. Следующие функции планируются, но еще не реализованы:
  - Query building для сложных запросов
  - Миграции схемы БД
  - Индексирование
  - TTL (Time To Live) для автоматического удаления устаревших записей
  - Change streams (только для MongoDB) для подписки на изменения в коллекции

Эти функции будут реализованы в последующих версиях. См. TODO комментарии в соответствующих файлах.

### InMemory адаптеры

**InMemory MessageBus:**
- **Воркеры**: Поддерживает параллельную обработку сообщений через пул воркеров (настраивается через `WorkerCount`)
- **Очереди**: Использует буферизованные очереди для каждого subject (размер настраивается через `BufferSize`)
- **FIFO гарантии**: Опциональная поддержка FIFO через `EnableOrdering`
- **Ограничения**: Подходит для тестирования и локальной разработки. Не рекомендуется для production, так как данные хранятся только в памяти и теряются при перезапуске

**InMemory Repository:**
- **Лимит сущностей**: Опциональное ограничение максимального количества сущностей через `MaxEntities` (0 = без ограничений)
- **Индексы**: Поддержка secondary индексов для быстрого поиска
- **Ограничения**: Подходит для тестирования и прототипирования. Не рекомендуется для production

**Пример конфигурации InMemory адаптеров:**
```go
// MessageBus с воркерами и очередями
mbConfig := messagebus.InMemoryConfig{
    BufferSize:     1000,  // Размер очереди для каждого subject
    WorkerCount:    10,    // Количество воркеров для обработки сообщений
    EnableOrdering: false, // FIFO гарантии (false = параллельная обработка)
}
mb := messagebus.NewInMemoryAdapter(mbConfig)

// Repository с лимитом сущностей
repoConfig := repository.InMemoryConfig{
    MaxEntities: 10000, // Максимум 10000 сущностей (0 = без ограничений)
}
repo := repository.NewInMemoryRepository[User](repoConfig)
```

## Best Practices

### Выбор адаптера

1. **MessageBus**: Выбирайте NATS для pub/sub, Kafka для event streaming, Redis для легковесных сценариев
2. **Event Publisher**: Используйте Kafka для event sourcing, NATS для event-driven архитектуры
3. **Repository**: PostgreSQL для реляционных данных, MongoDB для документных данных, InMemory для тестов

### Конфигурация

1. **Connection pooling**: Настройте connection pool для production окружений
2. **Retry policies**: Используйте exponential backoff для transient errors
3. **Timeouts**: Установите разумные таймауты для всех операций
4. **Metrics**: Включите метрики для мониторинга производительности

### Production готовность

1. **Graceful shutdown**: Все адаптеры поддерживают graceful shutdown
2. **Health checks**: Используйте health check endpoints для мониторинга
3. **Error handling**: Обрабатывайте ошибки и логируйте их
4. **Observability**: Интегрируйте с OpenTelemetry для distributed tracing

### Тестирование

1. Используйте InMemory адаптеры для unit тестов
2. Используйте Docker для integration тестов с реальными системами
3. Мокируйте адаптеры для изоляции тестов

## Примеры

Полные примеры использования каждого адаптера находятся в директории `examples/`:

- `examples/messagebus_nats_example.go` - NATS MessageBus
- `examples/events_kafka_example.go` - Kafka Event Publisher
- `examples/repository_postgres_example.go` - PostgreSQL Repository
- `examples/transport_rest_example.go` - REST Transport
- `examples/transport_websocket_example.go` - WebSocket Transport

## Миграция с internal/adapters

Если вы используете адаптеры из `internal/adapters/`, рекомендуется мигрировать на `framework/adapters/`:

1. Обновите импорты на использование `framework/adapters`
2. Используйте фабрики для создания адаптеров
3. Обновите конфигурацию согласно новым структурам конфигурации
4. Протестируйте изменения в staging окружении

Старые адаптеры в `internal/adapters/` помечены как deprecated и будут удалены в будущих версиях.

