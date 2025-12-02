# Potter Code Generator

Модуль для генерации CQRS приложений из protobuf спецификаций с Potter custom options.

## Архитектура

- **Parser** - парсинг proto файлов с извлечением Potter аннотаций
- **Generators** - генераторы для каждого слоя (domain, application, infrastructure, presentation, main, sdk)
- **Updater** - система обновления с сохранением пользовательского кода
- **CLI** - `potter-gen` инструмент для управления генерацией
- **Protoc Plugin** - `protoc-gen-potter` для интеграции с protoc

## Использование

### 1. Определение proto файла с Potter аннотациями

```protobuf
syntax = "proto3";
import "potter/options.proto";

service ProductService {
  option (potter.service) = {
    module_name: "product"
    transport: ["REST", "NATS"]
  };

  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse) {
    option (potter.command) = {
      aggregate: "Product"
      async: true
    };
  }

  rpc GetProduct(GetProductRequest) returns (GetProductResponse) {
    option (potter.query) = {
      cacheable: true
      cache_ttl_seconds: 300
    };
  }
}
```

### 2. Генерация приложения

```bash
# Через CLI
potter-gen init --proto api/service.proto --module myapp --output ./myapp

# Через protoc плагин
protoc --proto_path=api \
       --potter_out=. \
       --potter_opt=module=myapp \
       api/service.proto
```

### 3. Обновление при изменении proto

```bash
potter-gen update --proto api/service.proto --output ./myapp --interactive
```

### 3.1. Проверка синхронности (для CI)

```bash
# Проверка расхождений между proto и кодом без применения изменений
potter-gen check --proto api/service.proto --output ./myapp

# Команда завершится с ненулевым кодом (exit code 1), если есть расхождения
# Рекомендуется запускать в CI перед merge для гарантии синхронности proto и кода
```

**Пример использования в CI (GitHub Actions):**
```yaml
- name: Check codegen sync
  run: |
    potter-gen check --proto api/service.proto --output ./myapp
```

### 4. Генерация SDK

```bash
potter-gen sdk --proto api/service.proto --output ./myapp-sdk
```

## Поддерживаемые транспорты

Potter Code Generator поддерживает следующие транспорты:

- **REST** (Gin framework) - стандартный REST API
- **GraphQL** (gqlgen + Potter CQRS integration) - GraphQL API с автоматической генерацией схемы и резолверов
- **gRPC** (в разработке) - gRPC сервер
- **NATS** (message bus) - асинхронная обработка команд и событий
- **Kafka** (message bus) - интеграция с Kafka для событий

Транспорты указываются в proto файле через поле `transport` в `potter.service` опции:

```protobuf
service MyService {
  option (potter.service) = {
    module_name: "myapp"
    transport: ["REST", "GraphQL"]
  };
  // ...
}
```

## GraphQL Integration

При указании `transport: ["GraphQL"]` автоматически генерируется:

1. **GraphQL схема** (`api/graphql/schema.graphql`) - автоматическая генерация из proto
2. **gqlgen конфигурация** (`api/graphql/gqlgen.yml`) - настройка для генерации кода
3. **GraphQL адаптер** (`presentation/graphql/adapter.go`) - интеграция с Potter CQRS
4. **Резолверы** - автоматический маппинг RPC методов на Query/Mutation/Subscription

### Маппинг RPC методов

- **Query методы** → GraphQL Query type
- **Command методы** → GraphQL Mutation type
- **Event messages** → GraphQL Subscription type

### Интеграция с CQRS

GraphQL адаптер автоматически интегрируется с:
- `CommandBus` - для выполнения mutations
- `QueryBus` - для выполнения queries
- `EventBus` - для subscriptions через WebSocket

### Поддержка WebSocket

GraphQL subscriptions работают через WebSocket с поддержкой:
- graphql-ws protocol (Apollo subscriptions)
- Фильтрация событий по correlation ID и aggregate ID
- Автоматическая очистка при disconnect

## Структура сгенерированного проекта

```
myapp/
├── cmd/server/main.go          # Точка входа
├── domain/                      # Доменный слой
│   ├── aggregates.go           # Агрегаты
│   ├── events.go               # События
│   └── repository.go           # Интерфейсы репозиториев
├── application/                 # Application слой
│   ├── command/                # Команды и handlers
│   └── query/                  # Запросы и handlers
├── infrastructure/              # Infrastructure слой
│   ├── repository/             # Реализации репозиториев
│   └── cache/                  # Cache service
├── presentation/                # Presentation слой
│   ├── rest/                   # REST handlers (если REST включен)
│   └── graphql/                # GraphQL адаптер (если GraphQL включен)
├── api/                         # API определения
│   └── graphql/                # GraphQL схема и конфигурация (если GraphQL включен)
│       ├── schema.graphql      # GraphQL схема
│       └── gqlgen.yml          # gqlgen конфигурация
├── config/                      # Конфигурация
├── migrations/                  # SQL миграции
├── docker-compose.yml          # Инфраструктура
├── Makefile                    # Build команды
└── README.md                   # Документация
```

## Пользовательский код

Генератор создает заглушки для бизнес-логики с маркерами:

```go
func (h *CreateProductHandler) Handle(ctx context.Context, cmd transport.Command) error {
    // ... generated code ...

    // USER CODE BEGIN: Validation
    // Add your validation logic here
    // USER CODE END: Validation

    // USER CODE BEGIN: BusinessLogic
    // Implement your business logic here
    product := domain.NewProduct(createCmd.Name, createCmd.Description, createCmd.SKU)
    // USER CODE END: BusinessLogic

    // ... generated code ...
}
```

При обновлении proto файла и повторной генерации, код между маркерами сохраняется.

## Potter Custom Options

Подробное описание всех custom options см. в `api/proto/potter/options.proto`.

## SDK

Генератор создает type-safe SDK на базе `framework/invoke`:

```go
import "myapp-sdk"

config := sdk.DefaultConfig()
config.NATSUrl = "nats://localhost:4222"
client, _ := sdk.NewClient(config)
defer client.Close()

event, err := client.CreateProduct(ctx, sdk.CreateProductCommand{
    Name: "Laptop",
    SKU: "LAP-001",
})
```

## Примеры использования

### Указание транспортов в proto файле

```protobuf
syntax = "proto3";
import "potter/options.proto";

service ProductService {
  option (potter.service) = {
    module_name: "product"
    transport: ["REST", "GraphQL"]
  };

  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse) {
    option (potter.command) = {
      aggregate: "Product"
      async: true
    };
  }

  rpc GetProduct(GetProductRequest) returns (GetProductResponse) {
    option (potter.query) = {
      cacheable: true
      cache_ttl_seconds: 300
    };
  }
}
```

### Команды генерации

```bash
# Инициализация проекта
potter-gen init --proto api/service.proto --module myapp --output ./myapp

# Генерация кода (транспорты определяются из proto)
potter-gen generate --proto api/service.proto --output ./myapp

# Обновление кода
potter-gen update --proto api/service.proto --output ./myapp --interactive
```

### Структура сгенерированных файлов для разных транспортов

**Только REST:**
```
presentation/rest/handler.go
cmd/server/main.go (только REST сервер)
```

**REST + GraphQL:**
```
presentation/rest/handler.go
presentation/graphql/adapter.go
api/graphql/schema.graphql
api/graphql/gqlgen.yml
cmd/server/main.go (REST + GraphQL серверы)
```

## Troubleshooting

### Решение ошибки "Input is shadowed in proto_path"

Эта ошибка возникала при конфликте путей в protoc. Исправлено в последней версии:
- Используется абсолютный путь к proto файлу
- Убрана конфликтующая установка рабочей директории
- Proto paths настроены правильно

### Проблемы с импортами proto файлов

Убедитесь, что:
- Все импорты доступны через `--proto_path`
- Potter options импортированы: `import "potter/options.proto"` (рекомендуемый способ)
- Пути к proto файлам корректны

**Примечание:** Рекомендуется использовать короткий импорт `import "potter/options.proto";` вместо полного пути `github.com/akriventsev/potter/options.proto`. Potter-gen автоматически находит путь к Potter options.

### Ошибка импорта Potter options

Если вы видите ошибку `Import "github.com/akriventsev/potter/options.proto" was not found`, это означает, что используется неправильный формат импорта.

**Проблема:** В proto файле используется полный путь модуля Go вместо относительного пути.

**Решение:** Измените импорт в вашем proto файле:

❌ НЕПРАВИЛЬНО:
```protobuf
import "github.com/akriventsev/potter/options.proto";
```

✅ ПРАВИЛЬНО:
```protobuf
import "potter/options.proto";
```

**Почему это важно:** protoc ищет файлы относительно `--proto_path`, а не по полному пути модуля Go. Potter-gen автоматически настраивает пути для поиска `potter/options.proto`.

### Конфликты портов при использовании нескольких транспортов

По умолчанию:
- REST: порт 8080
- GraphQL: порт 8082

Измените порты через переменные окружения:
```bash
SERVER_PORT=8080
GRAPHQL_PORT=8082
```

### Настройка путей для Potter options

**Рекомендуемый импорт:** `import "potter/options.proto";`

Potter-gen автоматически находит путь к Potter options следующими способами:

1. **Поиск вверх по директориям** - поднимается от директории proto файла, пока не найдет `api/proto/potter/options.proto`
2. **Переменная окружения `POTTER_PROTO_PATH`** - если установлена, используется для нестандартных установок
   ```bash
   export POTTER_PROTO_PATH=/path/to/potter/api/proto
   ```
3. **Через `go list`** - если Potter установлен как зависимость в go.mod, путь определяется автоматически
4. **Go modules cache** - проверяется стандартный путь кеша модулей

**Примеры для разных сценариев:**

- **Локальная разработка:** просто используйте `import "potter/options.proto";` - путь будет найден автоматически
- **Установка через go modules:** добавьте Potter в go.mod, импорт остается `import "potter/options.proto";`
- **Нестандартная установка:** установите `POTTER_PROTO_PATH` с путем к директории, содержащей `potter/options.proto`

**Отладка:** установите `POTTER_DEBUG=1` для вывода путей поиска:
```bash
POTTER_DEBUG=1 potter-gen generate --proto api/service.proto
```

## Примеры

См. `examples/codegen/` для полных примеров использования кодогенератора.

