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
import "github.com/akriventsev/potter/options.proto";

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
│   └── rest/                   # REST handlers
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

## Примеры

См. `examples/codegen/` для полных примеров использования кодогенератора.

