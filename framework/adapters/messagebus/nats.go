// Package messagebus предоставляет адаптеры для различных message brokers.
package messagebus

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"potter/framework/core"
	"potter/framework/metrics"
	"potter/framework/transport"
)

// NATSConfig конфигурация для NATS адаптера
type NATSConfig struct {
	URL                string
	MaxReconnects      int
	ReconnectWait      time.Duration
	DrainTimeout       time.Duration
	ConnectionTimeout  time.Duration
	TLS                *tls.Config
	Token              string
	Username           string
	Password           string
	EnableMetrics      bool
	ConnectionPoolSize int
}

// Validate проверяет корректность конфигурации
func (c NATSConfig) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("URL cannot be empty")
	}
	if !strings.HasPrefix(c.URL, "nats://") && !strings.HasPrefix(c.URL, "tls://") {
		return fmt.Errorf("URL must start with nats:// or tls://")
	}
	return nil
}

// DefaultNATSConfig возвращает конфигурацию NATS по умолчанию
func DefaultNATSConfig() NATSConfig {
	return NATSConfig{
		URL:                "nats://localhost:4222",
		MaxReconnects:      10,
		ReconnectWait:      2 * time.Second,
		DrainTimeout:       30 * time.Second,
		ConnectionTimeout:  5 * time.Second,
		EnableMetrics:      true,
		ConnectionPoolSize: 1,
	}
}

// NATSAdapter реализация MessageBus через NATS
type NATSAdapter struct {
	config     NATSConfig
	conn       *nats.Conn
	conns      []*nats.Conn // Connection pool
	subs       map[string]*nats.Subscription
	mu         sync.RWMutex
	running    bool
	metrics    *metrics.Metrics
	connIndex  int // Round-robin для connection pool
	connMu     sync.Mutex
}

// NATSAdapterBuilder построитель для NATS адаптера
type NATSAdapterBuilder struct {
	config NATSConfig
}

// NewNATSAdapterBuilder создает новый построитель NATS адаптера
func NewNATSAdapterBuilder() *NATSAdapterBuilder {
	return &NATSAdapterBuilder{
		config: DefaultNATSConfig(),
	}
}

// WithURL устанавливает URL NATS сервера
func (b *NATSAdapterBuilder) WithURL(url string) *NATSAdapterBuilder {
	b.config.URL = url
	return b
}

// WithMaxReconnects устанавливает максимальное количество переподключений
func (b *NATSAdapterBuilder) WithMaxReconnects(maxReconnects int) *NATSAdapterBuilder {
	b.config.MaxReconnects = maxReconnects
	return b
}

// WithReconnectWait устанавливает задержку между переподключениями
func (b *NATSAdapterBuilder) WithReconnectWait(wait time.Duration) *NATSAdapterBuilder {
	b.config.ReconnectWait = wait
	return b
}

// WithDrainTimeout устанавливает таймаут для graceful shutdown
func (b *NATSAdapterBuilder) WithDrainTimeout(timeout time.Duration) *NATSAdapterBuilder {
	b.config.DrainTimeout = timeout
	return b
}

// WithConnectionTimeout устанавливает таймаут подключения
func (b *NATSAdapterBuilder) WithConnectionTimeout(timeout time.Duration) *NATSAdapterBuilder {
	b.config.ConnectionTimeout = timeout
	return b
}

// WithTLS устанавливает TLS конфигурацию
func (b *NATSAdapterBuilder) WithTLS(tls *tls.Config) *NATSAdapterBuilder {
	b.config.TLS = tls
	return b
}

// WithToken устанавливает токен аутентификации
func (b *NATSAdapterBuilder) WithToken(token string) *NATSAdapterBuilder {
	b.config.Token = token
	return b
}

// WithCredentials устанавливает username и password
func (b *NATSAdapterBuilder) WithCredentials(username, password string) *NATSAdapterBuilder {
	b.config.Username = username
	b.config.Password = password
	return b
}

// WithMetrics включает/выключает метрики
func (b *NATSAdapterBuilder) WithMetrics(enable bool) *NATSAdapterBuilder {
	b.config.EnableMetrics = enable
	return b
}

// WithConnectionPool устанавливает размер connection pool
func (b *NATSAdapterBuilder) WithConnectionPool(size int) *NATSAdapterBuilder {
	b.config.ConnectionPoolSize = size
	return b
}

// Build создает NATS адаптер
func (b *NATSAdapterBuilder) Build() (*NATSAdapter, error) {
	if err := b.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid nats config: %w", err)
	}

	adapter := &NATSAdapter{
		config:  b.config,
		subs:    make(map[string]*nats.Subscription),
		running: false,
	}

	if b.config.EnableMetrics {
		var err error
		adapter.metrics, err = metrics.NewMetrics()
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics: %w", err)
		}
	}

	return adapter, nil
}

// NewNATSAdapter создает новый NATS адаптер с конфигурацией по умолчанию
func NewNATSAdapter(url string) (*NATSAdapter, error) {
	if url == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}
	if !strings.HasPrefix(url, "nats://") && !strings.HasPrefix(url, "tls://") {
		return nil, fmt.Errorf("URL must start with nats:// or tls://")
	}
	builder := NewNATSAdapterBuilder().WithURL(url)
	return builder.Build()
}

// NewNATSAdapterFromConn создает NATS адаптер из существующего подключения
func NewNATSAdapterFromConn(conn *nats.Conn) *NATSAdapter {
	return &NATSAdapter{
		conn:    conn,
		subs:    make(map[string]*nats.Subscription),
		running: true,
		config:  DefaultNATSConfig(),
	}
}

// getConnection возвращает соединение из pool (round-robin)
func (n *NATSAdapter) getConnection() *nats.Conn {
	if n.conn != nil {
		return n.conn
	}

	if len(n.conns) == 0 {
		return nil
	}

	n.connMu.Lock()
	defer n.connMu.Unlock()

	conn := n.conns[n.connIndex]
	n.connIndex = (n.connIndex + 1) % len(n.conns)
	return conn
}

// Start запускает адаптер (реализация core.Lifecycle)
func (n *NATSAdapter) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.running {
		return nil
	}

	opts := []nats.Option{
		nats.MaxReconnects(n.config.MaxReconnects),
		nats.ReconnectWait(n.config.ReconnectWait),
		nats.Timeout(n.config.ConnectionTimeout),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				// Логируем ошибку отключения
				_ = err
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			// Логируем переподключение
		}),
	}

	if n.config.TLS != nil {
		opts = append(opts, nats.Secure(n.config.TLS))
	}

	if n.config.Token != "" {
		opts = append(opts, nats.Token(n.config.Token))
	}

	if n.config.Username != "" && n.config.Password != "" {
		opts = append(opts, nats.UserInfo(n.config.Username, n.config.Password))
	}

	// Создаем connection pool
	n.conns = make([]*nats.Conn, 0, n.config.ConnectionPoolSize)
	for i := 0; i < n.config.ConnectionPoolSize; i++ {
		conn, err := nats.Connect(n.config.URL, opts...)
		if err != nil {
			// Закрываем уже созданные соединения
			for _, c := range n.conns {
				c.Close()
			}
			return fmt.Errorf("failed to connect to NATS (connection %d): %w", i, err)
		}
		n.conns = append(n.conns, conn)
	}

	// Для обратной совместимости устанавливаем первое соединение как основное
	if len(n.conns) > 0 {
		n.conn = n.conns[0]
	}

	n.running = true
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (n *NATSAdapter) Stop(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return nil
	}

	// Отписываемся от всех подписок
	for subject := range n.subs {
		_ = n.Unsubscribe(subject)
	}

	// Drain и закрываем все соединения
	for _, conn := range n.conns {
		if conn != nil && conn.IsConnected() {
			_ = conn.Drain()
			conn.Close()
		}
	}

	if n.conn != nil && n.conn.IsConnected() {
		_ = n.conn.Drain()
		n.conn.Close()
	}

	n.running = false
	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (n *NATSAdapter) IsRunning() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.running
}

// Name возвращает имя компонента (реализация core.Component)
func (n *NATSAdapter) Name() string {
	return "nats-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (n *NATSAdapter) Type() core.ComponentType {
	return core.ComponentTypeAdapter
}

// Publish публикует сообщение в subject
func (n *NATSAdapter) Publish(ctx context.Context, subject string, data []byte, headers map[string]string) error {
	start := time.Now()
	conn := n.getConnection()
	if conn == nil {
		return fmt.Errorf("nats adapter is not connected")
	}

	msg := nats.NewMsg(subject)
	msg.Data = data

	// Добавляем заголовки
	if headers != nil {
		if msg.Header == nil {
			msg.Header = make(nats.Header)
		}
		for k, v := range headers {
			msg.Header.Set(k, v)
		}
	}

	err := conn.PublishMsg(msg)
	if err != nil {
		if n.metrics != nil {
			n.metrics.RecordTransport(ctx, "nats", time.Since(start), false)
		}
		return fmt.Errorf("failed to publish message: %w", err)
	}

	if n.metrics != nil {
		n.metrics.RecordTransport(ctx, "nats", time.Since(start), true)
	}

	return nil
}

// Subscribe подписывается на subject
func (n *NATSAdapter) Subscribe(ctx context.Context, subject string, handler transport.MessageHandler) error {
	conn := n.getConnection()
	if conn == nil {
		return fmt.Errorf("nats adapter is not connected")
	}

	sub, err := conn.Subscribe(subject, func(msg *nats.Msg) {
		mbMsg := &transport.Message{
			Subject: msg.Subject,
			Data:    msg.Data,
			Headers: make(map[string]string),
		}

		// Копируем заголовки
		if msg.Header != nil {
			for k, vals := range msg.Header {
				if len(vals) > 0 {
					mbMsg.Headers[k] = vals[0]
				}
			}
		}

		if err := handler(ctx, mbMsg); err != nil {
			// Логируем ошибку, но не прерываем обработку других сообщений
			_ = err
		}
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	n.mu.Lock()
	n.subs[subject] = sub
	n.mu.Unlock()

	return nil
}

// Unsubscribe отписывается от subject
func (n *NATSAdapter) Unsubscribe(subject string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	sub, exists := n.subs[subject]
	if !exists {
		return nil
	}

	if err := sub.Unsubscribe(); err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	delete(n.subs, subject)
	return nil
}

// Request отправляет запрос и ждет ответа
func (n *NATSAdapter) Request(ctx context.Context, subject string, data []byte, timeout time.Duration) (*transport.Message, error) {
	conn := n.getConnection()
	if conn == nil {
		return nil, fmt.Errorf("nats adapter is not connected")
	}

	msg := nats.NewMsg(subject)
	msg.Data = data

	// Создаем контекст с timeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reply, err := conn.RequestMsgWithContext(reqCtx, msg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	mbMsg := &transport.Message{
		Subject: reply.Subject,
		Data:    reply.Data,
		Headers: make(map[string]string),
	}

	if reply.Header != nil {
		for k, vals := range reply.Header {
			if len(vals) > 0 {
				mbMsg.Headers[k] = vals[0]
			}
		}
	}

	return mbMsg, nil
}

// Respond отвечает на запросы
func (n *NATSAdapter) Respond(ctx context.Context, subject string, handler func(ctx context.Context, request *transport.Message) (*transport.Message, error)) error {
	conn := n.getConnection()
	if conn == nil {
		return fmt.Errorf("nats adapter is not connected")
	}

	_, err := conn.Subscribe(subject, func(msg *nats.Msg) {
		mbRequest := &transport.Message{
			Subject: msg.Subject,
			Data:    msg.Data,
			Headers: make(map[string]string),
		}

		if msg.Header != nil {
			for k, vals := range msg.Header {
				if len(vals) > 0 {
					mbRequest.Headers[k] = vals[0]
				}
			}
		}

		mbReply, err := handler(ctx, mbRequest)
		if err != nil {
			// Отправляем пустой ответ или ошибку
			if msg.Reply != "" {
				_ = msg.Respond(nil)
			}
			return
		}

		if msg.Reply != "" && mbReply != nil {
			replyMsg := nats.NewMsg(msg.Reply)
			replyMsg.Data = mbReply.Data

			if mbReply.Headers != nil {
				if replyMsg.Header == nil {
					replyMsg.Header = make(nats.Header)
				}
				for k, v := range mbReply.Headers {
					replyMsg.Header.Set(k, v)
				}
			}

			_ = conn.PublishMsg(replyMsg)
		}
	})

	return err
}

// Close закрывает подключение (для обратной совместимости)
func (n *NATSAdapter) Close() error {
	return n.Stop(context.Background())
}

// Conn возвращает NATS соединение (для обратной совместимости)
func (n *NATSAdapter) Conn() *nats.Conn {
	return n.getConnection()
}

// NATS ошибки
var (
	ErrNATSConnectionFailed = fmt.Errorf("nats: connection failed")
	ErrNATSPublishFailed    = fmt.Errorf("nats: publish failed")
	ErrNATSSubscribeFailed  = fmt.Errorf("nats: subscribe failed")
	ErrNATSRequestFailed    = fmt.Errorf("nats: request failed")
)

