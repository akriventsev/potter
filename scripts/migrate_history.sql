-- Скрипт миграции истории миграций с Potter v1.3.x на goose
-- Использование: psql -d your_database -f scripts/migrate_history.sql
--
-- Скрипт проверяет наличие таблицы schema_migrations перед выполнением миграции.
-- Если таблица отсутствует, скрипт завершается с уведомлением и не выполняет никаких операций.
-- Если таблица существует, скрипт создает таблицу goose_db_version, мигрирует данные
-- и создает backup старой таблицы.

-- Создаем таблицу goose если не существует (независимо от наличия schema_migrations)
CREATE TABLE IF NOT EXISTS goose_db_version (
    id serial PRIMARY KEY,
    version_id bigint NOT NULL,
    is_applied boolean NOT NULL,
    tstamp timestamp DEFAULT now()
);

-- Создаем уникальный индекс для version_id если не существует
CREATE UNIQUE INDEX IF NOT EXISTS idx_goose_db_version_version_id ON goose_db_version(version_id);

-- Проверяем наличие schema_migrations и выполняем миграцию данных
DO $$
DECLARE
    table_exists BOOLEAN;
    migrated_count INTEGER;
    total_count INTEGER;
BEGIN
    -- Проверяем существование таблицы schema_migrations
    SELECT EXISTS (
        SELECT FROM pg_tables 
        WHERE schemaname = 'public' AND tablename = 'schema_migrations'
    ) INTO table_exists;

    IF NOT table_exists THEN
        RAISE NOTICE 'Table schema_migrations does not exist. Skipping data migration.';
        RAISE NOTICE 'Table goose_db_version has been created (if it did not exist).';
        RETURN;
    END IF;

    RAISE NOTICE 'Found schema_migrations table. Starting data migration...';

    -- Мигрируем данные из schema_migrations в goose_db_version
    -- Конвертируем строковые версии в числовые
    INSERT INTO goose_db_version (version_id, is_applied, tstamp)
    SELECT 
        CASE 
            -- Если версия - это число, используем его напрямую
            WHEN version ~ '^[0-9]+$' THEN CAST(version AS bigint)
            -- Если версия - это timestamp (например, 20240101120000), используем его
            WHEN version ~ '^[0-9]{14}$' THEN CAST(version AS bigint)
            -- Иначе пытаемся извлечь число из начала строки
            ELSE CAST(SUBSTRING(version FROM '^([0-9]+)') AS bigint)
        END AS version_id,
        true AS is_applied,
        COALESCE(applied_at, NOW()) AS tstamp
    FROM schema_migrations
    WHERE NOT EXISTS (
        SELECT 1 FROM goose_db_version 
        WHERE version_id = CASE 
            WHEN schema_migrations.version ~ '^[0-9]+$' THEN CAST(schema_migrations.version AS bigint)
            WHEN schema_migrations.version ~ '^[0-9]{14}$' THEN CAST(schema_migrations.version AS bigint)
            ELSE CAST(SUBSTRING(schema_migrations.version FROM '^([0-9]+)') AS bigint)
        END
    )
    ORDER BY 
        CASE 
            WHEN version ~ '^[0-9]+$' THEN CAST(version AS bigint)
            WHEN version ~ '^[0-9]{14}$' THEN CAST(version AS bigint)
            ELSE CAST(SUBSTRING(version FROM '^([0-9]+)') AS bigint)
        END;

    -- Создаем backup старой таблицы (если еще не существует)
    IF NOT EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'schema_migrations_backup') THEN
        CREATE TABLE schema_migrations_backup AS SELECT * FROM schema_migrations;
        RAISE NOTICE 'Created backup table schema_migrations_backup';
    ELSE
        -- Обновляем backup, добавляя только новые записи
        INSERT INTO schema_migrations_backup
        SELECT * FROM schema_migrations
        WHERE NOT EXISTS (
            SELECT 1 FROM schema_migrations_backup 
            WHERE schema_migrations_backup.version = schema_migrations.version
        );
        RAISE NOTICE 'Updated existing backup table schema_migrations_backup';
    END IF;

    -- Выводим статистику
    SELECT COUNT(*) INTO migrated_count FROM goose_db_version;
    SELECT COUNT(*) INTO total_count FROM schema_migrations;
    
    RAISE NOTICE 'Migration completed:';
    RAISE NOTICE '  Migrated records: %', migrated_count;
    RAISE NOTICE '  Total records in schema_migrations: %', total_count;
    
    IF migrated_count < total_count THEN
        RAISE WARNING 'Some records were not migrated. Check for version format issues.';
    END IF;
END $$;

-- Выводим примеры мигрированных версий (если есть данные)
DO $$
DECLARE
    sample_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO sample_count FROM goose_db_version;
    IF sample_count > 0 THEN
        RAISE NOTICE 'Sample of migrated versions:';
        FOR rec IN 
            SELECT version_id, is_applied, tstamp 
            FROM goose_db_version 
            ORDER BY version_id 
            LIMIT 10
        LOOP
            RAISE NOTICE '  Version: %, Applied: %, Timestamp: %', 
                rec.version_id, rec.is_applied, rec.tstamp;
        END LOOP;
    END IF;
END $$;

