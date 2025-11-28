// Package repository предоставляет generic адаптеры для работы с различными storage backends.
package repository

import (
	"context"
	"fmt"

	"potter/framework/core"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoConfig конфигурация для MongoDB репозитория
type MongoConfig struct {
	URI        string
	Database   string
	Collection string
	Timeout    int // в секундах
	MaxPoolSize int
	MinPoolSize int
}

// Validate проверяет корректность конфигурации
func (c MongoConfig) Validate() error {
	if c.URI == "" {
		return fmt.Errorf("URI cannot be empty")
	}
	if c.Database == "" {
		return fmt.Errorf("database cannot be empty")
	}
	if c.Collection == "" {
		return fmt.Errorf("collection cannot be empty")
	}
	if c.MaxPoolSize <= 0 {
		return fmt.Errorf("MaxPoolSize must be greater than 0")
	}
	return nil
}

// DefaultMongoConfig возвращает конфигурацию MongoDB по умолчанию
func DefaultMongoConfig() MongoConfig {
	return MongoConfig{
		Database:    "potter",
		Collection:  "entities",
		Timeout:     10,
		MaxPoolSize: 100,
		MinPoolSize: 10,
	}
}

// MongoRepository[T Entity] generic MongoDB репозиторий
// NOTE: Текущая реализация покрывает только базовые CRUD операции.
// Следующие функции планируются, но еще не реализованы:
// - Query building для сложных запросов
// - Миграции схемы БД
// - Индексирование
// - TTL (Time To Live) для автоматического удаления устаревших записей
// - Change streams для подписки на изменения в коллекции
// TODO: Реализовать query builder, migrations, indexing, TTL и change streams в последующих версиях
type MongoRepository[T Entity] struct {
	config     MongoConfig
	client     *mongo.Client
	collection *mongo.Collection
}

// NewMongoRepository создает новый MongoDB репозиторий
func NewMongoRepository[T Entity](config MongoConfig) (*MongoRepository[T], error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid mongodb config: %w", err)
	}

	ctx := context.Background()

	opts := options.Client().
		ApplyURI(config.URI).
		SetMaxPoolSize(uint64(config.MaxPoolSize)).
		SetMinPoolSize(uint64(config.MinPoolSize))

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Проверяем подключение
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	collection := client.Database(config.Database).Collection(config.Collection)

	return &MongoRepository[T]{
		config:     config,
		client:     client,
		collection: collection,
	}, nil
}

// Start запускает адаптер (реализация core.Lifecycle)
func (m *MongoRepository[T]) Start(ctx context.Context) error {
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (m *MongoRepository[T]) Stop(ctx context.Context) error {
	if m.client != nil {
		return m.client.Disconnect(ctx)
	}
	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (m *MongoRepository[T]) IsRunning() bool {
	return m.client != nil
}

// Name возвращает имя компонента (реализация core.Component)
func (m *MongoRepository[T]) Name() string {
	return "mongodb-repository"
}

// Type возвращает тип компонента (реализация core.Component)
func (m *MongoRepository[T]) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// Save сохраняет entity (InsertOne/UpdateOne)
func (m *MongoRepository[T]) Save(ctx context.Context, entity T) error {
	id := entity.ID()
	filter := map[string]interface{}{"_id": id}

	// Используем ReplaceOne для upsert
	opts := options.Replace().SetUpsert(true)
	_, err := m.collection.ReplaceOne(ctx, filter, entity, opts)
	if err != nil {
		return fmt.Errorf("failed to save entity: %w", err)
	}

	return nil
}

// FindByID находит entity по ID
func (m *MongoRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	var zero T

	filter := map[string]interface{}{"_id": id}
	var entity T

	err := m.collection.FindOne(ctx, filter).Decode(&entity)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return zero, fmt.Errorf("entity not found: %s", id)
		}
		return zero, fmt.Errorf("failed to find entity: %w", err)
	}

	return entity, nil
}

// FindAll возвращает все entities
func (m *MongoRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	cursor, err := m.collection.Find(ctx, map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var entities []T
	if err := cursor.All(ctx, &entities); err != nil {
		return nil, fmt.Errorf("failed to decode entities: %w", err)
	}

	return entities, nil
}

// Delete удаляет entity
func (m *MongoRepository[T]) Delete(ctx context.Context, id string) error {
	filter := map[string]interface{}{"_id": id}

	result, err := m.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("entity not found: %s", id)
	}

	return nil
}

