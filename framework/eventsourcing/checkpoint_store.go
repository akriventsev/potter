// Package eventsourcing предоставляет полную поддержку Event Sourcing паттерна.
package eventsourcing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CheckpointStore интерфейс для сохранения позиций проекций
type CheckpointStore interface {
	SaveCheckpoint(ctx context.Context, projectionName string, position int64) error
	GetCheckpoint(ctx context.Context, projectionName string) (int64, error)
	DeleteCheckpoint(ctx context.Context, projectionName string) error
	ListCheckpoints(ctx context.Context) (map[string]int64, error)
}

// PostgresCheckpointStore реализация CheckpointStore для PostgreSQL
type PostgresCheckpointStore struct {
	conn *pgx.Conn
}

// NewPostgresCheckpointStore создает новый PostgresCheckpointStore
func NewPostgresCheckpointStore(dsn string) (*PostgresCheckpointStore, error) {
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	store := &PostgresCheckpointStore{conn: conn}
	if err := store.ensureTable(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure table: %w", err)
	}

	return store, nil
}

func (s *PostgresCheckpointStore) ensureTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS projection_checkpoints (
			projection_name VARCHAR(255) PRIMARY KEY,
			position BIGINT NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`
	_, err := s.conn.Exec(ctx, query)
	return err
}

func (s *PostgresCheckpointStore) SaveCheckpoint(ctx context.Context, projectionName string, position int64) error {
	query := `
		INSERT INTO projection_checkpoints (projection_name, position, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (projection_name) 
		DO UPDATE SET position = $2, updated_at = NOW()
	`
	_, err := s.conn.Exec(ctx, query, projectionName, position)
	return err
}

func (s *PostgresCheckpointStore) GetCheckpoint(ctx context.Context, projectionName string) (int64, error) {
	query := `SELECT position FROM projection_checkpoints WHERE projection_name = $1`
	var position int64
	err := s.conn.QueryRow(ctx, query, projectionName).Scan(&position)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return position, nil
}

func (s *PostgresCheckpointStore) DeleteCheckpoint(ctx context.Context, projectionName string) error {
	query := `DELETE FROM projection_checkpoints WHERE projection_name = $1`
	_, err := s.conn.Exec(ctx, query, projectionName)
	return err
}

func (s *PostgresCheckpointStore) ListCheckpoints(ctx context.Context) (map[string]int64, error) {
	query := `SELECT projection_name, position FROM projection_checkpoints`
	rows, err := s.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	checkpoints := make(map[string]int64)
	for rows.Next() {
		var name string
		var position int64
		if err := rows.Scan(&name, &position); err != nil {
			continue
		}
		checkpoints[name] = position
	}

	return checkpoints, nil
}

// MongoCheckpointStore реализация CheckpointStore для MongoDB
type MongoCheckpointStore struct {
	collection *mongo.Collection
}

// NewMongoCheckpointStore создает новый MongoCheckpointStore
func NewMongoCheckpointStore(uri, database string) (*MongoCheckpointStore, error) {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	collection := client.Database(database).Collection("projection_checkpoints")
	store := &MongoCheckpointStore{collection: collection}
	if err := store.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure indexes: %w", err)
	}

	return store, nil
}

func (s *MongoCheckpointStore) ensureIndexes(ctx context.Context) error {
	// Создаем уникальный индекс по _id (используется как идентификатор проекции)
	// _id уже имеет уникальный индекс по умолчанию, но явно создаем для ясности
	// Также создаем индекс по projection_name для удобства запросов
	indexModels := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "projection_name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := s.collection.Indexes().CreateMany(ctx, indexModels)
	return err
}

func (s *MongoCheckpointStore) SaveCheckpoint(ctx context.Context, projectionName string, position int64) error {
	filter := bson.M{"_id": projectionName}
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"projection_name": projectionName,
			"position":        position,
			"updated_at":      now,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := s.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

func (s *MongoCheckpointStore) GetCheckpoint(ctx context.Context, projectionName string) (int64, error) {
	var result struct {
		Position int64 `bson:"position"`
	}
	err := s.collection.FindOne(ctx, bson.M{"_id": projectionName}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, err
	}
	return result.Position, nil
}

func (s *MongoCheckpointStore) DeleteCheckpoint(ctx context.Context, projectionName string) error {
	_, err := s.collection.DeleteOne(ctx, bson.M{"_id": projectionName})
	return err
}

func (s *MongoCheckpointStore) ListCheckpoints(ctx context.Context) (map[string]int64, error) {
	cursor, err := s.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	checkpoints := make(map[string]int64)
	for cursor.Next(ctx) {
		var result struct {
			ID       string `bson:"_id"`
			Position int64  `bson:"position"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		checkpoints[result.ID] = result.Position
	}

	return checkpoints, nil
}

// InMemoryCheckpointStore реализация CheckpointStore в памяти для тестирования
type InMemoryCheckpointStore struct {
	checkpoints map[string]int64
}

// NewInMemoryCheckpointStore создает новый InMemoryCheckpointStore
func NewInMemoryCheckpointStore() *InMemoryCheckpointStore {
	return &InMemoryCheckpointStore{
		checkpoints: make(map[string]int64),
	}
}

func (s *InMemoryCheckpointStore) SaveCheckpoint(ctx context.Context, projectionName string, position int64) error {
	s.checkpoints[projectionName] = position
	return nil
}

func (s *InMemoryCheckpointStore) GetCheckpoint(ctx context.Context, projectionName string) (int64, error) {
	position, exists := s.checkpoints[projectionName]
	if !exists {
		return 0, nil
	}
	return position, nil
}

func (s *InMemoryCheckpointStore) DeleteCheckpoint(ctx context.Context, projectionName string) error {
	delete(s.checkpoints, projectionName)
	return nil
}

func (s *InMemoryCheckpointStore) ListCheckpoints(ctx context.Context) (map[string]int64, error) {
	result := make(map[string]int64)
	for k, v := range s.checkpoints {
		result[k] = v
	}
	return result, nil
}

