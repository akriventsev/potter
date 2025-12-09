# OpenAPI Service Example

Пример использования автоматической генерации OpenAPI спецификаций из proto файлов.

## Возможности

- ✅ Автоматическая генерация OpenAPI 3.0 спецификации
- ✅ Swagger UI для интерактивного тестирования API
- ✅ Валидация запросов по OpenAPI схеме
- ✅ REST API с CRUD операциями
- ✅ Potter CQRS integration
- ✅ Docker Compose для локального запуска

## Архитектура

```
Proto файл (api/product_service.proto)
    ↓
Potter Code Generator (potter-gen)
    ↓
├── OpenAPI спецификация (api/openapi/openapi.yaml)
├── REST handlers (presentation/rest/)
├── Swagger UI integration (presentation/rest/swagger.go)
└── Validation middleware (main.go)
```

## Quick Start

### 1. Запуск инфраструктуры

```bash
make docker-up
```

Запускает:

- PostgreSQL (порт 5432)
- Redis (порт 6379)

### 2. Применение миграций

```bash
make migrate-up
```

### 3. Генерация кода

```bash
make generate
```

Генерирует:

- OpenAPI спецификацию из proto файла
- REST handlers
- Swagger UI integration
- CQRS компоненты

### 4. Запуск сервиса

```bash
make run
```

Сервис доступен на:

- REST API: http://localhost:8080/api/v1
- Swagger UI: http://localhost:8080/swagger/
- OpenAPI spec: http://localhost:8080/swagger/openapi.yaml
- Health check: http://localhost:8080/health

## Использование

### Swagger UI

Откройте http://localhost:8080/swagger/ для интерактивного тестирования API.

Swagger UI предоставляет:

- Полную документацию всех endpoints
- Интерактивное тестирование запросов
- Примеры request/response
- Валидацию по схеме

### REST API Examples

**Создание продукта:**

```bash
curl -X POST http://localhost:8080/api/v1/products \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Laptop",
    "description": "High-performance laptop",
    "price": 1299.99,
    "sku": "LAP-001"
  }'
```

**Получение продукта:**

```bash
curl http://localhost:8080/api/v1/products/LAP-001
```

**Список продуктов:**

```bash
curl http://localhost:8080/api/v1/products?limit=10&offset=0
```

**Обновление продукта:**

```bash
curl -X PUT http://localhost:8080/api/v1/products/LAP-001 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Gaming Laptop",
    "price": 1499.99
  }'
```

**Удаление продукта:**

```bash
curl -X DELETE http://localhost:8080/api/v1/products/LAP-001
```

### OpenAPI Validation

Пример валидации (некорректный запрос):

```bash
curl -X POST http://localhost:8080/api/v1/products \
  -H "Content-Type: application/json" \
  -d '{
    "name": "",
    "price": -100
  }'
```

Ответ:

```json
{
  "error": "validation_failed",
  "details": [
    {"field": "name", "message": "must not be empty"},
    {"field": "price", "message": "must be greater than 0"},
    {"field": "sku", "message": "required field missing"}
  ]
}
```

## Proto Annotations

Пример proto файла с OpenAPI аннотациями:

```protobuf
service ProductService {
  option (potter.service) = {
    module_name: "product"
    transport: ["REST"]
    openapi_info: {
      title: "Product Service API"
      version: "1.0.0"
      description: "API for managing products"
      contact: {
        name: "API Support"
        email: "support@example.com"
      }
    }
  };

  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse) {
    option (potter.command) = {
      aggregate: "Product"
      tags: ["Products"]
      summary: "Create a new product"
      description: "Creates a new product with the provided details"
    };
  }
}
```

## OpenAPI Спецификация

Сгенерированная OpenAPI спецификация доступна в `api/openapi/openapi.yaml`.

Основные секции:

- **info** - метаданные API
- **paths** - все endpoints с параметрами и responses
- **components/schemas** - модели данных
- **tags** - группировка операций

## Validation Middleware

Пример включения валидации в main.go:

```go
import "github.com/akriventsev/potter/framework/adapters/transport"

validator, err := transport.NewOpenAPIValidator(
  "./api/openapi/openapi.yaml",
  &transport.ValidationOptions{
    ValidateRequest: true,
    MultiError: true,
  },
)
if err != nil {
  log.Fatal(err)
}

router.Use(validator.Middleware())
```

## Customization

### Добавление custom endpoints

Добавьте в `presentation/rest/handler.go`:

```go
func (h *Handler) CustomEndpoint(c *gin.Context) {
  // Custom logic
}
```

Зарегистрируйте в `RegisterRoutes()`:

```go
api.GET("/custom", h.CustomEndpoint)
```

### Расширение OpenAPI спецификации

Отредактируйте `api/openapi/openapi.yaml` для добавления:

- Custom security schemes
- Additional responses
- Examples
- Extensions (x-*)

## Troubleshooting

**Swagger UI не загружается:**

- Проверьте, что сервис запущен на порту 8080
- Проверьте логи: `make logs`
- Проверьте доступность openapi.yaml: `curl http://localhost:8080/swagger/openapi.yaml`

**Validation errors:**

- Проверьте OpenAPI спецификацию на корректность
- Используйте Swagger Editor для валидации: https://editor.swagger.io/
- Проверьте логи для деталей

**Codegen errors:**

- Проверьте proto файл на синтаксические ошибки
- Убедитесь, что potter/options.proto импортирован корректно
- Запустите `potter-gen check` для диагностики

## Makefile Commands

```bash
make docker-up      # Запуск инфраструктуры
make docker-down    # Остановка инфраструктуры
make migrate-up     # Применение миграций
make migrate-down   # Откат миграций
make generate       # Генерация кода из proto
make run            # Запуск сервиса
make test           # Запуск тестов
make swagger        # Открыть Swagger UI в браузере
make logs           # Просмотр логов
```

## Дополнительные ресурсы

- [Potter Framework Documentation](../../README.md)
- [Code Generator Guide](../../framework/codegen/README.md)
- [OpenAPI Specification](https://swagger.io/specification/)
- [Swagger UI Documentation](https://swagger.io/tools/swagger-ui/)

Пример следует структуре существующих примеров в `examples/graphql-service/` и `examples/saga-order/`.

