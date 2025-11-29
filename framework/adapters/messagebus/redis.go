// Package messagebus предоставляет адаптеры для различных message brokers.
package messagebus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/transport"
	"github.com/redis/go-redis/v9"
)

// RedisConfig конфигурация для Redis адаптера
type RedisConfig struct {
	Addr          string
	Password      string
	DB            int
	PoolSize      int
	MaxRetries    int
	StreamMaxLen  int64 // Максимальная длина stream (0 = без ограничений)
	ConsumerGroup string
	BlockTimeout  time.Duration
	EnableMetrics bool
	StreamName    string // Имя stream для публикации сообщений
}

// Validate проверяет корректность конфигурации
func (c RedisConfig) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("addr cannot be empty")
	}
	if c.StreamName == "" {
		return fmt.Errorf("StreamName cannot be empty")
	}
	return nil
}

// DefaultRedisConfig возвращает конфигурацию Redis по умолчанию
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Addr:          "localhost:6379",
		Password:      "",
		DB:            0,
		PoolSize:      10,
		MaxRetries:    3,
		StreamMaxLen:  10000,
		ConsumerGroup: "potter-group",
		BlockTimeout:  5 * time.Second,
		EnableMetrics: true,
	}
}

// RedisAdapter реализация MessageBus через Redis Streams
type RedisAdapter struct {
	config    RedisConfig
	client    *redis.Client
	subs      map[string]*redis.Client // Consumer groups для каждого stream
	mu        sync.RWMutex
	running   bool
	consumers map[string]string // stream -> consumer name
}

// NewRedisAdapter создает новый Redis адаптер
func NewRedisAdapter(config RedisConfig) (*RedisAdapter, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid redis config: %w", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:       config.Addr,
		Password:   config.Password,
		DB:         config.DB,
		PoolSize:   config.PoolSize,
		MaxRetries: config.MaxRetries,
	})

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisAdapter{
		config:    config,
		client:   client,
		subs:     make(map[string]*redis.Client),
		running:  false,
		consumers: make(map[string]string),
	}, nil
}

// Start запускает адаптер (реализация core.Lifecycle)
func (r *RedisAdapter) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return nil
	}

	r.running = true
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (r *RedisAdapter) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	// Закрываем все клиенты
	for _, client := range r.subs {
		if client != nil {
			_ = client.Close()
		}
	}

	if r.client != nil {
		_ = r.client.Close()
	}

	r.running = false
	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (r *RedisAdapter) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// Name возвращает имя компонента (реализация core.Component)
func (r *RedisAdapter) Name() string {
	return "redis-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (r *RedisAdapter) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// Publish публикует сообщение в stream (XADD)
func (r *RedisAdapter) Publish(ctx context.Context, subject string, data []byte, headers map[string]string) error {
	stream := r.getStreamName(subject)

	// Создаем map для XADD
	values := make(map[string]interface{})
	values["data"] = string(data)

	// Добавляем headers
	if headers != nil {
		headersJSON, _ := json.Marshal(headers)
		values["headers"] = string(headersJSON)
	}

	// XADD с MAXLEN для автоматической очистки старых сообщений
	args := redis.XAddArgs{
		Stream: stream,
		Values: values,
	}

	if r.config.StreamMaxLen > 0 {
		args.MaxLen = r.config.StreamMaxLen
		args.Approx = true // Приблизительный MAXLEN для производительности
	}

	_, err := r.client.XAdd(ctx, &args).Result()
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Subscribe подписывается на stream (XREADGROUP)
func (r *RedisAdapter) Subscribe(ctx context.Context, subject string, handler transport.MessageHandler) error {
	stream := r.getStreamName(subject)
	consumerName := fmt.Sprintf("consumer-%d", time.Now().UnixNano())

	// Создаем consumer group если не существует
	err := r.client.XGroupCreateMkStream(ctx, stream, r.config.ConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	r.mu.Lock()
	r.consumers[stream] = consumerName
	r.mu.Unlock()

	// Запускаем goroutine для чтения сообщений
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// XREADGROUP для чтения новых сообщений
				streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
					Group:    r.config.ConsumerGroup,
					Consumer: consumerName,
					Streams:  []string{stream, ">"},
					Count:    10,
					Block:    r.config.BlockTimeout,
				}).Result()

				if err != nil {
					if err == redis.Nil || err == context.Canceled {
						continue
					}
					// Логируем ошибку
					time.Sleep(1 * time.Second)
					continue
				}

				for _, stream := range streams {
					for _, msg := range stream.Messages {
						mbMsg := &transport.Message{
							Subject: subject,
							Data:    []byte(msg.Values["data"].(string)),
							Headers: make(map[string]string),
						}

						// Парсим headers
						if headersStr, ok := msg.Values["headers"].(string); ok {
							_ = json.Unmarshal([]byte(headersStr), &mbMsg.Headers)
						}

						if err := handler(ctx, mbMsg); err != nil {
							// Логируем ошибку, но не прерываем обработку
							_ = err
						} else {
							// XACK для подтверждения обработки
							_ = r.client.XAck(ctx, stream.Stream, r.config.ConsumerGroup, msg.ID).Err()
						}
					}
				}
			}
		}
	}()

	return nil
}

// Unsubscribe отписывается от stream
func (r *RedisAdapter) Unsubscribe(subject string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	stream := r.getStreamName(subject)
	delete(r.consumers, stream)
	delete(r.subs, stream)

	return nil
}

// Request отправляет запрос и ждет ответа (через временные streams)
func (r *RedisAdapter) Request(ctx context.Context, subject string, data []byte, timeout time.Duration) (*transport.Message, error) {
	// Создаем временный reply stream
	replyStream := fmt.Sprintf("%s.reply.%d", subject, time.Now().UnixNano())
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())

	// Публикуем запрос с reply stream в headers
	headers := map[string]string{
		"request_id":   requestID,
		"reply_stream": replyStream,
	}

	if err := r.Publish(ctx, subject, data, headers); err != nil {
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	// Создаем контекст с timeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Читаем из reply stream
	start := "0"
	for {
		select {
		case <-reqCtx.Done():
			return nil, fmt.Errorf("request timeout: %w", reqCtx.Err())
		default:
			streams, err := r.client.XRead(reqCtx, &redis.XReadArgs{
				Streams: []string{replyStream, start},
				Count:   1,
				Block:   r.config.BlockTimeout,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue
				}
				return nil, fmt.Errorf("failed to read reply: %w", err)
			}

			if len(streams) > 0 && len(streams[0].Messages) > 0 {
				msg := streams[0].Messages[0]
				mbMsg := &transport.Message{
					Subject: subject,
					Data:    []byte(msg.Values["data"].(string)),
					Headers: make(map[string]string),
				}

				if headersStr, ok := msg.Values["headers"].(string); ok {
					_ = json.Unmarshal([]byte(headersStr), &mbMsg.Headers)
				}

				// Удаляем временный stream
				_ = r.client.Del(context.Background(), replyStream).Err()

				return mbMsg, nil
			}
		}
	}
}

// Respond отвечает на запросы
func (r *RedisAdapter) Respond(ctx context.Context, subject string, handler func(ctx context.Context, request *transport.Message) (*transport.Message, error)) error {
	stream := r.getStreamName(subject)
	consumerName := fmt.Sprintf("responder-%d", time.Now().UnixNano())

	// Создаем consumer group
	err := r.client.XGroupCreateMkStream(ctx, stream, r.config.ConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	// Обрабатываем запросы
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
					Group:    r.config.ConsumerGroup,
					Consumer: consumerName,
					Streams:  []string{stream, ">"},
					Count:    1,
					Block:    r.config.BlockTimeout,
				}).Result()

				if err != nil {
					if err == redis.Nil || err == context.Canceled {
						continue
					}
					time.Sleep(1 * time.Second)
					continue
				}

				for _, stream := range streams {
					for _, msg := range stream.Messages {
						mbRequest := &transport.Message{
							Subject: subject,
							Data:    []byte(msg.Values["data"].(string)),
							Headers: make(map[string]string),
						}

						if headersStr, ok := msg.Values["headers"].(string); ok {
							_ = json.Unmarshal([]byte(headersStr), &mbRequest.Headers)
						}

						// Получаем reply stream из headers
						replyStream, ok := mbRequest.Headers["reply_stream"]
						if !ok {
							_ = r.client.XAck(ctx, stream.Stream, r.config.ConsumerGroup, msg.ID).Err()
							continue
						}

						mbReply, err := handler(ctx, mbRequest)
						if err != nil {
							// Отправляем ошибку в reply stream
							errorData, _ := json.Marshal(map[string]string{"error": err.Error()})
							_ = r.Publish(ctx, replyStream, errorData, nil)
							_ = r.client.XAck(ctx, stream.Stream, r.config.ConsumerGroup, msg.ID).Err()
							continue
						}

						if mbReply != nil {
							// Отправляем ответ в reply stream
							if err := r.Publish(ctx, replyStream, mbReply.Data, mbReply.Headers); err != nil {
								_ = r.client.XAck(ctx, stream.Stream, r.config.ConsumerGroup, msg.ID).Err()
								continue
							}
						}

						_ = r.client.XAck(ctx, stream.Stream, r.config.ConsumerGroup, msg.ID).Err()
					}
				}
			}
		}
	}()

	return nil
}

// getStreamName преобразует subject в имя stream
func (r *RedisAdapter) getStreamName(subject string) string {
	if r.config.StreamName != "" {
		return fmt.Sprintf("%s:%s", r.config.StreamName, subject)
	}
	return fmt.Sprintf("stream:%s", subject)
}

// ProcessPendingMessages обрабатывает pending messages при рестарте
func (r *RedisAdapter) ProcessPendingMessages(ctx context.Context, subject string, handler transport.MessageHandler) error {
	stream := r.getStreamName(subject)
	consumerName := r.consumers[stream]

	if consumerName == "" {
		return fmt.Errorf("consumer not found for stream: %s", stream)
	}

	// XPENDING для получения pending messages
	pending, err := r.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  r.config.ConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	// Обрабатываем pending messages
	for _, p := range pending {
		msgs, err := r.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   stream,
			Group:    r.config.ConsumerGroup,
			Consumer: consumerName,
			MinIdle:  1 * time.Minute,
			Messages: []string{p.ID},
		}).Result()

		if err != nil {
			continue
		}

		for _, msg := range msgs {
			mbMsg := &transport.Message{
				Subject: subject,
				Data:    []byte(msg.Values["data"].(string)),
				Headers: make(map[string]string),
			}

			if headersStr, ok := msg.Values["headers"].(string); ok {
				_ = json.Unmarshal([]byte(headersStr), &mbMsg.Headers)
			}

			if err := handler(ctx, mbMsg); err == nil {
				_ = r.client.XAck(ctx, stream, r.config.ConsumerGroup, msg.ID).Err()
			}
		}
	}

	return nil
}

