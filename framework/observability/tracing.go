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

package observability

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	correlationIDKey = "X-Correlation-ID"
	traceIDKey       = "traceparent"
)

// TracingConfig конфигурация для distributed tracing
type TracingConfig struct {
	Enabled          bool
	ServiceName      string
	ServiceVersion   string
	Exporter         string // "jaeger", "zipkin", "otlp", "stdout"
	ExporterEndpoint string
	SamplingRate     float64 // 0.0 - 1.0
	Environment      string  // "development", "staging", "production"
}

// TracingManager менеджер для distributed tracing
type TracingManager struct {
	config   TracingConfig
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	exporter sdktrace.SpanExporter
	running  bool
	mu       sync.RWMutex
}

// NewTracingManager создает новый TracingManager
func NewTracingManager(config TracingConfig) (*TracingManager, error) {
	if !config.Enabled {
		return &TracingManager{config: config}, nil
	}

	// Создание resource attributes
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Создание exporter
	exporter, err := createExporter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Настройка sampler
	sampler := sdktrace.TraceIDRatioBased(config.SamplingRate)
	if config.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	}

	// Создание trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Регистрация global trace provider
	otel.SetTracerProvider(tp)

	// Настройка propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := tp.Tracer(config.ServiceName)

	return &TracingManager{
		config:   config,
		tracer:   tracer,
		provider: tp,
		exporter: exporter,
		running:  false,
	}, nil
}

// createExporter создает exporter на основе конфигурации
func createExporter(config TracingConfig) (sdktrace.SpanExporter, error) {
	switch config.Exporter {
	case "jaeger":
		return jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.ExporterEndpoint)))
	case "zipkin":
		return zipkin.New(config.ExporterEndpoint)
	case "otlp":
		client := otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(config.ExporterEndpoint),
			otlptracehttp.WithInsecure(),
		)
		return otlptrace.New(context.Background(), client)
	case "stdout":
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	default:
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	}
}

// Start запускает tracing (lifecycle)
func (tm *TracingManager) Start(ctx context.Context) error {
	tm.mu.Lock()
	tm.running = true
	tm.mu.Unlock()
	return nil
}

// Stop останавливает tracing с graceful shutdown
func (tm *TracingManager) Stop(ctx context.Context) error {
	tm.mu.Lock()
	tm.running = false
	tm.mu.Unlock()

	if tm.provider != nil {
		return tm.provider.Shutdown(ctx)
	}
	return nil
}

// IsRunning проверяет статус
func (tm *TracingManager) IsRunning() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.running
}

// Tracer возвращает tracer для создания spans
func (tm *TracingManager) Tracer() trace.Tracer {
	return tm.tracer
}

// HTTPTracingMiddleware Gin middleware для автоматической инструментации HTTP requests
func HTTPTracingMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Извлечение trace context из headers
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(c.Request.Header))

		// Создание span
		tracer := otel.Tracer(serviceName)
		ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path))
		defer span.End()

		// Добавление span attributes
		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.url", c.Request.URL.String()),
			attribute.String("http.route", c.FullPath()),
		)

		// Установка context в request
		c.Request = c.Request.WithContext(ctx)

		// Обработка запроса
		c.Next()

		// Запись статуса ответа
		span.SetAttributes(attribute.Int("http.status_code", c.Writer.Status()))

		// Запись ошибок
		if len(c.Errors) > 0 {
			span.RecordError(c.Errors.Last())
		}

		// Propagation trace context в response headers
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(c.Writer.Header()))
	}
}

// GRPCTracingInterceptor gRPC interceptor для автоматической инструментации gRPC calls
func GRPCTracingInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Извлечение trace context из metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			ctx = otel.GetTextMapPropagator().Extract(ctx, metadataTextMapCarrier(md))
		}

		// Создание span
		tracer := otel.Tracer("grpc")
		ctx, span := tracer.Start(ctx, info.FullMethod)
		defer span.End()

		// Добавление span attributes
		span.SetAttributes(
			attribute.String("rpc.service", info.FullMethod),
			attribute.String("rpc.method", info.FullMethod),
		)

		// Выполнение handler
		resp, err := handler(ctx, req)

		// Запись статуса и ошибок
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("rpc.grpc.status_code", err.Error()))
		} else {
			span.SetAttributes(attribute.String("rpc.grpc.status_code", "OK"))
		}

		return resp, err
	}
}

// metadataTextMapCarrier адаптер для propagation через gRPC metadata
type metadataTextMapCarrier metadata.MD

func (m metadataTextMapCarrier) Get(key string) string {
	values := metadata.MD(m).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (m metadataTextMapCarrier) Set(key, value string) {
	metadata.MD(m).Set(key, value)
}

func (m metadataTextMapCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ExtractCorrelationID извлекает correlation ID из context
func ExtractCorrelationID(ctx context.Context) string {
	// Пытаемся извлечь из baggage
	b := baggage.FromContext(ctx)
	if b.Len() > 0 {
		if member := b.Member(correlationIDKey); member.Key() == correlationIDKey {
			return member.Value()
		}
	}

	// Используем trace ID как fallback
	span := trace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().TraceID().IsValid() {
		return span.SpanContext().TraceID().String()
	}

	return ""
}

// InjectCorrelationID добавляет correlation ID в context
func InjectCorrelationID(ctx context.Context, correlationID string) context.Context {
	b := baggage.FromContext(ctx)
	member, err := baggage.NewMember(correlationIDKey, correlationID)
	if err != nil {
		// Если не удалось создать member, возвращаем исходный context
		return ctx
	}
	b, _ = b.SetMember(member)
	return baggage.ContextWithBaggage(ctx, b)
}

// PropagateCorrelationID propagates correlation ID через HTTP headers
func PropagateCorrelationID(ctx context.Context, headers http.Header) {
	correlationID := ExtractCorrelationID(ctx)
	if correlationID != "" {
		headers.Set(correlationIDKey, correlationID)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}

// CorrelationIDMiddleware Gin middleware для автоматической генерации/propagation correlation ID
func CorrelationIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Извлечение correlation ID из headers или генерация нового
		correlationID := c.GetHeader(correlationIDKey)
		if correlationID == "" {
			// Генерируем новый correlation ID
			span := trace.SpanFromContext(ctx)
			if span != nil {
				correlationID = span.SpanContext().TraceID().String()
			} else {
				correlationID = generateCorrelationID()
			}
		}

		// Добавление в context
		ctx = InjectCorrelationID(ctx, correlationID)

		// Установка в request
		c.Request = c.Request.WithContext(ctx)

		// Добавление в response headers
		c.Writer.Header().Set(correlationIDKey, correlationID)

		c.Next()
	}
}

// generateCorrelationID генерирует новый correlation ID
func generateCorrelationID() string {
	// Генерация UUID для уникального correlation ID
	return uuid.New().String()
}

// TraceCommand обертка для команд с автоматической инструментацией
func TraceCommand(ctx context.Context, commandName string, fn func(context.Context) error) error {
	tracer := otel.Tracer("potter.command")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("command.%s", commandName))
	defer span.End()

	span.SetAttributes(
		attribute.String("command.name", commandName),
	)

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("command.success", false))
	} else {
		span.SetAttributes(attribute.Bool("command.success", true))
	}

	return err
}

// TraceQuery обертка для запросов с автоматической инструментацией
func TraceQuery(ctx context.Context, queryName string, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	tracer := otel.Tracer("potter.query")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("query.%s", queryName))
	defer span.End()

	span.SetAttributes(
		attribute.String("query.name", queryName),
	)

	result, err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("query.success", false))
	} else {
		span.SetAttributes(attribute.Bool("query.success", true))
	}

	return result, err
}

// TraceEvent обертка для событий с автоматической инструментацией
func TraceEvent(ctx context.Context, eventType string, fn func(context.Context) error) error {
	tracer := otel.Tracer("potter.event")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("event.%s", eventType))
	defer span.End()

	span.SetAttributes(
		attribute.String("event.type", eventType),
	)

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("event.success", false))
	} else {
		span.SetAttributes(attribute.Bool("event.success", true))
	}

	return err
}
