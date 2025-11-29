// Package cqrs предоставляет построители для конфигурации обработчиков.
package cqrs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/akriventsev/potter/framework/metrics"
	"github.com/akriventsev/potter/framework/transport"
)

// CommandHandlerBuilder построитель обработчиков команд
type CommandHandlerBuilder struct {
	name         string
	handler      transport.CommandHandler
	withMetrics  bool
	metrics      *metrics.Metrics
	dependencies map[string]interface{}
	middleware   []CommandMiddleware
	retryConfig  *RetryConfig
	circuitBreaker *CircuitBreakerConfig
}

// RetryConfig конфигурация повторов
type RetryConfig struct {
	MaxAttempts int
	Delay       time.Duration
	Backoff     time.Duration
}

// CircuitBreakerConfig конфигурация circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int
	Timeout          time.Duration
}

// CommandMiddleware middleware для обработчиков команд
type CommandMiddleware func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error

// NewCommandHandlerBuilder создает новый построитель обработчиков команд
func NewCommandHandlerBuilder(name string, handler transport.CommandHandler) *CommandHandlerBuilder {
	return &CommandHandlerBuilder{
		name:         name,
		handler:      handler,
		dependencies: make(map[string]interface{}),
		middleware:   make([]CommandMiddleware, 0),
	}
}

// WithMetrics добавляет метрики к обработчику
func (b *CommandHandlerBuilder) WithMetrics(m *metrics.Metrics) *CommandHandlerBuilder {
	b.withMetrics = true
	b.metrics = m
	return b
}

// WithDependency добавляет зависимость
func (b *CommandHandlerBuilder) WithDependency(key string, value interface{}) *CommandHandlerBuilder {
	b.dependencies[key] = value
	return b
}

// WithMiddleware добавляет middleware
func (b *CommandHandlerBuilder) WithMiddleware(middleware CommandMiddleware) *CommandHandlerBuilder {
	b.middleware = append(b.middleware, middleware)
	return b
}

// WithRetry добавляет поддержку автоматических повторов
func (b *CommandHandlerBuilder) WithRetry(maxAttempts int, delay, backoff time.Duration) *CommandHandlerBuilder {
	b.retryConfig = &RetryConfig{
		MaxAttempts: maxAttempts,
		Delay:       delay,
		Backoff:     backoff,
	}
	return b
}

// WithCircuitBreaker добавляет circuit breaker для защиты от каскадных сбоев
func (b *CommandHandlerBuilder) WithCircuitBreaker(failureThreshold int, timeout time.Duration) *CommandHandlerBuilder {
	b.circuitBreaker = &CircuitBreakerConfig{
		FailureThreshold: failureThreshold,
		Timeout:          timeout,
	}
	return b
}

// WithConditionalMiddleware добавляет условный middleware
func (b *CommandHandlerBuilder) WithConditionalMiddleware(condition func(ctx context.Context, cmd transport.Command) bool, middleware CommandMiddleware) *CommandHandlerBuilder {
	conditionalMw := func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error {
		if condition(ctx, cmd) {
			return middleware(ctx, cmd, next)
		}
		return next(ctx, cmd)
	}
	b.middleware = append(b.middleware, conditionalMw)
	return b
}

// Build создает обработчик с примененными настройками
func (b *CommandHandlerBuilder) Build() transport.CommandHandler {
	handler := b.handler

	// Применяем middleware (в обратном порядке)
	for i := len(b.middleware) - 1; i >= 0; i-- {
		mw := b.middleware[i]
		next := handler
		handler = &wrappedCommandHandler{
			handler:    next,
			middleware: mw,
		}
	}

	// Применяем retry если настроен
	if b.retryConfig != nil {
		handler = &retryCommandHandler{
			handler:    handler,
			retryConfig: b.retryConfig,
		}
	}

	// Применяем circuit breaker если настроен
	if b.circuitBreaker != nil {
		handler = &circuitBreakerCommandHandler{
			handler:    handler,
			config:     b.circuitBreaker,
		}
	}

	// Применяем метрики
	if b.withMetrics && b.metrics != nil {
		handler = &metricsCommandHandler{
			handler: handler,
			metrics: b.metrics,
			name:    b.name,
		}
	}

	return handler
}

// wrappedCommandHandler обработчик с middleware
type wrappedCommandHandler struct {
	handler    transport.CommandHandler
	middleware CommandMiddleware
}

func (h *wrappedCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	return h.middleware(ctx, cmd, h.handler.Handle)
}

func (h *wrappedCommandHandler) CommandName() string {
	return h.handler.CommandName()
}

// retryCommandHandler обработчик с повторами
type retryCommandHandler struct {
	handler    transport.CommandHandler
	retryConfig *RetryConfig
}

func (h *retryCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	var lastErr error
	delay := h.retryConfig.Delay

	for attempt := 0; attempt < h.retryConfig.MaxAttempts; attempt++ {
		if err := h.handler.Handle(ctx, cmd); err == nil {
			return nil
		} else {
			lastErr = err
			if attempt < h.retryConfig.MaxAttempts-1 {
				time.Sleep(delay)
				delay += h.retryConfig.Backoff
			}
		}
	}
	return lastErr
}

func (h *retryCommandHandler) CommandName() string {
	return h.handler.CommandName()
}

// circuitBreakerCommandHandler обработчик с circuit breaker
type circuitBreakerCommandHandler struct {
	handler    transport.CommandHandler
	config     *CircuitBreakerConfig
	failures   int
	lastFailure time.Time
	mu         sync.Mutex
}

func (h *circuitBreakerCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	h.mu.Lock()
	if h.failures >= h.config.FailureThreshold {
		if time.Since(h.lastFailure) < h.config.Timeout {
			h.mu.Unlock()
			return fmt.Errorf("circuit breaker is open")
		}
		// Reset после timeout
		h.failures = 0
	}
	h.mu.Unlock()

	err := h.handler.Handle(ctx, cmd)
	if err != nil {
		h.mu.Lock()
		h.failures++
		h.lastFailure = time.Now()
		h.mu.Unlock()
	} else {
		h.mu.Lock()
		h.failures = 0
		h.mu.Unlock()
	}
	return err
}

func (h *circuitBreakerCommandHandler) CommandName() string {
	return h.handler.CommandName()
}

// metricsCommandHandler обработчик с метриками
type metricsCommandHandler struct {
	handler transport.CommandHandler
	metrics *metrics.Metrics
	name    string
}

func (h *metricsCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	h.metrics.IncrementActiveCommands(ctx)
	defer h.metrics.DecrementActiveCommands(ctx)

	start := time.Now()
	defer func() {
		h.metrics.RecordCommand(ctx, h.name, time.Since(start), true)
	}()

	if err := h.handler.Handle(ctx, cmd); err != nil {
		h.metrics.RecordCommand(ctx, h.name, time.Since(start), false)
		return err
	}

	return nil
}

func (h *metricsCommandHandler) CommandName() string {
	return h.handler.CommandName()
}

// QueryHandlerBuilder построитель обработчиков запросов
type QueryHandlerBuilder struct {
	name         string
	handler      transport.QueryHandler
	withMetrics  bool
	metrics      *metrics.Metrics
	dependencies map[string]interface{}
	middleware   []QueryMiddleware
	cacheConfig  *CacheConfig
	retryConfig  *RetryConfig
	circuitBreaker *CircuitBreakerConfig
}

// CacheConfig конфигурация кэширования
type CacheConfig struct {
	TTL time.Duration
}

// QueryMiddleware middleware для обработчиков запросов
type QueryMiddleware func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error)

// NewQueryHandlerBuilder создает новый построитель обработчиков запросов
func NewQueryHandlerBuilder(name string, handler transport.QueryHandler) *QueryHandlerBuilder {
	return &QueryHandlerBuilder{
		name:         name,
		handler:      handler,
		dependencies: make(map[string]interface{}),
		middleware:   make([]QueryMiddleware, 0),
	}
}

// WithMetrics добавляет метрики к обработчику
func (b *QueryHandlerBuilder) WithMetrics(m *metrics.Metrics) *QueryHandlerBuilder {
	b.withMetrics = true
	b.metrics = m
	return b
}

// WithDependency добавляет зависимость
func (b *QueryHandlerBuilder) WithDependency(key string, value interface{}) *QueryHandlerBuilder {
	b.dependencies[key] = value
	return b
}

// WithMiddleware добавляет middleware
func (b *QueryHandlerBuilder) WithMiddleware(middleware QueryMiddleware) *QueryHandlerBuilder {
	b.middleware = append(b.middleware, middleware)
	return b
}

// WithCache добавляет кэширование результатов запросов
func (b *QueryHandlerBuilder) WithCache(ttl time.Duration) *QueryHandlerBuilder {
	b.cacheConfig = &CacheConfig{TTL: ttl}
	return b
}

// WithRetry добавляет поддержку автоматических повторов
func (b *QueryHandlerBuilder) WithRetry(maxAttempts int, delay, backoff time.Duration) *QueryHandlerBuilder {
	b.retryConfig = &RetryConfig{
		MaxAttempts: maxAttempts,
		Delay:       delay,
		Backoff:     backoff,
	}
	return b
}

// WithCircuitBreaker добавляет circuit breaker для защиты от каскадных сбоев
func (b *QueryHandlerBuilder) WithCircuitBreaker(failureThreshold int, timeout time.Duration) *QueryHandlerBuilder {
	b.circuitBreaker = &CircuitBreakerConfig{
		FailureThreshold: failureThreshold,
		Timeout:          timeout,
	}
	return b
}

// WithConditionalMiddleware добавляет условный middleware
func (b *QueryHandlerBuilder) WithConditionalMiddleware(condition func(ctx context.Context, q transport.Query) bool, middleware QueryMiddleware) *QueryHandlerBuilder {
	conditionalMw := func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		if condition(ctx, q) {
			return middleware(ctx, q, next)
		}
		return next(ctx, q)
	}
	b.middleware = append(b.middleware, conditionalMw)
	return b
}

// Build создает обработчик с примененными настройками
func (b *QueryHandlerBuilder) Build() transport.QueryHandler {
	handler := b.handler

	// Применяем middleware (в обратном порядке)
	for i := len(b.middleware) - 1; i >= 0; i-- {
		mw := b.middleware[i]
		next := handler
		handler = &wrappedQueryHandler{
			handler:    next,
			middleware: mw,
		}
	}

	// Применяем кэш если настроен
	if b.cacheConfig != nil {
		handler = &cachedQueryHandler{
			handler:    handler,
			cacheConfig: b.cacheConfig,
		}
	}

	// Применяем retry если настроен
	if b.retryConfig != nil {
		handler = &retryQueryHandler{
			handler:    handler,
			retryConfig: b.retryConfig,
		}
	}

	// Применяем circuit breaker если настроен
	if b.circuitBreaker != nil {
		handler = &circuitBreakerQueryHandler{
			handler:    handler,
			config:     b.circuitBreaker,
		}
	}

	// Применяем метрики
	if b.withMetrics && b.metrics != nil {
		handler = &metricsQueryHandler{
			handler: handler,
			metrics: b.metrics,
			name:    b.name,
		}
	}

	return handler
}

// wrappedQueryHandler обработчик с middleware
type wrappedQueryHandler struct {
	handler    transport.QueryHandler
	middleware QueryMiddleware
}

func (h *wrappedQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	return h.middleware(ctx, q, h.handler.Handle)
}

func (h *wrappedQueryHandler) QueryName() string {
	return h.handler.QueryName()
}

// cachedQueryHandler обработчик с кэшированием
type cachedQueryHandler struct {
	handler    transport.QueryHandler
	cacheConfig *CacheConfig
	cache      map[string]cacheEntry
	mu         sync.RWMutex
}

type cacheEntry struct {
	value      interface{}
	expiresAt  time.Time
}

func (h *cachedQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	// Простая реализация кэша в памяти
	// В продакшене лучше использовать Redis или другой внешний кэш
	key := q.QueryName()
	
	h.mu.RLock()
	if entry, ok := h.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		h.mu.RUnlock()
		return entry.value, nil
	}
	h.mu.RUnlock()

	result, err := h.handler.Handle(ctx, q)
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	if h.cache == nil {
		h.cache = make(map[string]cacheEntry)
	}
	h.cache[key] = cacheEntry{
		value:     result,
		expiresAt: time.Now().Add(h.cacheConfig.TTL),
	}
	h.mu.Unlock()

	return result, nil
}

func (h *cachedQueryHandler) QueryName() string {
	return h.handler.QueryName()
}

// retryQueryHandler обработчик с повторами
type retryQueryHandler struct {
	handler    transport.QueryHandler
	retryConfig *RetryConfig
}

func (h *retryQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	var lastErr error
	delay := h.retryConfig.Delay

	for attempt := 0; attempt < h.retryConfig.MaxAttempts; attempt++ {
		if result, err := h.handler.Handle(ctx, q); err == nil {
			return result, nil
		} else {
			lastErr = err
			if attempt < h.retryConfig.MaxAttempts-1 {
				time.Sleep(delay)
				delay += h.retryConfig.Backoff
			}
		}
	}
	return nil, lastErr
}

func (h *retryQueryHandler) QueryName() string {
	return h.handler.QueryName()
}

// circuitBreakerQueryHandler обработчик с circuit breaker
type circuitBreakerQueryHandler struct {
	handler    transport.QueryHandler
	config     *CircuitBreakerConfig
	failures   int
	lastFailure time.Time
	mu         sync.Mutex
}

func (h *circuitBreakerQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	h.mu.Lock()
	if h.failures >= h.config.FailureThreshold {
		if time.Since(h.lastFailure) < h.config.Timeout {
			h.mu.Unlock()
			return nil, fmt.Errorf("circuit breaker is open")
		}
		// Reset после timeout
		h.failures = 0
	}
	h.mu.Unlock()

	result, err := h.handler.Handle(ctx, q)
	if err != nil {
		h.mu.Lock()
		h.failures++
		h.lastFailure = time.Now()
		h.mu.Unlock()
	} else {
		h.mu.Lock()
		h.failures = 0
		h.mu.Unlock()
	}
	return result, err
}

func (h *circuitBreakerQueryHandler) QueryName() string {
	return h.handler.QueryName()
}

// metricsQueryHandler обработчик с метриками
type metricsQueryHandler struct {
	handler transport.QueryHandler
	metrics *metrics.Metrics
	name    string
}

func (h *metricsQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	h.metrics.IncrementActiveQueries(ctx)
	defer h.metrics.DecrementActiveQueries(ctx)

	start := time.Now()
	defer func() {
		h.metrics.RecordQuery(ctx, h.name, time.Since(start), true)
	}()

	result, err := h.handler.Handle(ctx, q)
	if err != nil {
		h.metrics.RecordQuery(ctx, h.name, time.Since(start), false)
		return nil, err
	}

	return result, nil
}

func (h *metricsQueryHandler) QueryName() string {
	return h.handler.QueryName()
}

