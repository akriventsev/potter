// Package transport предоставляет базовые классы и утилиты для REST, gRPC, WebSocket транспортов.
package transport

import (
	"context"
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/metrics"
	"github.com/akriventsev/potter/framework/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCConfig конфигурация для gRPC адаптера
type GRPCConfig struct {
	Port                 int
	MaxConcurrentStreams uint32
	MaxReceiveMessageSize int
	EnableMetrics        bool
}

// DefaultGRPCConfig возвращает конфигурацию gRPC по умолчанию
func DefaultGRPCConfig() GRPCConfig {
	return GRPCConfig{
		Port:                 50051,
		MaxConcurrentStreams: 100,
		MaxReceiveMessageSize: 4 * 1024 * 1024, // 4MB
		EnableMetrics:        true,
	}
}

// GRPCAdapter базовый класс для gRPC services
type GRPCAdapter struct {
	config     GRPCConfig
	server     *grpc.Server
	commandBus transport.CommandBus
	queryBus   transport.QueryBus
	metrics    *metrics.Metrics
	running    bool
}

// NewGRPCAdapter создает новый gRPC адаптер
// NOTE: Текущая реализация не предоставляет встроенных server interceptors для
// логирования, recovery и метрик. Также отсутствуют встроенные health checking и
// reflection. Эти функции должны быть добавлены через опции grpc.ServerOption или
// реализованы на уровне приложения.
func NewGRPCAdapter(config GRPCConfig, commandBus transport.CommandBus, queryBus transport.QueryBus) (*GRPCAdapter, error) {
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(config.MaxConcurrentStreams),
		grpc.MaxRecvMsgSize(config.MaxReceiveMessageSize),
	}

	adapter := &GRPCAdapter{
		config:     config,
		server:     grpc.NewServer(opts...),
		commandBus: commandBus,
		queryBus:   queryBus,
		running:    false,
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
func (g *GRPCAdapter) Start(ctx context.Context) error {
	g.running = true
	// Запуск сервера должен быть реализован в конкретных сервисах
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (g *GRPCAdapter) Stop(ctx context.Context) error {
	g.running = false
	if g.server != nil {
		g.server.GracefulStop()
	}
	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (g *GRPCAdapter) IsRunning() bool {
	return g.running
}

// Name возвращает имя компонента (реализация core.Component)
func (g *GRPCAdapter) Name() string {
	return "grpc-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (g *GRPCAdapter) Type() core.ComponentType {
	return core.ComponentTypeTransport
}

// HandleCommand обрабатывает команду через gRPC
func (g *GRPCAdapter) HandleCommand(ctx context.Context, command transport.Command) error {
	start := time.Now()

	if g.metrics != nil {
		g.metrics.IncrementActiveCommands(ctx)
		defer g.metrics.DecrementActiveCommands(ctx)
	}

	err := g.commandBus.Send(ctx, command)
	if err != nil {
		if g.metrics != nil {
			g.metrics.RecordCommand(ctx, command.CommandName(), time.Since(start), false)
		}
		return status.Errorf(codes.Internal, "failed to execute command: %v", err)
	}

	if g.metrics != nil {
		g.metrics.RecordCommand(ctx, command.CommandName(), time.Since(start), true)
	}

	return nil
}

// HandleQuery обрабатывает запрос через gRPC
func (g *GRPCAdapter) HandleQuery(ctx context.Context, query transport.Query) (interface{}, error) {
	start := time.Now()

	if g.metrics != nil {
		g.metrics.IncrementActiveQueries(ctx)
		defer g.metrics.DecrementActiveQueries(ctx)
	}

	result, err := g.queryBus.Ask(ctx, query)
	if err != nil {
		if g.metrics != nil {
			g.metrics.RecordQuery(ctx, query.QueryName(), time.Since(start), false)
		}
		return nil, status.Errorf(codes.Internal, "failed to execute query: %v", err)
	}

	if g.metrics != nil {
		g.metrics.RecordQuery(ctx, query.QueryName(), time.Since(start), true)
	}

	return result, nil
}

