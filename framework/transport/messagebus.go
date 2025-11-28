// Package transport предоставляет абстракции для работы с message bus.
package transport

import (
	"context"
	"time"
)

// Message представляет сообщение в очереди
type Message struct {
	Subject string
	Data    []byte
	Headers map[string]string
}

// MessageHandler обработчик сообщений
type MessageHandler func(ctx context.Context, msg *Message) error

// MessageSerializer интерфейс для сериализации сообщений
type MessageSerializer interface {
	// Serialize сериализует сообщение
	Serialize(msg interface{}) ([]byte, error)
	// Deserialize десериализует сообщение
	Deserialize(data []byte, msg interface{}) error
}

// MessageRouter интерфейс для маршрутизации сообщений
type MessageRouter interface {
	// Route определяет subject для сообщения
	Route(msg interface{}) (string, error)
}

// Subscriber подписчик на сообщения
type Subscriber interface {
	// Subscribe подписывается на subject и вызывает handler при получении сообщения
	Subscribe(ctx context.Context, subject string, handler MessageHandler) error
	// Unsubscribe отписывается от subject
	Unsubscribe(subject string) error
}

// Publisher публикатор сообщений
type Publisher interface {
	// Publish публикует сообщение в subject
	Publish(ctx context.Context, subject string, data []byte, headers map[string]string) error
}

// RequestReply реализует паттерн request-reply
type RequestReply interface {
	// Request отправляет запрос и ждет ответа
	Request(ctx context.Context, subject string, data []byte, timeout time.Duration) (*Message, error)
	// Respond отвечает на запрос
	Respond(ctx context.Context, subject string, handler func(ctx context.Context, request *Message) (*Message, error)) error
}

// MessageBus объединяет возможности публикации и подписки
type MessageBus interface {
	Publisher
	Subscriber
}

// RequestReplyBus объединяет MessageBus и RequestReply
type RequestReplyBus interface {
	MessageBus
	RequestReply
}

// DeadLetterQueue интерфейс для dead letter queue
type DeadLetterQueue interface {
	// Publish публикует сообщение в DLQ
	Publish(ctx context.Context, msg *Message, reason string) error
	// Subscribe подписывается на DLQ
	Subscribe(ctx context.Context, handler func(ctx context.Context, msg *Message, reason string) error) error
}

// RetryPolicy политика повторов для сообщений
type RetryPolicy interface {
	// ShouldRetry определяет, нужно ли повторить попытку
	ShouldRetry(attempt int, err error) bool
	// GetDelay возвращает задержку перед повтором
	GetDelay(attempt int) time.Duration
	// GetMaxAttempts возвращает максимальное количество попыток
	GetMaxAttempts() int
}

// ExponentialBackoffRetryPolicy политика повторов с экспоненциальной задержкой
type ExponentialBackoffRetryPolicy struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	MaxAttempts  int
}

// ShouldRetry определяет, нужно ли повторить попытку
func (p *ExponentialBackoffRetryPolicy) ShouldRetry(attempt int, err error) bool {
	return attempt < p.MaxAttempts && err != nil
}

// GetDelay возвращает задержку перед повтором
func (p *ExponentialBackoffRetryPolicy) GetDelay(attempt int) time.Duration {
	delay := time.Duration(float64(p.InitialDelay) * float64(attempt) * p.Multiplier)
	if delay > p.MaxDelay {
		return p.MaxDelay
	}
	return delay
}

// GetMaxAttempts возвращает максимальное количество попыток
func (p *ExponentialBackoffRetryPolicy) GetMaxAttempts() int {
	return p.MaxAttempts
}

// MessageAcknowledger интерфейс для подтверждения получения сообщений
type MessageAcknowledger interface {
	// Ack подтверждает получение сообщения
	Ack(ctx context.Context, msg *Message) error
	// Nack отклоняет сообщение
	Nack(ctx context.Context, msg *Message) error
}

// MessageHandlerOption опции для обработчика сообщений
type MessageHandlerOption func(*handlerOptions)

type handlerOptions struct {
	queue        string // очередь для балансировки нагрузки
	retryPolicy  RetryPolicy
	ackOnSuccess bool
}

// WithQueue указывает очередь для обработчика
func WithQueue(queue string) MessageHandlerOption {
	return func(opts *handlerOptions) {
		opts.queue = queue
	}
}

// WithRetryPolicy устанавливает политику повторов
func WithRetryPolicy(policy RetryPolicy) MessageHandlerOption {
	return func(opts *handlerOptions) {
		opts.retryPolicy = policy
	}
}

// WithAckOnSuccess включает автоматическое подтверждение при успехе
func WithAckOnSuccess(ack bool) MessageHandlerOption {
	return func(opts *handlerOptions) {
		opts.ackOnSuccess = ack
	}
}

// Delivery гарантии доставки сообщений
type Delivery int

const (
	// AtMostOnce доставка максимум один раз (может потеряться)
	AtMostOnce Delivery = iota
	// AtLeastOnce доставка минимум один раз (может дублироваться)
	AtLeastOnce
	// ExactlyOnce доставка ровно один раз (идеальный вариант)
	ExactlyOnce
)

// Config конфигурация MessageBus
type Config struct {
	// Delivery гарантии доставки
	Delivery Delivery
	// MaxRetries максимальное количество повторных попыток
	MaxRetries int
	// RetryDelay задержка между повторами
	RetryDelay time.Duration
}

