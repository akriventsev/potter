# Event Sourcing Replay Example

Пример Event Replay и rebuilding проекций из event store с использованием нового API ProjectionManager.

## Описание

Этот пример демонстрирует различные сценарии replay событий для перестроения read models (проекций) из event store с использованием `ProjectionManager` и интерфейса `eventsourcing.Projection`.

## API

Пример использует `ProjectionManager` для управления проекциями:

- **ProjectionManager** - управляет жизненным циклом проекций
- **Интерфейс Projection** - каждая проекция реализует `Name()`, `HandleEvent()` и `Reset()`
- **CheckpointStore** - автоматическое сохранение позиции обработки
- **Rebuild()** - пересоздание проекции с нуля
- **Автоматическое восстановление** - после перезапуска проекции продолжают с последней позиции

## Use Cases

1. **Полный replay** - перестроение всех проекций из всех событий через ProjectionManager
2. **Replay агрегата** - перестроение проекции для конкретного агрегата
3. **Rebuild проекции** - пересоздание конкретной проекции с нуля
4. **Replay с момента времени** - replay событий с определенного момента
5. **Checkpoint восстановление** - автоматическое восстановление позиции после перезапуска

## Архитектура

- `domain/order.go` - Event Sourced агрегат Order
- `domain/events.go` - события заказов
- `projections/order_summary.go` - read model проекция (реализует `eventsourcing.Projection`)
- `projections/customer_stats.go` - проекция статистики клиентов (реализует `eventsourcing.Projection`)
- `cmd/replay/main.go` - CLI для различных сценариев replay с ProjectionManager

## Quick Start

```bash
make up    # Запустить PostgreSQL
make build # Собрать CLI
make run   # Запустить полный replay
```

## CLI Commands

### Полный replay всех событий

```bash
./bin/replay -command=replay-all
```

### Replay для конкретного агрегата

```bash
./bin/replay -command=replay-aggregate -aggregate-id=order-123
```

### Rebuild конкретной проекции (новый API)

```bash
# Использует ProjectionManager.Rebuild() для пересоздания проекции
./bin/replay -command=replay-projection -projection=order_summary
./bin/replay -command=replay-projection -projection=customer_stats
```

Проекция будет сброшена (`Reset()`) и пересоздана из всех событий.

### Replay с определенного момента

```bash
./bin/replay -command=replay-from -from-time=2024-01-01T00:00:00Z
```

### Опции

- `-dsn` - строка подключения к БД (по умолчанию: postgres://postgres:postgres@localhost:5432/eventsourcing_replay?sslmode=disable)
- `-batch-size` - размер батча для обработки (по умолчанию: 1000)
- `-parallel` - включить параллельную обработку

Пример с опциями:

```bash
./bin/replay -command=replay-all -batch-size=5000 -parallel
```

## Makefile Commands

```bash
make replay-all                    # Полный replay
make replay-aggregate ID=order-123 # Replay агрегата
make replay-projection PROJECTION=order_summary # Rebuild проекции
make replay-from TIME=2024-01-01T00:00:00Z # Replay с момента
```

## Progress Tracking

CLI автоматически показывает прогресс во время replay:

```
Progress: 1500/10000 (15.00%) | Position: 1500 | Elapsed: 2m30s
```

## Документация

См. [framework/eventsourcing/README.md](../../../framework/eventsourcing/README.md) для подробной документации по Event Sourcing и Replay.
