// Package repository предоставляет generic адаптеры для работы с различными storage backends.
package repository

import (
	"context"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ChangeStreamOperationType тип операции в change stream
type ChangeStreamOperationType string

const (
	ChangeStreamInsert     ChangeStreamOperationType = "insert"
	ChangeStreamUpdate     ChangeStreamOperationType = "update"
	ChangeStreamReplace    ChangeStreamOperationType = "replace"
	ChangeStreamDelete     ChangeStreamOperationType = "delete"
	ChangeStreamInvalidate ChangeStreamOperationType = "invalidate"
)

// ChangeEvent событие изменения в MongoDB
type ChangeEvent struct {
	OperationType     ChangeStreamOperationType
	FullDocument      bson.M
	DocumentKey       bson.M
	UpdateDescription *UpdateDescription
	ClusterTime       primitive.Timestamp
	Timestamp         int64
}

// UpdateDescription описание изменений в документе
type UpdateDescription struct {
	UpdatedFields bson.M
	RemovedFields []string
}

// ChangeStreamWatcher интерфейс для отслеживания изменений
type ChangeStreamWatcher interface {
	Watch(ctx context.Context, pipeline []bson.D) (<-chan ChangeEvent, error)
	WatchCollection(ctx context.Context) (<-chan ChangeEvent, error)
	WatchDatabase(ctx context.Context) (<-chan ChangeEvent, error)
	Close() error
}

// ChangeStreamHandler интерфейс для обработчиков изменений
type ChangeStreamHandler interface {
	HandleChange(ctx context.Context, event ChangeEvent) error
}

// ChangeStreamOptions опции для change stream
type ChangeStreamOptions struct {
	FullDocument         string // "updateLookup" или "default"
	ResumeAfter          bson.M
	StartAtOperationTime *primitive.Timestamp
	BatchSize            int32
}

// MongoChangeStreamWatcher реализация ChangeStreamWatcher для MongoDB
type MongoChangeStreamWatcher[T Entity] struct {
	collection *mongo.Collection
	stream     *mongo.ChangeStream
	mu         sync.RWMutex
	running    bool
}

// NewMongoChangeStreamWatcher создает новый MongoChangeStreamWatcher
func NewMongoChangeStreamWatcher[T Entity](collection *mongo.Collection) *MongoChangeStreamWatcher[T] {
	return &MongoChangeStreamWatcher[T]{
		collection: collection,
		running:    false,
	}
}

// Watch отслеживает изменения с использованием aggregation pipeline
func (w *MongoChangeStreamWatcher[T]) Watch(ctx context.Context, pipeline []bson.D) (<-chan ChangeEvent, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return nil, fmt.Errorf("change stream already running")
	}

	opts := options.ChangeStream().
		SetFullDocument(options.UpdateLookup)

	stream, err := w.collection.Watch(ctx, pipeline, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create change stream: %w", err)
	}

	w.stream = stream
	w.running = true

	events := make(chan ChangeEvent, 100)

	go w.watchLoop(ctx, events)

	return events, nil
}

// WatchCollection отслеживает изменения в коллекции
func (w *MongoChangeStreamWatcher[T]) WatchCollection(ctx context.Context) (<-chan ChangeEvent, error) {
	return w.Watch(ctx, []bson.D{})
}

// WatchDatabase отслеживает изменения во всей базе данных
func (w *MongoChangeStreamWatcher[T]) WatchDatabase(ctx context.Context) (<-chan ChangeEvent, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return nil, fmt.Errorf("change stream already running")
	}

	db := w.collection.Database()
	opts := options.ChangeStream().
		SetFullDocument(options.UpdateLookup)

	stream, err := db.Watch(ctx, []bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create database change stream: %w", err)
	}

	w.stream = stream
	w.running = true

	events := make(chan ChangeEvent, 100)

	go w.watchLoop(ctx, events)

	return events, nil
}

// watchLoop основной цикл отслеживания изменений
func (w *MongoChangeStreamWatcher[T]) watchLoop(ctx context.Context, events chan<- ChangeEvent) {
	defer close(events)
	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	for {
		if !w.stream.Next(ctx) {
			if w.stream.Err() != nil {
				// Логируем ошибку, но продолжаем попытки переподключения
				// В production здесь должна быть логика retry с exponential backoff
				continue
			}
			break
		}

		var changeDoc bson.M
		if err := w.stream.Decode(&changeDoc); err != nil {
			continue
		}

		event := w.parseChangeEvent(changeDoc)
		select {
		case events <- event:
		case <-ctx.Done():
			return
		}
	}
}

// parseChangeEvent парсит change event из MongoDB документа
func (w *MongoChangeStreamWatcher[T]) parseChangeEvent(doc bson.M) ChangeEvent {
	event := ChangeEvent{}

	if opType, ok := doc["operationType"].(string); ok {
		event.OperationType = ChangeStreamOperationType(opType)
	}

	if fullDoc, ok := doc["fullDocument"].(bson.M); ok {
		event.FullDocument = fullDoc
	}

	if docKey, ok := doc["documentKey"].(bson.M); ok {
		event.DocumentKey = docKey
	}

	if updateDesc, ok := doc["updateDescription"].(bson.M); ok {
		event.UpdateDescription = &UpdateDescription{}
		if updated, ok := updateDesc["updatedFields"].(bson.M); ok {
			event.UpdateDescription.UpdatedFields = updated
		}
		if removed, ok := updateDesc["removedFields"].(bson.A); ok {
			fields := make([]string, len(removed))
			for i, v := range removed {
				if str, ok := v.(string); ok {
					fields[i] = str
				}
			}
			event.UpdateDescription.RemovedFields = fields
		}
	}

	if clusterTime, ok := doc["clusterTime"].(primitive.Timestamp); ok {
		event.ClusterTime = clusterTime
		event.Timestamp = int64(clusterTime.T)
	}

	return event
}

// Close закрывает change stream
func (w *MongoChangeStreamWatcher[T]) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stream != nil {
		if err := w.stream.Close(context.Background()); err != nil {
			return fmt.Errorf("failed to close change stream: %w", err)
		}
		w.stream = nil
	}
	w.running = false
	return nil
}
