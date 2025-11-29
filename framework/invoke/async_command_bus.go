// Package invoke предоставляет AsyncCommandBus для публикации команд в NATS pub/sub.
package invoke

import (
	"context"
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/metrics"
	"github.com/akriventsev/potter/framework/transport"
)

// AsyncCommandBus чистый producer команд для NATS pub/sub (produce command)
type AsyncCommandBus struct {
	pubSub         transport.Publisher
	serializer     transport.MessageSerializer
	subjectResolver SubjectResolver
	idGenerator    func() string
	metrics        *metrics.Metrics
}

// NewAsyncCommandBus создает новый AsyncCommandBus
func NewAsyncCommandBus(pubSub transport.Publisher) *AsyncCommandBus {
	return &AsyncCommandBus{
		pubSub:          pubSub,
		serializer:      DefaultSerializer(),
		subjectResolver: NewDefaultSubjectResolver("commands", "events"),
		idGenerator:     GenerateCommandID,
	}
}

// WithSerializer устанавливает сериализатор
func (b *AsyncCommandBus) WithSerializer(serializer transport.MessageSerializer) *AsyncCommandBus {
	b.serializer = serializer
	return b
}

// WithSubjectPrefix устанавливает префикс subject (создает новый DefaultSubjectResolver)
func (b *AsyncCommandBus) WithSubjectPrefix(prefix string) *AsyncCommandBus {
	b.subjectResolver = NewDefaultSubjectResolver(prefix, "events")
	return b
}

// WithSubjectResolver устанавливает кастомный SubjectResolver
func (b *AsyncCommandBus) WithSubjectResolver(resolver SubjectResolver) *AsyncCommandBus {
	b.subjectResolver = resolver
	return b
}

// WithCommandSubjectFunc устанавливает функцию для определения subject команды
func (b *AsyncCommandBus) WithCommandSubjectFunc(commandFunc func(transport.Command) string) *AsyncCommandBus {
	// Сохраняем текущую функцию для событий, если есть
	var eventFunc func(string) string
	if funcResolver, ok := b.subjectResolver.(*FunctionSubjectResolver); ok {
		eventFunc = funcResolver.eventFunc
	} else if defaultResolver, ok := b.subjectResolver.(*DefaultSubjectResolver); ok {
		eventPrefix := defaultResolver.eventPrefix
		eventFunc = func(eventType string) string {
			return fmt.Sprintf("%s.%s", eventPrefix, eventType)
		}
	}

	b.subjectResolver = NewFunctionSubjectResolver(commandFunc, eventFunc)
	return b
}

// WithIDGenerator устанавливает генератор ID
func (b *AsyncCommandBus) WithIDGenerator(generator func() string) *AsyncCommandBus {
	b.idGenerator = generator
	return b
}

// WithMetrics устанавливает метрики
func (b *AsyncCommandBus) WithMetrics(m *metrics.Metrics) *AsyncCommandBus {
	b.metrics = m
	return b
}

// SendAsync публикует команду асинхронно (pure produce)
func (b *AsyncCommandBus) SendAsync(ctx context.Context, cmd transport.Command, metadata *transport.BaseCommandMetadata) error {
	start := time.Now()

	// Генерируем ID и correlation ID если не указаны
	if metadata == nil {
		correlationID := GenerateCorrelationID()
		commandID := b.idGenerator()
		metadata = transport.NewBaseCommandMetadata(commandID, correlationID, "")
	}

	// Сериализуем команду
	data, err := b.serializer.Serialize(cmd)
	if err != nil {
		if b.metrics != nil {
			b.metrics.RecordCommand(ctx, cmd.CommandName(), time.Since(start), false)
		}
		return fmt.Errorf("failed to serialize command: %w", err)
	}

	// Формируем subject через resolver
	subject := b.subjectResolver.ResolveCommandSubject(cmd)
	if subject == "" {
		if b.metrics != nil {
			b.metrics.RecordCommand(ctx, cmd.CommandName(), time.Since(start), false)
		}
		return fmt.Errorf("failed to resolve subject for command: %s", cmd.CommandName())
	}

	// Формируем headers
	headers := map[string]string{
		"command_id":     metadata.ID(),
		"correlation_id": metadata.CorrelationID(),
		"causation_id":   metadata.CausationID(),
		"timestamp":      metadata.Timestamp().Format(time.RFC3339),
		"command_name":   cmd.CommandName(),
	}

	// Публикуем команду (fire-and-forget)
	err = b.pubSub.Publish(ctx, subject, data, headers)
	if err != nil {
		if b.metrics != nil {
			b.metrics.RecordCommand(ctx, cmd.CommandName(), time.Since(start), false)
		}
		return NewCommandPublishFailedError(cmd.CommandName(), err)
	}

	if b.metrics != nil {
		b.metrics.RecordCommand(ctx, cmd.CommandName(), time.Since(start), true)
	}

	return nil
}

