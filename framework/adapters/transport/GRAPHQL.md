# GraphQL Transport

GraphQL Transport для Potter Framework предоставляет полнофункциональный GraphQL API с интеграцией CQRS, автоматической генерацией схем из proto файлов и поддержкой real-time subscriptions.

## Introduction

GraphQL Transport в Potter Framework позволяет:

- Автоматически генерировать GraphQL схемы из proto файлов с Potter annotations
- Интегрировать GraphQL queries с CQRS QueryBus
- Интегрировать GraphQL mutations с CQRS CommandBus
- Реализовать real-time subscriptions через EventBus
- Использовать GraphQL Playground для разработки
- Применять query complexity analysis для защиты от DoS

## Architecture

```
┌─────────────┐
│ GraphQL     │
│ Client      │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│ GraphQLAdapter  │
└──────┬──────────┘
       │
       ├──► QueryResolver ──► QueryBus ──► QueryHandler
       ├──► CommandResolver ──► CommandBus ──► CommandHandler
       └──► SubscriptionResolver ──► EventBus ──► Events
```

## Getting Started

### Установка зависимостей

GraphQL Transport использует [gqlgen](https://gqlgen.com/) для реализации GraphQL сервера. Зависимости уже включены в `go.mod`:

```go
require (
    github.com/99designs/gqlgen v0.17.49
    github.com/vektah/gqlparser/v2 v2.5.16
)
```

### Базовая настройка

```go
package main

import (
    "context"
    "github.com/akriventsev/potter/framework/adapters/transport"
    "github.com/akriventsev/potter/framework/events"
    "github.com/akriventsev/potter/framework/transport"
)

func main() {
    // Создание buses
    commandBus := transport.NewInMemoryCommandBus()
    queryBus := transport.NewInMemoryQueryBus()
    eventBus := events.NewInMemoryEventBus()
    
    // Создание базовой GraphQL схемы (см. раздел Schema Generation)
    baseSchema := createGraphQLSchema()
    
    // Создание GraphQL адаптера с автоматической интеграцией CQRS
    // Используйте NewGraphQLAdapterWithCQRS для автоматической интеграции
    // или NewGraphQLAdapter для ручной настройки
    config := transport.DefaultGraphQLConfig()
    adapter, err := transport.NewGraphQLAdapterWithCQRS(
        config,
        commandBus,
        queryBus,
        eventBus,
        baseSchema,
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Запуск сервера
    ctx := context.Background()
    if err := adapter.Start(ctx); err != nil {
        log.Fatal(err)
    }
    
    // GraphQL Playground доступен на http://localhost:8082/playground
    // GraphQL endpoint: http://localhost:8082/graphql
}
```

## Schema Generation

### Автогенерация из proto файлов

GraphQL схемы автоматически генерируются из proto файлов с Potter annotations:

```bash
potter-gen generate --proto api/proto/product_service.proto --with-graphql
```

Это создаст:
- `api/graphql/schema.graphql` - GraphQL schema
- `api/graphql/gqlgen.yml` - gqlgen конфигурация
- `api/graphql/resolvers.go` - Resolver stubs (будет перезаписан gqlgen)
- `api/graphql/potter_resolvers.go` - Документация и примеры dispatch резолверов

Затем запустите gqlgen для генерации кода:

```bash
cd api/graphql && gqlgen generate
```

**Важно:** После генерации схемы через `potter-gen`, используйте `NewGraphQLAdapterWithCQRS` для автоматической интеграции CQRS. Все поля Query/Mutation/Subscription автоматически будут маппиться на соответствующие Potter резолверы без необходимости ручной реализации.

### Автоматическая регистрация резолверов

При использовании `NewGraphQLAdapterWithCQRS`, резолверы автоматически регистрируются на основе схемы:

- **Query поля** → `QueryResolver.Resolve(queryName, args)` → `QueryBus.Ask()`
- **Mutation поля** → `CommandResolver.Resolve(commandName, args)` → `CommandBus.Send()`
- **Subscription поля** → `SubscriptionResolver.Subscribe(ctx, eventType)` → `EventBus.Subscribe()`

Имена полей в GraphQL схеме автоматически маппятся на имена команд/запросов/событий. Например:
- `Query.getProduct` → `QueryResolver.Resolve("getProduct", args)`
- `Mutation.createProduct` → `CommandResolver.Resolve("createProduct", args)`
- `Subscription.productCreated` → `SubscriptionResolver.Subscribe(ctx, "productCreated")`

### Маппинг Potter annotations → GraphQL

| Potter Annotation | GraphQL Result |
|-------------------|----------------|
| `potter.query` | Query field |
| `potter.command` | Mutation field |
| `potter.event` | Subscription field |
| `potter.aggregate` | GraphQL type |

**Пример proto файла:**

```proto
service ProductService {
  rpc GetProduct(GetProductRequest) returns (GetProductResponse) {
    option (potter.query) = {
      cacheable: true
      cache_ttl_seconds: 300
    };
  }
  
  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse) {
    option (potter.command) = {
      aggregate: "Product"
      async: true
      idempotent: true
    };
  }
}

message ProductCreatedEvent {
  option (potter.event) = {
    event_type: "product.created"
    aggregate: "Product"
  };
  string product_id = 1;
}
```

**Сгенерированная GraphQL schema:**

```graphql
type Query {
  getProduct(id: ID!): Product @cacheControl(maxAge: 300)
}

type Mutation {
  createProduct(input: CreateProductInput!): CreateProductResponse @async @idempotent
}

type Subscription {
  productCreated: ProductCreatedEvent!
}
```

## Resolvers

### Автоматические resolvers (Drop-in CQRS Integration)

GraphQL Transport автоматически создает и интегрирует resolvers для CQRS через `potterExecutableSchema`:

- **QueryResolver**: Маппинг GraphQL queries → CQRS queries через QueryBus
- **CommandResolver**: Маппинг GraphQL mutations → CQRS commands через CommandBus
- **SubscriptionResolver**: Маппинг GraphQL subscriptions → EventBus через SubscriptionManager

**Автоматическая интеграция:**

При использовании `NewGraphQLAdapterWithCQRS`, все поля Query/Mutation/Subscription автоматически регистрируются как dispatch resolvers, которые перенаправляют вызовы в соответствующие Potter резолверы:

1. **Автоматическая регистрация**: При вызове `adapter.Start()`, `potterExecutableSchema.AutoRegisterResolvers()` анализирует AST схемы и создает dispatch resolvers для всех полей Query/Mutation/Subscription.

2. **Динамическая маршрутизация**: `AroundFields` middleware перехватывает вызовы полей и перенаправляет их в Potter резолверы через `GetResolver()`.

3. **Приоритет резолверов**: Сначала проверяется `ResolverRegistry` (кастомные резолверы), затем используются дефолтные Potter резолверы для Query/Mutation/Subscription.

**Пример автоматической интеграции:**

```go
// Создание базовой схемы (может быть сгенерирована через gqlgen)
baseSchema := createGraphQLSchema()

// Создание адаптера с автоматической интеграцией CQRS
config := transport.DefaultGraphQLConfig()
adapter, err := transport.NewGraphQLAdapterWithCQRS(
    config,
    commandBus,
    queryBus,
    eventBus,
    baseSchema,
)
if err != nil {
    log.Fatal(err)
}

// При вызове Start() автоматически регистрируются dispatch resolvers:
// - Query.getProduct → QueryResolver.Resolve("getProduct", args)
// - Mutation.createProduct → CommandResolver.Resolve("createProduct", args)
// - Subscription.productCreated → SubscriptionResolver.Subscribe(ctx, "productCreated")
ctx := context.Background()
if err := adapter.Start(ctx); err != nil {
    log.Fatal(err)
}
```

**Как это работает:**

1. `NewGraphQLAdapterWithCQRS` создает `potterExecutableSchema`, который оборачивает `baseSchema`.
2. При `adapter.Start()`, вызывается `potterExecutableSchema.AutoRegisterResolvers()`, который:
   - Анализирует AST схемы (`schema.Types["Query"]`, `schema.Types["Mutation"]`, `schema.Types["Subscription"]`)
   - Для каждого поля создает dispatch resolver, который вызывает соответствующий Potter резолвер
   - Регистрирует резолверы в `ResolverRegistry`
3. `AroundFields` middleware перехватывает вызовы полей:
   - Получает `typeName` и `fieldName` из `graphql.GetFieldContext(ctx)`
   - Вызывает `potterExecutableSchema.GetResolver(typeName, fieldName)`
   - Если резолвер найден, вызывает его вместо базового резолвера
   - Если не найден, делегирует в базовый резолвер

### Кастомные resolvers

Вы можете зарегистрировать кастомные resolvers через `RegisterResolver`:

```go
adapter.RegisterResolver("Query", "customField", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
    // Кастомная логика
    return result, nil
})
```

**Приоритет резолверов:**
1. Кастомные резолверы (зарегистрированные через `RegisterResolver`)
2. Автоматические Potter dispatch resolvers (для Query/Mutation/Subscription)
3. Базовые резолверы из `baseSchema` (если не найдены выше)

**Примечание:** Кастомные резолверы имеют приоритет над автоматическими, что позволяет переопределить поведение для конкретных полей.

## Queries and Mutations

### Определение queries в proto

```proto
rpc GetProduct(GetProductRequest) returns (GetProductResponse) {
  option (potter.query) = {
    cacheable: true
    cache_ttl_seconds: 300
  };
}
```

### Определение mutations в proto

```proto
rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse) {
  option (potter.command) = {
    aggregate: "Product"
    async: true
    idempotent: true
  };
}
```

### Примеры GraphQL запросов

**Query:**
```graphql
query GetProduct {
  product(id: "123") {
    id
    name
    price
  }
}
```

**Mutation:**
```graphql
mutation CreateProduct {
  createProduct(input: {
    name: "Laptop"
    price: 999.99
    stock: 10
  }) {
    productId
    message
  }
}
```

## Subscriptions

### Real-time updates через WebSocket

GraphQL subscriptions используют WebSocket для real-time обновлений:

```graphql
subscription ProductUpdates {
  productUpdated {
    productId
    name
    stock
  }
}
```

### Автоматическая интеграция subscriptions

При использовании `NewGraphQLAdapterWithCQRS`, subscription поля автоматически маппятся на `SubscriptionResolver.Subscribe()`:

1. **Автоматическая регистрация**: При `adapter.Start()`, все поля `Subscription` типа автоматически регистрируются как dispatch resolvers.

2. **Интеграция с EventBus**: `SubscriptionResolver.Subscribe()` создает подписку через `SubscriptionManager`, который интегрируется с `EventBus`.

3. **WebSocket transport**: WebSocket transport автоматически включается в `GraphQLAdapter.Start()` для поддержки subscriptions.

**Пример:**

```go
// В GraphQL схеме:
// type Subscription {
//   productCreated: ProductCreatedEvent!
// }

// Автоматически маппится на:
// SubscriptionResolver.Subscribe(ctx, "productCreated")
// → SubscriptionManager.Subscribe(ctx, "productCreated", nil)
// → EventBus.Subscribe("productCreated", handler)
```

**Примечание:** Для subscriptions, `AroundFields` middleware перехватывает вызов и возвращает канал событий из `SubscriptionResolver.Subscribe()`. gqlgen автоматически обрабатывает канал и отправляет события клиенту через WebSocket.

### Фильтрация событий

Вы можете фильтровать события по correlation ID или aggregate ID:

```go
// Фильтр по correlation ID
filter := &transport.CorrelationIDFilter{CorrelationID: "corr-123"}
channel, err := subscriptionManager.Subscribe(ctx, "product.updated", filter)

// Фильтр по aggregate ID
filter := &transport.AggregateIDFilter{AggregateID: "product-456"}
channel, err := subscriptionManager.Subscribe(ctx, "product.updated", filter)

// Композитный фильтр
filter := &transport.CompositeFilter{
    Filters: []transport.EventFilter{corrFilter, aggFilter},
    Op:      "AND",
}
```

## Advanced Features

### Query Complexity Analysis

GraphQL Transport автоматически анализирует сложность запросов для защиты от DoS:

```go
config := transport.DefaultGraphQLConfig()
config.ComplexityLimit = 1000  // Максимальная сложность запроса (через FixedComplexityLimit)
config.MaxDepth = 15           // Максимальная глубина вложенности (через кастомное расширение)
```

**Реализация:**
- `ComplexityLimit` использует встроенное расширение `extension.FixedComplexityLimit` для ограничения сложности запросов
- `MaxDepth` использует кастомное расширение `maxDepthExtension`, которое проверяет глубину запроса в `MutateOperationContext` и отклоняет запросы, превышающие лимит

### Caching

Queries с `cacheable: true` автоматически кэшируются:

```go
// QueryResolver автоматически кэширует результаты
// TTL настраивается через cache_ttl_seconds в proto
```

### Metrics

GraphQL Transport интегрируется с `framework/metrics`:

- Запись метрик запросов (время выполнения, успешность)
- Счетчики активных запросов
- Метрики subscriptions

## Security

### Query Complexity Limits

```go
config.ComplexityLimit = 1000  // Защита от сложных запросов
config.MaxDepth = 15           // Защита от глубокой вложенности
```

### Introspection

В production отключите introspection:

```go
config.EnableIntrospection = false
```

**Реализация:**
- `EnableIntrospection` контролируется через кастомное расширение `introspectionDisableExtension`
- Расширение проверяет raw query и имя операции на наличие `__schema` или `__type` полей
- Если introspection отключен, запросы с этими полями отклоняются с ошибкой "introspection is disabled"

### Authentication

Добавьте authentication middleware:

```go
// Пример с JWT
adapter.RegisterResolver("Query", "protectedQuery", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
    token := getTokenFromContext(ctx)
    if !validateToken(token) {
        return nil, errors.New("unauthorized")
    }
    // ...
})
```

## Integration with Existing GraphQL Servers

### Apollo Federation

GraphQL Transport может быть интегрирован с Apollo Federation:

```go
// Регистрация как federated service
// См. документацию Apollo Federation
```

### Schema Stitching

Вы можете объединить несколько GraphQL схем:

```go
// Использование schema stitching
// См. документацию gqlgen
```

## Testing

### Unit Testing

```go
func TestGraphQLAdapter(t *testing.T) {
    config := transport.DefaultGraphQLConfig()
    commandBus := transport.NewInMemoryCommandBus()
    queryBus := transport.NewInMemoryQueryBus()
    eventBus := events.NewInMemoryEventBus()
    schema := createTestSchema()
    
    adapter, err := transport.NewGraphQLAdapter(config, commandBus, queryBus, eventBus, schema)
    require.NoError(t, err)
    
    ctx := context.Background()
    err = adapter.Start(ctx)
    require.NoError(t, err)
    defer adapter.Stop(ctx)
    
    // Тестирование запросов
}
```

### Integration Testing

См. `framework/adapters/transport/graphql_integration_test.go` для примеров integration тестов.

## Configuration

### GraphQLConfig

```go
type GraphQLConfig struct {
    Port              int    // Порт сервера (default: 8082)
    Path              string // Путь GraphQL endpoint (default: "/graphql")
    PlaygroundPath    string // Путь GraphQL Playground (default: "/playground")
    EnablePlayground  bool   // Включить Playground (default: true)
    EnableIntrospection bool // Включить introspection (default: true)
    EnableMetrics     bool   // Включить метрики (default: true)
    ComplexityLimit   int    // Лимит сложности (default: 1000)
    MaxDepth          int    // Максимальная глубина (default: 15)
}
```

## Examples

Полный пример использования см. в `examples/graphql-service/`:

- Proto файлы с Potter annotations
- Сгенерированные GraphQL схемы
- Resolvers implementation
- Docker Compose setup
- Примеры запросов

## Troubleshooting

### Проблемы с генерацией схемы

Убедитесь, что proto файлы содержат правильные Potter annotations:
```bash
potter-gen check --proto api/proto/product_service.proto
```

### Проблемы с subscriptions

Проверьте, что WebSocket transport включен:
```go
// WebSocket transport автоматически включается в GraphQLAdapter
```

### Проблемы с метриками

Убедитесь, что метрики включены:
```go
config.EnableMetrics = true
```

## References

- [gqlgen Documentation](https://gqlgen.com/)
- [GraphQL Specification](https://graphql.org/learn/)
- [Potter Framework Documentation](../../../README.md)

