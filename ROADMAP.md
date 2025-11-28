# Roadmap

План развития Potter Framework.

## Версия 1.3 (Планируется)

### Event Sourcing
- [ ] Полная поддержка Event Sourcing паттерна
- [ ] Event Store адаптеры (PostgreSQL, MongoDB, EventStore)
- [ ] Snapshot механизм для оптимизации восстановления состояния
- [ ] Event replay и projection rebuilding

### Saga Pattern через FSM
- [ ] Расширенная поддержка Saga Pattern через FSM модуль
- [ ] Компенсирующие транзакции (compensating transactions)
- [ ] Saga orchestrator и coordinator
- [ ] Интеграция с 2PC для распределенных транзакций

### GraphQL Transport
- [ ] GraphQL транспорт для запросов
- [ ] Автоматическая генерация GraphQL схем из proto
- [ ] GraphQL subscriptions для real-time обновлений
- [ ] Интеграция с существующими GraphQL серверами

## Версия 1.4 (Планируется)

### Автоматическая генерация OpenAPI спецификаций
- [ ] Генерация OpenAPI 3.0 спецификаций из proto файлов
- [ ] Интеграция с Swagger UI
- [ ] Автоматическая документация REST endpoints
- [ ] Валидация запросов по OpenAPI схеме

### Расширенная поддержка distributed tracing
- [ ] Интеграция с Jaeger, Zipkin, Datadog
- [ ] Автоматическая инструментация всех компонентов
- [ ] Correlation ID propagation через все слои
- [ ] Distributed tracing для 2PC транзакций
- [ ] Performance profiling и bottleneck detection

### Улучшения производительности
- [ ] Connection pooling оптимизация
- [ ] Batch processing для событий
- [ ] Кэширование на уровне фреймворка
- [ ] Оптимизация сериализации (Protobuf, MessagePack)

## Версия 2.0 (Планируется)

### Breaking Changes
- [ ] Удаление deprecated типов (`core.Error`)
- [ ] Рефакторинг API для улучшения консистентности
- [ ] Упрощение конфигурации и инициализации
- [ ] Миграция на новые версии зависимостей

### Новые возможности
- [ ] Поддержка WebAssembly для edge computing
- [ ] Multi-tenancy на уровне фреймворка
- [ ] Автоматическое масштабирование через Kubernetes
- [ ] Serverless deployment support (AWS Lambda, Google Cloud Functions)

### Улучшения разработки
- [ ] Расширенная документация с примерами
- [ ] Интерактивные туториалы
- [ ] Видео-курсы и вебинары
- [ ] Best practices guide для production deployments

## Завершенные задачи

### Версия 1.2.0
- [x] Интеграция с популярными message brokers (Kafka, Redis)
- [x] Поддержка WebSocket транспорта
- [x] Интеграция с популярными базами данных (PostgreSQL, MongoDB)
- [x] Code Generator с incremental updates
- [x] Invoke Module с type-safe API
- [x] Testing utilities для приложений

### Версия 1.1.0
- [x] Invoke Module для type-safe работы с CQRS
- [x] Event Sourcing базовая поддержка
- [x] Метрики OpenTelemetry

### Версия 1.0.0
- [x] Базовая структура фреймворка
- [x] CQRS Framework
- [x] Transport Layer
- [x] Events System
- [x] Container (DI)
- [x] Metrics
- [x] FSM

## Приоритеты

1. **Высокий приоритет**: Event Sourcing, Saga Pattern, GraphQL Transport
2. **Средний приоритет**: OpenAPI генерация, расширенный distributed tracing
3. **Низкий приоритет**: WebAssembly, multi-tenancy, serverless support

## Обратная связь

Если у вас есть предложения по улучшению фреймворка или новые фичи, которые вы хотели бы видеть, пожалуйста, создайте issue в репозитории проекта.

## Примечания

- Roadmap может изменяться в зависимости от обратной связи сообщества
- Приоритеты могут быть пересмотрены на основе реальных потребностей пользователей
- Breaking changes будут объявлены заранее в CHANGELOG.md

