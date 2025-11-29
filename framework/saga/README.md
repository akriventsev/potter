# Saga Pattern Module

Полная реализация Saga Pattern через FSM для оркестрации долгоживущих транзакций с автоматической компенсацией.

## Содержание

1. [Введение](#введение)
2. [Архитектура](#архитектура)
3. [Quick Start](#quick-start)
4. [Core Concepts](#core-concepts)
5. [Persistence](#persistence)
6. [Integration](#integration)
7. [Advanced Topics](#advanced-topics)
8. [Best Practices](#best-practices)
9. [API Reference](#api-reference)
10. [Examples](#examples)
11. [Troubleshooting](#troubleshooting)

## Введение

### Что такое Saga Pattern?

Saga Pattern - это паттерн для управления долгоживущими транзакциями в распределенных системах. Вместо традиционных ACID транзакций, Saga разбивает транзакцию на последовательность локальных транзакций, каждая из которых может быть компенсирована при ошибке.

### Преимущества

- ✅ **Распределенные транзакции** - координация между несколькими сервисами
- ✅ **Автоматическая компенсация** - откат изменений при ошибках
- ✅ **Долгоживущие транзакции** - поддержка транзакций, длящихся минуты или часы
- ✅ **Гибкость** - поддержка параллельного выполнения, условных шагов
- ✅ **Восстановление** - возможность возобновления после сбоя

### Когда использовать Saga?

- Распределенные транзакции между микросервисами
- Долгоживущие бизнес-процессы (заказы, платежи, доставка)
- Когда нужна компенсация вместо rollback
- Интеграция с внешними системами

## Архитектура

```mermaid
graph TB
    A[SagaOrchestrator] --> B[Saga Definition]
    B --> C[SagaStep 1]
    B --> D[SagaStep 2]
    B --> E[SagaStep 3]
    
    C --> F[Execute]
    C --> G[Compensate]
    
    F --> H{Success?}
    H -->|Yes| D
    H -->|No| I[Compensate All]
    
    A --> J[SagaPersistence]
    A --> K[EventBus]
    A --> L[CommandBus]
    
    B --> M[FSM]
    M --> N[State Transitions]
    
    J --> O[EventStore]
    J --> P[Snapshots]
```

### Последовательность выполнения

```mermaid
sequenceDiagram
    participant Client
    participant Orchestrator
    participant Saga
    participant Step1
    participant Step2
    participant Step3
    participant Persistence
    participant EventBus

    Client->>Orchestrator: Execute(saga)
    Orchestrator->>Persistence: Save(saga)
    Orchestrator->>Saga: Execute()
    Saga->>Step1: Execute(ctx, sagaCtx)
    Step1->>EventBus: Publish StepStarted
    Step1-->>Saga: Success
    Saga->>Persistence: Save(saga)
    Saga->>Step2: Execute(ctx, sagaCtx)
    Step2->>EventBus: Publish StepStarted
    Step2-->>Saga: Error
    Saga->>Step2: Compensate()
    Saga->>Step1: Compensate()
    Saga->>Persistence: Save(saga)
    Saga->>EventBus: Publish SagaCompensated
    Saga-->>Orchestrator: Error
    Orchestrator-->>Client: Error
```

### Компоненты

- **Saga** - экземпляр выполняющейся саги
- **SagaDefinition** - определение саги с шагами
- **SagaStep** - атомарная единица работы с forward/compensate actions
- **SagaOrchestrator** - координация выполнения через FSM
- **SagaPersistence** - сохранение состояния для восстановления
- **SagaContext** - передача данных между шагами

## Quick Start

### Создание простой саги

```go
package main

import (
    "context"
    "github.com/akriventsev/potter/framework/saga"
    "github.com/akriventsev/potter/framework/events"
    "github.com/akriventsev/potter/framework/transport"
)

func main() {
    // Создание persistence и event bus
    persistence := saga.NewInMemoryPersistence()
    eventBus := events.NewInMemoryEventBus()
    // commandBus должен быть создан отдельно

    // Определение саги
    sagaDef := saga.NewSagaBuilder("order_saga").
        AddStep(
            saga.NewCommandStep(
                "reserve_inventory",
                commandBus,
                ReserveInventoryCommand{ProductID: "123", Quantity: 10},
                ReleaseInventoryCommand{ProductID: "123", Quantity: 10},
            ).WithTimeout(30*time.Second).
              WithRetry(saga.ExponentialBackoff(3, 1*time.Second, 2.0)),
        ).
        AddStep(
            saga.NewCommandStep(
                "process_payment",
                commandBus,
                ProcessPaymentCommand{Amount: 1000},
                RefundPaymentCommand{Amount: 1000},
            ).WithTimeout(30*time.Second),
        ).
        WithPersistence(persistence).
        WithEventBus(eventBus).
        Build()

    // Создание orchestrator
    orchestrator := saga.NewDefaultOrchestrator(persistence, eventBus)

    // Создание и выполнение саги
    ctx := context.Background()
    sagaCtx := saga.NewSagaContext()
    sagaCtx.Set("order_id", "order-123")
    
    instance := sagaDef.CreateInstance(ctx, sagaCtx)
    err := orchestrator.Execute(ctx, instance)
    if err != nil {
        // Обработка ошибки - компенсация выполнится автоматически
        log.Printf("Saga failed: %v", err)
    }
}
```

## Core Concepts

### SagaStep

Шаг саги с forward и compensate actions:

```go
step := saga.NewBaseStep("reserve_inventory").
    WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
        // Forward action
        productID := sagaCtx.GetString("product_id")
        quantity := sagaCtx.GetInt("quantity")
        return reserveInventory(ctx, productID, quantity)
    }).
    WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
        // Compensate action
        productID := sagaCtx.GetString("product_id")
        quantity := sagaCtx.GetInt("quantity")
        return releaseInventory(ctx, productID, quantity)
    }).
    WithTimeout(30*time.Second).
    WithRetry(saga.ExponentialBackoff(3, 1*time.Second, 2.0))
```

### Типы шагов

- **CommandStep** - выполнение команды через CommandBus
- **EventStep** - публикация события через EventBus
- **TwoPhaseCommitStep** - интеграция с 2PC координатором
- **ParallelStep** - параллельное выполнение нескольких шагов
- **ConditionalStep** - условное выполнение на основе контекста

### Retry Policies

```go
// Без повторов
noRetry := saga.NoRetry()

// Простые повторы
simpleRetry := saga.SimpleRetry(3)

// Exponential backoff
expBackoff := saga.ExponentialBackoff(5, 1*time.Second, 2.0)
```

## Persistence

### InMemoryPersistence (для тестирования)

```go
persistence := saga.NewInMemoryPersistence()
```

### EventStorePersistence (для production)

```go
eventStore := eventsourcing.NewPostgresEventStore(config)
snapshotStore := eventsourcing.NewPostgresSnapshotStore(config)
registry := saga.NewSagaRegistry()

// Регистрируем определения саг
registry.RegisterSaga("order_saga", orderSagaDefinition)

// Создаем persistence с registry (обязательно для Load())
persistence := saga.NewEventStorePersistence(eventStore, snapshotStore).WithRegistry(registry)
```

**Важно:** Для `EventStorePersistence.Load()` необходимо настроить `SagaRegistry` через `WithRegistry()`, иначе восстановление определений саг не будет работать.

### PostgresPersistence

```go
registry := saga.NewSagaRegistry()
registry.RegisterSaga("order_saga", orderSagaDefinition)

persistence, err := saga.NewPostgresPersistence("postgres://user:pass@localhost/db")
persistence = persistence.WithRegistry(registry)
```

**Важно:** Для `PostgresPersistence.Load()` необходимо настроить `SagaRegistry` через `WithRegistry()`.

## Integration

### С CommandBus

```go
step := saga.NewCommandStep(
    "reserve_inventory",
    commandBus,
    ReserveInventoryCommand{...},
    ReleaseInventoryCommand{...},
)
```

### С EventBus

```go
step := saga.NewEventStep(
    "notify_customer",
    eventBus,
    OrderCreatedEvent{...},
)
```

### С 2PC

```go
step := saga.NewTwoPhaseCommitStep(
    "distributed_transaction",
    twopcCoordinator,
    func(ctx context.Context, sagaCtx SagaContext) []saga.TwoPhaseCommitParticipant {
        return []saga.TwoPhaseCommitParticipant{
            stockParticipant,
            paymentParticipant,
        }
    },
)
```

## Advanced Topics

### Параллельное выполнение

```go
parallelStep := saga.NewParallelStep(
    "parallel_operations",
    step1,
    step2,
    step3,
)
```

### Условное выполнение

```go
conditionalStep := saga.NewConditionalStep(
    "conditional_step",
    func(ctx context.Context, sagaCtx SagaContext) bool {
        return sagaCtx.GetBool("needs_verification")
    },
    verificationStep,
)
```

## Best Practices

1. **Idempotency** - все шаги должны быть идемпотентными
2. **Compensating actions** - всегда определяйте компенсирующие действия
3. **Timeouts** - устанавливайте разумные таймауты для каждого шага
4. **Retry policies** - используйте exponential backoff для transient errors
5. **Monitoring** - отслеживайте выполнение саг через события
6. **Registry setup** - всегда настраивайте `SagaRegistry` для persistence и orchestrator при использовании `Load()` или `Resume()`
7. **Event sourcing** - при использовании `EventStorePersistence` все события шагов (`StepStarted`, `StepCompleted`, `StepFailed`, `StepCompensating`, `StepCompensated`) автоматически сохраняются и восстанавливаются

## API Reference

### SagaBuilder

```go
builder := saga.NewSagaBuilder("name").
    AddStep(step).
    WithTimeout(duration).
    WithRetryPolicy(policy).
    WithPersistence(persistence).
    WithEventBus(eventBus).
    Build()
```

### SagaOrchestrator

```go
registry := saga.NewSagaRegistry()
registry.RegisterSaga("order_saga", orderSagaDefinition)

orchestrator := saga.NewDefaultOrchestrator(persistence, eventBus).WithRegistry(registry)
// Или регистрируем через orchestrator
orchestrator.RegisterSaga("order_saga", orderSagaDefinition)

err := orchestrator.Execute(ctx, saga)
err = orchestrator.Compensate(ctx, saga)
saga, err = orchestrator.Resume(ctx, sagaID)
```

**Важно:** Для `Resume()` и `GetStatus()` необходимо настроить `SagaRegistry` в orchestrator, чтобы он мог восстановить определения саг из persistence.

## Examples

### Order Saga (`examples/saga-order/`)

Полнофункциональный пример Order Saga с EventSourced Order aggregate:

- **Шаги**: ReserveInventory → ProcessPayment → CreateShipment → CompleteOrder
- **Event Sourcing**: Order агрегат с полной историей событий
- **Persistence**: EventStore с snapshots каждые 5 шагов
- **CQRS**: CommandInvoker для type-safe вызова команд
- **API**: REST API с endpoints для создания заказов, получения статуса, истории, отмены и возобновления

**Запуск:**
```bash
cd examples/saga-order
make docker-up migrate run
```

**API примеры:**
```bash
# Создание заказа
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"c1","items":[{"product_id":"p1","quantity":2,"price":10}]}'

# Получение статуса
curl http://localhost:8080/api/v1/sagas/{saga_id}

# История выполнения
curl http://localhost:8080/api/v1/sagas/{saga_id}/history
```

### Архитектура Saga + FSM + EventStore

```mermaid
graph TB
    A[SagaOrchestrator] --> B[SagaDefinition]
    B --> C[FSM]
    C --> D[State: initial]
    C --> E[State: step_1]
    C --> F[State: step_2]
    C --> G[State: step_3]
    
    B --> H[SagaStep 1]
    B --> I[SagaStep 2]
    B --> J[SagaStep 3]
    
    H --> K[Execute]
    H --> L[Compensate]
    
    A --> M[EventStorePersistence]
    M --> N[EventStore]
    M --> O[SnapshotStore]
    
    A --> P[EventBus]
    P --> Q[StepStarted]
    P --> R[StepCompleted]
    P --> S[StepFailed]
    P --> T[SagaCompleted]
    
    A --> U[CommandBus]
    U --> V[CommandInvoker]
    V --> W[AsyncCommandBus]
    V --> X[EventAwaiter]
```

### Последовательность выполнения с компенсацией

```mermaid
sequenceDiagram
    participant Client
    participant Orchestrator
    participant Saga
    participant ReserveStep
    participant PaymentStep
    participant EventStore
    participant EventBus
    
    Client->>Orchestrator: POST /orders
    Orchestrator->>Saga: CreateInstance
    Orchestrator->>EventStore: Save(initial)
    Orchestrator->>Saga: Execute()
    
    Saga->>ReserveStep: Execute
    ReserveStep->>EventBus: Publish StepStarted
    ReserveStep->>CommandBus: ReserveInventoryCommand
    ReserveStep-->>Saga: InventoryReservedEvent
    Saga->>EventStore: Save(step1_completed)
    EventStore->>EventBus: Publish StepCompleted
    
    Saga->>PaymentStep: Execute
    PaymentStep->>EventBus: Publish StepStarted
    PaymentStep->>CommandBus: ProcessPaymentCommand
    PaymentStep-->>Saga: PaymentFailedEvent
    
    Saga->>PaymentStep: Compensate
    PaymentStep->>EventBus: Publish StepCompensating
    PaymentStep->>CommandBus: RefundPaymentCommand
    PaymentStep-->>Saga: Success
    EventStore->>EventBus: Publish StepCompensated
    
    Saga->>ReserveStep: Compensate
    ReserveStep->>EventBus: Publish StepCompensating
    ReserveStep->>CommandBus: ReleaseInventoryCommand
    ReserveStep-->>Saga: Success
    EventStore->>EventBus: Publish StepCompensated
    
    Saga->>EventStore: Save(compensated)
    EventStore->>EventBus: Publish SagaCompensated
    Saga-->>Orchestrator: Error
    Orchestrator-->>Client: 202 Accepted (compensation in progress)
```

## Troubleshooting

### Saga не компенсируется

- Проверьте, что все шаги имеют compensate actions
- Убедитесь, что компенсирующие действия идемпотентны

### Saga не возобновляется

- Проверьте, что persistence настроен правильно
- Убедитесь, что состояние сохранено после каждого шага
- Проверьте, что `SagaRegistry` настроен в persistence и orchestrator
- Убедитесь, что определение саги зарегистрировано в registry перед вызовом `Load()` или `Resume()`

### Timeout ошибки

- Увеличьте timeout для медленных шагов
- Проверьте производительность внешних сервисов

