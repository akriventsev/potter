// Package transport предоставляет resolvers для интеграции GraphQL с Potter CQRS.
package transport

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/transport"
)

// ResolverFunc тип функции resolver
// type ResolverFunc func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// BaseResolver базовый resolver с доступом к buses
type BaseResolver struct {
	commandBus transport.CommandBus
	queryBus   transport.QueryBus
	eventBus   events.EventBus
}

// NewBaseResolver создает новый базовый resolver
func NewBaseResolver(commandBus transport.CommandBus, queryBus transport.QueryBus, eventBus events.EventBus) *BaseResolver {
	return &BaseResolver{
		commandBus: commandBus,
		queryBus:   queryBus,
		eventBus:   eventBus,
	}
}

// CommandResolver resolver для GraphQL mutations → CQRS commands
type CommandResolver struct {
	*BaseResolver
}

// NewCommandResolver создает новый command resolver
func NewCommandResolver(base *BaseResolver) *CommandResolver {
	return &CommandResolver{BaseResolver: base}
}

// Resolve создает Command из args и отправляет через CommandBus
func (r *CommandResolver) Resolve(ctx context.Context, commandName string, args map[string]interface{}) (interface{}, error) {
	// Создаем базовую команду с метаданными
	correlationID := getCorrelationID(ctx)
	commandID := uuid.New().String()
	
	metadata := transport.NewBaseCommandMetadata(
		commandID,
		correlationID,
		"", // causation ID будет установлен при необходимости
	)
	
	// Создаем команду с именем
	command := transport.NewBaseCommand(commandName, metadata)
	
	// Маппинг GraphQL args → Command fields
	// В реальной реализации нужно использовать reflection или code generation
	// для маппинга args в конкретный тип команды
	
	// Отправка команды
	if err := r.commandBus.Send(ctx, command); err != nil {
		return nil, fmt.Errorf("failed to execute command %s: %w", commandName, err)
	}
	
	return map[string]interface{}{
		"success": true,
		"command_id": commandID,
		"message": fmt.Sprintf("Command %s executed successfully", commandName),
	}, nil
}

// QueryResolver resolver для GraphQL queries → CQRS queries
type QueryResolver struct {
	*BaseResolver
	cache map[string]interface{} // Простой in-memory cache (в production использовать Redis)
}

// NewQueryResolver создает новый query resolver
func NewQueryResolver(base *BaseResolver) *QueryResolver {
	return &QueryResolver{
		BaseResolver: base,
		cache:        make(map[string]interface{}),
	}
}

// Resolve создает Query из args и отправляет через QueryBus
func (r *QueryResolver) Resolve(ctx context.Context, queryName string, args map[string]interface{}) (interface{}, error) {
	// Проверка кэша (если QueryOptions.cacheable = true)
	cacheKey := fmt.Sprintf("%s:%v", queryName, args)
	if cached, ok := r.cache[cacheKey]; ok {
		return cached, nil
	}
	
	// Создаем базовый запрос с метаданными
	correlationID := getCorrelationID(ctx)
	queryID := uuid.New().String()
	
	metadata := transport.NewBaseQueryMetadata(queryID, correlationID)
	
	// Создаем запрос с именем
	// В реальной реализации нужно использовать reflection или code generation
	// для создания конкретного типа запроса из args
	query := &BaseQuery{
		queryName: queryName,
		metadata:  metadata,
		args:      args,
	}
	
	// Отправка запроса
	result, err := r.queryBus.Ask(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query %s: %w", queryName, err)
	}
	
	// Сохранение в кэш (если нужно)
	// В реальной реализации проверять QueryOptions.cacheable
	r.cache[cacheKey] = result
	
	return result, nil
}

// BaseQuery базовая реализация Query для GraphQL resolver
type BaseQuery struct {
	queryName string
	metadata  transport.QueryMetadata
	args      map[string]interface{}
}

// QueryName возвращает имя запроса
func (q *BaseQuery) QueryName() string {
	return q.queryName
}

// Metadata возвращает метаданные запроса
func (q *BaseQuery) Metadata() transport.QueryMetadata {
	return q.metadata
}

// Args возвращает аргументы GraphQL запроса
func (q *BaseQuery) Args() map[string]interface{} {
	return q.args
}

// SubscriptionResolver resolver для GraphQL subscriptions → EventBus
type SubscriptionResolver struct {
	*BaseResolver
	subscriptionManager *SubscriptionManager
}

// NewSubscriptionResolver создает новый subscription resolver
func NewSubscriptionResolver(base *BaseResolver, subscriptionManager *SubscriptionManager) *SubscriptionResolver {
	return &SubscriptionResolver{
		BaseResolver:       base,
		subscriptionManager: subscriptionManager,
	}
}

// Subscribe подписывается на события через EventBus
func (r *SubscriptionResolver) Subscribe(ctx context.Context, eventType string) (<-chan events.Event, error) {
	// Создаем подписку через SubscriptionManager
	channel, err := r.subscriptionManager.Subscribe(ctx, eventType, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to event %s: %w", eventType, err)
	}
	
	return channel, nil
}

// ResolverRegistry реестр resolvers
type ResolverRegistry struct {
	resolvers map[string]ResolverFunc
	mu        sync.RWMutex
}

// NewResolverRegistry создает новый реестр resolvers
func NewResolverRegistry() *ResolverRegistry {
	return &ResolverRegistry{
		resolvers: make(map[string]ResolverFunc),
	}
}

// Register регистрирует resolver
func (r *ResolverRegistry) Register(typeName, fieldName string, resolver ResolverFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s.%s", typeName, fieldName)
	r.resolvers[key] = resolver
}

// Get получает resolver
func (r *ResolverRegistry) Get(typeName, fieldName string) (ResolverFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := fmt.Sprintf("%s.%s", typeName, fieldName)
	resolver, ok := r.resolvers[key]
	return resolver, ok
}

// AutoRegister автоматически регистрирует resolvers из ParsedSpec
func (r *ResolverRegistry) AutoRegister(spec interface{}) error {
	// В реальной реализации нужно парсить ParsedSpec и создавать resolvers
	// для каждого Command/Query/Event
	// Это будет интегрировано с codegen системой
	return nil
}

// getCorrelationID извлекает correlation ID из context
func getCorrelationID(ctx context.Context) string {
	// Пытаемся получить из context metadata
	if val := ctx.Value("correlation_id"); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return uuid.New().String()
}

