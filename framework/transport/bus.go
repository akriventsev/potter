// Package transport предоставляет базовые реализации шин команд и запросов.
package transport

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// InMemoryCommandBus реализация шины команд в памяти
type InMemoryCommandBus struct {
	mu              sync.RWMutex
	handlers        map[string]CommandHandler
	middleware      []CommandInterceptor
	shutdown        chan struct{}
	shutdownTimeout time.Duration
}

// NewInMemoryCommandBus создает новую шину команд
func NewInMemoryCommandBus() *InMemoryCommandBus {
	return &InMemoryCommandBus{
		handlers:        make(map[string]CommandHandler),
		middleware:       make([]CommandInterceptor, 0),
		shutdown:         make(chan struct{}),
		shutdownTimeout:  30 * time.Second,
	}
}

// Send отправляет команду через шину
func (b *InMemoryCommandBus) Send(ctx context.Context, cmd Command) error {
	b.mu.RLock()
	handler, exists := b.handlers[cmd.CommandName()]
	middleware := b.middleware
	b.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no handler registered for command: %s", cmd.CommandName())
	}

	// Применяем middleware
	next := func(ctx context.Context, cmd Command) error {
		return handler.Handle(ctx, cmd)
	}

	for i := len(middleware) - 1; i >= 0; i-- {
		mw := middleware[i]
		prevNext := next
		next = func(ctx context.Context, cmd Command) error {
			return mw.Intercept(ctx, cmd, prevNext)
		}
	}

	return next(ctx, cmd)
}

// Register регистрирует обработчик команды
func (b *InMemoryCommandBus) Register(handler CommandHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.handlers == nil {
		b.handlers = make(map[string]CommandHandler)
	}

	commandName := handler.CommandName()
	if _, exists := b.handlers[commandName]; exists {
		return fmt.Errorf("handler already registered for command: %s", commandName)
	}

	b.handlers[commandName] = handler
	return nil
}

// WithMiddleware добавляет middleware к шине
func (b *InMemoryCommandBus) WithMiddleware(middleware CommandInterceptor) *InMemoryCommandBus {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.middleware = append(b.middleware, middleware)
	return b
}

// Shutdown корректно завершает работу шины
func (b *InMemoryCommandBus) Shutdown(ctx context.Context) error {
	select {
	case <-b.shutdown:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(b.shutdownTimeout):
		close(b.shutdown)
		return nil
	}
}

// InMemoryQueryBus реализация шины запросов в памяти
type InMemoryQueryBus struct {
	mu              sync.RWMutex
	handlers        map[string]QueryHandler
	middleware      []QueryInterceptor
	cache           QueryCache
	shutdown        chan struct{}
	shutdownTimeout time.Duration
}

// NewInMemoryQueryBus создает новую шину запросов
func NewInMemoryQueryBus() *InMemoryQueryBus {
	return &InMemoryQueryBus{
		handlers:        make(map[string]QueryHandler),
		middleware:      make([]QueryInterceptor, 0),
		shutdown:        make(chan struct{}),
		shutdownTimeout: 30 * time.Second,
	}
}

// WithCache устанавливает кэш для шины
func (b *InMemoryQueryBus) WithCache(cache QueryCache) *InMemoryQueryBus {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cache = cache
	return b
}

// Ask отправляет запрос через шину
func (b *InMemoryQueryBus) Ask(ctx context.Context, q Query) (interface{}, error) {
	// Проверяем кэш
	if b.cache != nil {
		if result, ok := b.cache.Get(ctx, q); ok {
			return result, nil
		}
	}

	b.mu.RLock()
	handler, exists := b.handlers[q.QueryName()]
	middleware := b.middleware
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for query: %s", q.QueryName())
	}

	// Применяем middleware
	next := func(ctx context.Context, q Query) (interface{}, error) {
		return handler.Handle(ctx, q)
	}

	for i := len(middleware) - 1; i >= 0; i-- {
		mw := middleware[i]
		prevNext := next
		next = func(ctx context.Context, q Query) (interface{}, error) {
			return mw.Intercept(ctx, q, prevNext)
		}
	}

	result, err := next(ctx, q)
	if err != nil {
		return nil, err
	}

	// Сохраняем в кэш
	if b.cache != nil && err == nil {
		_ = b.cache.Set(ctx, q, result)
	}

	return result, nil
}

// Register регистрирует обработчик запроса
func (b *InMemoryQueryBus) Register(handler QueryHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.handlers == nil {
		b.handlers = make(map[string]QueryHandler)
	}

	queryName := handler.QueryName()
	if _, exists := b.handlers[queryName]; exists {
		return fmt.Errorf("handler already registered for query: %s", queryName)
	}

	b.handlers[queryName] = handler
	return nil
}

// WithMiddleware добавляет middleware к шине
func (b *InMemoryQueryBus) WithMiddleware(middleware QueryInterceptor) *InMemoryQueryBus {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.middleware = append(b.middleware, middleware)
	return b
}

// Shutdown корректно завершает работу шины
func (b *InMemoryQueryBus) Shutdown(ctx context.Context) error {
	select {
	case <-b.shutdown:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(b.shutdownTimeout):
		close(b.shutdown)
		return nil
	}
}

