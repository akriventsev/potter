// Package invoke предоставляет утилиты для работы с correlation ID и causation ID.
package invoke

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"potter/framework/events"
	"potter/framework/transport"
)

// Константы для ключей контекста
const (
	CorrelationIDKey = "correlation_id"
	CausationIDKey   = "causation_id"
	CommandIDKey     = "command_id"
)

// GenerateCorrelationID генерирует уникальный correlation ID
func GenerateCorrelationID() string {
	return uuid.New().String()
}

// GenerateCommandID генерирует уникальный ID команды
func GenerateCommandID() string {
	return fmt.Sprintf("cmd-%d", time.Now().UnixNano())
}

// ExtractCorrelationID извлекает correlation ID из контекста
func ExtractCorrelationID(ctx context.Context) string {
	if val := ctx.Value(CorrelationIDKey); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

// WithCorrelationID добавляет correlation ID в контекст
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, id)
}

// ExtractCausationID извлекает causation ID из контекста
func ExtractCausationID(ctx context.Context) string {
	if val := ctx.Value(CausationIDKey); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

// WithCausationID добавляет causation ID в контекст
func WithCausationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CausationIDKey, id)
}

// ExtractCommandID извлекает command ID из контекста
func ExtractCommandID(ctx context.Context) string {
	if val := ctx.Value(CommandIDKey); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

// WithCommandID добавляет command ID в контекст
func WithCommandID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CommandIDKey, id)
}

// PropagateMetadata распространяет метаданные из команды в контекст
func PropagateMetadata(ctx context.Context, metadata *transport.BaseCommandMetadata) context.Context {
	if metadata == nil {
		return ctx
	}

	if metadata.CorrelationID() != "" {
		ctx = WithCorrelationID(ctx, metadata.CorrelationID())
	}
	if metadata.CausationID() != "" {
		ctx = WithCausationID(ctx, metadata.CausationID())
	}
	if metadata.ID() != "" {
		ctx = WithCommandID(ctx, metadata.ID())
	}

	return ctx
}

// CreateMetadataFromContext создает метаданные команды из контекста
func CreateMetadataFromContext(ctx context.Context) *transport.BaseCommandMetadata {
	correlationID := ExtractCorrelationID(ctx)
	if correlationID == "" {
		correlationID = GenerateCorrelationID()
	}

	causationID := ExtractCausationID(ctx)
	commandID := ExtractCommandID(ctx)
	if commandID == "" {
		commandID = GenerateCommandID()
	}

	return transport.NewBaseCommandMetadata(commandID, correlationID, causationID)
}

// ExtractCorrelationIDFromEvent извлекает correlation ID из события
func ExtractCorrelationIDFromEvent(event events.Event) string {
	if event == nil {
		return ""
	}
	return event.Metadata().CorrelationID()
}

// ExtractCausationIDFromEvent извлекает causation ID из события
func ExtractCausationIDFromEvent(event events.Event) string {
	if event == nil {
		return ""
	}
	return event.Metadata().CausationID()
}

