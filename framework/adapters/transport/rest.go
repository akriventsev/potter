// Package transport предоставляет базовые классы и утилиты для REST, gRPC, WebSocket транспортов.
package transport

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/metrics"
	"github.com/akriventsev/potter/framework/transport"
)

// RESTConfig конфигурация для REST адаптера
type RESTConfig struct {
	Port      int
	BasePath  string
	EnableMetrics bool
}

// DefaultRESTConfig возвращает конфигурацию REST по умолчанию
func DefaultRESTConfig() RESTConfig {
	return RESTConfig{
		Port:      8080,
		BasePath:  "/api/v1",
		EnableMetrics: true,
	}
}

// RESTAdapter базовый класс для REST API
type RESTAdapter struct {
	config     RESTConfig
	router     *gin.Engine
	commandBus transport.CommandBus
	queryBus   transport.QueryBus
	metrics    *metrics.Metrics
	running    bool
	server     *http.Server
}

// NewRESTAdapter создает новый REST адаптер
func NewRESTAdapter(config RESTConfig, commandBus transport.CommandBus, queryBus transport.QueryBus) (*RESTAdapter, error) {
	adapter := &RESTAdapter{
		config:     config,
		router:     gin.Default(),
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
func (r *RESTAdapter) Start(ctx context.Context) error {
	r.running = true

	r.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", r.config.Port),
		Handler: r.router,
	}

	go func() {
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Логируем ошибку
			_ = err
		}
	}()

	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (r *RESTAdapter) Stop(ctx context.Context) error {
	r.running = false

	if r.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return r.server.Shutdown(shutdownCtx)
	}

	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (r *RESTAdapter) IsRunning() bool {
	return r.running
}

// Name возвращает имя компонента (реализация core.Component)
func (r *RESTAdapter) Name() string {
	return "rest-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (r *RESTAdapter) Type() core.ComponentType {
	return core.ComponentTypeTransport
}

// RegisterCommand регистрирует command handler
// NOTE: Текущая реализация поддерживает только JSON body binding.
// Query parameters и form data не поддерживаются. Для поддержки дополнительных
// источников данных необходимо расширить адаптер или использовать middleware.
// Также отсутствуют встроенные middleware для CORS, rate limiting и аутентификации -
// эти функции должны быть реализованы на уровне приложения.
func (r *RESTAdapter) RegisterCommand(method, path string, command transport.Command) {
	r.router.Handle(method, path, func(c *gin.Context) {
		ctx := c.Request.Context()
		start := time.Now()

		if r.metrics != nil {
			r.metrics.IncrementActiveCommands(ctx)
			defer r.metrics.DecrementActiveCommands(ctx)
		}

		// Биндинг и валидация (только JSON body)
		if err := c.ShouldBindJSON(command); err != nil {
			if r.metrics != nil {
				r.metrics.RecordCommand(ctx, command.CommandName(), time.Since(start), false)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Отправка команды
		if err := r.commandBus.Send(ctx, command); err != nil {
			if r.metrics != nil {
				r.metrics.RecordCommand(ctx, command.CommandName(), time.Since(start), false)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if r.metrics != nil {
			r.metrics.RecordCommand(ctx, command.CommandName(), time.Since(start), true)
		}
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
}

// RegisterQuery регистрирует query handler
// NOTE: Текущая реализация поддерживает только JSON body binding.
// Query parameters и form data не поддерживаются.
func (r *RESTAdapter) RegisterQuery(method, path string, query transport.Query) {
	r.router.Handle(method, path, func(c *gin.Context) {
		ctx := c.Request.Context()
		start := time.Now()

		if r.metrics != nil {
			r.metrics.IncrementActiveQueries(ctx)
			defer r.metrics.DecrementActiveQueries(ctx)
		}

		// Биндинг и валидация (только JSON body)
		if err := c.ShouldBindJSON(query); err != nil {
			if r.metrics != nil {
				r.metrics.RecordQuery(ctx, query.QueryName(), time.Since(start), false)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Отправка запроса
		result, err := r.queryBus.Ask(ctx, query)
		if err != nil {
			if r.metrics != nil {
				r.metrics.RecordQuery(ctx, query.QueryName(), time.Since(start), false)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if r.metrics != nil {
			r.metrics.RecordQuery(ctx, query.QueryName(), time.Since(start), true)
		}
		c.JSON(http.StatusOK, result)
	})
}

