-- Миграция для создания таблиц Event Store и Snapshots в PostgreSQL
-- Версия: 001
-- Дата: 2025-01-XX

-- Таблица для хранения событий
CREATE TABLE IF NOT EXISTS event_store (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    event_data JSONB NOT NULL,
    metadata JSONB DEFAULT '{}',
    version BIGINT NOT NULL,
    position BIGSERIAL UNIQUE,
    occurred_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Уникальный индекс для предотвращения конфликтов версий
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_store_aggregate_version 
    ON event_store(aggregate_id, version);

-- Индексы для быстрых запросов
CREATE INDEX IF NOT EXISTS idx_event_store_aggregate_id 
    ON event_store(aggregate_id);

CREATE INDEX IF NOT EXISTS idx_event_store_event_type 
    ON event_store(event_type);

CREATE INDEX IF NOT EXISTS idx_event_store_occurred_at 
    ON event_store(occurred_at);

CREATE INDEX IF NOT EXISTS idx_event_store_position 
    ON event_store(position);

-- Комментарии к таблице и полям
COMMENT ON TABLE event_store IS 'Хранилище событий для Event Sourcing';
COMMENT ON COLUMN event_store.id IS 'Уникальный идентификатор события';
COMMENT ON COLUMN event_store.aggregate_id IS 'Идентификатор агрегата';
COMMENT ON COLUMN event_store.aggregate_type IS 'Тип агрегата';
COMMENT ON COLUMN event_store.event_type IS 'Тип события';
COMMENT ON COLUMN event_store.event_data IS 'Данные события в формате JSONB';
COMMENT ON COLUMN event_store.metadata IS 'Метаданные события';
COMMENT ON COLUMN event_store.version IS 'Версия события в потоке агрегата';
COMMENT ON COLUMN event_store.position IS 'Глобальная позиция события для replay';
COMMENT ON COLUMN event_store.occurred_at IS 'Время возникновения события';
COMMENT ON COLUMN event_store.created_at IS 'Время создания записи';

-- Таблица для снапшотов
CREATE TABLE IF NOT EXISTS snapshots (
    aggregate_id VARCHAR(255) PRIMARY KEY,
    aggregate_type VARCHAR(255) NOT NULL,
    version BIGINT NOT NULL,
    state JSONB NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Индекс для поиска по типу агрегата
CREATE INDEX IF NOT EXISTS idx_snapshots_aggregate_type 
    ON snapshots(aggregate_type);

-- Комментарии к таблице снапшотов
COMMENT ON TABLE snapshots IS 'Хранилище снапшотов для оптимизации загрузки агрегатов';
COMMENT ON COLUMN snapshots.aggregate_id IS 'Идентификатор агрегата (первичный ключ)';
COMMENT ON COLUMN snapshots.aggregate_type IS 'Тип агрегата';
COMMENT ON COLUMN snapshots.version IS 'Версия агрегата на момент создания снапшота';
COMMENT ON COLUMN snapshots.state IS 'Сериализованное состояние агрегата';
COMMENT ON COLUMN snapshots.metadata IS 'Метаданные снапшота';
COMMENT ON COLUMN snapshots.created_at IS 'Время создания снапшота';
COMMENT ON COLUMN snapshots.updated_at IS 'Время последнего обновления снапшота';

