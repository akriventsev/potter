# Event Sourcing Module

Полная поддержка Event Sourcing паттерна для построения систем с полной историей изменений и возможностью восстановления состояния из событий.

## Содержание

1. [Введение](#введение)
2. [Архитектура](#архитектура)
3. [Quick Start](#quick-start)
4. [Event Store Adapters](#event-store-adapters)
5. [Snapshots](#snapshots)
6. [Event Replay](#event-replay)
7. [Оптимистичная конкурентность](#оптимистичная-конкурентность)
8. [Best Practices](#best-practices)
9. [Примеры](#примеры)
10. [API Reference](#api-reference)
11. [Migration Guide](#migration-guide)
12. [Troubleshooting](#troubleshooting)

## Введение

### Что такое Event Sourcing?

Event Sourcing - это паттерн проектирования, при котором состояние приложения определяется последовательностью событий, а не текущим снимком данных. Вместо обновления записи в базе данных, каждое изменение сохраняется как новое событие в append-only хранилище.

### Преимущества

- ✅ **Полная история изменений** - каждое изменение сохраняется как событие
- ✅ **Восстановление состояния** - можно восстановить состояние на любой момент времени
- ✅ **Аудит и отладка** - полная трассировка всех действий
- ✅ **Гибкость** - можно создавать новые проекции из существующих событий
- ✅ **Масштабируемость** - события можно обрабатывать асинхронно

### Недостатки

- ⚠️ **Сложность** - требует понимания паттерна и правильного дизайна
- ⚠️ **Производительность** - восстановление состояния может быть медленным для больших потоков
- ⚠️ **Хранилище** - события накапливаются и требуют места
- ⚠️ **Миграции** - изменение структуры событий требует специальных подходов

### Когда использовать Event Sourcing?

- Системы с высокими требованиями к аудиту
- Финансовые и банковские системы
- Системы с сложной бизнес-логикой
- Когда нужна возможность "перемотки" времени
- Системы с CQRS архитектурой

## Архитектура

```mermaid
graph TB
    A[Command] --> B[Command Handler]
    B --> C[EventSourcedAggregate]
    C --> D[RaiseEvent]
    D --> E[EventStore]
    E --> F[AppendEvents]
    F --> G[SnapshotStore]
    G --> H[SaveSnapshot]
    
    I[Query] --> J[Query Handler]
    J --> K[EventSourcedRepository]
    K --> L[GetByID]
    L --> M{Snapshot exists?}
    M -->|Yes| N[Load from Snapshot]
    M -->|No| O[Load from Events]
    N --> P[Load Delta Events]
    O --> Q[Replay All Events]
    P --> R[Aggregate State]
    Q --> R
```

### Компоненты

- **EventStore** - центральное хранилище событий с версионированием
- **EventSourcedAggregate** - агрегаты с восстановлением состояния через replay
- **SnapshotStore** - оптимизация через снапшоты
- **EventReplayer** - механизм replay событий и rebuilding проекций
- **EventSourcedRepository** - generic репозиторий для Event Sourced агрегатов

## Quick Start

### Создание Event Sourced агрегата

```go
package domain

import (
    "potter/framework/events"
    "potter/framework/eventsourcing"
)

type BankAccount struct {
    eventsourcing.EventSourcedAggregate
    accountNumber string
    ownerName     string
    balance       int64
    isActive      bool
}

func NewBankAccount(accountNumber, ownerName string) *BankAccount {
    account := &BankAccount{
        EventSourcedAggregate: *eventsourcing.NewEventSourcedAggregate(accountNumber),
    }
    event := &AccountOpenedEvent{
        BaseEvent: events.NewBaseEvent("account.opened", accountNumber),
        AccountNumber: accountNumber,
        OwnerName:     ownerName,
    }
    account.RaiseEvent(event)
    return account
}

func (a *BankAccount) Deposit(amount int64) error {
    if !a.isActive {
        return errors.New("account is closed")
    }
    event := &MoneyDepositedEvent{
        BaseEvent: events.NewBaseEvent("account.deposited", a.ID()),
        Amount: amount,
    }
    a.RaiseEvent(event)
    return nil
}

func (a *BankAccount) Withdraw(amount int64) error {
    if !a.isActive {
        return errors.New("account is closed")
    }
    if a.balance < amount {
        return errors.New("insufficient funds")
    }
    event := &MoneyWithdrawnEvent{
        BaseEvent: events.NewBaseEvent("account.withdrawn", a.ID()),
        Amount: amount,
    }
    a.RaiseEvent(event)
    return nil
}

// Apply применяет события для восстановления состояния
func (a *BankAccount) Apply(event events.Event) error {
    switch e := event.(type) {
    case *AccountOpenedEvent:
        a.accountNumber = e.AccountNumber
        a.ownerName = e.OwnerName
        a.balance = 0
        a.isActive = true
    case *MoneyDepositedEvent:
        a.balance += e.Amount
    case *MoneyWithdrawnEvent:
        a.balance -= e.Amount
    }
    return nil
}
```

### Использование репозитория

```go
package infrastructure

import (
    "context"
    "potter/framework/eventsourcing"
)

func SetupRepository() *eventsourcing.EventSourcedRepository[*BankAccount] {
    // Создаем Event Store
    eventStoreConfig := eventsourcing.DefaultPostgresEventStoreConfig()
    eventStoreConfig.DSN = "postgres://user:pass@localhost/potter"
    eventStore, _ := eventsourcing.NewPostgresEventStore(eventStoreConfig)
    
    // Создаем Snapshot Store
    snapshotStore, _ := eventsourcing.NewPostgresSnapshotStore(eventStoreConfig)
    
    // Создаем репозиторий
    config := eventsourcing.DefaultRepositoryConfig()
    config.SnapshotFrequency = 100
    config.UseSnapshots = true
    
    // Фабрика для создания агрегатов
    factory := func(id string) *BankAccount {
        return NewBankAccount(id, "")
    }
    
    repo := eventsourcing.NewEventSourcedRepository[*BankAccount](
        eventStore,
        snapshotStore,
        config,
        factory,
    )
    
    return repo
}

// Использование
func DepositMoney(ctx context.Context, repo *eventsourcing.EventSourcedRepository[*BankAccount], accountID string, amount int64) error {
    // Загружаем агрегат
    account, err := repo.GetByID(ctx, accountID)
    if err != nil {
        return err
    }
    
    // Выполняем операцию
    if err := account.Deposit(amount); err != nil {
        return err
    }
    
    // Сохраняем
    return repo.Save(ctx, account)
}
```

### Использование Builder

```go
factory := func(id string) *BankAccount {
    return NewBankAccount(id, "")
}

repo, err := eventsourcing.NewEventSourcingBuilder().
    WithEventStore(postgresStore).
    WithSnapshotStore(postgresSnapshotStore).
    WithSnapshotStrategy(eventsourcing.FrequencyStrategy(100)).
    WithSnapshotsEnabled(true).
    Build[*BankAccount](factory)
```

## Event Store Adapters

### Поддерживаемые адаптеры

| Адаптер | Статус | Описание |
|---------|--------|----------|
| `InMemoryEventStore` | ✅ Ready for testing | In-memory хранилище для тестирования и разработки |
| `PostgresEventStore` | ✅ Production-ready | Полнофункциональный адаптер для PostgreSQL |
| `MongoDBEventStore` | ✅ Production-ready | Полнофункциональный адаптер для MongoDB |
| `EventStoreDB` | ⏳ Planned | Поддержка EventStore DB планируется в будущих версиях |

### InMemory

Для тестирования и разработки:

```go
config := eventsourcing.DefaultInMemoryEventStoreConfig()
store := eventsourcing.NewInMemoryEventStore(config)
```

### PostgreSQL

Production-ready адаптер:

```go
config := eventsourcing.DefaultPostgresEventStoreConfig()
config.DSN = "postgres://user:pass@localhost/potter"
config.SchemaName = "public"
config.TableName = "event_store"

// Базовое создание (без десериализатора - для обратной совместимости)
store, err := eventsourcing.NewPostgresEventStore(config)

// С десериализатором событий (рекомендуется)
deserializer := NewYourEventDeserializer()
store, err := eventsourcing.NewPostgresEventStoreWithDeserializer(config, deserializer)
```

**Десериализация событий:**

EventStore адаптеры требуют `EventDeserializer` для восстановления событий из хранилища. При сохранении событий сохраняется `event_type` и сериализованные данные. При загрузке десериализатор использует `event_type` для выбора правильного типа события.

**Миграции:**

```bash
psql -d potter -f framework/eventsourcing/migrations/postgres/001_create_event_store.sql
```

### MongoDB

NoSQL вариант:

```go
config := eventsourcing.DefaultMongoDBEventStoreConfig()
config.URI = "mongodb://localhost:27017"
config.Database = "potter"
config.Collection = "events"

// Базовое создание (без десериализатора - для обратной совместимости)
store, err := eventsourcing.NewMongoDBEventStore(config)

// С десериализатором событий (рекомендуется)
deserializer := NewYourEventDeserializer()
store, err := eventsourcing.NewMongoDBEventStoreWithDeserializer(config, deserializer)
```

**Безопасная работа с типами:**

MongoDBEventStore использует безопасные helper-функции (`getInt64`, `getString`, `getTime`) для работы с различными числовыми типами в BSON, что предотвращает паники при работе с данными разных версий.

## Roadmap

### Планируемые улучшения

- **EventStoreDB Adapter**: Поддержка EventStore DB будет добавлена в будущих версиях при наличии стабильного Go клиента. См. [ROADMAP.md](../../ROADMAP.md) для отслеживания прогресса.

## Snapshots

### Зачем нужны снапшоты?

При большом количестве событий восстановление состояния может быть медленным. Снапшоты сохраняют состояние агрегата на определенный момент, позволяя загружать только события после снапшота.

### Стратегии создания

#### Frequency Strategy

Создание каждые N событий:

```go
strategy := eventsourcing.NewFrequencySnapshotStrategy(100)
```

#### Time Based Strategy

Создание по времени:

```go
strategy := eventsourcing.NewTimeBasedSnapshotStrategy(1 * time.Hour)
```

#### Hybrid Strategy

Комбинация частоты и времени:

```go
strategy := eventsourcing.NewHybridSnapshotStrategy(100, 1*time.Hour)
```

### Конфигурация

```go
config := eventsourcing.RepositoryConfig{
    SnapshotFrequency: 100,
    UseSnapshots:      true,
    SnapshotStrategy:  eventsourcing.NewFrequencySnapshotStrategy(100),
    Serializer:        eventsourcing.NewJSONSnapshotSerializer(),
}
```

## Event Replay

### Восстановление состояния агрегата

```go
replayer := eventsourcing.NewDefaultEventReplayer(eventStore, snapshotStore)

// Replay для конкретного агрегата
err := replayer.ReplayAggregate(ctx, "account-1", 0)
```

### Rebuilding проекций

```go
type ProjectionHandler struct{}

func (h *ProjectionHandler) HandleEvent(ctx context.Context, event eventsourcing.StoredEvent) error {
    // Обработка события для обновления проекции
    return nil
}

handler := &ProjectionHandler{}
options := eventsourcing.DefaultReplayOptions()
options.BatchSize = 1000

err := replayer.ReplayAll(ctx, handler, 0, options)
```

### Progress Tracking

```go
progressCallback := func(progress eventsourcing.ReplayProgress) {
    fmt.Printf("Processed: %d/%d, Position: %d, Elapsed: %v\n",
        progress.ProcessedEvents,
        progress.TotalEvents,
        progress.CurrentPosition,
        progress.ElapsedTime,
    )
}

err := replayer.ReplayWithProgress(ctx, handler, 0, options, progressCallback)
```

## Оптимистичная конкурентность

Event Sourcing использует версионирование для предотвращения конфликтов:

```go
// При сохранении проверяется версия
err := eventStore.AppendEvents(ctx, aggregateID, expectedVersion, events)
if err == eventsourcing.ErrConcurrencyConflict {
    // Обработка конфликта - перезагрузка и повтор
    aggregate, _ := repo.GetByID(ctx, aggregateID)
    // Повтор операции
}
```

### Retry стратегия

```go
maxRetries := 3
for i := 0; i < maxRetries; i++ {
    err := repo.Save(ctx, aggregate)
    if err == nil {
        break
    }
    if err != eventsourcing.ErrConcurrencyConflict {
        return err
    }
    // Небольшая задержка перед повтором
    time.Sleep(time.Duration(i+1) * 10 * time.Millisecond)
    aggregate, _ = repo.GetByID(ctx, aggregate.ID())
}
```

## Best Practices

### Дизайн событий

1. **Immutable** - события не должны изменяться после создания
2. **Past tense** - используйте прошедшее время для имен (AccountOpened, MoneyDeposited)
3. **Семантическая ясность** - имена должны четко описывать что произошло
4. **Версионирование** - при изменении структуры создавайте новую версию события

### Версионирование событий

```go
type AccountOpenedEventV1 struct {
    *events.BaseEvent
    AccountNumber string
    OwnerName     string
}

type AccountOpenedEventV2 struct {
    *events.BaseEvent
    AccountNumber string
    OwnerName     string
    InitialBalance int64  // Новое поле
}
```

### Миграция событий

При изменении структуры событий используйте конвертеры:

```go
func ConvertV1ToV2(v1 *AccountOpenedEventV1) *AccountOpenedEventV2 {
    return &AccountOpenedEventV2{
        BaseEvent:     v1.BaseEvent,
        AccountNumber: v1.AccountNumber,
        OwnerName:     v1.OwnerName,
        InitialBalance: 0, // Значение по умолчанию
    }
}
```

### Тестирование

```go
func TestBankAccount_Deposit(t *testing.T) {
    account := NewBankAccount("ACC001", "John Doe")
    
    err := account.Deposit(1000)
    assert.NoError(t, err)
    
    events := account.GetUncommittedEvents()
    assert.Len(t, events, 1)
    assert.Equal(t, "account.deposited", events[0].EventType())
}
```

### Производительность

- Используйте снапшоты для агрегатов с большой историей
- Настройте частоту снапшотов в зависимости от нагрузки
- Используйте batch processing для replay операций
- Оптимизируйте индексы в базе данных

## Примеры

- [`examples/eventsourcing-basic`](../../examples/eventsourcing-basic) - базовый пример с банковским счетом
- [`examples/warehouse`](../../examples/warehouse) - продвинутый пример с Event Sourced Product

## API Reference

### EventStore

```go
type EventStore interface {
    AppendEvents(ctx context.Context, aggregateID string, expectedVersion int64, events []events.Event) error
    GetEvents(ctx context.Context, aggregateID string, fromVersion int64) ([]StoredEvent, error)
    GetEventsByType(ctx context.Context, eventType string, fromTimestamp time.Time) ([]StoredEvent, error)
    GetAllEvents(ctx context.Context, fromPosition int64) (<-chan StoredEvent, error)
}
```

### EventSourcedAggregate

```go
type EventSourcedAggregate struct {
    id               string
    version          int64
    uncommittedEvents []events.Event
}

func (a *EventSourcedAggregate) RaiseEvent(event events.Event)
func (a *EventSourcedAggregate) LoadFromHistory(events []events.Event) error
func (a *EventSourcedAggregate) GetUncommittedEvents() []events.Event
func (a *EventSourcedAggregate) MarkEventsAsCommitted()
func (a *EventSourcedAggregate) Version() int64
```

### EventSourcedRepository

```go
type EventSourcedRepository[T AggregateInterface] struct {
    eventStore    EventStore
    snapshotStore SnapshotStore
    config        RepositoryConfig
    factory       AggregateFactory[T]
}

func NewEventSourcedRepository[T AggregateInterface](
    eventStore EventStore,
    snapshotStore SnapshotStore,
    config RepositoryConfig,
    factory AggregateFactory[T],
) *EventSourcedRepository[T]

func (r *EventSourcedRepository[T]) Save(ctx context.Context, aggregate T) error
func (r *EventSourcedRepository[T]) GetByID(ctx context.Context, aggregateID string) (T, error)
func (r *EventSourcedRepository[T]) GetVersion(ctx context.Context, aggregateID string) (int64, error)
func (r *EventSourcedRepository[T]) Exists(ctx context.Context, aggregateID string) (bool, error)
```

## Migration Guide

### Переход от обычных агрегатов к Event Sourced

1. **Определите события** - какие события генерирует ваш агрегат
2. **Реализуйте Apply методы** - как события изменяют состояние
3. **Обновите конструкторы** - создавайте события при создании агрегата
4. **Обновите методы** - генерируйте события вместо прямого изменения состояния
5. **Мигрируйте данные** - создайте события из существующих данных

### Интеграция с существующими системами

Event Sourcing можно использовать вместе с обычными репозиториями:

```go
// Event Sourced агрегаты для write модели
eventStore := NewPostgresEventStore(config)
repo := NewEventSourcedRepository[*BankAccount](eventStore, snapshotStore, config)

// Обычные репозитории для read модели
readRepo := NewPostgresRepository[*AccountView](config, mapper)
```

## Troubleshooting

### Проблема: Медленная загрузка агрегатов

**Решение:**
- Используйте снапшоты
- Увеличьте частоту создания снапшотов
- Оптимизируйте индексы в базе данных

### Проблема: Конфликты конкурентности

**Решение:**
- Реализуйте retry механизм
- Используйте более короткие транзакции
- Рассмотрите использование Saga паттерна для длительных операций

### Проблема: Большой размер хранилища

**Решение:**
- Реализуйте архивирование старых событий
- Используйте сжатие для событий
- Рассмотрите использование EventStore DB с оптимизациями

### Проблема: Миграция событий

**Решение:**
- Используйте версионирование событий
- Создайте конвертеры между версиями
- Применяйте миграции при загрузке событий

## Performance Tuning

### Настройка снапшотов

```go
// Для агрегатов с частыми изменениями
config.SnapshotFrequency = 50

// Для агрегатов с редкими изменениями
config.SnapshotFrequency = 500
```

### Batch Processing

```go
options := eventsourcing.DefaultReplayOptions()
options.BatchSize = 5000  // Увеличьте для больших объемов
options.Parallel = true   // Параллельная обработка
```

### Database Optimization

```sql
-- Оптимизация индексов для PostgreSQL
CREATE INDEX CONCURRENTLY idx_event_store_aggregate_version 
    ON event_store(aggregate_id, version);

-- Партиционирование для больших таблиц
CREATE TABLE event_store_2025 PARTITION OF event_store
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
```

## Debugging Tips

1. **Логирование событий** - логируйте все события при сохранении
2. **Версионирование** - отслеживайте версии агрегатов
3. **Replay тестирование** - тестируйте восстановление состояния
4. **Мониторинг** - отслеживайте размер потоков событий

---

Для дополнительной информации см. [основную документацию](../../README.md) и [примеры](../../examples/).

