# Transport Adapters

Пакет `framework/adapters/transport` предоставляет адаптеры для различных транспортных протоколов, включая REST, gRPC, WebSocket и GraphQL.

## GraphQL Transport

GraphQL Transport предоставляет полнофункциональный GraphQL API с интеграцией CQRS, автоматической генерацией схем из proto файлов и поддержкой real-time subscriptions.

### Основные возможности

- Автоматическая интеграция с CQRS (CommandBus, QueryBus, EventBus)
- Автогенерация GraphQL схем из proto файлов с Potter annotations
- Real-time subscriptions через WebSocket
- Query complexity analysis для защиты от DoS
- GraphQL Playground для разработки
- Контроль introspection и глубины запросов

### Быстрый старт

```go
import "github.com/akriventsev/potter/framework/adapters/transport"

// Создание buses
commandBus := transport.NewInMemoryCommandBus()
queryBus := transport.NewInMemoryQueryBus()
eventBus := events.NewInMemoryEventBus()

// Создание GraphQL схемы (см. GRAPHQL.md для деталей)
schema := createGraphQLSchema()

// Создание адаптера с автоматической интеграцией CQRS
config := transport.DefaultGraphQLConfig()
adapter, err := transport.NewGraphQLAdapterWithCQRS(
    config,
    commandBus,
    queryBus,
    eventBus,
    schema,
)
if err != nil {
    log.Fatal(err)
}

// Запуск сервера
ctx := context.Background()
if err := adapter.Start(ctx); err != nil {
    log.Fatal(err)
}
```

### Документация

Подробная документация по GraphQL Transport находится в [GRAPHQL.md](GRAPHQL.md).

## REST Transport

REST Transport предоставляет HTTP API с интеграцией CommandBus и QueryBus.

## gRPC Transport

gRPC Transport предоставляет gRPC сервисы с интеграцией CQRS.

## WebSocket Transport

WebSocket Transport предоставляет real-time коммуникацию через WebSocket.

