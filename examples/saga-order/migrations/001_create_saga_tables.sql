-- Миграция для создания таблиц Saga Pattern в PostgreSQL
-- Версия: 1.0
-- Дата: 2024-01-01

-- Таблица для хранения экземпляров саг
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

-- Индексы для быстрого поиска
CREATE INDEX IF NOT EXISTS idx_saga_status ON saga_instances(status);
CREATE INDEX IF NOT EXISTS idx_saga_correlation ON saga_instances(correlation_id);
CREATE INDEX IF NOT EXISTS idx_saga_created ON saga_instances(created_at);
CREATE INDEX IF NOT EXISTS idx_saga_definition ON saga_instances(definition_name);

-- Таблица для хранения истории выполнения шагов
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

-- Индексы для истории
CREATE INDEX IF NOT EXISTS idx_history_saga ON saga_history(saga_id);
CREATE INDEX IF NOT EXISTS idx_history_step ON saga_history(step_name);
CREATE INDEX IF NOT EXISTS idx_history_started ON saga_history(started_at);

