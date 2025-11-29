-- Создание таблицы для Event Store
CREATE SCHEMA IF NOT EXISTS public;

CREATE TABLE IF NOT EXISTS public.event_store (
    id VARCHAR(255) PRIMARY KEY,
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    event_data JSONB NOT NULL,
    metadata JSONB,
    version BIGINT NOT NULL,
    position BIGSERIAL,
    occurred_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(aggregate_id, version)
);

CREATE INDEX IF NOT EXISTS idx_event_store_aggregate_id ON public.event_store(aggregate_id);
CREATE INDEX IF NOT EXISTS idx_event_store_event_type ON public.event_store(event_type);
CREATE INDEX IF NOT EXISTS idx_event_store_occurred_at ON public.event_store(occurred_at);
CREATE INDEX IF NOT EXISTS idx_event_store_position ON public.event_store(position);

-- Создание таблицы для снапшотов
CREATE TABLE IF NOT EXISTS public.snapshots (
    aggregate_id VARCHAR(255) PRIMARY KEY,
    aggregate_type VARCHAR(255) NOT NULL,
    version BIGINT NOT NULL,
    state BYTEA NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_snapshots_aggregate_id ON public.snapshots(aggregate_id);

