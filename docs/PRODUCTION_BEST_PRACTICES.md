# Production Deployment Best Practices

Руководство по развертыванию Potter Framework приложений в production.

## Содержание

1. Configuration Management
2. Database Migrations
3. Observability
4. Security
5. Performance Optimization
6. High Availability
7. Disaster Recovery
8. Monitoring and Alerting
9. CI/CD Pipeline
10. Troubleshooting

## 1. Configuration Management

### Environment Variables

```bash
# Database
DATABASE_URL=postgres://user:pass@host:5432/db
DATABASE_MAX_CONNECTIONS=100
DATABASE_MAX_IDLE_CONNECTIONS=10

# Message Bus
NATS_URL=nats://nats-cluster:4222
KAFKA_BROKERS=kafka1:9092,kafka2:9092

# Observability
JAEGER_ENDPOINT=http://jaeger:14268/api/traces
TRACING_SAMPLING_RATE=0.1
METRICS_PORT=9090

# Security
JWT_SECRET=<strong-secret>
CORS_ALLOWED_ORIGINS=https://app.example.com
```

### Config validation

- Валидация всех обязательных переменных при старте
- Fail fast если конфигурация некорректна
- Логирование конфигурации (без секретов)

### Secrets management

- Используйте Kubernetes Secrets или HashiCorp Vault
- Никогда не коммитьте секреты в git
- Ротация секретов каждые 90 дней

## 2. Database Migrations

### Pre-deployment

```bash
# Backup БД перед миграцией
pg_dump -h $DB_HOST -U $DB_USER $DB_NAME > backup_$(date +%Y%m%d_%H%M%S).sql

# Dry-run миграций
goose -dir migrations postgres "$DATABASE_URL" status

# Применение миграций
goose -dir migrations postgres "$DATABASE_URL" up
```

### Best practices

- Всегда тестируйте миграции на staging
- Используйте транзакции для миграций
- Пишите rollback миграции (down)
- Избегайте breaking changes (используйте expand-contract pattern)
- Мониторьте время выполнения миграций

### Zero-downtime migrations

1. Добавьте новую колонку (nullable)
2. Deploy новой версии приложения (пишет в обе колонки)
3. Backfill данных
4. Deploy финальной версии (читает из новой колонки)
5. Удалите старую колонку

## 3. Observability

### Distributed Tracing

```go
config := observability.TracingConfig{
  Enabled: true,
  ServiceName: "product-service",
  Exporter: "otlp",
  ExporterEndpoint: os.Getenv("OTLP_ENDPOINT"),
  SamplingRate: 0.1, // 10% sampling в production
}
```

### Metrics

- Экспортируйте метрики в Prometheus
- Настройте dashboards в Grafana
- Мониторьте: latency (p50, p95, p99), error rate, throughput

### Logging

- Структурированное логирование (JSON)
- Централизованный сбор логов (ELK, Loki)
- Log levels: ERROR для критических ошибок, WARN для предупреждений, INFO для важных событий
- Correlation ID в каждом логе

### Health Checks

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 2
```

## 4. Security

### Authentication & Authorization

- JWT tokens с коротким TTL (15 минут)
- Refresh tokens для продления сессии
- RBAC для authorization
- Rate limiting на authentication endpoints

### API Security

```go
// CORS
router.Use(cors.New(cors.Config{
  AllowOrigins: []string{os.Getenv("CORS_ALLOWED_ORIGINS")},
  AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
  AllowHeaders: []string{"Authorization", "Content-Type"},
  MaxAge: 12 * time.Hour,
}))

// Rate limiting
router.Use(ratelimit.New(ratelimit.Config{
  Max: 100,
  Duration: time.Minute,
}))

// Request size limit
router.Use(gin.MaxRequestBodySize(10 * 1024 * 1024)) // 10MB
```

### Input Validation

- Валидация всех входных данных
- Sanitization для предотвращения XSS/SQL injection
- OpenAPI validation middleware

### TLS/SSL

- Используйте TLS 1.3
- Автоматическое обновление сертификатов (Let's Encrypt)
- HSTS headers

## 5. Performance Optimization

### Database

```go
// Connection pooling
db.SetMaxOpenConns(100)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(time.Hour)

// Prepared statements
stmt, _ := db.Prepare("SELECT * FROM products WHERE id = $1")
defer stmt.Close()
```

### Caching

- Redis для session storage и caching
- Cache-Control headers для HTTP responses
- Query result caching для read-heavy endpoints

### Message Bus

- Batch processing для событий
- Consumer groups для параллельной обработки
- Dead letter queue для failed messages

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

## 6. High Availability

### Horizontal Scaling

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: product-service
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: product-service
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Load Balancing

- Kubernetes Service для internal load balancing
- Ingress для external traffic
- Health checks для automatic failover

### Database HA

- PostgreSQL replication (primary-replica)
- Connection pooling через PgBouncer
- Automatic failover через Patroni

### Message Bus HA

- NATS cluster (3+ nodes)
- Kafka cluster (3+ brokers)
- Redis Sentinel для failover

## 7. Disaster Recovery

### Backups

```bash
# Automated daily backups
0 2 * * * pg_dump -h $DB_HOST -U $DB_USER $DB_NAME | gzip > /backups/db_$(date +\%Y\%m\%d).sql.gz

# Retention: 7 daily, 4 weekly, 12 monthly
```

### Recovery Testing

- Тестируйте восстановление из backup ежемесячно
- Документируйте RTO (Recovery Time Objective) и RPO (Recovery Point Objective)
- Runbook для disaster recovery процедур

### Multi-region Deployment

- Active-passive для disaster recovery
- Active-active для global availability
- Database replication между регионами

## 8. Monitoring and Alerting

### Key Metrics

- **Latency:** p50, p95, p99 response times
- **Error Rate:** % failed requests
- **Throughput:** requests per second
- **Saturation:** CPU, memory, disk usage

### Alerts

```yaml
# Prometheus AlertManager
groups:
- name: product-service
  rules:
  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
    for: 5m
    annotations:
      summary: "High error rate detected"
  
  - alert: HighLatency
    expr: histogram_quantile(0.95, http_request_duration_seconds) > 1
    for: 5m
    annotations:
      summary: "High latency detected"
```

### On-call Rotation

- PagerDuty/Opsgenie для alerting
- Escalation policy
- Runbooks для common incidents

## 9. CI/CD Pipeline

### GitHub Actions Example

```yaml
name: Deploy
on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    - name: Run tests
      run: make test
    - name: Check codegen sync
      run: potter-gen check --proto api/service.proto --output .
  
  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
    - name: Build Docker image
      run: docker build -t myapp:${{ github.sha }} .
    - name: Push to registry
      run: docker push myapp:${{ github.sha }}
  
  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
    - name: Deploy to Kubernetes
      run: kubectl set image deployment/myapp myapp=myapp:${{ github.sha }}
```

### Deployment Strategy

- Blue-green deployment для zero-downtime
- Canary deployment для gradual rollout
- Automatic rollback при ошибках

## 10. Troubleshooting

### Common Issues

### High Latency

1. Проверьте database slow queries
2. Проверьте CPU/memory usage
3. Проверьте network latency
4. Используйте pprof для profiling

### Memory Leaks

```bash
# Heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine leaks
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Database Connection Issues

- Проверьте connection pool settings
- Проверьте max_connections в PostgreSQL
- Используйте PgBouncer для connection pooling

### Message Bus Issues

- Проверьте consumer lag
- Проверьте dead letter queue
- Увеличьте consumer replicas

## Checklist

Перед production deployment:

- [ ] Все тесты проходят (unit, integration, e2e)
- [ ] Codegen синхронизирован с proto (potter-gen check)
- [ ] Database миграции протестированы на staging
- [ ] Secrets настроены через Kubernetes Secrets/Vault
- [ ] Health checks настроены
- [ ] Metrics экспортируются в Prometheus
- [ ] Distributed tracing настроен
- [ ] Alerts настроены в AlertManager
- [ ] Backup strategy реализована
- [ ] Disaster recovery plan документирован
- [ ] Load testing выполнен
- [ ] Security scan выполнен (OWASP, dependency check)
- [ ] Documentation обновлена
- [ ] Runbooks созданы для on-call

## Дополнительные ресурсы

- [Potter Framework Documentation](../README.md)
- [Observability Module](../framework/observability/README.md)
- [Saga Pattern Best Practices](../framework/saga/README.md)
- [Event Sourcing Guide](../framework/eventsourcing/README.md)

Документ основан на production опыте и best practices индустрии.

