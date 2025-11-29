# Event Sourcing Snapshots Example

Пример работы со снапшотами в Event Sourcing для оптимизации производительности.

## Описание

Этот пример демонстрирует три стратегии создания снапшотов:
1. **FrequencyStrategy** - создание снапшота каждые N событий
2. **TimeBasedStrategy** - создание снапшота по времени
3. **HybridStrategy** - комбинация частоты и времени

## Use Case

Управление продуктом с большим количеством событий (изменение цены, обновление остатков, изменение описания). Снапшоты позволяют быстро восстанавливать состояние без применения всех событий.

## Архитектура

- `domain/product.go` - Event Sourced агрегат Product
- `domain/events.go` - события продукта
- `application/product_service.go` - сервис для работы с продуктами
- `infrastructure/repository.go` - репозиторий с настройкой снапшотов

## Quick Start

```bash
make up    # Запустить PostgreSQL
make run   # Запустить приложение
```

## API Examples

```http
POST /products
Content-Type: application/json

{
  "name": "Product 1",
  "price": 100.00
}

PUT /products/{id}/price
Content-Type: application/json

{
  "price": 120.00
}
```

## Документация

См. [framework/eventsourcing/README.md](../../../framework/eventsourcing/README.md) для подробной документации по Event Sourcing.

