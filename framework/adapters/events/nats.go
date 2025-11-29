// Package events предоставляет адаптеры для публикации доменных событий.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/metrics"
	"github.com/akriventsev/potter/framework/transport"

	"github.com/nats-io/nats.go"
)

// NATSEventConfig конфигурация для NATS Event Publisher
type NATSEventConfig struct {
	Conn          *nats.Conn
	SubjectPrefix string
	Serializer    transport.MessageSerializer
	RetryPolicy   events.RetryConfig
	EnableMetrics bool
}

// DefaultNATSEventConfig возвращает конфигурацию NATS Event Publisher по умолчанию
func DefaultNATSEventConfig() NATSEventConfig {
	return NATSEventConfig{
		SubjectPrefix: "events",
		Serializer:    &JSONSerializer{},
		RetryPolicy:   events.DefaultRetryConfig(),
		EnableMetrics: true,
	}
}

// NATSEventAdapter реализация Event Publisher через NATS
type NATSEventAdapter struct {
	config  NATSEventConfig
	conn    *nats.Conn
	metrics *metrics.Metrics
	running bool
}

// NewNATSEventAdapter создает новый NATS Event Publisher
func NewNATSEventAdapter(config NATSEventConfig) (*NATSEventAdapter, error) {
	if config.Conn == nil {
		return nil, fmt.Errorf("NATS connection is required")
	}

	adapter := &NATSEventAdapter{
		config:  config,
		conn:    config.Conn,
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

// Start запускает адаптер (реализация core.Lifecycle)
func (n *NATSEventAdapter) Start(ctx context.Context) error {
	n.running = true
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (n *NATSEventAdapter) Stop(ctx context.Context) error {
	n.running = false
	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (n *NATSEventAdapter) IsRunning() bool {
	return n.running
}

// Name возвращает имя компонента (реализация core.Component)
func (n *NATSEventAdapter) Name() string {
	return "nats-event-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (n *NATSEventAdapter) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// Publish публикует событие
func (n *NATSEventAdapter) Publish(ctx context.Context, event events.Event) error {

	// Формируем subject по шаблону: events.{aggregate}.{event_type}
	subject := n.getSubject(event)

	// Сериализуем событие
	data, err := n.serializeEvent(event)
	if err != nil {
		if n.metrics != nil {
			n.metrics.RecordEvent(ctx, event.EventType())
		}
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Публикуем с retry
	err = n.publishWithRetry(ctx, subject, data, event)
	if err != nil {
		if n.metrics != nil {
			n.metrics.RecordEvent(ctx, event.EventType())
		}
		return err
	}

	if n.metrics != nil {
		n.metrics.RecordEvent(ctx, event.EventType())
	}

	return nil
}

// getSubject формирует subject для события
func (n *NATSEventAdapter) getSubject(event events.Event) string {
	// Извлекаем aggregate type из aggregate ID (предполагаем формат: {aggregate_type}-{id})
	aggregateID := event.AggregateID()
	aggregateType := "unknown"

	// Пытаемся извлечь тип агрегата
	if len(aggregateID) > 0 {
		parts := splitAggregateID(aggregateID)
		if len(parts) > 0 {
			aggregateType = parts[0]
		}
	}

	// Формат: events.{aggregate_type}.{event_type}
	return fmt.Sprintf("%s.%s.%s", n.config.SubjectPrefix, aggregateType, event.EventType())
}

// serializeEvent сериализует событие
func (n *NATSEventAdapter) serializeEvent(event events.Event) ([]byte, error) {
	if n.config.Serializer != nil {
		return n.config.Serializer.Serialize(event)
	}

	// Default JSON serialization
	eventData := map[string]interface{}{
		"event_id":     event.EventID(),
		"event_type":   event.EventType(),
		"aggregate_id": event.AggregateID(),
		"occurred_at":  event.OccurredAt().Format(time.RFC3339),
	}

	// Добавляем метаданные
	if metadata := event.Metadata(); metadata != nil {
		eventData["metadata"] = metadata
	}

	// Сериализуем специфичные данные события
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	var eventPayload map[string]interface{}
	if err := json.Unmarshal(data, &eventPayload); err != nil {
		return nil, err
	}

	// Объединяем базовые поля с payload
	for k, v := range eventPayload {
		if _, exists := eventData[k]; !exists {
			eventData[k] = v
		}
	}

	return json.Marshal(eventData)
}

// publishWithRetry публикует событие с retry логикой
func (n *NATSEventAdapter) publishWithRetry(ctx context.Context, subject string, data []byte, event events.Event) error {
	retryConfig := n.config.RetryPolicy
	delay := retryConfig.InitialDelay

	for attempt := 0; attempt < retryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := n.conn.Publish(subject, data)
		if err == nil {
			return nil
		}

		// Увеличиваем задержку для следующей попытки
		delay = time.Duration(float64(delay) * retryConfig.BackoffMultiplier)
		if delay > retryConfig.MaxDelay {
			delay = retryConfig.MaxDelay
		}
	}

	return fmt.Errorf("failed to publish event after %d attempts", retryConfig.MaxAttempts)
}

// JSONSerializer реализация MessageSerializer для JSON
type JSONSerializer struct{}

// Serialize сериализует сообщение в JSON
func (j *JSONSerializer) Serialize(msg interface{}) ([]byte, error) {
	return json.Marshal(msg)
}

// Deserialize десериализует JSON в сообщение
func (j *JSONSerializer) Deserialize(data []byte, msg interface{}) error {
	return json.Unmarshal(data, msg)
}
