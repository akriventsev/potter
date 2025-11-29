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

## Миграции для MongoDB

Для MongoDB рекомендуется использовать Go-миграции вместо SQL, так как MongoDB не поддерживает SQL напрямую. Пример Go-миграции для создания коллекций и индексов:

```go
package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client

func init() {
	goose.AddMigration(upInitCollections, downInitCollections)
}

func upInitCollections(tx *sql.Tx) error {
	// В реальном приложении используйте dependency injection для получения MongoDB клиента
	ctx := context.Background()
	
	// Получаем MongoDB клиент (в реальном приложении это должно быть через DI)
	client := getMongoClient()
	if client == nil {
		return fmt.Errorf("MongoDB client not initialized")
	}
	
	db := client.Database("potter")
	
	// Создаем коллекцию event_store
	if err := db.CreateCollection(ctx, "event_store"); err != nil {
		// Игнорируем ошибку если коллекция уже существует
		if !isCollectionExistsError(err) {
			return err
		}
	}
	
	// Создаем индексы для event_store
	eventStoreCollection := db.Collection("event_store")
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "aggregate_id", Value: 1}, {Key: "version", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "aggregate_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "event_type", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "occurred_at", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "position", Value: 1}},
		},
	}
	
	_, err := eventStoreCollection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return err
	}
	
	// Создаем коллекцию snapshots
	if err := db.CreateCollection(ctx, "snapshots"); err != nil {
		if !isCollectionExistsError(err) {
			return err
		}
	}
	
	// Создаем индекс для snapshots
	snapshotsCollection := db.Collection("snapshots")
	snapshotIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "aggregate_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
	
	_, err = snapshotsCollection.Indexes().CreateMany(ctx, snapshotIndexes)
	return err
}

func downInitCollections(tx *sql.Tx) error {
	client := getMongoClient()
	if client == nil {
		return fmt.Errorf("MongoDB client not initialized")
	}
	
	ctx := context.Background()
	db := client.Database("potter")
	
	// Удаляем коллекции
	if err := db.Collection("event_store").Drop(ctx); err != nil {
		return err
	}
	
	if err := db.Collection("snapshots").Drop(ctx); err != nil {
		return err
	}
	
	return nil
}

func getMongoClient() *mongo.Client {
	// В реальном приложении это должно быть через dependency injection
	return mongoClient
}

func isCollectionExistsError(err error) bool {
	// Проверяем, является ли ошибка ошибкой существования коллекции
	return err != nil && (err.Error() == "collection already exists" || 
		err.Error() == "namespace already exists")
}
```

Подробнее о Go-миграциях для MongoDB см. [framework/migrations/README.md](../../../framework/migrations/README.md) и [документацию goose](https://github.com/pressly/goose).

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
