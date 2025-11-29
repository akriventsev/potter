-- Миграция для создания таблиц Saga Pattern в PostgreSQL

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

