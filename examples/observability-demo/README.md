# Observability Demo

Comprehensive пример использования observability модуля Potter Framework.

## Возможности

- ✅ Distributed tracing с Jaeger
- ✅ Correlation ID propagation через все слои
- ✅ Health checks и readiness probes
- ✅ Pprof profiling endpoints
- ✅ Metrics с Prometheus
- ✅ Grafana dashboards
- ✅ Request/response logging
- ✅ Performance profiling
- ✅ Bottleneck detection

## Архитектура

```
Application
    ↓
├── Distributed Tracing → Jaeger
├── Metrics → Prometheus → Grafana
├── Logs → stdout (можно интегрировать с ELK)
├── Health Checks → Kubernetes probes
└── Pprof → Go profiling tools
```

## Quick Start

### 1. Запуск observability stack

```bash
make docker-up
```

Запускает:

- PostgreSQL (порт 5432)
- NATS (порт 4222)
- Jaeger (UI: 16686, collector: 14268)
- Prometheus (порт 9090)
- Grafana (порт 3000, admin/admin)

### 2. Запуск приложения

```bash
make run
```

Приложение доступно на:

- REST API: http://localhost:8080
- Health check: http://localhost:8080/health
- Readiness check: http://localhost:8080/ready
- Pprof: http://localhost:6060/debug/pprof/
- Metrics: http://localhost:9090/metrics

### 3. Открытие UI

```bash
make open-jaeger    # Jaeger UI
make open-grafana   # Grafana dashboards
make open-prometheus # Prometheus UI
```

## Distributed Tracing

### Просмотр traces в Jaeger

1. Откройте http://localhost:16686
2. Выберите сервис "observability-demo"
3. Нажмите "Find Traces"

### Генерация traces

```bash
# Создание продукта (генерирует trace)
curl -X POST http://localhost:8080/api/v1/products \
  -H "Content-Type: application/json" \
  -d '{"name": "Laptop", "price": 1299.99}'

# Получение продукта (генерирует trace)
curl http://localhost:8080/api/v1/products/1
```

### Trace structure

Каждый HTTP request создает trace с spans:

```
HTTP Request
├── Command: CreateProduct
│   ├── Repository: Save
│   └── EventBus: Publish
└── Response
```

### Correlation ID

Каждый request автоматически получает correlation ID:

```bash
curl -v http://localhost:8080/api/v1/products/1

# Response headers:
# X-Correlation-ID: 550e8400-e29b-41d4-a716-446655440000
```

Correlation ID propagates через:

- HTTP headers (X-Correlation-ID)
- Trace context
- Logs
- Events

## Metrics

### Prometheus

Откройте http://localhost:9090 для просмотра метрик.

**Доступные метрики:**

- `http_requests_total` - общее количество запросов
- `http_request_duration_seconds` - latency histogram
- `command_executions_total` - количество выполненных команд
- `query_executions_total` - количество выполненных запросов
- `event_publications_total` - количество опубликованных событий
- `active_commands` - активные команды
- `active_queries` - активные запросы

**Примеры PromQL запросов:**

```promql
# Request rate
rate(http_requests_total[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m])

# P95 latency
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Active requests
sum(active_commands) + sum(active_queries)
```

### Grafana Dashboards

Откройте http://localhost:3000 (admin/admin).

**Pre-configured dashboards:**

1. **Application Overview**
   - Request rate, error rate, latency
   - Active requests
   - Resource usage (CPU, memory)

2. **CQRS Metrics**
   - Command/query execution times
   - Event publication rate
   - Handler performance

3. **Infrastructure**
   - Database connections
   - Message bus stats
   - Health check status

## Health Checks

### Liveness Probe

```bash
curl http://localhost:8080/health
```

Response:

```json
{
  "status": "healthy",
  "checks": {
    "database": {"status": "healthy", "duration": "2ms"},
    "nats": {"status": "healthy", "duration": "1ms"},
    "disk": {"status": "healthy", "message": "85% free"},
    "memory": {"status": "healthy", "message": "45% used"}
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Readiness Probe

```bash
curl http://localhost:8080/ready
```

Возвращает 200 OK если сервис готов принимать трафик.

### Kubernetes Integration

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

## Profiling

### Pprof Endpoints

Доступны на http://localhost:6060/debug/pprof/

**Heap profile:**

```bash
go tool pprof http://localhost:6060/debug/pprof/heap
```

**CPU profile (30s):**

```bash
go tool pprof http://localhost:6060/debug/pprof/profile
```

**Goroutine profile:**

```bash
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

**Trace (5s):**

```bash
wget http://localhost:6060/debug/pprof/trace?seconds=5
go tool trace trace
```

### Performance Analysis

**Обнаружение медленных операций:**

```bash
# Генерация нагрузки
make load-test

# Просмотр медленных операций в логах
make logs | grep "slow operation"
```

**Memory leak detection:**

```bash
# Heap profile до нагрузки
go tool pprof -base http://localhost:6060/debug/pprof/heap \
  http://localhost:6060/debug/pprof/heap
```

## Request Logging

### Debug Mode

Включите debug режим для полного логирования:

```bash
DEBUG=true make run
```

Логи включают:

- Полные HTTP requests (headers, body, query params)
- Полные HTTP responses
- Sanitized sensitive data
- Correlation ID в каждом логе

### Log Format

```json
{
  "level": "info",
  "timestamp": "2024-01-15T10:30:00Z",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "message": "Command executed",
  "command": "CreateProduct",
  "duration_ms": 45
}
```

## Load Testing

### Генерация нагрузки

```bash
make load-test
```

Использует Apache Bench для генерации:

- 10000 requests
- 100 concurrent connections
- Различные endpoints

### Мониторинг под нагрузкой

1. Запустите load test: `make load-test`
2. Откройте Grafana: http://localhost:3000
3. Наблюдайте метрики в реальном времени
4. Проверьте traces в Jaeger
5. Используйте pprof для profiling

## Production Best Practices

### Sampling Rate

```go
// Development: 100% sampling
config := observability.TracingConfig{
  SamplingRate: 1.0,
}

// Production: 10% sampling
config := observability.TracingConfig{
  SamplingRate: 0.1,
}
```

### Security

```go
// Ограничение доступа к pprof
if os.Getenv("ENVIRONMENT") == "production" {
  // Отключить pprof или ограничить доступ
  config.EnablePprof = false
}
```

### Resource Limits

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

## Troubleshooting

**Traces не отображаются в Jaeger:**

```bash
# Проверка подключения к Jaeger
curl http://localhost:14268/api/traces

# Проверка логов приложения
make logs | grep "jaeger"

# Проверка sampling rate
echo $TRACING_SAMPLING_RATE
```

**Metrics не собираются:**

```bash
# Проверка Prometheus targets
open http://localhost:9090/targets

# Проверка metrics endpoint
curl http://localhost:9090/metrics
```

**Health check fails:**

```bash
# Детальная информация
curl http://localhost:8080/health | jq

# Проверка зависимостей
make docker-ps
```

## Makefile Commands

```bash
make docker-up         # Запуск observability stack
make docker-down       # Остановка stack
make run               # Запуск приложения
make open-jaeger       # Открыть Jaeger UI
make open-grafana      # Открыть Grafana
make open-prometheus   # Открыть Prometheus
make load-test         # Генерация нагрузки
make logs              # Просмотр логов
make profile-heap      # Heap profiling
make profile-cpu       # CPU profiling
```

## Дополнительные ресурсы

- [Observability Module Documentation](../../framework/observability/README.md)
- [Production Best Practices](../../docs/PRODUCTION_BEST_PRACTICES.md)
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)

Пример следует структуре существующих примеров и демонстрирует все возможности observability модуля.

