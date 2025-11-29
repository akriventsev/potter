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

-- Комментарии к таблице
COMMENT ON TABLE saga_instances IS 'Хранит экземпляры саг с их текущим состоянием';
COMMENT ON COLUMN saga_instances.id IS 'Уникальный идентификатор экземпляра саги';
COMMENT ON COLUMN saga_instances.definition_name IS 'Имя определения саги';
COMMENT ON COLUMN saga_instances.status IS 'Текущий статус саги (pending, running, completed, compensating, compensated, failed)';
COMMENT ON COLUMN saga_instances.context IS 'Контекст выполнения саги в формате JSONB';
COMMENT ON COLUMN saga_instances.correlation_id IS 'Correlation ID для трассировки';
COMMENT ON COLUMN saga_instances.current_step IS 'Текущий выполняемый шаг';
COMMENT ON COLUMN saga_instances.created_at IS 'Время создания саги';
COMMENT ON COLUMN saga_instances.updated_at IS 'Время последнего обновления';
COMMENT ON COLUMN saga_instances.completed_at IS 'Время завершения саги';

-- Таблица для хранения истории выполнения шагов
CREATE TABLE IF NOT EXISTS saga_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_id UUID NOT NULL REFERENCES saga_instances(id) ON DELETE CASCADE,
    step_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    error TEXT,
    retry_attempt INT DEFAULT 0,
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    CONSTRAINT fk_saga_history_saga FOREIGN KEY (saga_id) REFERENCES saga_instances(id) ON DELETE CASCADE
);

-- Индексы для истории
CREATE INDEX IF NOT EXISTS idx_history_saga ON saga_history(saga_id);
CREATE INDEX IF NOT EXISTS idx_history_step ON saga_history(step_name);
CREATE INDEX IF NOT EXISTS idx_history_started ON saga_history(started_at);

-- Комментарии к таблице истории
COMMENT ON TABLE saga_history IS 'Хранит историю выполнения шагов саги';
COMMENT ON COLUMN saga_history.id IS 'Уникальный идентификатор записи истории';
COMMENT ON COLUMN saga_history.saga_id IS 'Ссылка на экземпляр саги';
COMMENT ON COLUMN saga_history.step_name IS 'Имя выполненного шага';
COMMENT ON COLUMN saga_history.status IS 'Статус выполнения шага (pending, running, completed, failed, compensating, compensated)';
COMMENT ON COLUMN saga_history.error IS 'Текст ошибки, если шаг завершился с ошибкой';
COMMENT ON COLUMN saga_history.retry_attempt IS 'Номер попытки выполнения (для retry)';
COMMENT ON COLUMN saga_history.started_at IS 'Время начала выполнения шага';
COMMENT ON COLUMN saga_history.completed_at IS 'Время завершения шага';

-- Таблица для хранения snapshots состояния саг
CREATE TABLE IF NOT EXISTS saga_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_id UUID NOT NULL REFERENCES saga_instances(id) ON DELETE CASCADE,
    step_number INT NOT NULL,
    state JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_saga_snapshots_saga FOREIGN KEY (saga_id) REFERENCES saga_instances(id) ON DELETE CASCADE,
    CONSTRAINT uq_saga_snapshot UNIQUE (saga_id, step_number)
);

-- Индексы для snapshots
CREATE INDEX IF NOT EXISTS idx_snapshot_saga ON saga_snapshots(saga_id);
CREATE INDEX IF NOT EXISTS idx_snapshot_step ON saga_snapshots(step_number);

-- Комментарии к таблице snapshots
COMMENT ON TABLE saga_snapshots IS 'Хранит snapshots состояния саг для оптимизации восстановления';
COMMENT ON COLUMN saga_snapshots.id IS 'Уникальный идентификатор snapshot';
COMMENT ON COLUMN saga_snapshots.saga_id IS 'Ссылка на экземпляр саги';
COMMENT ON COLUMN saga_snapshots.step_number IS 'Номер шага, на котором создан snapshot';
COMMENT ON COLUMN saga_snapshots.state IS 'Состояние саги в формате JSONB';
COMMENT ON COLUMN saga_snapshots.created_at IS 'Время создания snapshot';

-- Функция для автоматического обновления updated_at
CREATE OR REPLACE FUNCTION update_saga_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Триггер для автоматического обновления updated_at
CREATE TRIGGER trigger_update_saga_updated_at
    BEFORE UPDATE ON saga_instances
    FOR EACH ROW
    EXECUTE FUNCTION update_saga_updated_at();

-- Примеры запросов:
-- 
-- Получить все саги со статусом 'running':
-- SELECT * FROM saga_instances WHERE status = 'running';
--
-- Получить историю выполнения саги:
-- SELECT * FROM saga_history WHERE saga_id = '...' ORDER BY started_at ASC;
--
-- Получить последний snapshot саги:
-- SELECT * FROM saga_snapshots WHERE saga_id = '...' ORDER BY step_number DESC LIMIT 1;
--
-- Получить все саги с определенным correlation_id:
-- SELECT * FROM saga_instances WHERE correlation_id = '...';

