-- Миграции для EventStore и Saga
CREATE SCHEMA IF NOT EXISTS public;

-- EventStore таблицы
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

-- Snapshot таблицы
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

-- Checkpoint таблицы для проекций
CREATE TABLE IF NOT EXISTS projection_checkpoints (
	projection_name VARCHAR(255) PRIMARY KEY,
	position BIGINT NOT NULL,
	updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Saga таблицы
CREATE TABLE IF NOT EXISTS saga_instances (
	id UUID PRIMARY KEY,
	definition_name VARCHAR(255) NOT NULL,
	status VARCHAR(50) NOT NULL,
	context JSONB NOT NULL DEFAULT '{}',
	correlation_id VARCHAR(255),
	current_step VARCHAR(255),
	created_at TIMESTAMP NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
	completed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_saga_status ON saga_instances(status);
CREATE INDEX IF NOT EXISTS idx_saga_correlation ON saga_instances(correlation_id);
CREATE INDEX IF NOT EXISTS idx_saga_created ON saga_instances(created_at);
CREATE INDEX IF NOT EXISTS idx_saga_definition ON saga_instances(definition_name);

CREATE TABLE IF NOT EXISTS saga_history (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	saga_id UUID NOT NULL REFERENCES saga_instances(id) ON DELETE CASCADE,
	step_name VARCHAR(255) NOT NULL,
	status VARCHAR(50) NOT NULL,
	error TEXT,
	retry_attempt INT DEFAULT 0,
	started_at TIMESTAMP NOT NULL,
	completed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_history_saga ON saga_history(saga_id);
CREATE INDEX IF NOT EXISTS idx_history_step ON saga_history(step_name);
CREATE INDEX IF NOT EXISTS idx_history_started ON saga_history(started_at);

-- Функция для автоматического обновления updated_at
CREATE OR REPLACE FUNCTION update_saga_updated_at()
RETURNS TRIGGER AS $$
BEGIN
	NEW.updated_at = NOW();
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_saga_updated_at ON saga_instances;
CREATE TRIGGER trigger_update_saga_updated_at
	BEFORE UPDATE ON saga_instances
	FOR EACH ROW
	EXECUTE FUNCTION update_saga_updated_at();

-- Saga Read Models таблицы для проекций
CREATE TABLE IF NOT EXISTS saga_read_models (
	saga_id VARCHAR(255) PRIMARY KEY,
	definition_name VARCHAR(255) NOT NULL,
	status VARCHAR(50) NOT NULL,
	current_step VARCHAR(255),
	total_steps INTEGER,
	completed_steps INTEGER,
	failed_steps INTEGER,
	started_at TIMESTAMP NOT NULL,
	completed_at TIMESTAMP,
	duration_ms INTEGER,
	correlation_id VARCHAR(255),
	context JSONB,
	last_error TEXT,
	retry_count INTEGER,
	updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_saga_rm_status ON saga_read_models(status);
CREATE INDEX IF NOT EXISTS idx_saga_rm_definition ON saga_read_models(definition_name);
CREATE INDEX IF NOT EXISTS idx_saga_rm_correlation ON saga_read_models(correlation_id);
CREATE INDEX IF NOT EXISTS idx_saga_rm_started_at ON saga_read_models(started_at);

-- Saga Step Read Models таблица для истории шагов
CREATE TABLE IF NOT EXISTS saga_step_read_models (
	saga_id VARCHAR(255) NOT NULL,
	step_name VARCHAR(255) NOT NULL,
	status VARCHAR(50) NOT NULL,
	started_at TIMESTAMP NOT NULL,
	completed_at TIMESTAMP,
	duration_ms INTEGER,
	retry_attempt INTEGER DEFAULT 0,
	error TEXT,
	updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
	PRIMARY KEY (saga_id, step_name, started_at)
);

CREATE INDEX IF NOT EXISTS idx_saga_step_rm_saga_id ON saga_step_read_models(saga_id);
CREATE INDEX IF NOT EXISTS idx_saga_step_rm_status ON saga_step_read_models(status);

