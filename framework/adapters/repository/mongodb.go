// Package repository предоставляет generic адаптеры для работы с различными storage backends.
package repository

import (
	"context"
	"fmt"
	"time"

	"potter/framework/core"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoConfig конфигурация для MongoDB репозитория
type MongoConfig struct {
	URI         string
	Database    string
	Collection  string
	Timeout     int // в секундах
	MaxPoolSize int
	MinPoolSize int
	TTLField    string        // поле для TTL индекса
	TTLDuration time.Duration // время жизни для TTL
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

// MongoRepository[T Entity] generic MongoDB репозиторий.
// 
// Provides basic CRUD operations with BSON serialization and advanced query builder.
// См. framework/adapters/repository/query_builder.go для Query Builder API.
type MongoRepository[T Entity] struct {
	config             MongoConfig
	client             *mongo.Client
	collection         *mongo.Collection
	indexManager       *MongoIndexManager[T]
	changeStreamWatcher *MongoChangeStreamWatcher[T]
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

	repo := &MongoRepository[T]{
		config:       config,
		client:       client,
		collection:   collection,
		indexManager: NewMongoIndexManager[T](collection, config),
	}

	// Автоматическое создание TTL индекса если указаны TTLField и TTLDuration
	if config.TTLField != "" && config.TTLDuration > 0 {
		if err := repo.EnableTTL(config.TTLField, config.TTLDuration); err != nil {
			return nil, fmt.Errorf("failed to enable TTL: %w", err)
		}
	}

	return repo, nil
}

// Start запускает адаптер (реализация core.Lifecycle)
func (m *MongoRepository[T]) Start(ctx context.Context) error {
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (m *MongoRepository[T]) Stop(ctx context.Context) error {
	if m.changeStreamWatcher != nil {
		_ = m.changeStreamWatcher.Close()
	}
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

// Query возвращает QueryBuilder для построения сложных запросов
func (m *MongoRepository[T]) Query() *MongoQueryBuilder[T] {
	return NewMongoQueryBuilder[T](m.collection, m.config)
}

// IndexManager возвращает IndexManager для управления индексами
func (m *MongoRepository[T]) IndexManager() *MongoIndexManager[T] {
	return m.indexManager
}

// EnableTTL включает TTL (Time-To-Live) для автоматической очистки документов
func (m *MongoRepository[T]) EnableTTL(field string, duration time.Duration) error {
	ctx := context.Background()
	
	// Создаем TTL индекс
	indexModel := mongo.IndexModel{
		Keys: map[string]interface{}{field: 1},
		Options: options.Index().
			SetExpireAfterSeconds(int32(duration.Seconds())).
			SetName(fmt.Sprintf("ttl_%s", field)),
	}

	_, err := m.collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return fmt.Errorf("failed to create TTL index: %w", err)
	}

	m.config.TTLField = field
	m.config.TTLDuration = duration
	return nil
}

// DisableTTL отключает TTL индекс
func (m *MongoRepository[T]) DisableTTL() error {
	if m.config.TTLField == "" {
		return nil
	}

	ctx := context.Background()
	indexName := fmt.Sprintf("ttl_%s", m.config.TTLField)
	_, err := m.collection.Indexes().DropOne(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to drop TTL index: %w", err)
	}

	m.config.TTLField = ""
	m.config.TTLDuration = 0
	return nil
}

// WatchChanges возвращает ChangeStreamWatcher для реактивных обновлений
func (m *MongoRepository[T]) WatchChanges() *MongoChangeStreamWatcher[T] {
	if m.changeStreamWatcher == nil {
		m.changeStreamWatcher = NewMongoChangeStreamWatcher[T](m.collection)
	}
	return m.changeStreamWatcher
}

