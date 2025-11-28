// Package transport предоставляет базовые классы и утилиты для REST, gRPC, WebSocket транспортов.
package transport

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"potter/framework/core"
	"potter/framework/events"
	"potter/framework/metrics"
	"potter/framework/transport"
)

// WebSocketConfig конфигурация для WebSocket адаптера
type WebSocketConfig struct {
	Port            int
	Path            string
	ReadBufferSize  int
	WriteBufferSize int
	PingInterval    time.Duration
	PongWait        time.Duration
	MaxMessageSize  int64
	EnableMetrics   bool
}

// DefaultWebSocketConfig возвращает конфигурацию WebSocket по умолчанию
func DefaultWebSocketConfig() WebSocketConfig {
	return WebSocketConfig{
		Port:            8081,
		Path:            "/ws",
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		PingInterval:    54 * time.Second,
		PongWait:        60 * time.Second,
		MaxMessageSize:  512,
		EnableMetrics:   true,
	}
}

// WebSocketAdapter WebSocket server с CommandBus/QueryBus/EventPublisher
type WebSocketAdapter struct {
	config        WebSocketConfig
	upgrader      websocket.Upgrader
	commandBus    transport.CommandBus
	queryBus      transport.QueryBus
	eventPublisher events.EventPublisher
	connections   map[*websocket.Conn]bool
	mu            sync.RWMutex
	metrics       *metrics.Metrics
	running       bool
	server        *http.Server
}

// NewWebSocketAdapter создает новый WebSocket адаптер
func NewWebSocketAdapter(config WebSocketConfig, commandBus transport.CommandBus, queryBus transport.QueryBus, eventPublisher events.EventPublisher) (*WebSocketAdapter, error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  config.ReadBufferSize,
		WriteBufferSize: config.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return true // В production должна быть правильная проверка origin
		},
	}

	adapter := &WebSocketAdapter{
		config:        config,
		upgrader:      upgrader,
		commandBus:    commandBus,
		queryBus:      queryBus,
		eventPublisher: eventPublisher,
		connections:   make(map[*websocket.Conn]bool),
		running:       false,
	}

	if config.EnableMetrics {
		var err error
		adapter.metrics, err = metrics.NewMetrics()
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics: %w", err)
		}
	}

	return adapter, nil
}

// Start запускает адаптер (реализация core.Lifecycle)
func (w *WebSocketAdapter) Start(ctx context.Context) error {
	w.running = true

	mux := http.NewServeMux()
	mux.HandleFunc(w.config.Path, w.handleWebSocket)

	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.config.Port),
		Handler: mux,
	}

	go func() {
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			_ = err
		}
	}()

	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (w *WebSocketAdapter) Stop(ctx context.Context) error {
	w.running = false

	// Закрываем все соединения
	w.mu.Lock()
	for conn := range w.connections {
		_ = conn.Close()
		delete(w.connections, conn)
	}
	w.mu.Unlock()

	if w.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return w.server.Shutdown(shutdownCtx)
	}

	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (w *WebSocketAdapter) IsRunning() bool {
	return w.running
}

// Name возвращает имя компонента (реализация core.Component)
func (w *WebSocketAdapter) Name() string {
	return "websocket-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (w *WebSocketAdapter) Type() core.ComponentType {
	return core.ComponentTypeTransport
}

// handleWebSocket обрабатывает WebSocket соединения
func (w *WebSocketAdapter) handleWebSocket(rw http.ResponseWriter, r *http.Request) {
	conn, err := w.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return
	}

	w.mu.Lock()
	w.connections[conn] = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		delete(w.connections, conn)
		w.mu.Unlock()
		_ = conn.Close()
	}()

	// Настройка ping/pong
	_ = conn.SetReadDeadline(time.Now().Add(w.config.PongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(w.config.PongWait))
		return nil
	})

	// Обработка сообщений
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		// Обработка команды/запроса через WebSocket
		// NOTE: Текущая реализация только управляет соединениями и не предоставляет
		// встроенной маршрутизации сообщений к command/query handlers или broadcasting
		// событий через eventPublisher. Высокоуровневая маршрутизация должна быть
		// реализована на уровне приложения.
		_ = msg
	}
}

// Broadcast отправляет сообщение всем подключенным клиентам
func (w *WebSocketAdapter) Broadcast(message interface{}) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for conn := range w.connections {
		if err := conn.WriteJSON(message); err != nil {
			_ = conn.Close()
			delete(w.connections, conn)
		}
	}

	return nil
}

