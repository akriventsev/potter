package eventsourcing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"potter/framework/core"
	"potter/framework/events"
)

// MongoDBEventStoreConfig конфигурация для MongoDB Event Store
type MongoDBEventStoreConfig struct {
	URI            string
	Database       string
	Collection     string
	Timeout        int // в секундах
	MaxPoolSize    int
	MinPoolSize    int
}

// Validate проверяет корректность конфигурации
func (c MongoDBEventStoreConfig) Validate() error {
	if c.URI == "" {
		return fmt.Errorf("URI cannot be empty")
	}
	if c.Database == "" {
		c.Database = "potter"
	}
	if c.Collection == "" {
		c.Collection = "events"
	}
	return nil
}

// DefaultMongoDBEventStoreConfig возвращает конфигурацию по умолчанию
func DefaultMongoDBEventStoreConfig() MongoDBEventStoreConfig {
	return MongoDBEventStoreConfig{
		Database:    "potter",
		Collection: "events",
		Timeout:    10,
		MaxPoolSize: 100,
		MinPoolSize: 10,
	}
}

// MongoDBEventStore реализация EventStore для MongoDB
type MongoDBEventStore struct {
	config       MongoDBEventStoreConfig
	client       *mongo.Client
	collection   *mongo.Collection
	deserializer EventDeserializer
}

// NewMongoDBEventStore создает новый MongoDB Event Store
func NewMongoDBEventStore(config MongoDBEventStoreConfig) (*MongoDBEventStore, error) {
	return NewMongoDBEventStoreWithDeserializer(config, nil)
}

// NewMongoDBEventStoreWithDeserializer создает новый MongoDB Event Store с десериализатором
func NewMongoDBEventStoreWithDeserializer(config MongoDBEventStoreConfig, deserializer EventDeserializer) (*MongoDBEventStore, error) {
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

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	collection := client.Database(config.Database).Collection(config.Collection)

	// Создаем индексы
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "aggregate_id", Value: 1},
				{Key: "version", Value: 1},
			},
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

	_, err = collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return &MongoDBEventStore{
		config:       config,
		client:       client,
		collection:   collection,
		deserializer: deserializer,
	}, nil
}

// Start запускает адаптер
func (s *MongoDBEventStore) Start(ctx context.Context) error {
	return nil
}

// Stop останавливает адаптер
func (s *MongoDBEventStore) Stop(ctx context.Context) error {
	if s.client != nil {
		return s.client.Disconnect(ctx)
	}
	return nil
}

// IsRunning проверяет, запущен ли адаптер
func (s *MongoDBEventStore) IsRunning() bool {
	return s.client != nil
}

// Name возвращает имя компонента
func (s *MongoDBEventStore) Name() string {
	return "mongodb-event-store"
}

// Type возвращает тип компонента
func (s *MongoDBEventStore) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// AppendEvents добавляет события в поток агрегата
func (s *MongoDBEventStore) AppendEvents(ctx context.Context, aggregateID string, expectedVersion int64, events []events.Event) error {
	// Начинаем транзакцию
	session, err := s.client.StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	err = mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		if err := session.StartTransaction(); err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}

		// Проверяем текущую версию
		filter := bson.M{"aggregate_id": aggregateID}
		opts := options.Find().SetSort(bson.D{{Key: "version", Value: -1}}).SetLimit(1)
		cursor, err := s.collection.Find(sc, filter, opts)
		if err != nil {
			return fmt.Errorf("failed to find events: %w", err)
		}

		var lastEvent bson.M
		currentVersion := int64(0)
		if cursor.Next(sc) {
			if err := cursor.Decode(&lastEvent); err != nil {
				cursor.Close(sc)
				return fmt.Errorf("failed to decode event: %w", err)
			}
			currentVersion = getInt64(lastEvent, "version")
		}
		cursor.Close(sc)

		// Проверяем оптимистичную конкурентность
		if expectedVersion != currentVersion {
			session.AbortTransaction(sc)
			return fmt.Errorf("%w: expected %d, got %d", ErrConcurrencyConflict, expectedVersion, currentVersion)
		}

		// Получаем текущую позицию
		var lastDoc bson.M
		opts2 := options.FindOne().SetSort(bson.D{{Key: "position", Value: -1}})
		err = s.collection.FindOne(sc, bson.M{}, opts2).Decode(&lastDoc)
		position := int64(0)
		if err == nil {
			position = getInt64(lastDoc, "position")
		}

		// Вставляем события
		docs := make([]interface{}, len(events))
		for i, event := range events {
			eventData, err := json.Marshal(event)
			if err != nil {
				return fmt.Errorf("failed to marshal event: %w", err)
			}

			position++
			doc := bson.M{
				"aggregate_id":  aggregateID,
				"aggregate_type": getAggregateType(event),
				"event_type":    event.EventType(),
				"event_data":    bson.Raw(eventData),
				"metadata":      convertMetadata(event.Metadata()),
				"version":       expectedVersion + int64(i) + 1,
				"position":      position,
				"occurred_at":   event.OccurredAt(),
				"created_at":    time.Now(),
			}
			docs[i] = doc
		}

		_, err = s.collection.InsertMany(sc, docs)
		if err != nil {
			session.AbortTransaction(sc)
			return fmt.Errorf("failed to insert events: %w", err)
		}

		return session.CommitTransaction(sc)
	})

	return err
}

// GetEvents возвращает события агрегата
func (s *MongoDBEventStore) GetEvents(ctx context.Context, aggregateID string, fromVersion int64) ([]StoredEvent, error) {
	filter := bson.M{
		"aggregate_id": aggregateID,
		"version":      bson.M{"$gte": fromVersion},
	}
	opts := options.Find().SetSort(bson.D{{Key: "version", Value: 1}})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find events: %w", err)
	}
	defer cursor.Close(ctx)

	var result []StoredEvent
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		stored := StoredEvent{
			AggregateID:  aggregateID,
			EventType:    getString(doc, "event_type"),
			Version:      getInt64(doc, "version"),
			Position:     getInt64(doc, "position"),
			OccurredAt:   getTime(doc, "occurred_at"),
			CreatedAt:    getTime(doc, "created_at"),
		}

		if id, ok := doc["_id"].(string); ok {
			stored.ID = id
		}

		if metadata, ok := doc["metadata"].(bson.M); ok {
			stored.Metadata = convertBSONToMap(metadata)
		}

		// Десериализуем eventData обратно в events.Event
		if eventDataRaw, ok := doc["event_data"]; ok && s.deserializer != nil {
			var eventDataBytes []byte
			if raw, ok := eventDataRaw.(bson.Raw); ok {
				eventDataBytes = raw
			} else if bytes, ok := eventDataRaw.([]byte); ok {
				eventDataBytes = bytes
			} else {
				// Пытаемся преобразовать в JSON
				if jsonBytes, err := bson.MarshalExtJSON(eventDataRaw, false, false); err == nil {
					eventDataBytes = jsonBytes
				}
			}
			if len(eventDataBytes) > 0 {
				event, err := s.deserializer.DeserializeEvent(stored.EventType, eventDataBytes)
				if err == nil {
					stored.EventData = event
				}
			}
		}

		result = append(result, stored)
	}

	if len(result) == 0 && fromVersion > 0 {
		return nil, ErrStreamNotFound
	}

	return result, nil
}

// GetEventsByType возвращает события определенного типа
func (s *MongoDBEventStore) GetEventsByType(ctx context.Context, eventType string, fromTimestamp time.Time) ([]StoredEvent, error) {
	filter := bson.M{
		"event_type":  eventType,
		"occurred_at": bson.M{"$gte": fromTimestamp},
	}
	opts := options.Find().SetSort(bson.D{{Key: "position", Value: 1}})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find events by type: %w", err)
	}
	defer cursor.Close(ctx)

	var result []StoredEvent
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		stored := StoredEvent{
			AggregateID:  getString(doc, "aggregate_id"),
			AggregateType: getString(doc, "aggregate_type"),
			EventType:    eventType,
			Version:      getInt64(doc, "version"),
			Position:     getInt64(doc, "position"),
			OccurredAt:   getTime(doc, "occurred_at"),
			CreatedAt:    getTime(doc, "created_at"),
		}

		if id, ok := doc["_id"].(string); ok {
			stored.ID = id
		}

		if metadata, ok := doc["metadata"].(bson.M); ok {
			stored.Metadata = convertBSONToMap(metadata)
		}

		result = append(result, stored)
	}

	return result, nil
}

// GetAllEvents возвращает все события начиная с указанной позиции
func (s *MongoDBEventStore) GetAllEvents(ctx context.Context, fromPosition int64) (<-chan StoredEvent, error) {
	ch := make(chan StoredEvent, 100)

	go func() {
		defer close(ch)
		filter := bson.M{"position": bson.M{"$gte": fromPosition}}
		opts := options.Find().SetSort(bson.D{{Key: "position", Value: 1}})

		cursor, err := s.collection.Find(ctx, filter, opts)
		if err != nil {
			return
		}
		defer cursor.Close(ctx)

		for cursor.Next(ctx) {
			var doc bson.M
			if err := cursor.Decode(&doc); err != nil {
				continue
			}

			stored := StoredEvent{
				AggregateID:  getString(doc, "aggregate_id"),
				AggregateType: getString(doc, "aggregate_type"),
				EventType:    getString(doc, "event_type"),
				Version:      getInt64(doc, "version"),
				Position:     getInt64(doc, "position"),
				OccurredAt:   getTime(doc, "occurred_at"),
				CreatedAt:    getTime(doc, "created_at"),
			}

			if id, ok := doc["_id"].(string); ok {
				stored.ID = id
			}

			if metadata, ok := doc["metadata"].(bson.M); ok {
				stored.Metadata = convertBSONToMap(metadata)
			}

			select {
			case ch <- stored:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// MongoDBSnapshotStore реализация SnapshotStore для MongoDB
type MongoDBSnapshotStore struct {
	config     MongoDBEventStoreConfig
	client     *mongo.Client
	collection *mongo.Collection
}

// NewMongoDBSnapshotStore создает новый MongoDB Snapshot Store
func NewMongoDBSnapshotStore(config MongoDBEventStoreConfig) (*MongoDBSnapshotStore, error) {
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

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	collectionName := "snapshots"
	if config.Collection != "" {
		collectionName = config.Collection + "_snapshots"
	}
	collection := client.Database(config.Database).Collection(collectionName)

	// Создаем TTL индекс для автоматической очистки
	indexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(86400 * 90), // 90 дней
	}
	_, err = collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create TTL index: %w", err)
	}

	return &MongoDBSnapshotStore{
		config:     config,
		client:     client,
		collection: collection,
	}, nil
}

// SaveSnapshot сохраняет снапшот
func (s *MongoDBSnapshotStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	doc := bson.M{
		"_id":            snapshot.AggregateID,
		"aggregate_type": snapshot.AggregateType,
		"version":        snapshot.Version,
		"state":          snapshot.State,
		"metadata":       snapshot.Metadata,
		"created_at":     snapshot.CreatedAt,
		"updated_at":     time.Now(),
	}

	opts := options.Replace().SetUpsert(true)
	_, err := s.collection.ReplaceOne(ctx, bson.M{"_id": snapshot.AggregateID}, doc, opts)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	return nil
}

// GetSnapshot возвращает последний снапшот
func (s *MongoDBSnapshotStore) GetSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error) {
	var doc bson.M
	err := s.collection.FindOne(ctx, bson.M{"_id": aggregateID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	snapshot := &Snapshot{
		AggregateID:  aggregateID,
		AggregateType: getString(doc, "aggregate_type"),
		Version:      getInt64(doc, "version"),
		CreatedAt:    getTime(doc, "created_at"),
	}

	if state, ok := doc["state"].(bson.Raw); ok {
		snapshot.State = state
	}

	if metadata, ok := doc["metadata"].(bson.M); ok {
		snapshot.Metadata = convertBSONToMap(metadata)
	}

	return snapshot, nil
}

// DeleteSnapshots удаляет старые снапшоты
func (s *MongoDBSnapshotStore) DeleteSnapshots(ctx context.Context, aggregateID string, beforeVersion int64) error {
	filter := bson.M{
		"_id":     aggregateID,
		"version": bson.M{"$lt": beforeVersion},
	}

	_, err := s.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete snapshots: %w", err)
	}

	return nil
}

// Вспомогательные функции
func getString(doc bson.M, key string) string {
	if val, ok := doc[key].(string); ok {
		return val
	}
	return ""
}

func getInt64(doc bson.M, key string) int64 {
	val, ok := doc[key]
	if !ok {
		return 0
	}
	
	// Поддерживаем различные числовые типы
	switch v := val.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		// Пытаемся преобразовать через bson
		if num, ok := val.(bson.RawValue); ok {
			if num.Type == bson.TypeInt32 {
				var i32 int32
				if err := num.Unmarshal(&i32); err == nil {
					return int64(i32)
				}
			} else if num.Type == bson.TypeInt64 {
				var i64 int64
				if err := num.Unmarshal(&i64); err == nil {
					return i64
				}
			} else if num.Type == bson.TypeDouble {
				var d float64
				if err := num.Unmarshal(&d); err == nil {
					return int64(d)
				}
			}
		}
		return 0
	}
}

func getTime(doc bson.M, key string) time.Time {
	if val, ok := doc[key].(time.Time); ok {
		return val
	}
	return time.Now()
}

func convertBSONToMap(bsonMap bson.M) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range bsonMap {
		result[k] = v
	}
	return result
}

