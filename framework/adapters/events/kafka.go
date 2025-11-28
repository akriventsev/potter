// Package events предоставляет адаптеры для публикации доменных событий.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"potter/framework/core"
	"potter/framework/events"
	"potter/framework/metrics"

	"github.com/segmentio/kafka-go"
)

// KafkaEventConfig конфигурация для Kafka Event Publisher
type KafkaEventConfig struct {
	Brokers          []string
	TopicPrefix      string
	Partitioner      func(aggregateID string) int // Partitioning по aggregate ID
	Compression      string                       // none, gzip, snappy, lz4, zstd
	IdempotentWrites bool
	TransactionalID  string
	EnableMetrics    bool
	BatchSize        int
	FlushInterval    time.Duration
}

// DefaultKafkaEventConfig возвращает конфигурацию Kafka Event Publisher по умолчанию
func DefaultKafkaEventConfig() KafkaEventConfig {
	return KafkaEventConfig{
		Brokers:          []string{"localhost:9092"},
		TopicPrefix:      "events",
		Compression:      "snappy",
		IdempotentWrites: true,
		EnableMetrics:    true,
		BatchSize:        100,
		FlushInterval:    10 * time.Millisecond,
	}
}

// KafkaEventAdapter реализация Event Publisher через Kafka
type KafkaEventAdapter struct {
	config  KafkaEventConfig
	writer  *kafka.Writer
	metrics *metrics.Metrics
	running bool
}

// NewKafkaEventAdapter создает новый Kafka Event Publisher
func NewKafkaEventAdapter(config KafkaEventConfig) (*KafkaEventAdapter, error) {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Balancer:     &kafka.Hash{}, // Hash partitioning для гарантии порядка
		RequiredAcks: -1,            // all replicas
		Async:        false,
		BatchSize:    config.BatchSize,
		BatchTimeout: config.FlushInterval,
		Compression:  getKafkaCompression(config.Compression),
		WriteTimeout: 10 * time.Second,
	}

	if config.IdempotentWrites {
		writer.RequiredAcks = -1 // all replicas
	}

	// NOTE: Transactional writes are not yet implemented. Use config.TransactionalID for future compatibility.
	// The current version of kafka-go library does not support transactional transport configuration.
	// Transactional ID is set for future compatibility but not used in current implementation
	_ = config.TransactionalID

	adapter := &KafkaEventAdapter{
		config:  config,
		writer:  writer,
		running: false,
	}

	if config.EnableMetrics {
		var err error
		adapter.metrics, err = metrics.NewMetrics()
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics: %w", err)
		}
	}

	return adapter, nil
}

// getKafkaCompression преобразует строку в kafka.Compression
func getKafkaCompression(compression string) kafka.Compression {
	switch compression {
	case "gzip":
		return kafka.Gzip
	case "snappy":
		return kafka.Snappy
	case "lz4":
		return kafka.Lz4
	case "zstd":
		return kafka.Zstd
	default:
		return kafka.Compression(0) // zero value - no compression
	}
}

// Start запускает адаптер (реализация core.Lifecycle)
func (k *KafkaEventAdapter) Start(ctx context.Context) error {
	k.running = true
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (k *KafkaEventAdapter) Stop(ctx context.Context) error {
	k.running = false
	if k.writer != nil {
		return k.writer.Close()
	}
	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (k *KafkaEventAdapter) IsRunning() bool {
	return k.running
}

// Name возвращает имя компонента (реализация core.Component)
func (k *KafkaEventAdapter) Name() string {
	return "kafka-event-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (k *KafkaEventAdapter) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// Publish публикует событие
func (k *KafkaEventAdapter) Publish(ctx context.Context, event events.Event) error {

	// Формируем topic по шаблону: events.{aggregate_type}.{event_type}
	topic := k.getTopic(event)

	// Сериализуем событие
	data, err := k.serializeEvent(event)
	if err != nil {
		if k.metrics != nil {
			k.metrics.RecordEvent(ctx, event.EventType())
		}
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Формируем Kafka message
	msg := kafka.Message{
		Topic: topic,
		Value: data,
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte(event.EventID())},
			{Key: "event_type", Value: []byte(event.EventType())},
			{Key: "aggregate_id", Value: []byte(event.AggregateID())},
			{Key: "occurred_at", Value: []byte(event.OccurredAt().Format(time.RFC3339))},
		},
	}

	// Добавляем версию события в headers (для версионирования)
	if metadata := event.Metadata(); metadata != nil {
		if version, ok := metadata.Get("version"); ok {
			if versionStr, ok := version.(string); ok {
				msg.Headers = append(msg.Headers, kafka.Header{
					Key:   "event_version",
					Value: []byte(versionStr),
				})
			}
		}

		if correlationID := metadata.CorrelationID(); correlationID != "" {
			msg.Headers = append(msg.Headers, kafka.Header{
				Key:   "correlation_id",
				Value: []byte(correlationID),
			})
		}

		if causationID := metadata.CausationID(); causationID != "" {
			msg.Headers = append(msg.Headers, kafka.Header{
				Key:   "causation_id",
				Value: []byte(causationID),
			})
		}
	}

	// Partitioning по aggregate ID для гарантии порядка
	if k.config.Partitioner != nil {
		partition := k.config.Partitioner(event.AggregateID())
		msg.Partition = partition
	}

	// Публикуем событие
	err = k.writer.WriteMessages(ctx, msg)
	if err != nil {
		if k.metrics != nil {
			k.metrics.RecordEvent(ctx, event.EventType())
		}
		return fmt.Errorf("failed to publish event: %w", err)
	}

	if k.metrics != nil {
		k.metrics.RecordEvent(ctx, event.EventType())
	}

	return nil
}

// getTopic формирует topic для события
func (k *KafkaEventAdapter) getTopic(event events.Event) string {
	aggregateID := event.AggregateID()
	aggregateType := "unknown"

	if len(aggregateID) > 0 {
		parts := splitAggregateID(aggregateID)
		if len(parts) > 0 {
			aggregateType = parts[0]
		}
	}

	// Формат: events.{aggregate_type}.{event_type}
	return fmt.Sprintf("%s.%s.%s", k.config.TopicPrefix, aggregateType, event.EventType())
}

// serializeEvent сериализует событие
func (k *KafkaEventAdapter) serializeEvent(event events.Event) ([]byte, error) {
	eventData := map[string]interface{}{
		"event_id":     event.EventID(),
		"event_type":   event.EventType(),
		"aggregate_id": event.AggregateID(),
		"occurred_at":  event.OccurredAt().Format(time.RFC3339),
	}

	if metadata := event.Metadata(); metadata != nil {
		eventData["metadata"] = metadata
	}

	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	var eventPayload map[string]interface{}
	if err := json.Unmarshal(data, &eventPayload); err != nil {
		return nil, err
	}

	for k, v := range eventPayload {
		if _, exists := eventData[k]; !exists {
			eventData[k] = v
		}
	}

	return json.Marshal(eventData)
}
