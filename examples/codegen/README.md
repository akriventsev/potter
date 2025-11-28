# Code Generation Example

Этот пример демонстрирует использование Potter Code Generator для создания полноценного CQRS приложения из protobuf спецификации.

## Файлы

- `simple-service.proto` - protobuf спецификация с Potter аннотациями
- `generated/` - директория с сгенерированным кодом (создается после генерации)

## Шаги

### 1. Установка potter-gen

```bash
cd /Users/alexander/development/potter
go install ./cmd/potter-gen
```

### 2. Генерация приложения

```bash
cd examples/codegen
potter-gen init --proto simple-service.proto --module simple-service --output ./generated
```

Это создаст полную структуру проекта в `generated/`:

```
generated/
├── cmd/server/main.go
├── domain/
│   ├── aggregates.go
│   ├── events.go
│   └── repository.go
├── application/
│   ├── command/
│   │   ├── create_item.go
│   │   └── update_item.go
│   └── query/
│       ├── get_item.go
│       └── list_items.go
├── infrastructure/
│   ├── repository/
│   │   └── item_repository.go
│   └── cache/
│       └── redis_cache.go
├── presentation/
│   └── rest/
│       └── handler.go
├── config/
│   └── config.go
├── migrations/
│   └── 001_create_tables.sql
├── docker-compose.yml
├── Makefile
├── go.mod
└── README.md
```

### 3. Запуск приложения

```bash
cd generated
make docker-up    # Запуск PostgreSQL, Redis, NATS
make migrate      # Применение миграций
make run          # Запуск приложения
```

Приложение будет доступно на `http://localhost:8080`.

### 4. Добавление бизнес-логики

Откройте `application/command/create_item.go` и добавьте логику между маркерами:

```go
// USER CODE BEGIN: Validation
if createCmd.Quantity < 0 {
    return fmt.Errorf("quantity cannot be negative")
}
// USER CODE END: Validation

// USER CODE BEGIN: BusinessLogic
item := domain.NewItem(createCmd.Name, createCmd.Description, createCmd.Quantity)
// USER CODE END: BusinessLogic
```

### 5. Обновление proto и регенерация

Измените `simple-service.proto` (например, добавьте новое поле) и обновите код:

```bash
potter-gen update --proto simple-service.proto --output ./generated --interactive
```

Ваша бизнес-логика между маркерами будет сохранена.

### 6. Генерация SDK

```bash
potter-gen sdk --proto simple-service.proto --output ./simple-service-sdk
```

Использование SDK в другом сервисе:

```go
import "simple-service-sdk"

config := sdk.DefaultConfig()
config.NATSUrl = "nats://localhost:4222"
client, _ := sdk.NewClient(config)
defer client.Close()

event, err := client.CreateItem(ctx, sdk.CreateItemCommand{
    Name: "Laptop",
    Description: "Gaming laptop",
    Quantity: 10,
})
```

## API Endpoints

После запуска доступны следующие endpoints:

- `POST /api/v1/item/create` - создание item
- `GET /api/v1/item/get?id={id}` - получение item
- `POST /api/v1/item/update` - обновление item
- `GET /api/v1/item/list?page=1&page_size=10` - список items

## Мониторинг

- Prometheus: http://localhost:9090
- NATS monitoring: http://localhost:8222
- Metrics: http://localhost:2112/metrics

## Очистка

```bash
make docker-down
rm -rf generated/
```

