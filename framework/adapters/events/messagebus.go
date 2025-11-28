// Package events предоставляет адаптеры для публикации доменных событий.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"potter/framework/core"
	"potter/framework/events"
	"potter/framework/metrics"
	"potter/framework/transport"
)

// MessageBusEventConfig конфигурация для MessageBus Event Publisher
type MessageBusEventConfig struct {
	Bus           transport.Publisher
	SubjectPrefix string
	HeaderMapping map[string]string // Маппинг полей события в headers
	Serializer    transport.MessageSerializer
	RetryPolicy   events.RetryConfig
	EnableBatch   bool
	BatchSize     int
	BatchTimeout  time.Duration
	EnableMetrics bool
}

// DefaultMessageBusEventConfig возвращает конфигурацию MessageBus Event Publisher по умолчанию
func DefaultMessageBusEventConfig() MessageBusEventConfig {
	return MessageBusEventConfig{
		SubjectPrefix: "events",
		HeaderMapping: map[string]string{
			"event_type":   "event_type",
			"aggregate_id": "aggregate_id",
			"event_id":     "event_id",
		},
		Serializer:    &JSONSerializer{},
		RetryPolicy:   events.DefaultRetryConfig(),
		EnableBatch:   false,
		BatchSize:     100,
		BatchTimeout:  100 * time.Millisecond,
		EnableMetrics: true,
	}
}

// MessageBusEventAdapter реализация Event Publisher через MessageBus
type MessageBusEventAdapter struct {
	config  MessageBusEventConfig
	bus     transport.Publisher
	metrics *metrics.Metrics
	running bool
	batch   []batchEvent
	batchMu sync.Mutex
}

type batchEvent struct {
	ctx   context.Context
	event events.Event
}

// NewMessageBusEventAdapter создает новый MessageBus Event Publisher
func NewMessageBusEventAdapter(config MessageBusEventConfig) (*MessageBusEventAdapter, error) {
	if config.Bus == nil {
		return nil, fmt.Errorf("message bus is required")
	}

	adapter := &MessageBusEventAdapter{
		config: config,
		bus:    config.Bus,
		batch:  make([]batchEvent, 0, config.BatchSize),
	}

	if config.EnableMetrics {
		var err error
		adapter.metrics, err = metrics.NewMetrics()
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics: %w", err)
		}
	}

	// Запускаем batch processor если включен
	if config.EnableBatch {
		go adapter.batchProcessor()
	}

	return adapter, nil
}

// Start запускает адаптер (реализация core.Lifecycle)
func (m *MessageBusEventAdapter) Start(ctx context.Context) error {
	m.running = true
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (m *MessageBusEventAdapter) Stop(ctx context.Context) error {
	m.running = false

	// Flush оставшихся событий в batch
	if m.config.EnableBatch {
		_ = m.flushBatch(ctx)
	}

	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (m *MessageBusEventAdapter) IsRunning() bool {
	return m.running
}

// Name возвращает имя компонента (реализация core.Component)
func (m *MessageBusEventAdapter) Name() string {
	return "messagebus-event-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (m *MessageBusEventAdapter) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// Publish публикует событие
func (m *MessageBusEventAdapter) Publish(ctx context.Context, event events.Event) error {
	start := time.Now()

	if m.config.EnableBatch {
		return m.publishBatch(ctx, event)
	}

	return m.publishSingle(ctx, event, start)
}

// publishSingle публикует одно событие
func (m *MessageBusEventAdapter) publishSingle(ctx context.Context, event events.Event, start time.Time) error {
	// Формируем subject по шаблону
	subject := m.getSubject(event)

	// Сериализуем событие
	data, err := m.serializeEvent(event)
	if err != nil {
		if m.metrics != nil {
			m.metrics.RecordEvent(ctx, event.EventType())
		}
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Формируем headers с метаданными события
	headers := m.buildHeaders(event)

	// Публикуем с retry
	err = m.publishWithRetry(ctx, subject, data, headers)
	if err != nil {
		if m.metrics != nil {
			m.metrics.RecordEvent(ctx, event.EventType())
		}
		return err
	}

	if m.metrics != nil {
		m.metrics.RecordEvent(ctx, event.EventType())
	}

	return nil
}

// publishBatch добавляет событие в batch
func (m *MessageBusEventAdapter) publishBatch(ctx context.Context, event events.Event) error {
	m.batchMu.Lock()
	defer m.batchMu.Unlock()

	m.batch = append(m.batch, batchEvent{ctx: ctx, event: event})

	// Если batch заполнен, публикуем
	if len(m.batch) >= m.config.BatchSize {
		return m.flushBatch(ctx)
	}

	return nil
}

// flushBatch публикует все события из batch
func (m *MessageBusEventAdapter) flushBatch(ctx context.Context) error {
	m.batchMu.Lock()
	events := make([]batchEvent, len(m.batch))
	copy(events, m.batch)
	m.batch = m.batch[:0]
	m.batchMu.Unlock()

	for _, be := range events {
		_ = m.publishSingle(be.ctx, be.event, time.Now())
	}

	return nil
}

// batchProcessor обрабатывает batch по таймауту
func (m *MessageBusEventAdapter) batchProcessor() {
	ticker := time.NewTicker(m.config.BatchTimeout)
	defer ticker.Stop()

	for range ticker.C {
		if len(m.batch) > 0 {
			_ = m.flushBatch(context.Background())
		}
	}
}

// getSubject формирует subject для события
func (m *MessageBusEventAdapter) getSubject(event events.Event) string {
	// Поддержка шаблонов: events.{aggregate}.{event_type}
	aggregateID := event.AggregateID()
	aggregateType := "unknown"

	if len(aggregateID) > 0 {
		parts := splitAggregateID(aggregateID)
		if len(parts) > 0 {
			aggregateType = parts[0]
		}
	}

	return fmt.Sprintf("%s.%s.%s", m.config.SubjectPrefix, aggregateType, event.EventType())
}

// serializeEvent сериализует событие
func (m *MessageBusEventAdapter) serializeEvent(event events.Event) ([]byte, error) {
	if m.config.Serializer != nil {
		return m.config.Serializer.Serialize(event)
	}

	// Default JSON serialization
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

// buildHeaders формирует headers из метаданных события
func (m *MessageBusEventAdapter) buildHeaders(event events.Event) map[string]string {
	headers := make(map[string]string)

	// Маппинг полей события в headers
	if mapping := m.config.HeaderMapping; mapping != nil {
		if key, ok := mapping["event_type"]; ok {
			headers[key] = event.EventType()
		}
		if key, ok := mapping["aggregate_id"]; ok {
			headers[key] = event.AggregateID()
		}
		if key, ok := mapping["event_id"]; ok {
			headers[key] = event.EventID()
		}
	}

	// Добавляем метаданные события
	if metadata := event.Metadata(); metadata != nil {
		if correlationID := metadata.CorrelationID(); correlationID != "" {
			headers["correlation_id"] = correlationID
		}
		if causationID := metadata.CausationID(); causationID != "" {
			headers["causation_id"] = causationID
		}
		if userID := metadata.UserID(); userID != "" {
			headers["user_id"] = userID
		}
	}

	return headers
}

// publishWithRetry публикует событие с retry логикой
func (m *MessageBusEventAdapter) publishWithRetry(ctx context.Context, subject string, data []byte, headers map[string]string) error {
	retryConfig := m.config.RetryPolicy
	delay := retryConfig.InitialDelay

	for attempt := 0; attempt < retryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := m.bus.Publish(ctx, subject, data, headers)
		if err == nil {
			return nil
		}

		delay = time.Duration(float64(delay) * retryConfig.BackoffMultiplier)
		if delay > retryConfig.MaxDelay {
			delay = retryConfig.MaxDelay
		}
	}

	return fmt.Errorf("failed to publish event after %d attempts", retryConfig.MaxAttempts)
}
