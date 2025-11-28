// Package transport предоставляет интерфейсы и реализации для работы с запросами CQRS.
package transport

import (
	"context"
	"time"
)

// Query представляет запрос CQRS
type Query interface {
	QueryName() string
}

// QueryMetadata интерфейс для метаданных запросов
type QueryMetadata interface {
	// ID возвращает уникальный идентификатор запроса
	ID() string
	// Timestamp возвращает время создания запроса
	Timestamp() time.Time
	// CorrelationID возвращает correlation ID для трассировки
	CorrelationID() string
}

// QueryHandler обработчик запросов
type QueryHandler interface {
	Handle(ctx context.Context, q Query) (interface{}, error)
	QueryName() string
}

// QueryValidator интерфейс для валидации запросов
type QueryValidator interface {
	Validate(ctx context.Context, q Query) error
}

// QueryInterceptor интерфейс для перехвата запросов
type QueryInterceptor interface {
	// Intercept вызывается перед выполнением запроса
	Intercept(ctx context.Context, q Query, next func(ctx context.Context, q Query) (interface{}, error)) (interface{}, error)
}

// QueryCache интерфейс для кэширования результатов запросов
type QueryCache interface {
	// Get возвращает закэшированный результат
	Get(ctx context.Context, query Query) (interface{}, bool)
	// Set сохраняет результат в кэш
	Set(ctx context.Context, query Query, result interface{}) error
	// Invalidate инвалидирует кэш
	Invalidate(ctx context.Context, query Query) error
}

// QueryBus шина запросов
type QueryBus interface {
	Ask(ctx context.Context, q Query) (interface{}, error)
	Register(handler QueryHandler) error
}

// BaseQueryMetadata базовая реализация метаданных запроса
type BaseQueryMetadata struct {
	id            string
	timestamp     time.Time
	correlationID string
}

// NewBaseQueryMetadata создает новые метаданные запроса
func NewBaseQueryMetadata(id, correlationID string) *BaseQueryMetadata {
	return &BaseQueryMetadata{
		id:            id,
		timestamp:     time.Now(),
		correlationID: correlationID,
	}
}

func (m *BaseQueryMetadata) ID() string {
	return m.id
}

func (m *BaseQueryMetadata) Timestamp() time.Time {
	return m.timestamp
}

func (m *BaseQueryMetadata) CorrelationID() string {
	return m.correlationID
}

