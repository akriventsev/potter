package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// mongoClient должен быть установлен перед применением миграций
// В реальном приложении используйте dependency injection
var mongoClient *mongo.Client

// SetMongoClient устанавливает MongoDB клиент для использования в миграциях
func SetMongoClient(client *mongo.Client) {
	mongoClient = client
}

func init() {
	goose.AddMigration(upInitCollections, downInitCollections)
}

func upInitCollections(tx *sql.Tx) error {
	ctx := context.Background()

	// Получаем MongoDB клиент
	client := getMongoClient()
	if client == nil {
		return fmt.Errorf("MongoDB client not initialized. Call SetMongoClient() before running migrations")
	}

	db := client.Database("potter")

	// Создаем коллекцию event_store
	err := db.CreateCollection(ctx, "event_store")
	if err != nil {
		// Игнорируем ошибку если коллекция уже существует
		if !isCollectionExistsError(err) {
			return fmt.Errorf("failed to create event_store collection: %w", err)
		}
	}

	// Создаем индексы для event_store
	eventStoreCollection := db.Collection("event_store")
	indexes := []mongo.IndexModel{
		{
			// Уникальный составной индекс для оптимистичной конкурентности
			Keys: bson.D{
				{Key: "aggregate_id", Value: 1},
				{Key: "version", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("idx_aggregate_version"),
		},
		{
			// Индекс для быстрого поиска событий агрегата
			Keys: bson.D{{Key: "aggregate_id", Value: 1}},
			Options: options.Index().SetName("idx_aggregate_id"),
		},
		{
			// Индекс для фильтрации по типу события
			Keys: bson.D{{Key: "event_type", Value: 1}},
			Options: options.Index().SetName("idx_event_type"),
		},
		{
			// Индекс для временных запросов
			Keys: bson.D{{Key: "occurred_at", Value: 1}},
			Options: options.Index().SetName("idx_occurred_at"),
		},
		{
			// Индекс для последовательного чтения событий
			Keys: bson.D{{Key: "position", Value: 1}},
			Options: options.Index().SetName("idx_position"),
		},
	}

	_, err = eventStoreCollection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create event_store indexes: %w", err)
	}

	// Создаем коллекцию snapshots
	err = db.CreateCollection(ctx, "snapshots")
	if err != nil {
		// Игнорируем ошибку если коллекция уже существует
		if !isCollectionExistsError(err) {
			return fmt.Errorf("failed to create snapshots collection: %w", err)
		}
	}

	// Создаем индекс для snapshots
	snapshotsCollection := db.Collection("snapshots")
	snapshotIndexes := []mongo.IndexModel{
		{
			// Уникальный индекс на aggregate_id для snapshots
			Keys: bson.D{{Key: "aggregate_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_snapshot_aggregate_id"),
		},
	}

	_, err = snapshotsCollection.Indexes().CreateMany(ctx, snapshotIndexes)
	if err != nil {
		return fmt.Errorf("failed to create snapshots indexes: %w", err)
	}

	return nil
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
		return fmt.Errorf("failed to drop event_store collection: %w", err)
	}

	if err := db.Collection("snapshots").Drop(ctx); err != nil {
		return fmt.Errorf("failed to drop snapshots collection: %w", err)
	}

	return nil
}

func getMongoClient() *mongo.Client {
	// В реальном приложении это должно быть через dependency injection
	return mongoClient
}

func isCollectionExistsError(err error) bool {
	// Проверяем, является ли ошибка ошибкой существования коллекции
	// MongoDB возвращает различные ошибки в зависимости от версии
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "collection already exists" ||
		errStr == "namespace already exists" ||
		errStr == "(NamespaceExists) Collection already exists"
}

