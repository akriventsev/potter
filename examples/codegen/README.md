# Пример использования Potter Code Generator

Этот пример демонстрирует создание полнофункционального CQRS приложения "с нуля" на основе protobuf спецификации с использованием Potter Code Generator.

## Что генерируется

Из одного proto файла (`api/simple-service.proto`) генерируется полное приложение со следующей структурой:

```
simple-service/
├── cmd/server/main.go              # Точка входа приложения
├── domain/                          # Доменный слой
│   ├── aggregates.go               # Агрегаты (Item)
│   ├── events.go                   # Доменные события
│   └── repository.go               # Интерфейсы репозиториев
├── application/                     # Application слой
│   ├── command/                    # Команды и handlers
│   │   ├── create_item.go
│   │   └── update_item.go
│   └── query/                      # Запросы и handlers
│       ├── get_item.go
│       └── list_items.go
├── infrastructure/                  # Infrastructure слой
│   ├── repository/                 # Реализации репозиториев
│   │   └── item_repository.go
│   └── cache/                      # Cache service
│       └── redis_cache.go
├── presentation/                    # Presentation слой
│   ├── rest/                       # REST API handlers
│   │   └── handler.go
│   └── graphql/                    # GraphQL адаптер (если включен)
│       └── adapter.go
├── api/                             # API определения
│   └── graphql/                    # GraphQL схема и конфигурация (если включен)
│       ├── schema.graphql
│       ├── gqlgen.yml
│       └── potter_resolvers.go
├── config/                         # Конфигурация приложения
│   └── config.go
├── migrations/                      # SQL миграции
│   └── 001_create_tables.sql
├── docker-compose.yml              # Docker Compose для инфраструктуры
├── Makefile                        # Build команды
├── go.mod                          # Go модуль
└── README.md                       # Документация проекта
```

## Предварительные требования

### Обязательные инструменты

1. **Go 1.21+** - для компиляции и запуска приложения
   ```bash
   go version  # Проверка версии
   ```

2. **Protocol Buffers Compiler (protoc)** - для парсинга proto файлов
   ```bash
   protoc --version  # Проверка установки
   ```
   
   Установка:
   - macOS: `brew install protobuf`
   - Linux: `sudo apt-get install protobuf-compiler` или `sudo yum install protobuf-compiler`
   - Windows: скачайте с [releases](https://github.com/protocolbuffers/protobuf/releases)

3. **Potter Code Generator (potter-gen)** - CLI инструмент для генерации кода
   ```bash
   # Установка из исходников
   cd /path/to/potter
   go install ./cmd/potter-gen
   
   # Проверка установки
   potter-gen version
   ```

4. **Docker и Docker Compose** - для запуска инфраструктуры (PostgreSQL, Redis, NATS)
   ```bash
   docker --version
   docker-compose --version
   ```

### Опциональные инструменты (для GraphQL)

5. **gqlgen** - для генерации GraphQL кода (если используется GraphQL транспорт)
   ```bash
   go install github.com/99designs/gqlgen@latest
   gqlgen version
   ```

## Создание проекта "с нуля"

### Шаг 1: Подготовка proto файла

Создайте директорию для вашего проекта и подготовьте proto файл с Potter аннотациями.

**Пример: `api/simple-service.proto`**

```protobuf
syntax = "proto3";
package example;

import "potter/options.proto";

option go_package = "example/api";

// SimpleService демонстрирует базовое использование Potter кодогенератора
service SimpleService {
  option (potter.service) = {
    module_name: "simple-service"  // Имя Go модуля
    transport: ["REST", "GraphQL"]  // Включенные транспорты
  };

  // CreateItem создает новый item (команда)
  rpc CreateItem(CreateItemRequest) returns (CreateItemResponse) {
    option (potter.command) = {
      aggregate: "Item"      // Связь с агрегатом
      async: true            // Асинхронное выполнение
      idempotent: true       // Идемпотентность
      timeout_seconds: 30    // Таймаут выполнения
    };
  }

  // GetItem получает item по ID (запрос)
  rpc GetItem(GetItemRequest) returns (GetItemResponse) {
    option (potter.query) = {
      cacheable: true           // Кэшируемый запрос
      cache_ttl_seconds: 300     // TTL кэша
      read_model: "ItemReadModel" // Использование read model
    };
  }

  // UpdateItem обновляет item (команда)
  rpc UpdateItem(UpdateItemRequest) returns (UpdateItemResponse) {
    option (potter.command) = {
      aggregate: "Item"
      async: false
    };
  }

  // ListItems возвращает список items (запрос)
  rpc ListItems(ListItemsRequest) returns (ListItemsResponse) {
    option (potter.query) = {
      cacheable: false
    };
  }
}

// Item aggregate - доменная сущность
message Item {
  option (potter.aggregate) = {
    name: "Item"
    repository: "postgres"  // Тип репозитория
  };

  string id = 1;
  string name = 2;
  string description = 3;
  int32 quantity = 4;
}

// ItemCreatedEvent - событие создания item
message ItemCreatedEvent {
  option (potter.event) = {
    event_type: "item.created"
    aggregate: "Item"
    version: 1
  };

  string item_id = 1;
  string name = 2;
  string description = 3;
  int32 quantity = 4;
}

// Request/Response сообщения
message CreateItemRequest {
  string name = 1;
  string description = 2;
  int32 quantity = 3;
}

message CreateItemResponse {
  string item_id = 1;
  string message = 2;
}

message GetItemRequest {
  string id = 1;
}

message GetItemResponse {
  string id = 1;
  string name = 2;
  string description = 3;
  int32 quantity = 4;
}

message UpdateItemRequest {
  string id = 1;
  string name = 2;
  string description = 3;
  int32 quantity = 4;
}

message UpdateItemResponse {
  string message = 1;
}

message ListItemsRequest {
  int32 page = 1;
  int32 page_size = 2;
}

message ListItemsResponse {
  repeated Item items = 1;
  int32 total = 2;
}
```

**Важные моменты:**

- **Импорт Potter options**: `import "potter/options.proto";` - используйте короткий путь, не полный Go module path
- **module_name**: должен соответствовать будущему Go module path (например, `simple-service`)
- **transport**: список транспортов, которые будут сгенерированы (`REST`, `GraphQL`, `NATS`, `gRPC`)

### Шаг 2: Генерация проекта

Используйте команду `potter-gen init` для создания проекта:

```bash
# Базовая команда
potter-gen init \
  --proto api/simple-service.proto \
  --module simple-service \
  --output ./simple-service

# С явным указанием пути к Potter framework (для форков/зеркал)
potter-gen init \
  --proto api/simple-service.proto \
  --module simple-service \
  --output ./simple-service \
  --potter-import-path github.com/your-fork/potter@v1.5.0
```

**Параметры команды:**

- `--proto` - путь к proto файлу (обязательный)
- `--module` - Go module path (обязательный, или укажите `module_name` в proto)
- `--output` - директория для генерации (по умолчанию: текущая директория)
- `--potter-import-path` - путь к Potter framework (по умолчанию: `github.com/akriventsev/potter`)

**Что происходит при выполнении:**

1. Парсинг proto файла и извлечение Potter аннотаций
2. Генерация всех слоев приложения:
   - Domain (агрегаты, события, репозитории)
   - Application (команды, запросы, handlers)
   - Infrastructure (репозитории, cache)
   - Presentation (REST, GraphQL адаптеры)
   - Main (точка входа, конфигурация)
3. Создание вспомогательных файлов:
   - `go.mod` - Go модуль
   - `Makefile` - команды для сборки и запуска
   - `docker-compose.yml` - инфраструктура
   - `README.md` - документация проекта
   - SQL миграции
4. Автоматическая инициализация зависимостей (если возможно)

**Примечание:** Если автоматическая инициализация зависимостей не удалась (например, при работе с форками или в изолированном окружении), выполните вручную:

```bash
cd simple-service
make deps
```

### Шаг 3: Настройка GraphQL (если включен)

Если в proto файле указан транспорт `GraphQL`, необходимо сгенерировать GraphQL код через gqlgen:

```bash
cd simple-service

# Генерация GraphQL кода
cd api/graphql
gqlgen generate
cd ../..
```

**Что генерируется:**

- `api/graphql/generated.go` - сгенерированный код резолверов
- `api/graphql/models_gen.go` - модели данных
- Обновляется `api/graphql/schema.graphql` - GraphQL схема

### Шаг 4: Реализация бизнес-логики

Сгенерированный код содержит заглушки для бизнес-логики с маркерами `USER CODE BEGIN/END`. Найдите эти секции и реализуйте логику:

**Пример: `application/command/create_item.go`**

```go
func (h *CreateItemHandler) Handle(ctx context.Context, cmd transport.Command) error {
    _, ok := cmd.(CreateItemCommand)
    if !ok {
        return fmt.Errorf("invalid command type: %T", cmd)
    }

    // USER CODE BEGIN: Validation
    // Добавьте валидацию здесь
    // Например: проверка обязательных полей, форматов и т.д.
    // USER CODE END: Validation

    // USER CODE BEGIN: BusinessLogic
    // Реализуйте бизнес-логику здесь
    // Раскомментируйте переменную команды:
    createitem := cmd.(CreateItemCommand)
    
    // Создайте агрегат
    item := domain.NewItem(
        uuid.New().String(),
        createitem.Name,
        createitem.Description,
        createitem.Quantity,
    )
    // USER CODE END: BusinessLogic

    // Раскомментируйте код ниже после создания переменной 'item'
    // Сохранение item
    if err := h.itemRepo.Save(ctx, item); err != nil {
        return fmt.Errorf("failed to save item: %w", err)
    }

    // Публикация событий
    for _, event := range item.Events() {
        if err := h.eventPublisher.Publish(ctx, event); err != nil {
            return fmt.Errorf("failed to publish event: %w", err)
        }
    }
    item.ClearEvents()

    return nil
}
```

**Аналогично для query handlers:**

```go
func (h *GetItemHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
    getitem := q.(GetItemQuery)

    // USER CODE BEGIN: LoadData
    // Загрузите данные из репозитория или read model
    item, err := h.itemRepo.FindByID(ctx, getitem.ID)
    if err != nil {
        return nil, err
    }
    // USER CODE END: LoadData

    // Маппинг в response
    response := GetItemResponse{
        ID:          item.ID(),
        Name:        item.Name(),
        Description: item.Description(),
        Quantity:    item.Quantity(),
    }

    return response, nil
}
```

### Шаг 5: Запуск инфраструктуры

Запустите необходимые сервисы через Docker Compose:

```bash
cd simple-service
make docker-up
```

Это запустит:
- **PostgreSQL** (порт 5432) - основная база данных
- **Redis** (порт 6379) - кэш
- **NATS** (порт 4222) - message bus для событий

Проверка статуса:

```bash
docker-compose ps
```

### Шаг 6: Применение миграций

Примените SQL миграции для создания таблиц:

```bash
make migrate
```

Или вручную:

```bash
# Ожидание готовности PostgreSQL
until PGPASSWORD=postgres psql -h localhost -U postgres -d postgres -c '\q' 2>/dev/null; do sleep 1; done

# Создание базы данных
PGPASSWORD=postgres psql -h localhost -U postgres -d postgres -c 'CREATE DATABASE db;' 2>/dev/null || true

# Применение миграций
PGPASSWORD=postgres psql -h localhost -U postgres -d db -f migrations/001_create_tables.sql
```

### Шаг 7: Запуск приложения

Запустите приложение:

```bash
make run
```

Или напрямую:

```bash
go run cmd/server/main.go
```

**Проверка работы:**

- **REST API**: `http://localhost:8080/api/v1/items/create_item` (POST)
- **GraphQL Playground**: `http://localhost:8082/playground` (если включен GraphQL)
- **GraphQL Endpoint**: `http://localhost:8082/graphql`

## Структура proto файла

### Основные компоненты

1. **Service Definition** - определение сервиса с Potter опциями
   ```protobuf
   service SimpleService {
     option (potter.service) = {
       module_name: "simple-service"
       transport: ["REST", "GraphQL"]
     };
   }
   ```

2. **RPC Methods** - методы сервиса с аннотациями:
   - `potter.command` - для команд (изменение состояния)
   - `potter.query` - для запросов (чтение данных)

3. **Aggregate Messages** - доменные сущности
   ```protobuf
   message Item {
     option (potter.aggregate) = {
       name: "Item"
       repository: "postgres"
     };
   }
   ```

4. **Event Messages** - доменные события
   ```protobuf
   message ItemCreatedEvent {
     option (potter.event) = {
       event_type: "item.created"
       aggregate: "Item"
       version: 1
     };
   }
   ```

5. **Request/Response Messages** - сообщения для RPC методов

### Поддерживаемые транспорты

- **REST** - REST API через Gin framework
- **GraphQL** - GraphQL API с автоматической генерацией схемы
- **NATS** - Асинхронная обработка через NATS message bus
- **gRPC** - gRPC сервер (в разработке)

### Опции команд (potter.command)

- `aggregate` - имя агрегата, с которым связана команда
- `async` - асинхронное выполнение (true/false)
- `idempotent` - идемпотентность команды
- `timeout_seconds` - таймаут выполнения

### Опции запросов (potter.query)

- `cacheable` - возможность кэширования (true/false)
- `cache_ttl_seconds` - время жизни кэша
- `read_model` - использование read model (опционально)

## Команды генерации

### Инициализация нового проекта

```bash
potter-gen init \
  --proto api/service.proto \
  --module myapp \
  --output ./myapp
```

### Регенерация кода

```bash
potter-gen generate \
  --proto api/service.proto \
  --output ./myapp \
  --overwrite
```

### Обновление существующего проекта

```bash
# Автоматическое обновление
potter-gen update \
  --proto api/service.proto \
  --output ./myapp

# Интерактивное обновление (с выбором изменений)
potter-gen update \
  --proto api/service.proto \
  --output ./myapp \
  --interactive
```

**Важно:** При обновлении пользовательский код между маркерами `USER CODE BEGIN/END` сохраняется автоматически.

### Проверка синхронности (для CI)

```bash
# Проверка без применения изменений
potter-gen check \
  --proto api/service.proto \
  --output ./myapp

# Команда завершится с exit code 1, если есть расхождения
```

**Использование в CI:**

```yaml
# .github/workflows/ci.yml
- name: Check codegen sync
  run: |
    potter-gen check --proto api/service.proto --output ./myapp
```

### Генерация SDK

```bash
potter-gen sdk \
  --proto api/service.proto \
  --output ./myapp-sdk \
  --module myapp-sdk
```

## Что делать после генерации

### 1. Реализация бизнес-логики

Найдите все секции `USER CODE BEGIN/END` и реализуйте:

- **Валидацию** - проверка входных данных
- **Бизнес-логику** - создание/изменение агрегатов
- **Загрузку данных** - для query handlers

### 2. Настройка конфигурации

Отредактируйте `config/config.go` или используйте переменные окружения:

```bash
export SERVER_PORT=8080
export DATABASE_DSN=postgres://user:pass@localhost:5432/db
export REDIS_ADDR=localhost:6379
export NATS_URL=nats://localhost:4222
```

### 3. Генерация GraphQL кода (если используется)

```bash
cd api/graphql
gqlgen generate
cd ../..
```

### 4. Запуск и тестирование

```bash
# Запуск инфраструктуры
make docker-up

# Применение миграций
make migrate

# Запуск приложения
make run

# Тестирование REST API
curl -X POST http://localhost:8080/api/v1/items/create_item \
  -H "Content-Type: application/json" \
  -d '{"name": "Test Item", "description": "Test", "quantity": 10}'

# Тестирование GraphQL (если включен)
curl -X POST http://localhost:8082/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "query { getItem(id: \"1\") { id name } }"}'
```

## Структура сгенерированного кода

### Domain Layer (`domain/`)

- **aggregates.go** - доменные агрегаты с методами бизнес-логики
- **events.go** - доменные события
- **repository.go** - интерфейсы репозиториев

### Application Layer (`application/`)

- **command/** - команды и их handlers
  - Каждая команда имеет свой файл: `create_item.go`, `update_item.go`
  - Handlers содержат заглушки для бизнес-логики
- **query/** - запросы и их handlers
  - Каждый запрос имеет свой файл: `get_item.go`, `list_items.go`
  - Поддержка кэширования для cacheable queries

### Infrastructure Layer (`infrastructure/`)

- **repository/** - реализации репозиториев
  - PostgreSQL репозитории с поддержкой кэширования
  - Методы: Save, FindByID, Delete
- **cache/** - Redis cache service

### Presentation Layer (`presentation/`)

- **rest/** - REST API handlers (Gin)
  - Автоматическая маршрутизация команд и запросов
  - Endpoints: `/api/v1/{resource}/{action}`
- **graphql/** - GraphQL адаптер (если включен)
  - Интеграция с CQRS (CommandBus, QueryBus, EventBus)
  - Автоматическая регистрация резолверов

### Main (`cmd/server/main.go`)

- Инициализация всех компонентов
- Настройка транспортов (REST, GraphQL)
- Graceful shutdown
- Интеграция метрик (OpenTelemetry)

## Примеры использования API

### REST API

**Создание item:**
```bash
curl -X POST http://localhost:8080/api/v1/items/create_item \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Laptop",
    "description": "Gaming laptop",
    "quantity": 5
  }'
```

**Получение item:**
```bash
curl http://localhost:8080/api/v1/items/get_item?id=item-123
```

**Обновление item:**
```bash
curl -X POST http://localhost:8080/api/v1/items/update_item \
  -H "Content-Type: application/json" \
  -d '{
    "id": "item-123",
    "name": "Updated Laptop",
    "description": "Updated description",
    "quantity": 10
  }'
```

**Список items:**
```bash
curl "http://localhost:8080/api/v1/items/list_items?page=1&page_size=10"
```

### GraphQL API (если включен)

**Query:**
```graphql
query {
  getItem(id: "item-123") {
    id
    name
    description
    quantity
  }
  
  listItems(page: 1, pageSize: 10) {
    items {
      id
      name
    }
    total
  }
}
```

**Mutation:**
```graphql
mutation {
  createItem(input: {
    name: "Laptop"
    description: "Gaming laptop"
    quantity: 5
  }) {
    itemId
    message
  }
}
```

**Subscription:**
```graphql
subscription {
  itemCreatedEvent {
    itemId
    name
    quantity
  }
}
```

## Troubleshooting

### Ошибка: "Import potter/options.proto was not found"

**Проблема:** protoc не может найти файл `potter/options.proto`.

**Решение:**

1. Убедитесь, что используете короткий импорт: `import "potter/options.proto";`
2. Проверьте, что Potter framework доступен:
   ```bash
   # Если используете локальную разработку
   export POTTER_PROTO_PATH=/path/to/potter/api/proto
   
   # Или установите через go modules
   go get github.com/akriventsev/potter@main
   ```

### Ошибка: "Failed to initialize Go modules automatically"

**Проблема:** Автоматическая инициализация зависимостей не удалась.

**Решение:**

```bash
cd simple-service
make deps

# Или вручную
go get github.com/akriventsev/potter@main
go mod tidy
```

### Ошибка: "executableSchema is not initialized"

**Проблема:** GraphQL транспорт включен, но схема не сгенерирована.

**Решение:**

```bash
cd api/graphql
gqlgen generate
cd ../..
```

### Ошибка компиляции: несовместимые версии зависимостей

**Проблема:** Версии зависимостей не совместимы.

**Решение:**

```bash
cd simple-service
go mod tidy
go get -u ./...
```

### Ошибка: "cannot connect to database"

**Проблема:** PostgreSQL не запущен или недоступен.

**Решение:**

```bash
# Проверка статуса
docker-compose ps

# Запуск инфраструктуры
make docker-up

# Проверка подключения
PGPASSWORD=postgres psql -h localhost -U postgres -d postgres -c '\q'
```

## Дополнительные ресурсы

- [Potter Code Generator Documentation](../../framework/codegen/README.md) - полная документация кодогенератора
- [Potter Custom Options](../../api/proto/potter/options.proto) - описание всех доступных опций
- [Potter Framework Documentation](../../README.md) - общая документация фреймворка
- [GraphQL Transport Documentation](../../framework/adapters/transport/GRAPHQL.md) - документация GraphQL транспорта

## Примеры использования

Этот пример демонстрирует:

- ✅ Создание проекта из proto файла
- ✅ Генерация всех слоев приложения
- ✅ Поддержка REST и GraphQL транспортов
- ✅ Интеграция с PostgreSQL, Redis, NATS
- ✅ Кэширование запросов
- ✅ Публикация событий
- ✅ Graceful shutdown
- ✅ Метрики OpenTelemetry

## Следующие шаги

После успешной генерации и запуска:

1. **Реализуйте бизнес-логику** в секциях `USER CODE BEGIN/END`
2. **Добавьте валидацию** входных данных
3. **Настройте конфигурацию** через переменные окружения
4. **Добавьте тесты** для handlers
5. **Настройте CI/CD** для автоматической генерации и проверки

## Поддержка

Если у вас возникли проблемы:

1. Проверьте [Troubleshooting](#troubleshooting) секцию выше
2. Убедитесь, что все предварительные требования установлены
3. Проверьте логи приложения и инфраструктуры
4. Создайте issue в репозитории проекта
