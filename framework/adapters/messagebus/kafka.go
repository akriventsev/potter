// Package messagebus предоставляет адаптеры для различных message brokers.
package messagebus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/metrics"
	"github.com/akriventsev/potter/framework/transport"
	"github.com/segmentio/kafka-go"
)

// KafkaConfig конфигурация для Kafka адаптера
type KafkaConfig struct {
	Brokers           []string
	GroupID           string
	Topics            []string
	Partitions        int
	ReplicationFactor int
	Compression       string // none, gzip, snappy, lz4, zstd
	BatchSize         int
	FlushInterval     time.Duration
	ConsumerConfig    KafkaConsumerConfig
	ProducerConfig    KafkaProducerConfig
	EnableMetrics     bool
}

// Validate проверяет корректность конфигурации
func (c KafkaConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("brokers cannot be empty")
	}
	for i, broker := range c.Brokers {
		if broker == "" {
			return fmt.Errorf("broker[%d] cannot be empty", i)
		}
		// Простая проверка формата host:port
		if !strings.Contains(broker, ":") {
			return fmt.Errorf("broker[%d] must be in format host:port", i)
		}
	}
	return nil
}

// KafkaConsumerConfig конфигурация для Kafka consumer
type KafkaConsumerConfig struct {
	MinBytes     int
	MaxBytes     int
	MaxWait      time.Duration
	StartOffset  int64 // -2 (earliest), -1 (latest), или конкретный offset
	CommitInterval time.Duration
}

// KafkaProducerConfig конфигурация для Kafka producer
type KafkaProducerConfig struct {
	RequiredAcks int // 0, 1, -1 (all)
	Idempotent   bool
	MaxAttempts  int
}

// DefaultKafkaConfig возвращает конфигурацию Kafka по умолчанию
func DefaultKafkaConfig() KafkaConfig {
	return KafkaConfig{
		Brokers:           []string{"localhost:9092"},
		GroupID:           "potter-group",
		Partitions:        1,
		ReplicationFactor: 1,
		Compression:       "snappy",
		BatchSize:         100,
		FlushInterval:     10 * time.Millisecond,
		ConsumerConfig: KafkaConsumerConfig{
			MinBytes:      10e3, // 10KB
			MaxBytes:      10e6, // 10MB
			MaxWait:       1 * time.Second,
			StartOffset:   kafka.LastOffset,
			CommitInterval: 1 * time.Second,
		},
		ProducerConfig: KafkaProducerConfig{
			RequiredAcks: -1, // all
			Idempotent:   true,
			MaxAttempts:  3,
		},
		EnableMetrics: true,
	}
}

// KafkaAdapter реализация MessageBus через Kafka
type KafkaAdapter struct {
	config         KafkaConfig
	writer         *kafka.Writer
	subs           map[string]*kafka.Reader
	mu             sync.RWMutex
	running        bool
	metrics        *metrics.Metrics
	requestTopics  map[string]string // correlation ID -> reply topic
	requestMu      sync.RWMutex
}

// NewKafkaAdapter создает новый Kafka адаптер
func NewKafkaAdapter(config KafkaConfig) (*KafkaAdapter, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid kafka config: %w", err)
	}

	adapter := &KafkaAdapter{
		config:        config,
		subs:          make(map[string]*kafka.Reader),
		running:       false,
		requestTopics: make(map[string]string),
	}

	if config.EnableMetrics {
		var err error
		adapter.metrics, err = metrics.NewMetrics()
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics: %w", err)
		}
	}

	// Создаем writer для producer
	adapter.writer = &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequiredAcks(config.ProducerConfig.RequiredAcks),
		Async:        false,
		BatchSize:    config.BatchSize,
		BatchTimeout: config.FlushInterval,
		Compression:  getCompression(config.Compression),
	}

	return adapter, nil
}

// getCompression преобразует строку в kafka.Compression
func getCompression(compression string) kafka.Compression {
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
func (k *KafkaAdapter) Start(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.running {
		return nil
	}

	k.running = true
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (k *KafkaAdapter) Stop(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.running {
		return nil
	}

	// Закрываем все readers
	for topic, reader := range k.subs {
		if reader != nil {
			_ = reader.Close()
		}
		delete(k.subs, topic)
	}

	// Закрываем writer
	if k.writer != nil {
		_ = k.writer.Close()
	}

	k.running = false
	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (k *KafkaAdapter) IsRunning() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.running
}

// Name возвращает имя компонента (реализация core.Component)
func (k *KafkaAdapter) Name() string {
	return "kafka-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (k *KafkaAdapter) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// Publish публикует сообщение в топик
func (k *KafkaAdapter) Publish(ctx context.Context, subject string, data []byte, headers map[string]string) error {
	start := time.Now()

	msg := kafka.Message{
		Topic: subject,
		Value: data,
	}

	// Добавляем headers
	if headers != nil {
		msg.Headers = make([]kafka.Header, 0, len(headers))
		for k, v := range headers {
			msg.Headers = append(msg.Headers, kafka.Header{
				Key:   k,
				Value: []byte(v),
			})
		}
	}

	err := k.writer.WriteMessages(ctx, msg)
	if err != nil {
		if k.metrics != nil {
			k.metrics.RecordTransport(ctx, "kafka", time.Since(start), false)
		}
		return fmt.Errorf("failed to publish message: %w", err)
	}

	if k.metrics != nil {
		k.metrics.RecordTransport(ctx, "kafka", time.Since(start), true)
	}

	return nil
}

// Subscribe подписывается на топик
func (k *KafkaAdapter) Subscribe(ctx context.Context, subject string, handler transport.MessageHandler) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        k.config.Brokers,
		Topic:          subject,
		GroupID:        k.config.GroupID,
		MinBytes:       k.config.ConsumerConfig.MinBytes,
		MaxBytes:       k.config.ConsumerConfig.MaxBytes,
		MaxWait:        k.config.ConsumerConfig.MaxWait,
		StartOffset:    k.config.ConsumerConfig.StartOffset,
		CommitInterval: k.config.ConsumerConfig.CommitInterval,
	})

	k.mu.Lock()
	k.subs[subject] = reader
	k.mu.Unlock()

	// Запускаем goroutine для чтения сообщений
	go func() {
		defer func() {
			_ = reader.Close()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := reader.FetchMessage(ctx)
				if err != nil {
					if err == context.Canceled {
						return
					}
					// Логируем ошибку
					continue
				}

				mbMsg := &transport.Message{
					Subject: msg.Topic,
					Data:    msg.Value,
					Headers: make(map[string]string),
				}

				// Копируем headers
				for _, h := range msg.Headers {
					mbMsg.Headers[h.Key] = string(h.Value)
				}

				if err := handler(ctx, mbMsg); err != nil {
					// Логируем ошибку, но не прерываем обработку
					_ = err
				} else {
					// Commit offset только при успешной обработке
					_ = reader.CommitMessages(ctx, msg)
				}
			}
		}
	}()

	return nil
}

// Unsubscribe отписывается от топика
func (k *KafkaAdapter) Unsubscribe(subject string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	reader, exists := k.subs[subject]
	if !exists {
		return nil
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("failed to close reader: %w", err)
	}

	delete(k.subs, subject)
	return nil
}

// Request отправляет запрос и ждет ответа (через correlation ID)
func (k *KafkaAdapter) Request(ctx context.Context, subject string, data []byte, timeout time.Duration) (*transport.Message, error) {
	// Генерируем correlation ID и reply topic
	correlationID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	replyTopic := fmt.Sprintf("%s.reply.%s", subject, correlationID)

	// Создаем временный reader для reply topic
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     k.config.Brokers,
		Topic:       replyTopic,
		StartOffset: kafka.LastOffset,
		MaxWait:     timeout,
	})
	defer func() {
		_ = reader.Close()
	}()

	// Сохраняем mapping для cleanup
	k.requestMu.Lock()
	k.requestTopics[correlationID] = replyTopic
	k.requestMu.Unlock()

	// Публикуем запрос с correlation ID
	headers := map[string]string{
		"correlation_id": correlationID,
		"reply_topic":    replyTopic,
	}

	if err := k.Publish(ctx, subject, data, headers); err != nil {
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	// Ждем ответа
	msg, err := reader.FetchMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("request timeout or failed: %w", err)
	}

	mbMsg := &transport.Message{
		Subject: msg.Topic,
		Data:    msg.Value,
		Headers: make(map[string]string),
	}

	for _, h := range msg.Headers {
		mbMsg.Headers[h.Key] = string(h.Value)
	}

	return mbMsg, nil
}

// Respond отвечает на запросы
func (k *KafkaAdapter) Respond(ctx context.Context, subject string, handler func(ctx context.Context, request *transport.Message) (*transport.Message, error)) error {
	// Создаем reader для subject
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        k.config.Brokers,
		Topic:          subject,
		GroupID:        k.config.GroupID,
		MinBytes:       k.config.ConsumerConfig.MinBytes,
		MaxBytes:       k.config.ConsumerConfig.MaxBytes,
		MaxWait:        k.config.ConsumerConfig.MaxWait,
		StartOffset:    k.config.ConsumerConfig.StartOffset,
		CommitInterval: k.config.ConsumerConfig.CommitInterval,
	})

	// Запускаем обработку запросов
	go func() {
		defer func() {
			_ = reader.Close()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := reader.FetchMessage(ctx)
				if err != nil {
					if err == context.Canceled {
						return
					}
					continue
				}

				mbRequest := &transport.Message{
					Subject: msg.Topic,
					Data:    msg.Value,
					Headers: make(map[string]string),
				}

				for _, h := range msg.Headers {
					mbRequest.Headers[h.Key] = string(h.Value)
				}

				// Получаем reply topic из headers
				replyTopic, ok := mbRequest.Headers["reply_topic"]
				if !ok {
					// Нет reply topic - пропускаем
					_ = reader.CommitMessages(ctx, msg)
					continue
				}

				mbReply, err := handler(ctx, mbRequest)
				if err != nil {
					// Отправляем ошибку в reply topic
					errorData, _ := json.Marshal(map[string]string{"error": err.Error()})
					_ = k.Publish(ctx, replyTopic, errorData, nil)
					_ = reader.CommitMessages(ctx, msg)
					continue
				}

				if mbReply != nil {
					// Отправляем ответ в reply topic
					if err := k.Publish(ctx, replyTopic, mbReply.Data, mbReply.Headers); err != nil {
						_ = reader.CommitMessages(ctx, msg)
						continue
					}
				}

				_ = reader.CommitMessages(ctx, msg)
			}
		}
	}()

	return nil
}

// DeadLetterQueue отправляет failed messages в DLQ топик
func (k *KafkaAdapter) DeadLetterQueue(ctx context.Context, topic string, msg *transport.Message, reason string) error {
	dlqTopic := fmt.Sprintf("%s.dlq", topic)
	headers := map[string]string{
		"original_topic": msg.Subject,
		"reason":          reason,
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	return k.Publish(ctx, dlqTopic, msg.Data, headers)
}

