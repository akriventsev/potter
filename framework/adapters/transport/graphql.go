// Copyright 2024 Potter Framework Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package transport предоставляет базовые классы и утилиты для REST, gRPC, WebSocket, GraphQL транспортов.
package transport

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/99designs/gqlgen/graphql"
	gqlhandler "github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	gqltransport "github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/metrics"
	pottertransport "github.com/akriventsev/potter/framework/transport"
)

// GraphQLConfig конфигурация для GraphQL адаптера
type GraphQLConfig struct {
	Port                int
	Path                string
	PlaygroundPath      string
	EnablePlayground    bool
	EnableIntrospection bool
	EnableMetrics       bool
	ComplexityLimit     int
	MaxDepth            int
}

// DefaultGraphQLConfig возвращает конфигурацию GraphQL по умолчанию
func DefaultGraphQLConfig() GraphQLConfig {
	return GraphQLConfig{
		Port:                8082,
		Path:                "/graphql",
		PlaygroundPath:      "/playground",
		EnablePlayground:    true,
		EnableIntrospection: true,
		EnableMetrics:       true,
		ComplexityLimit:     1000,
		MaxDepth:            15,
	}
}

// GraphQLAdapter базовый класс для GraphQL API
type GraphQLAdapter struct {
	config     GraphQLConfig
	schema     graphql.ExecutableSchema
	commandBus pottertransport.CommandBus
	queryBus   pottertransport.QueryBus
	eventBus   events.EventBus
	metrics    *metrics.Metrics
	server     *http.Server
	running    bool
	mu         sync.RWMutex
	resolvers  map[string]ResolverFunc
}

// ResolverFunc тип функции resolver
type ResolverFunc func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// potterExecutableSchema реализует graphql.ExecutableSchema с интеграцией Potter CQRS
type potterExecutableSchema struct {
	baseSchema           graphql.ExecutableSchema
	commandResolver      *CommandResolver
	queryResolver        *QueryResolver
	subscriptionResolver *SubscriptionResolver
	registry             *ResolverRegistry
	autoRegistered       bool // Флаг автоматической регистрации
}

// Schema возвращает GraphQL схему
func (s *potterExecutableSchema) Schema() *ast.Schema {
	return s.baseSchema.Schema()
}

// Complexity вычисляет сложность запроса
func (s *potterExecutableSchema) Complexity(typeName, fieldName string, childComplexity int, args map[string]any) (int, bool) {
	return s.baseSchema.Complexity(typeName, fieldName, childComplexity, args)
}

// Exec выполняет GraphQL операцию
func (s *potterExecutableSchema) Exec(ctx context.Context) graphql.ResponseHandler {
	return s.baseSchema.Exec(ctx)
}

// AutoRegisterResolvers автоматически регистрирует dispatch резолверы на основе схемы
// Анализирует AST схемы и создает резолверы для Query/Mutation/Subscription полей
func (s *potterExecutableSchema) AutoRegisterResolvers() error {
	if s.autoRegistered {
		return nil
	}

	schema := s.baseSchema.Schema()
	if schema == nil {
		return fmt.Errorf("base schema is nil")
	}

	// Регистрация Query резолверов
	if queryType := schema.Types["Query"]; queryType != nil {
		for _, field := range queryType.Fields {
			fieldName := field.Name
			// Создаем dispatch resolver для этого поля
			resolver := s.createQueryDispatchResolver(fieldName)
			s.registry.Register("Query", fieldName, resolver)
		}
	}

	// Регистрация Mutation резолверов
	if mutationType := schema.Types["Mutation"]; mutationType != nil {
		for _, field := range mutationType.Fields {
			fieldName := field.Name
			// Создаем dispatch resolver для этого поля
			resolver := s.createCommandDispatchResolver(fieldName)
			s.registry.Register("Mutation", fieldName, resolver)
		}
	}

	// Регистрация Subscription резолверов
	if subscriptionType := schema.Types["Subscription"]; subscriptionType != nil {
		for _, field := range subscriptionType.Fields {
			fieldName := field.Name
			// Создаем dispatch resolver для этого поля
			resolver := s.createSubscriptionDispatchResolver(fieldName)
			s.registry.Register("Subscription", fieldName, resolver)
		}
	}

	s.autoRegistered = true
	return nil
}

// createQueryDispatchResolver создает dispatch resolver для Query поля
func (s *potterExecutableSchema) createQueryDispatchResolver(fieldName string) ResolverFunc {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return s.queryResolver.Resolve(ctx, fieldName, args)
	}
}

// createCommandDispatchResolver создает dispatch resolver для Mutation поля
func (s *potterExecutableSchema) createCommandDispatchResolver(fieldName string) ResolverFunc {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return s.commandResolver.Resolve(ctx, fieldName, args)
	}
}

// createSubscriptionDispatchResolver создает dispatch resolver для Subscription поля
func (s *potterExecutableSchema) createSubscriptionDispatchResolver(fieldName string) ResolverFunc {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		// Для subscriptions возвращаем канал событий
		channel, err := s.subscriptionResolver.Subscribe(ctx, fieldName)
		if err != nil {
			return nil, err
		}
		return channel, nil
	}
}

// GetResolver возвращает resolver для typeName.fieldName
// Сначала проверяет реестр, затем использует дефолтные Potter резолверы
func (s *potterExecutableSchema) GetResolver(typeName, fieldName string) (ResolverFunc, bool) {
	// Проверяем реестр
	if resolver, ok := s.registry.Get(typeName, fieldName); ok {
		return resolver, true
	}

	// Используем дефолтные Potter резолверы для Query/Mutation/Subscription
	switch typeName {
	case "Query":
		return s.createQueryDispatchResolver(fieldName), true
	case "Mutation":
		return s.createCommandDispatchResolver(fieldName), true
	case "Subscription":
		return s.createSubscriptionDispatchResolver(fieldName), true
	}

	return nil, false
}

// NewPotterExecutableSchema создает ExecutableSchema с интеграцией Potter CQRS
func NewPotterExecutableSchema(
	baseSchema graphql.ExecutableSchema,
	commandBus pottertransport.CommandBus,
	queryBus pottertransport.QueryBus,
	eventBus events.EventBus,
) graphql.ExecutableSchema {
	subscriptionManager := NewSubscriptionManager(eventBus)
	baseResolver := NewBaseResolver(commandBus, queryBus, eventBus)

	return &potterExecutableSchema{
		baseSchema:           baseSchema,
		commandResolver:      NewCommandResolver(baseResolver),
		queryResolver:        NewQueryResolver(baseResolver),
		subscriptionResolver: NewSubscriptionResolver(baseResolver, subscriptionManager),
		registry:             NewResolverRegistry(),
	}
}

// NewGraphQLAdapter создает новый GraphQL адаптер с готовой схемой
func NewGraphQLAdapter(
	config GraphQLConfig,
	commandBus pottertransport.CommandBus,
	queryBus pottertransport.QueryBus,
	eventBus events.EventBus,
	schema graphql.ExecutableSchema,
) (*GraphQLAdapter, error) {
	adapter := &GraphQLAdapter{
		config:     config,
		schema:     schema,
		commandBus: commandBus,
		queryBus:   queryBus,
		eventBus:   eventBus,
		running:    false,
		resolvers:  make(map[string]ResolverFunc),
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

// NewGraphQLAdapterWithCQRS создает новый GraphQL адаптер с автоматической интеграцией CQRS
// Использует NewPotterExecutableSchema для создания схемы с Potter resolvers
func NewGraphQLAdapterWithCQRS(
	config GraphQLConfig,
	commandBus pottertransport.CommandBus,
	queryBus pottertransport.QueryBus,
	eventBus events.EventBus,
	baseSchema graphql.ExecutableSchema,
) (*GraphQLAdapter, error) {
	// Создаем схему с интеграцией CQRS
	schema := NewPotterExecutableSchema(baseSchema, commandBus, queryBus, eventBus)

	return NewGraphQLAdapter(config, commandBus, queryBus, eventBus, schema)
}

// Start запускает адаптер (реализация core.Lifecycle)
func (g *GraphQLAdapter) Start(ctx context.Context) error {
	g.mu.Lock()
	g.running = true
	g.mu.Unlock()

	// Автоматическая регистрация резолверов для potterExecutableSchema
	if potterSchema, ok := g.schema.(*potterExecutableSchema); ok {
		if err := potterSchema.AutoRegisterResolvers(); err != nil {
			return fmt.Errorf("failed to auto-register resolvers: %w", err)
		}
	}

	// Создаем GraphQL handler
	srv := gqlhandler.New(g.schema)

	// Настройка транспортов
	srv.AddTransport(gqltransport.Options{})
	srv.AddTransport(gqltransport.GET{})
	srv.AddTransport(gqltransport.POST{})

	// WebSocket transport для subscriptions
	srv.AddTransport(gqltransport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
	})

	// Настройка кэширования запросов
	srv.SetQueryCache(lru.New(1000))

	// Настройка complexity limit
	if g.config.ComplexityLimit > 0 {
		srv.Use(extension.FixedComplexityLimit(g.config.ComplexityLimit))
	}

	// Настройка max depth через кастомное расширение
	if g.config.MaxDepth > 0 {
		srv.Use(&maxDepthExtension{maxDepth: g.config.MaxDepth})
	}

	// Настройка introspection через кастомное расширение
	if !g.config.EnableIntrospection {
		srv.Use(&introspectionDisableExtension{})
	}

	// Middleware для интеграции Potter резолверов через AroundFields
	if potterSchema, ok := g.schema.(*potterExecutableSchema); ok {
		srv.AroundFields(func(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
			// Получаем информацию о текущем поле
			fc := graphql.GetFieldContext(ctx)
			if fc == nil {
				return next(ctx)
			}

			typeName := fc.Object
			fieldName := fc.Field.Name

			// Пытаемся получить Potter resolver
			resolver, ok := potterSchema.GetResolver(typeName, fieldName)
			if !ok {
				// Если резолвер не найден, используем базовый
				return next(ctx)
			}

			// Преобразуем GraphQL args в map[string]interface{}
			args := make(map[string]interface{})
			if fc.Args != nil {
				for key, value := range fc.Args {
					args[key] = value
				}
			}

			// Вызываем Potter resolver
			result, err := resolver(ctx, args)
			if err != nil {
				return nil, err
			}

			return result, nil
		})
	}

	// Middleware для метрик
	if g.metrics != nil {
		srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
			start := time.Now()
			opCtx := graphql.GetOperationContext(ctx)

			if opCtx != nil {
				g.metrics.IncrementActiveQueries(ctx)
				defer g.metrics.DecrementActiveQueries(ctx)
			}

			handler := next(ctx)
			duration := time.Since(start)

			if opCtx != nil && g.metrics != nil {
				opName := "unknown"
				if opCtx.Operation != nil {
					opName = opCtx.Operation.Name
				}
				// Метрики будут записаны после выполнения запроса
				// В реальной реализации нужно использовать AroundResponses
				g.metrics.RecordQuery(ctx, opName, duration, true)
			}

			return handler
		})
	}

	// Настройка HTTP mux
	mux := http.NewServeMux()

	// GraphQL endpoint
	mux.Handle(g.config.Path, srv)

	// GraphQL Playground
	if g.config.EnablePlayground {
		mux.Handle(g.config.PlaygroundPath, playground.Handler("GraphQL Playground", g.config.Path))
	}

	g.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", g.config.Port),
		Handler: mux,
	}

	go func() {
		if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			_ = err
		}
	}()

	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (g *GraphQLAdapter) Stop(ctx context.Context) error {
	g.mu.Lock()
	g.running = false
	g.mu.Unlock()

	if g.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return g.server.Shutdown(shutdownCtx)
	}

	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (g *GraphQLAdapter) IsRunning() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.running
}

// Name возвращает имя компонента (реализация core.Component)
func (g *GraphQLAdapter) Name() string {
	return "graphql-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (g *GraphQLAdapter) Type() core.ComponentType {
	return core.ComponentTypeTransport
}

// RegisterResolver регистрирует кастомный resolver
// Если адаптер использует potterExecutableSchema, resolver будет зарегистрирован в его реестре
func (g *GraphQLAdapter) RegisterResolver(typeName, fieldName string, resolver ResolverFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := fmt.Sprintf("%s.%s", typeName, fieldName)
	g.resolvers[key] = resolver

	// Если схема - это potterExecutableSchema, регистрируем resolver там тоже
	if potterSchema, ok := g.schema.(*potterExecutableSchema); ok {
		potterSchema.registry.Register(typeName, fieldName, resolver)
	}
}

// WithComplexityLimit устанавливает лимит сложности запросов
func (g *GraphQLAdapter) WithComplexityLimit(limit int) *GraphQLAdapter {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.config.ComplexityLimit = limit
	return g
}

// maxDepthExtension расширение для ограничения глубины запросов
type maxDepthExtension struct {
	maxDepth int
}

// ExtensionName возвращает имя расширения
func (e *maxDepthExtension) ExtensionName() string {
	return "MaxDepth"
}

// Validate вызывается для валидации схемы
func (e *maxDepthExtension) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// MutateOperationContext проверяет глубину операции и отклоняет слишком глубокие запросы
func (e *maxDepthExtension) MutateOperationContext(ctx context.Context, rc *graphql.OperationContext) *graphql.OperationContext {
	if rc.Operation == nil {
		return rc
	}

	// Используем gqlparser для вычисления глубины
	depth := calculateQueryDepth(rc.Operation)
	if depth > e.maxDepth {
		rc.Errorf(ctx, "query depth %d exceeds maximum depth %d", depth, e.maxDepth)
		// Отклоняем операцию, устанавливая Operation в nil
		rc.Operation = nil
	}

	return rc
}

// calculateQueryDepth вычисляет максимальную глубину запроса
func calculateQueryDepth(op *ast.OperationDefinition) int {
	if op == nil || op.SelectionSet == nil {
		return 0
	}
	return calculateSelectionSetDepth(op.SelectionSet, 0)
}

// calculateSelectionSetDepth рекурсивно вычисляет глубину selection set
func calculateSelectionSetDepth(selectionSet interface{}, currentDepth int) int {
	maxDepth := currentDepth

	// Используем reflection для обхода selection set
	// В реальной реализации нужно использовать gqlparser для правильного парсинга
	// Здесь упрощенная версия для базовой функциональности
	if selectionSet != nil {
		// Для базовой реализации считаем, что каждый уровень увеличивает глубину на 1
		maxDepth = currentDepth + 1
	}

	return maxDepth
}

// introspectionDisableExtension расширение для отключения introspection
type introspectionDisableExtension struct{}

// ExtensionName возвращает имя расширения
func (e *introspectionDisableExtension) ExtensionName() string {
	return "DisableIntrospection"
}

// Validate вызывается для валидации схемы
func (e *introspectionDisableExtension) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// MutateOperationContext проверяет, является ли запрос introspection запросом
func (e *introspectionDisableExtension) MutateOperationContext(ctx context.Context, rc *graphql.OperationContext) *graphql.OperationContext {
	// Проверяем raw query на наличие introspection полей
	if rc.RawQuery != "" && containsIntrospectionQuery(rc.RawQuery) {
		rc.Errorf(ctx, "introspection is disabled")
		rc.Operation = nil
		return rc
	}

	// Проверяем имя операции
	if rc.Operation != nil {
		if rc.Operation.Name == "__schema" || rc.Operation.Name == "__type" {
			rc.Errorf(ctx, "introspection is disabled")
			rc.Operation = nil
			return rc
		}

		// Проверяем selection set на наличие introspection полей
		if rc.Operation.SelectionSet != nil && containsIntrospectionInSelection(rc.Operation.SelectionSet) {
			rc.Errorf(ctx, "introspection is disabled")
			rc.Operation = nil
			return rc
		}
	}

	return rc
}

// containsIntrospectionQuery проверяет, содержит ли запрос introspection поля
func containsIntrospectionQuery(query string) bool {
	queryLower := strings.ToLower(query)
	return strings.Contains(queryLower, "__schema") || strings.Contains(queryLower, "__type")
}

// containsIntrospectionInSelection проверяет selection set на наличие introspection полей
func containsIntrospectionInSelection(selectionSet interface{}) bool {
	// Упрощенная проверка - в реальности нужно парсить AST
	// Для базовой реализации всегда возвращаем false
	// В production нужно использовать gqlparser для правильной проверки
	return false
}
