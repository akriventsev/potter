# Observability Module

Модуль для расширенной observability: distributed tracing, debugging utilities, health checks.

## Компоненты

1. **Distributed Tracing** (`tracing.go`):
   - OpenTelemetry интеграция
   - Поддержка Jaeger, Zipkin, OTLP exporters
   - Correlation ID propagation
   - Middleware для HTTP/gRPC
   - Интеграция с CQRS (команды, запросы, события)

2. **Debugging Utilities** (`debugging.go`):
   - Pprof endpoints для profiling
   - Health check и readiness probes
   - Request/response logging
   - Performance profiling
   - Bottleneck detection

## Quick Start

### Tracing

```go
import "github.com/akriventsev/potter/framework/observability"

config := observability.TracingConfig{
  Enabled: true,
  ServiceName: "my-service",
  Exporter: "jaeger",
  ExporterEndpoint: "http://localhost:14268/api/traces",
  SamplingRate: 1.0,
}
tracingMgr, _ := observability.NewTracingManager(config)
tracingMgr.Start(ctx)
defer tracingMgr.Stop(ctx)

// Middleware
router.Use(observability.HTTPTracingMiddleware("my-service"))
router.Use(observability.CorrelationIDMiddleware())
```

### Health Checks

```go
debugMgr := observability.NewDebugManager(observability.DebugConfig{
  EnableHealthCheck: true,
  EnablePprof: true,
})
debugMgr.RegisterHealthCheck(observability.NewDatabaseHealthCheck(db))
debugMgr.Start(ctx)
```

## Distributed Tracing

### Поддерживаемые exporters

- Jaeger - для локальной разработки и production
- Zipkin - альтернатива Jaeger
- OTLP - OpenTelemetry Protocol (для cloud providers)
- Stdout - для debugging

### Trace context propagation

- W3C Trace Context через HTTP headers
- gRPC metadata для gRPC calls
- Автоматическая propagation через middleware

### Интеграция с CQRS

```go
err := observability.TraceCommand(ctx, "CreateProduct", func(ctx context.Context) error {
  return commandBus.Send(ctx, cmd)
})
```

### Custom spans

```go
tracer := tracingMgr.Tracer()
ctx, span := tracer.Start(ctx, "custom-operation")
defer span.End()

span.SetAttributes(
  attribute.String("user.id", userID),
  attribute.Int("items.count", len(items)),
)
```

## Correlation ID

### Автоматическая генерация

```go
router.Use(observability.CorrelationIDMiddleware())
```

### Ручное использование

```go
correlationID := observability.ExtractCorrelationID(ctx)
ctx = observability.InjectCorrelationID(ctx, correlationID)
```

### Propagation в downstream services

```go
headers := http.Header{}
observability.PropagateCorrelationID(ctx, headers)
req.Header = headers
```

## Health Checks

### Built-in checks

- DatabaseHealthCheck - проверка БД
- MessageBusHealthCheck - проверка message bus
- DiskSpaceHealthCheck - проверка диска
- MemoryHealthCheck - проверка памяти

### Custom health check

```go
type MyHealthCheck struct{}

func (h *MyHealthCheck) Name() string { return "my-check" }

func (h *MyHealthCheck) Check(ctx context.Context) error {
  // Проверка
  return nil
}

debugMgr.RegisterHealthCheck(&MyHealthCheck{})
```

### Kubernetes integration

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

## Debugging

### Pprof profiling

```bash
# Heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# CPU profile (30s)
go tool pprof http://localhost:6060/debug/pprof/profile

# Goroutine profile
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Request logging

```go
if config.Debug {
  router.Use(observability.RequestDumpMiddleware())
}
```

### Performance profiling

```go
err := observability.ProfileCommand(ctx, "SlowCommand", func() error {
  // Медленная операция
  return nil
})
```

## Best Practices

### Production

1. Используйте sampling rate < 1.0 для снижения overhead
2. Отключайте RequestDumpMiddleware
3. Ограничивайте доступ к pprof endpoints (firewall/auth)
4. Используйте OTLP exporter для cloud providers

### Development

1. Используйте sampling rate = 1.0 для полного tracing
2. Включайте RequestDumpMiddleware для debugging
3. Используйте Jaeger UI для визуализации traces

### Staging

1. Используйте sampling rate 0.1-0.5
2. Включайте все health checks
3. Тестируйте Kubernetes probes

## Интеграция с существующими компонентами

### Metrics

Модуль интегрируется с `framework/metrics` для unified observability:

```go
metrics, _ := metrics.NewMetrics()
tracingMgr, _ := observability.NewTracingManager(config)
// Metrics и traces автоматически коррелируются через trace ID
```

### CQRS

Автоматическая инструментация команд, запросов, событий через middleware.

### Event Sourcing

Tracing для event replay и projection rebuilding.

### Saga Pattern

Tracing для saga orchestration с визуализацией шагов.

## Troubleshooting

### Traces не отображаются в Jaeger

- Проверьте ExporterEndpoint
- Проверьте SamplingRate (должен быть > 0)
- Проверьте логи для ошибок экспорта

### Health check возвращает unhealthy

- Проверьте логи для деталей
- Проверьте подключение к зависимостям
- Используйте /health endpoint для диагностики

### Pprof endpoints недоступны

- Проверьте EnablePprof = true
- Проверьте PprofPort
- Проверьте firewall rules

## Примеры

См. `examples/observability-demo/` для полных примеров использования.

