# GraphQL Service Example

Полнофункциональный Product Catalog Service с GraphQL API, демонстрирующий возможности GraphQL Transport в Potter Framework.

## Overview

Этот пример демонстрирует:

- Автогенерация GraphQL схем из proto файлов
- GraphQL queries для чтения данных (CQRS queries)
- GraphQL mutations для команд (CQRS commands)
- GraphQL subscriptions для real-time обновлений (EventBus)
- Интеграция с Event Sourcing
- Query complexity limits и security
- GraphQL Playground для разработки

## Architecture

Сервис использует следующие паттерны:

- **CQRS**: Разделение команд и запросов
- **Event Sourcing**: Хранение событий для восстановления состояния
- **GraphQL**: Гибкий API для клиентов
- **Event Bus**: Публикация событий для subscriptions

## Prerequisites

- Go 1.25+
- Docker и Docker Compose
- PostgreSQL (через Docker)
- NATS (через Docker)

## Quick Start

```bash
# Запуск dependencies
docker-compose up -d

# Применение миграций
make migrate-up

# Генерация кода из proto
make generate

# Запуск сервера
make run

# Открыть GraphQL Playground
make playground
# или
open http://localhost:8082/playground
```

## Project Structure

```
graphql-service/
├── api/
│   ├── proto/              # Proto файлы с Potter annotations
│   └── graphql/             # Сгенерированные GraphQL схемы
├── domain/                  # Domain models и events
├── application/             # Application services и handlers
├── infrastructure/          # Repositories и persistence
├── cmd/server/              # Main server
├── migrations/              # Database migrations
├── docker-compose.yml       # Docker setup
├── Makefile                 # Build commands
└── queries.graphql          # Примеры GraphQL запросов
```

## GraphQL Schema

### Queries

- `product(id: ID!): Product` - Получить продукт по ID
- `products(page: Int, pageSize: Int, category: String, minPrice: Float, maxPrice: Float): ListProductsResponse` - Список продуктов с фильтрацией

### Mutations

- `createProduct(input: CreateProductInput!): CreateProductResponse` - Создать продукт
- `updateProduct(input: UpdateProductInput!): UpdateProductResponse` - Обновить продукт
- `deleteProduct(input: DeleteProductInput!): DeleteProductResponse` - Удалить продукт

### Subscriptions

- `productCreated: ProductCreatedEvent` - Подписка на создание продуктов
- `productUpdated: ProductUpdatedEvent` - Подписка на обновление продуктов
- `productDeleted: ProductDeletedEvent` - Подписка на удаление продуктов

## Example Queries

См. файл `queries.graphql` для примеров запросов, mutations и subscriptions.

### Получить продукт

```graphql
query GetProduct {
  product(id: "123") {
    id
    name
    price
    stock
  }
}
```

### Создать продукт

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

### Подписка на обновления

```graphql
subscription ProductUpdates {
  productUpdated {
    productId
    name
    stock
  }
}
```

## Code Generation

Генерация кода из proto файлов:

```bash
make generate
```

Это выполнит:
1. Парсинг proto файлов с Potter annotations
2. Генерация GraphQL schema
3. Генерация gqlgen конфигурации
4. Запуск gqlgen для генерации resolvers

## Configuration

### Environment Variables

- `DATABASE_URL` - PostgreSQL connection string
- `NATS_URL` - NATS connection string
- `GRAPHQL_PLAYGROUND_ENABLED` - Включить GraphQL Playground (default: true)
- `GRAPHQL_INTROSPECTION_ENABLED` - Включить introspection (default: true)
- `SERVER_PORT` - HTTP server port (default: 8080)

### GraphQL Config

Настройки GraphQL адаптера можно изменить в `cmd/server/main.go`:

```go
config := transport.DefaultGraphQLConfig()
config.Port = 8082
config.EnablePlayground = true
config.ComplexityLimit = 1000
config.MaxDepth = 15
```

## Testing

```bash
# Unit tests
make test

# Integration tests
make test-integration
```

## API Documentation

После запуска сервера:

- **GraphQL Playground**: http://localhost:8082/playground
- **GraphQL Endpoint**: http://localhost:8082/graphql
- **Health Check**: http://localhost:8080/health

## Troubleshooting

### Проблемы с подключением к БД

Убедитесь, что PostgreSQL запущен:
```bash
docker-compose ps
```

### Проблемы с генерацией кода

Убедитесь, что proto файлы доступны:
```bash
ls api/proto/product_service.proto
```

### Проблемы с GraphQL Playground

Проверьте, что сервер запущен на порту 8082:
```bash
curl http://localhost:8082/playground
```

## Next Steps

1. Изучите сгенерированные GraphQL схемы в `api/graphql/schema.graphql`
2. Посмотрите примеры запросов в `queries.graphql`
3. Изучите реализацию resolvers в `application/`
4. Настройте кастомные resolvers для специфичной логики

## References

- [Potter Framework Documentation](../../../README.md)
- [GraphQL Transport Documentation](../../../framework/adapters/transport/GRAPHQL.md)
- [gqlgen Documentation](https://gqlgen.com/)

