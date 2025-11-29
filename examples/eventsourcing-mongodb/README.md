# Event Sourcing MongoDB Example

Пример Event Sourcing с использованием MongoDB вместо PostgreSQL.

## Описание

Этот пример демонстрирует использование `MongoDBEventStore` для хранения событий в MongoDB. Показывает особенности работы с BSON типами, индексами и гибкой схемой MongoDB.

## Use Case

Управление инвентарем склада с Event Sourcing:
- Добавление товара на склад
- Резервирование товара
- Списание товара
- Корректировка остатков

## Архитектура

- `domain/inventory.go` - Event Sourced агрегат Inventory
- `domain/events.go` - события инвентаря
- `application/inventory_service.go` - сервис управления инвентарем
- `infrastructure/mongodb_store.go` - настройка MongoDB event store с десериализатором
- `cmd/server/main.go` - REST API сервер

## Особенности MongoDB

- **Гибкая схема** - метаданные событий хранятся как BSON документы
- **Встроенная поддержка вложенных документов** - удобно для сложных структур событий
- **Эффективные запросы** - индексы по aggregate_id, event_type, occurred_at, position
- **Горизонтальное масштабирование** - MongoDB поддерживает шардирование
- **BSON типы** - безопасная работа с различными числовыми типами через helper-функции

## Индексы

MongoDBEventStore автоматически создает следующие индексы:

1. **Уникальный составной индекс** на `(aggregate_id, version)` - для оптимистичной конкурентности
2. **Индекс на aggregate_id** - для быстрого поиска событий агрегата
3. **Индекс на event_type** - для фильтрации по типу события
4. **Индекс на occurred_at** - для временных запросов
5. **Индекс на position** - для последовательного чтения событий

## Quick Start

```bash
make up    # Запустить MongoDB
make run   # Запустить приложение
```

## API Examples

### Добавление товара на склад

```http
POST /inventory/items
Content-Type: application/json

{
  "product_id": "product-1",
  "warehouse_id": "warehouse-1",
  "quantity": 100
}
```

### Резервирование товара

```http
POST /inventory/reserve
Content-Type: application/json

{
  "inventory_id": "inventory-123",
  "quantity": 10
}
```

### Получение информации о товаре

```http
GET /inventory/{id}
```

### Получение событий по типу (BSON запросы)

```http
GET /events/by-type?type=inventory.item.added
```

## Сравнение с PostgreSQL

| Особенность | PostgreSQL | MongoDB |
|------------|------------|---------|
| Схема | Фиксированная | Гибкая (BSON) |
| Метаданные | JSONB | BSON документы |
| Индексы | B-tree | B-tree, Text, Geospatial |
| Масштабирование | Вертикальное | Горизонтальное (шардирование) |
| Запросы | SQL | MongoDB Query Language |
| Транзакции | ACID | Multi-document transactions |

## Десериализация событий

MongoDBEventStore использует `EventDeserializer` для восстановления событий из BSON. Десериализатор определяет тип события по полю `event_type` и создает соответствующий объект события.

## Документация

См. [framework/eventsourcing/README.md](../../../framework/eventsourcing/README.md) для подробной документации по Event Sourcing и MongoDB адаптеру.
