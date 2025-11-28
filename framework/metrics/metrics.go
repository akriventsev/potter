// Package metrics предоставляет систему метрик на основе OpenTelemetry.
package metrics

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics сборщик метрик приложения
type Metrics struct {
	meter           metric.Meter
	commandsTotal   metric.Int64Counter
	queriesTotal    metric.Int64Counter
	eventsTotal     metric.Int64Counter
	commandDuration metric.Float64Histogram
	queryDuration   metric.Float64Histogram
	errorsTotal     metric.Int64Counter
	activeCommands  metric.Int64UpDownCounter
	activeQueries   metric.Int64UpDownCounter
	customMetrics   map[string]interface{}
	mu              sync.RWMutex
}

// NewMetrics создает новый сборщик метрик
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter("potter")

	commandsTotal, err := meter.Int64Counter(
		"commands_total",
		metric.WithDescription("Total number of commands processed"),
	)
	if err != nil {
		return nil, err
	}

	queriesTotal, err := meter.Int64Counter(
		"queries_total",
		metric.WithDescription("Total number of queries processed"),
	)
	if err != nil {
		return nil, err
	}

	eventsTotal, err := meter.Int64Counter(
		"events_total",
		metric.WithDescription("Total number of events published"),
	)
	if err != nil {
		return nil, err
	}

	commandDuration, err := meter.Float64Histogram(
		"command_duration_seconds",
		metric.WithDescription("Command processing duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	queryDuration, err := meter.Float64Histogram(
		"query_duration_seconds",
		metric.WithDescription("Query processing duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	errorsTotal, err := meter.Int64Counter(
		"errors_total",
		metric.WithDescription("Total number of errors"),
	)
	if err != nil {
		return nil, err
	}

	activeCommands, err := meter.Int64UpDownCounter(
		"active_commands",
		metric.WithDescription("Number of active commands being processed"),
	)
	if err != nil {
		return nil, err
	}

	activeQueries, err := meter.Int64UpDownCounter(
		"active_queries",
		metric.WithDescription("Number of active queries being processed"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		meter:           meter,
		commandsTotal:   commandsTotal,
		queriesTotal:     queriesTotal,
		eventsTotal:      eventsTotal,
		commandDuration: commandDuration,
		queryDuration:   queryDuration,
		errorsTotal:     errorsTotal,
		activeCommands:  activeCommands,
		activeQueries:   activeQueries,
		customMetrics:   make(map[string]interface{}),
	}, nil
}

// RecordCommand записывает метрику команды
func (m *Metrics) RecordCommand(ctx context.Context, commandName string, duration time.Duration, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("command", commandName),
		attribute.Bool("success", success),
	}

	m.commandsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.commandDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if !success {
		m.errorsTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "command"),
			attribute.String("command", commandName),
		))
	}
}

// RecordQuery записывает метрику запроса
func (m *Metrics) RecordQuery(ctx context.Context, queryName string, duration time.Duration, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("query", queryName),
		attribute.Bool("success", success),
	}

	m.queriesTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.queryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if !success {
		m.errorsTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "query"),
			attribute.String("query", queryName),
		))
	}
}

// RecordEvent записывает метрику события
func (m *Metrics) RecordEvent(ctx context.Context, eventType string) {
	attrs := []attribute.KeyValue{
		attribute.String("event", eventType),
	}

	m.eventsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// IncrementActiveCommands увеличивает счетчик активных команд
func (m *Metrics) IncrementActiveCommands(ctx context.Context) {
	m.activeCommands.Add(ctx, 1)
}

// DecrementActiveCommands уменьшает счетчик активных команд
func (m *Metrics) DecrementActiveCommands(ctx context.Context) {
	m.activeCommands.Add(ctx, -1)
}

// IncrementActiveQueries увеличивает счетчик активных запросов
func (m *Metrics) IncrementActiveQueries(ctx context.Context) {
	m.activeQueries.Add(ctx, 1)
}

// DecrementActiveQueries уменьшает счетчик активных запросов
func (m *Metrics) DecrementActiveQueries(ctx context.Context) {
	m.activeQueries.Add(ctx, -1)
}

// Register регистрирует кастомную метрику
func (m *Metrics) Register(name string, metric interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customMetrics[name] = metric
	return nil
}

// Unregister удаляет кастомную метрику
func (m *Metrics) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.customMetrics, name)
	return nil
}

// RecordTransport записывает метрику транспорта
func (m *Metrics) RecordTransport(ctx context.Context, transportName string, duration time.Duration, success bool) {
	// Записываем метрики транспорта (нужно добавить отдельные счетчики)
	// Пока используем errorsTotal только для ошибок
	if !success {
		m.errorsTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "transport"),
			attribute.String("transport", transportName),
		))
	}
}

// RecordContainer записывает метрику контейнера
func (m *Metrics) RecordContainer(ctx context.Context, operation string, duration time.Duration, success bool) {
	// Записываем метрики контейнера (нужно добавить отдельные счетчики)
	// Пока используем errorsTotal только для ошибок
	if !success {
		m.errorsTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "container"),
			attribute.String("operation", operation),
		))
	}
}

