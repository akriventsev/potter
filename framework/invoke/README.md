# Invoke Module

Модуль `invoke` предоставляет generic-based API для type-safe работы с CQRS командами и запросами. Модуль реализует чистый produce/consume паттерн для асинхронной отправки команд с ожиданием результирующих событий через correlation ID.

## Архитектура

Модуль Invoke использует чистый produce/consume паттерн без request-reply overhead:

1. **CommandInvoker[TCmd, TSuccessEvent, TErrorEvent]** - публикует команду в NATS (produce), ожидает успешное или ошибочное событие по correlation ID (consume)
2. **QueryInvoker[TQuery, TResult]** - type-safe обертка над QueryBus с автоматическим приведением типов
3. **EventAwaiter** - подписывается на события по correlation ID с timeout
4. **AsyncCommandBus** - чистый producer команд в NATS pub/sub

### Produce/Consume Flow

```
Client -> CommandInvoker -> AsyncCommandBus -> NATS Pub/Sub (produce command)
                                                      |
                                                      v
CommandHandler <- NATS Subscribe (consume command) -> EventPublisher -> EventBus
                                                      |
                                                      v
EventAwaiter <- EventBus (consume event) <- Match correlation ID -> Client
```

## Основные компоненты

### CommandInvoker

Generic invoker для команд с ожиданием событий по correlation ID.

**Быстрый конструктор (для простых случаев):**
```go
// Создание invoker с поддержкой ошибочных событий
asyncBus := invoke.NewAsyncCommandBus(natsAdapter)
awaiter := invoke.NewEventAwaiterFromEventBus(eventBus)
invoker := invoke.NewCommandInvoker[CreateProductCommand, ProductCreatedEvent, ProductCreationFailedEvent](
    asyncBus,
    awaiter,
    "product.created",    // тип успешного события
    "product.creation_failed", // тип ошибочного события
)
```

**Options-based конструктор (рекомендуется для сложных сценариев):**
```go
// Создание invoker с полной настройкой через опции
asyncBus := invoke.NewAsyncCommandBus(natsAdapter)
invoker, err := invoke.NewCommandInvokerWithOptions[
    CreateProductCommand,
    ProductCreatedEvent,
    ProductCreationFailedEvent,
](
    asyncBus,
    invoke.WithEventBus(eventBus),
    invoke.WithSuccessEventType("product.created"),
    invoke.WithErrorEventType("product.creation_failed"),
    invoke.WithTimeout(60*time.Second),
    invoke.WithSubjectResolver(customResolver),
)
if err != nil {
    log.Fatal(err)
}
```

// Использование
cmd := CreateProductCommand{Name: "Laptop", SKU: "LAP-001"}
event, err := invoker.Invoke(ctx, cmd)
if err != nil {
    // обработка ошибки (может быть из ErrorEvent)
    var frameworkErr *core.FrameworkError
    if errors.As(err, &frameworkErr) && frameworkErr.Code == invoke.ErrErrorEventReceived {
        // получено ошибочное событие
    }
}
// event имеет тип ProductCreatedEvent

// Для обратной совместимости (без ошибочных событий)
invokerWithoutError := invoke.NewCommandInvokerWithoutError[CreateProductCommand, ProductCreatedEvent](
    asyncBus,
    awaiter,
    "product.created",
)
```

**Методы:**
- `Invoke(ctx, cmd)` - синхронное выполнение с ожиданием успешного или ошибочного события
- `InvokeAsync(ctx, cmd)` - асинхронное выполнение, возвращает канал с результатом
- `InvokeWithBothResults(ctx, cmd)` - возвращает оба типа событий для детального анализа
- `WithTimeout(timeout)` - установка таймаута ожидания
- `WithSerializer(serializer)` - установка сериализатора

### QueryInvoker

Generic type-safe обертка над QueryBus.

**Быстрый конструктор (для простых случаев):**
```go
// Создание invoker
invoker := invoke.NewQueryInvoker[GetProductQuery, GetProductResponse](queryBus)
```

**Options-based конструктор (рекомендуется для сложных сценариев):**
```go
// Создание invoker с полной настройкой через опции
invoker := invoke.NewQueryInvokerWithOptions[GetProductQuery, GetProductResponse](
    queryBus,
    invoke.WithTimeout(30*time.Second),
    invoke.WithMetadata(map[string]interface{}{
        "trace_id": traceID,
    }),
)
```

// Использование
query := GetProductQuery{ID: "product-123"}
result, err := invoker.Invoke(ctx, query)
if err != nil {
    // обработка ошибки
}
// result имеет тип GetProductResponse
```

**Методы:**
- `Invoke(ctx, query)` - выполнение запроса с type assertion
- `InvokeWithMetadata(ctx, query, metadata)` - выполнение с метаданными
- `InvokeBatch(ctx, queries)` - пакетное выполнение
- `WithCache(cache)` - установка кэша
- `WithTimeout(timeout)` - установка таймаута
- `WithValidator(validator)` - установка валидатора результата

### EventAwaiter

Consumer событий по correlation ID с timeout.

```go
// Создание awaiter из EventBus
awaiter := invoke.NewEventAwaiterFromEventBus(eventBus)

// Создание awaiter из transport.Subscriber (NATS, Kafka, Redis)
subjectResolver := invoke.NewDefaultSubjectResolver("commands", "events")
awaiter := invoke.NewEventAwaiterFromTransport(
    natsSubscriber,
    invoke.NewJSONSerializer(),
    subjectResolver,
)

// Ожидание события
event, err := awaiter.Await(ctx, correlationID, "product_created", 30*time.Second)

// Ожидание любого из нескольких событий (первое полученное)
event, receivedType, err := awaiter.AwaitAny(ctx, correlationID, []string{"success", "error"}, 30*time.Second)

// Ожидание успешного или ошибочного события
event, isSuccess, err := awaiter.AwaitSuccessOrError(ctx, correlationID, "product.created", "product.creation_failed", 30*time.Second)

// Ожидание нескольких событий
events, err := awaiter.AwaitMultiple(ctx, correlationID, []string{"event1", "event2"}, 30*time.Second)

// Отмена ожидания
awaiter.Cancel(correlationID)

// Остановка awaiter
awaiter.Stop(ctx)
```

### AsyncCommandBus

Чистый producer команд для NATS pub/sub.

```go
// Создание bus с дефолтным SubjectResolver
asyncBus := invoke.NewAsyncCommandBus(natsAdapter)
asyncBus.WithSubjectPrefix("commands")
asyncBus.WithSerializer(invoke.NewJSONSerializer())

// Создание bus с кастомным SubjectResolver
customResolver := invoke.NewFunctionSubjectResolver(
    func(cmd transport.Command) string {
        return fmt.Sprintf("cmd.%s", cmd.CommandName())
    },
    func(eventType string) string {
        return fmt.Sprintf("evt.%s", eventType)
    },
)
asyncBus.WithSubjectResolver(customResolver)

// Создание bus со статическим маппингом subjects
staticResolver := invoke.NewStaticSubjectResolver(
    map[string]string{
        "create_product": "commands.product.create",
        "update_product": "commands.product.update",
    },
    map[string]string{
        "product.created": "events.product.created",
        "product.updated": "events.product.updated",
    },
)
asyncBus.WithSubjectResolver(staticResolver)

// Публикация команды
metadata := invoke.CreateMetadataFromContext(ctx)
err := asyncBus.SendAsync(ctx, cmd, metadata)
```

### Correlation ID утилиты

```go
// Генерация ID
correlationID := invoke.GenerateCorrelationID()
commandID := invoke.GenerateCommandID()

// Работа с контекстом
ctx = invoke.WithCorrelationID(ctx, correlationID)
correlationID := invoke.ExtractCorrelationID(ctx)

// Создание метаданных из контекста
metadata := invoke.CreateMetadataFromContext(ctx)
```

## Примеры использования

Полноценные рабочие примеры доступны в директории [`examples/`](./examples/).

### Commands

- **[NATS](./examples/command_nats_example.go)** - Fire-and-await с EventBus
  - Демонстрирует fire-and-await паттерн с EventBus
  - Использование `CommandInvoker` с NATS транспортом
  - Обработка успешных и ошибочных событий
  - Распространение correlation ID через контекст
  - Запуск: `make test-command-nats` или `go test -v -run ExampleCommandInvokerWithNATS`

- **[Kafka](./examples/command_kafka_example.go)** - Pub/Sub с error events
  - Kafka pub/sub для команд и событий
  - Использование `KafkaEventAdapter` для публикации событий
  - Настройка compression, idempotent writes, partitioning
  - Обработка DLQ для failed messages
  - Запуск: `make test-command-kafka` или `go test -v -run ExampleCommandInvokerWithKafka`

### Queries

- **[NATS](./examples/query_nats_example.go)** - Request-Reply
  - NATS Request-Reply паттерн
  - Использование `QueryInvoker` с NATS транспортом
  - Пакетные запросы через `InvokeBatch()`
  - Валидация результатов
  - Запуск: `make test-query-nats` или `go test -v -run ExampleQueryInvokerWithNATS`

- **[Kafka](./examples/query_kafka_example.go)** - Request-Reply с correlation ID
  - Kafka Request-Reply с correlation ID
  - Временные reply topics
  - Consumer groups для масштабирования
  - Запуск: `make test-query-kafka` или `go test -v -run ExampleQueryInvokerWithKafka`

- **[REST](./examples/query_rest_example.go)** - HTTP endpoints
  - HTTP REST endpoints с Gin
  - Интеграция с `QueryBus`
  - Query parameters, headers, authentication
  - Content negotiation (JSON/XML)
  - Запуск: `make test-query-rest` или `go test -v -run ExampleQueryInvokerWithREST`

- **[gRPC](./examples/query_grpc_example.go)** - gRPC services
  - gRPC services с protobuf
  - Интеграция с `QueryBus`
  - Server-side и client-side streaming
  - gRPC status codes и metadata
  - Запуск: `make test-query-grpc` или `go test -v -run ExampleQueryInvokerWithGRPC`

### Advanced

- **[Mixed Transports](./examples/mixed_transports_example.go)** - Комбинация транспортов
  - Использование разных транспортов одновременно:
    - NATS для команд (легковесные, быстрые)
    - Kafka для событий (высокая пропускная способность)
    - REST для синхронных запросов (публичный API)
    - gRPC для внутренних запросов (производительность)
  - Разные SubjectResolver для каждого транспорта
  - Единый EventBus для координации
  - Запуск: `make test-mixed` или `go test -v -run ExampleMixedTransports`

Подробные инструкции см. в [examples/README.md](./examples/README.md)

### Концептуальный пример

Для быстрого понимания концепции:

```go
// 1. Инициализация
natsAdapter, _ := messagebus.NewNATSAdapter("nats://localhost:4222")
natsAdapter.Start(ctx)

eventBus := events.NewInMemoryEventBus()
asyncBus := invoke.NewAsyncCommandBus(natsAdapter)
awaiter := invoke.NewEventAwaiterFromEventBus(eventBus)

// 2. Создание invoker с поддержкой ошибочных событий
invoker := invoke.NewCommandInvoker[CreateProductCommand, ProductCreatedEvent, ProductCreationFailedEvent](
    asyncBus,
    awaiter,
    "product.created",
    "product.creation_failed",
).WithTimeout(30 * time.Second)

// 3. Использование
cmd := CreateProductCommand{
    Name: "Laptop",
    SKU:  "LAP-001",
}

event, err := invoker.Invoke(ctx, cmd)
if err != nil {
    log.Printf("Error: %v", err)
    return
}

log.Printf("Product created: %s", event.ProductID())
```

### Пример с запросом

```go
// 1. Инициализация
queryBus := transport.NewInMemoryQueryBus()
handler := query.NewGetProductHandler(productRepo)
queryBus.Register(handler)

// 2. Создание invoker
invoker := invoke.NewQueryInvoker[GetProductQuery, GetProductResponse](queryBus)

// 3. Использование
query := GetProductQuery{ID: "product-123"}
result, err := invoker.Invoke(ctx, query)
if err != nil {
    log.Printf("Error: %v", err)
    return
}

log.Printf("Product: %+v", result)
```

### Интеграция с handler

В command handler нужно извлекать correlation ID и добавлять его в события:

```go
func (h *CreateProductHandler) Handle(ctx context.Context, cmd transport.Command) error {
    createCmd := cmd.(CreateProductCommand)
    
    // Извлекаем correlation ID из контекста
    correlationID := invoke.ExtractCorrelationID(ctx)
    
    // Создаем продукт
    product := domain.NewProduct(createCmd.Name, createCmd.Description, createCmd.SKU)
    
    // Сохраняем
    if err := h.productRepo.Save(ctx, product); err != nil {
        return err
    }
    
    // Публикуем события с correlation ID
    for _, event := range product.Events() {
        if baseEvent, ok := event.(*events.BaseEvent); ok {
            baseEvent.WithCorrelationID(correlationID)
        }
        if err := h.eventPublisher.Publish(ctx, event); err != nil {
            return err
        }
    }
    
    return nil
}
```

## Subject Resolution

Модуль поддерживает гибкую настройку subjects для команд и событий через интерфейс `SubjectResolver`.

### DefaultSubjectResolver

Использует префиксы для формирования subjects:

```go
resolver := invoke.NewDefaultSubjectResolver("commands", "events")
// Команда "create_product" -> "commands.create_product"
// Событие "product.created" -> "events.product.created"
```

### FunctionSubjectResolver

Позволяет задать кастомную логику для определения subjects:

```go
resolver := invoke.NewFunctionSubjectResolver(
    func(cmd transport.Command) string {
        // Маршрутизация по типу агрегата
        if strings.HasPrefix(cmd.CommandName(), "product_") {
            return fmt.Sprintf("product.commands.%s", cmd.CommandName())
        }
        return fmt.Sprintf("commands.%s", cmd.CommandName())
    },
    func(eventType string) string {
        return fmt.Sprintf("events.%s", eventType)
    },
)
```

### StaticSubjectResolver

Использует статический маппинг для subjects:

```go
resolver := invoke.NewStaticSubjectResolver(
    map[string]string{
        "create_product": "commands.product.create",
        "update_product": "commands.product.update",
    },
    map[string]string{
        "product.created": "events.product.created",
        "product.updated": "events.product.updated",
    },
)
```

## Error Handling

Модуль Invoke возвращает все ошибки как экземпляры `core.FrameworkError` с определенными кодами ошибок. Это обеспечивает единообразную обработку ошибок и возможность pattern-matching по кодам.

### Коды ошибок

Модуль определяет следующие коды ошибок в `framework/invoke/errors.go`:

- `ErrEventTimeout` - таймаут ожидания события
- `ErrInvalidResultType` - неверный тип результата запроса
- `ErrCommandPublishFailed` - ошибка публикации команды
- `ErrValidationFailed` - ошибка валидации
- `ErrQueryTimeout` - таймаут выполнения запроса
- `ErrCorrelationIDNotFound` - отсутствует correlation ID в контексте
- `ErrEventAwaiterStopped` - EventAwaiter остановлен
- `ErrInvalidSubjectResolver` - некорректный SubjectResolver
- `ErrEventSourceNotConfigured` - источник событий не настроен
- `ErrErrorEventReceived` - получено ошибочное событие

### Обработка ошибок

Все ошибки возвращаются как `*core.FrameworkError` с определенным кодом. Для проверки типа ошибки используйте pattern-matching по коду:

```go
import (
    "errors"
    "potter/framework/core"
    "potter/framework/invoke"
)

// ...

event, err := invoker.Invoke(ctx, cmd)
if err != nil {
    var frameworkErr *core.FrameworkError
    if errors.As(err, &frameworkErr) {
        switch frameworkErr.Code {
        case invoke.ErrEventTimeout:
            log.Printf("Event timeout: %v", frameworkErr)
        case invoke.ErrErrorEventReceived:
            log.Printf("Error event received: %v", frameworkErr)
            // Можно извлечь детали из wrapped error
            if cause := frameworkErr.Unwrap(); cause != nil {
                log.Printf("Cause: %v", cause)
            }
        case invoke.ErrCommandPublishFailed:
            log.Printf("Failed to publish command: %v", frameworkErr)
        default:
            log.Printf("Unknown error: %v", frameworkErr)
        }
    } else {
        // Не FrameworkError - неожиданная ошибка
        log.Printf("Unexpected error type: %v", err)
    }
}
```

**Примечание:** Используйте `errors.As()` из стандартной библиотеки `errors` для безопасного извлечения `FrameworkError` из цепочки ошибок. Это предпочтительнее прямого type assertion, так как работает с wrapped errors.

### Вспомогательные конструкторы

Для создания ошибок используйте конструкторы из `framework/invoke/errors.go`:

```go
// Создание ошибки таймаута
err := invoke.NewEventTimeoutError(correlationID, "30s")

// Создание ошибки с оберткой
err := invoke.NewCommandPublishFailedError("create_product", originalErr)

// Создание ошибки валидации
err := invoke.NewValidationFailedError(validationErr)
```

Все конструкторы возвращают `*core.FrameworkError` с соответствующим кодом и сообщением.

## Error Events

Модуль поддерживает обработку событий об ошибках через интерфейс `ErrorEvent`.

### Создание событий об ошибках

```go
// Использование BaseErrorEvent
errorEvent := invoke.NewBaseErrorEvent(
    "product.creation_failed",
    productID,
    "VALIDATION_ERROR",
    "Product SKU already exists",
    err,
    false, // не повторяемая
).WithOriginalCommand(cmd)

// Кастомное событие об ошибке
type ProductCreationFailedEvent struct {
    *invoke.BaseErrorEvent
    SKU    string
    Reason string
}

func NewProductCreationFailedEvent(sku, reason string, err error) *ProductCreationFailedEvent {
    return &ProductCreationFailedEvent{
        BaseErrorEvent: invoke.NewBaseErrorEvent(
            "product.creation_failed",
            "",
            "VALIDATION_ERROR",
            reason,
            err,
            false,
        ),
        SKU:    sku,
        Reason: reason,
    }
}
```

### Использование в CommandInvoker

```go
invoker := invoke.NewCommandInvoker[
    CreateProductCommand,
    ProductCreatedEvent,
    ProductCreationFailedEvent,
](
    asyncBus,
    awaiter,
    "product.created",
    "product.creation_failed",
)

event, err := invoker.Invoke(ctx, cmd)
if err != nil {
    // Проверяем, является ли это ошибочным событием
    var frameworkErr *core.FrameworkError
    if errors.As(err, &frameworkErr) && frameworkErr.Code == invoke.ErrErrorEventReceived {
        // Получено ошибочное событие
        // Можно извлечь детали из wrapped error
        if cause := frameworkErr.Unwrap(); cause != nil {
            log.Printf("Error event cause: %v", cause)
        }
    }
}

// Или используем InvokeWithBothResults для детального анализа
success, errorEvent, err := invoker.InvokeWithBothResults(ctx, cmd)
if errorEvent != nil {
    log.Printf("Error: %s (retryable: %v)", errorEvent.ErrorMessage(), errorEvent.IsRetryable())
}
```

## Transport Flexibility

Модуль поддерживает работу с различными транспортами через адаптеры.

**См. примеры в [`examples/`](./examples/) для практических демонстраций использования каждого транспорта.**

### Использование NATS для событий

```go
natsSubscriber := natsAdapter // transport.Subscriber
subjectResolver := invoke.NewDefaultSubjectResolver("commands", "events")
awaiter := invoke.NewEventAwaiterFromTransport(
    natsSubscriber,
    invoke.NewJSONSerializer(),
    subjectResolver,
)
```

### Использование Kafka для событий

```go
kafkaSubscriber := kafkaAdapter // transport.Subscriber
subjectResolver := invoke.NewDefaultSubjectResolver("commands", "events")
awaiter := invoke.NewEventAwaiterFromTransport(
    kafkaSubscriber,
    invoke.NewJSONSerializer(),
    subjectResolver,
)
```

### Использование разных транспортов для команд и событий

```go
// NATS для команд
natsPublisher := natsAdapter
commandBus := invoke.NewAsyncCommandBus(natsPublisher)

// Kafka для событий
kafkaSubscriber := kafkaAdapter
eventAwaiter := invoke.NewEventAwaiterFromTransport(
    kafkaSubscriber,
    invoke.NewJSONSerializer(),
    subjectResolver,
)
```

## Migration Guide

### Миграция с версии 1.0.x

#### 1. Обновление CommandInvoker

**Было:**
```go
invoker := invoke.NewCommandInvoker[CreateProductCommand, ProductCreatedEvent](
    asyncBus,
    awaiter,
    "product_created",
)
```

**Стало (с поддержкой ошибок):**
```go
invoker := invoke.NewCommandInvoker[CreateProductCommand, ProductCreatedEvent, ProductCreationFailedEvent](
    asyncBus,
    awaiter,
    "product.created",
    "product.creation_failed",
)
```

**Или (без ошибок, для обратной совместимости):**
```go
invoker := invoke.NewCommandInvokerWithoutError[CreateProductCommand, ProductCreatedEvent](
    asyncBus,
    awaiter,
    "product.created",
)
```

#### 2. Обновление EventAwaiter

**Было:**
```go
awaiter := invoke.NewEventAwaiter(eventBus)
```

**Стало:**
```go
awaiter := invoke.NewEventAwaiterFromEventBus(eventBus)
```

**Или с transport.Subscriber:**
```go
awaiter := invoke.NewEventAwaiterFromTransport(
    natsSubscriber,
    invoke.NewJSONSerializer(),
    subjectResolver,
)
```

#### 3. Обновление AsyncCommandBus

**Было:**
```go
asyncBus := invoke.NewAsyncCommandBus(natsAdapter)
asyncBus.WithSubjectPrefix("commands")
```

**Стало (совместимо, но можно использовать SubjectResolver):**
```go
asyncBus := invoke.NewAsyncCommandBus(natsAdapter)
asyncBus.WithSubjectPrefix("commands") // все еще работает

// Или с кастомным resolver
resolver := invoke.NewDefaultSubjectResolver("commands", "events")
asyncBus.WithSubjectResolver(resolver)
```

## Best Practices

1. **Используйте типизированные invokers** - избегайте `interface{}`, используйте generics для type safety
2. **Устанавливайте разумные таймауты** - по умолчанию 30s для команд, 10s для запросов
3. **Всегда добавляйте correlation ID в события** - это необходимо для матчинга событий с командами
4. **Используйте события об ошибках** - публикуйте `ErrorEvent` вместо возврата ошибок из handlers для лучшей трассируемости
5. **Выбирайте подходящий транспорт** - используйте NATS для легковесных сценариев, Kafka для высокой пропускной способности
6. **Настраивайте subjects через SubjectResolver** - используйте кастомные резолверы для сложной маршрутизации
7. **Используйте метрики** - AsyncCommandBus поддерживает интеграцию с metrics
8. **Graceful shutdown** - всегда вызывайте `awaiter.Stop(ctx)` при завершении приложения

**См. примеры в [`examples/`](./examples/) для практических демонстраций best practices.**

## Troubleshooting

**Подробные решения типичных проблем см. в [examples/README.md#troubleshooting](./examples/README.md#troubleshooting).**

### Событие не получено (timeout)

**Причины:**
- Handler не публикует событие с correlation ID
- Событие публикуется с другим correlation ID
- EventBus не доставляет события к EventAwaiter

**Решение:**
1. Проверьте, что handler извлекает correlation ID из контекста
2. Убедитесь, что событие публикуется с правильным correlation ID
3. Проверьте подписку EventAwaiter на тип события

### Неверный тип результата

**Причины:**
- QueryHandler возвращает неправильный тип
- Type assertion в QueryInvoker не проходит

**Решение:**
1. Убедитесь, что QueryHandler возвращает правильный тип
2. Проверьте generic параметры QueryInvoker

### Команда не публикуется

**Причины:**
- NATS недоступен
- Неправильный subject prefix
- Ошибка сериализации

**Решение:**
1. Проверьте подключение к NATS
2. Проверьте subject prefix в AsyncCommandBus
3. Убедитесь, что команда сериализуется корректно

## Интеграция с существующим кодом

Модуль Invoke полностью совместим с существующим кодом:

- Использует существующие `transport.Command`, `transport.Query`, `events.Event`
- Интегрируется с существующими `EventBus`, `QueryBus`
- Не требует изменений в существующих handlers (кроме добавления correlation ID в события)

## Производительность

- **Produce/consume паттерн** - нет overhead от request-reply
- **Type safety** - compile-time проверка типов через generics
- **Асинхронность** - неблокирующая публикация команд
- **Метрики** - автоматическая инструментация всех операций

