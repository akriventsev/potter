// Package migrations предоставляет framework для управления миграциями схемы базы данных.
package migrations

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoMigrationDB реализация MigrationDB для MongoDB
type MongoMigrationDB struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
}

// NewMongoMigrationDB создает новый MongoMigrationDB
func NewMongoMigrationDB(uri, database string) (*MongoMigrationDB, error) {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(database)
	collection := db.Collection("schema_migrations")

	return &MongoMigrationDB{
		client:     client,
		database:   db,
		collection: collection,
	}, nil
}

// Exec выполняет команду (для MongoDB это может быть createCollection, createIndex и т.д.)
func (m *MongoMigrationDB) Exec(ctx context.Context, query string, args ...interface{}) error {
	// Для MongoDB "Exec" означает выполнение JavaScript команды или операции
	// В реальной реализации здесь должен быть парсер и выполнение команд
	// Пока возвращаем ошибку, так как это требует более сложной реализации
	return fmt.Errorf("Exec not fully implemented for MongoDB - use JavaScript migrations")
}

// Query выполняет запрос
func (m *MongoMigrationDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	// Для MongoDB Query означает find операцию
	// Пока возвращаем ошибку
	return nil, fmt.Errorf("Query not fully implemented for MongoDB")
}

// Begin начинает транзакцию (MongoDB 4.0+)
func (m *MongoMigrationDB) Begin(ctx context.Context) (Tx, error) {
	session, err := m.client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("failed to start session: %w", err)
	}

	if err := session.StartTransaction(); err != nil {
		session.EndSession(ctx)
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	return &MongoTx{
		session:  session,
		client:   m.client,
		database: m.database,
		ctx:      mongo.NewSessionContext(ctx, session),
	}, nil
}

// MongoRows обертка для mongo.Cursor
type MongoRows struct {
	cursor *mongo.Cursor
}

func (r *MongoRows) Next() bool {
	return r.cursor.Next(context.Background())
}

func (r *MongoRows) Scan(dest ...interface{}) error {
	return r.cursor.Decode(dest[0])
}

func (r *MongoRows) Close() error {
	return r.cursor.Close(context.Background())
}

// MongoTx обертка для mongo.Session
type MongoTx struct {
	session  mongo.Session
	client   *mongo.Client
	database *mongo.Database
	ctx      context.Context
}

func (t *MongoTx) Commit() error {
	if err := mongo.WithSession(context.Background(), t.session, func(sc mongo.SessionContext) error {
		return t.session.CommitTransaction(sc)
	}); err != nil {
		return err
	}
	t.session.EndSession(context.Background())
	return nil
}

func (t *MongoTx) Rollback() error {
	if err := mongo.WithSession(context.Background(), t.session, func(sc mongo.SessionContext) error {
		return t.session.AbortTransaction(sc)
	}); err != nil {
		return err
	}
	t.session.EndSession(context.Background())
	return nil
}

func (t *MongoTx) Exec(ctx context.Context, query string, args ...interface{}) error {
	// Для MongoDB Exec в транзакции - выполнение операций через session
	// Пока возвращаем ошибку
	return fmt.Errorf("Exec not fully implemented for MongoDB transactions")
}

// MongoMigrationHistory реализация MigrationHistory для MongoDB
type MongoMigrationHistory struct {
	db *MongoMigrationDB
}

// NewMongoMigrationHistory создает новый MongoMigrationHistory
func NewMongoMigrationHistory(db *MongoMigrationDB) *MongoMigrationHistory {
	return &MongoMigrationHistory{db: db}
}

// EnsureTable создает коллекцию schema_migrations
func (h *MongoMigrationHistory) EnsureTable(ctx context.Context) error {
	// Создаем коллекцию если не существует
	opts := options.CreateCollection()
	if err := h.db.database.CreateCollection(ctx, "schema_migrations", opts); err != nil {
		// Игнорируем ошибку если коллекция уже существует
		if mongo.IsDuplicateKeyError(err) {
			return nil
		}
		// Проверяем, существует ли коллекция
		collections, _ := h.db.database.ListCollectionNames(ctx, bson.M{"name": "schema_migrations"})
		if len(collections) > 0 {
			return nil
		}
		return fmt.Errorf("failed to create collection: %w", err)
	}

	// Создаем уникальный индекс на version
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := h.db.collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// Lock блокирует concurrent migrations через distributed lock
func (h *MongoMigrationHistory) Lock(ctx context.Context) error {
	lockCollection := h.db.database.Collection("migration_locks")

	// Создаем TTL индекс для автоматической очистки старых блокировок
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(300), // 5 минут
	}
	_, _ = lockCollection.Indexes().CreateOne(ctx, indexModel)

	// Пытаемся получить блокировку
	lockDoc := bson.M{
		"_id":        "migration_lock",
		"created_at": time.Now(),
	}

	opts := options.Update().SetUpsert(true)
	_, err := lockCollection.UpdateOne(
		ctx,
		bson.M{"_id": "migration_lock"},
		bson.M{"$set": lockDoc},
		opts,
	)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	return nil
}

// Unlock снимает блокировку
func (h *MongoMigrationHistory) Unlock(ctx context.Context) error {
	lockCollection := h.db.database.Collection("migration_locks")
	_, err := lockCollection.DeleteOne(ctx, bson.M{"_id": "migration_lock"})
	return err
}

// GetApplied возвращает список примененных версий
func (h *MongoMigrationHistory) GetApplied(ctx context.Context) ([]string, error) {
	cursor, err := h.db.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"version": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var versions []string
	for cursor.Next(ctx) {
		var record MigrationRecord
		if err := cursor.Decode(&record); err != nil {
			continue
		}
		versions = append(versions, record.Version)
	}

	return versions, nil
}

// GetAll возвращает все записи истории
func (h *MongoMigrationHistory) GetAll(ctx context.Context) ([]*MigrationRecord, error) {
	cursor, err := h.db.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"version": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var records []*MigrationRecord
	for cursor.Next(ctx) {
		var record MigrationRecord
		if err := cursor.Decode(&record); err != nil {
			continue
		}
		records = append(records, &record)
	}

	return records, nil
}

// RecordApplied записывает примененную миграцию
func (h *MongoMigrationHistory) RecordApplied(ctx context.Context, tx Tx, version, name string, executionTime int64, checksum string) error {
	record := bson.M{
		"_id":               version,
		"version":           version,
		"name":              name,
		"applied_at":        time.Now(),
		"execution_time_ms": executionTime,
		"checksum":          checksum,
	}

	mongoTx, ok := tx.(*MongoTx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := h.db.collection.InsertOne(mongoTx.ctx, record)
	return err
}

// RecordRollback записывает откат миграции
func (h *MongoMigrationHistory) RecordRollback(ctx context.Context, tx Tx, version string) error {
	mongoTx, ok := tx.(*MongoTx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := h.db.collection.DeleteOne(mongoTx.ctx, bson.M{"_id": version})
	return err
}
