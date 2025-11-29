// Package invoke предоставляет опции конфигурации для модуля Invoke.
package invoke

import (
	"time"

	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/transport"
)

// InvokeOptions опции для вызова команды или запроса
type InvokeOptions struct {
	Timeout          time.Duration
	RetryPolicy      *RetryPolicy
	Metadata         map[string]interface{}
	CorrelationID    string
	CausationID      string
	SubjectResolver  SubjectResolver
	EventSource      EventSource
	SuccessEventType string
	ErrorEventType   string
}

// RetryPolicy политика повторов
type RetryPolicy struct {
	MaxAttempts      int
	InitialDelay     time.Duration
	MaxDelay         time.Duration
	BackoffMultiplier float64
}

// DefaultRetryPolicy возвращает политику повторов по умолчанию
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:      3,
		InitialDelay:     time.Second,
		MaxDelay:         30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// InvokeOption функция для настройки опций
type InvokeOption func(*InvokeOptions)

// WithTimeout устанавливает таймаут
func WithTimeout(timeout time.Duration) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.Timeout = timeout
	}
}

// WithRetry устанавливает политику повторов
func WithRetry(policy *RetryPolicy) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.RetryPolicy = policy
	}
}

// WithMetadata устанавливает метаданные
func WithMetadata(metadata map[string]interface{}) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.Metadata = metadata
	}
}

// WithCorrelationIDOption устанавливает correlation ID в опциях
func WithCorrelationIDOption(id string) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.CorrelationID = id
	}
}

// WithCausationIDOption устанавливает causation ID в опциях
func WithCausationIDOption(id string) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.CausationID = id
	}
}

// WithSubjectResolver устанавливает SubjectResolver в опциях
func WithSubjectResolver(resolver SubjectResolver) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.SubjectResolver = resolver
	}
}

// WithEventSource устанавливает EventSource в опциях
func WithEventSource(source EventSource) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.EventSource = source
	}
}

// WithSuccessEventType устанавливает тип успешного события в опциях
func WithSuccessEventType(eventType string) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.SuccessEventType = eventType
	}
}

// WithErrorEventType устанавливает тип ошибочного события в опциях
func WithErrorEventType(eventType string) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.ErrorEventType = eventType
	}
}

// WithTransportSubscriber создает EventSource через TransportSubscriberAdapter
func WithTransportSubscriber(
	subscriber transport.Subscriber,
	serializer transport.MessageSerializer,
	resolver SubjectResolver,
) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.EventSource = NewTransportSubscriberAdapter(subscriber, serializer, resolver)
	}
}

// WithEventBus создает EventSource через EventBusAdapter
func WithEventBus(eventBus events.EventBus) InvokeOption {
	return func(opts *InvokeOptions) {
		opts.EventSource = NewEventBusAdapter(eventBus)
	}
}

// ApplyOptions применяет опции к InvokeOptions
func ApplyOptions(options ...InvokeOption) *InvokeOptions {
	opts := &InvokeOptions{
		Timeout: 30 * time.Second,
	}
	for _, opt := range options {
		opt(opts)
	}
	return opts
}

// ToCommandMetadata преобразует опции в метаданные команды
func (opts *InvokeOptions) ToCommandMetadata() *transport.BaseCommandMetadata {
	correlationID := opts.CorrelationID
	if correlationID == "" {
		correlationID = GenerateCorrelationID()
	}

	commandID := GenerateCommandID()
	return transport.NewBaseCommandMetadata(commandID, correlationID, opts.CausationID)
}

