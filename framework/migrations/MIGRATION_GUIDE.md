# Руководство по миграции с Potter v1.3.x на v1.5.0

Это руководство поможет вам мигрировать существующие проекты с Potter Framework v1.3.x на v1.5.0, где самописная система миграций была заменена на интеграцию с goose.

## Обзор изменений

### Что изменилось

- **Система миграций**: Заменена самописная система (~1000 строк кода) на интеграцию с [goose](https://github.com/pressly/goose)
- **Формат файлов**: Изменен с отдельных `.up.sql` и `.down.sql` файлов на единый файл с аннотациями `-- +goose Up` и `-- +goose Down`
- **Таблица истории**: Goose использует таблицу `goose_db_version` вместо `schema_migrations`
- **CLI инструмент**: `potter-migrate` переписан на использование goose, но сохраняет обратную совместимость интерфейса

### Почему это изменение

- **Индустриальный стандарт**: goose - широко используемый инструмент с активной поддержкой сообщества
- **Лучшая поддержка БД**: Поддержка PostgreSQL, MySQL, SQLite, MongoDB и других
- **Go миграции**: Возможность использовать Go код для сложных миграций (особенно для MongoDB)
- **Out-of-order миграции**: Поддержка применения миграций вне порядка
- **Упрощение кодовой базы**: Удаление ~1000 строк самописного кода

## Установка goose

### macOS / Linux

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Windows

```powershell
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Проверка установки

```bash
goose -version
```

## Конвертация файлов миграций

### Автоматическая конвертация

Используйте скрипт `scripts/convert_migrations.sh` для автоматической конвертации:

```bash
./scripts/convert_migrations.sh ./migrations
```

**Требования:** Скрипт требует bash 4.0+ для поддержки ассоциативных массивов.

**На macOS:** Стандартная версия bash на macOS - 3.2, которая не поддерживает ассоциативные массивы. Установите обновленную версию через Homebrew:

```bash
brew install bash
```

Затем запустите скрипт с указанием полного пути к новой версии:

```bash
/usr/local/bin/bash ./scripts/convert_migrations.sh ./migrations
```

Или обновите shebang в начале скрипта на `#!/usr/local/bin/bash`.

Скрипт:
- Находит все пары `.up.sql` и `.down.sql` файлов
- Создает новые файлы в формате goose
- Создает backup старых файлов в `.backup/`
- Выводит отчет о конвертированных файлах

Опции:
- `--dry-run`: Предпросмотр без изменений
- `--no-backup`: Пропустить создание backup

### Ручная конвертация

Если у вас есть миграция:

**Старый формат:**
```
migrations/
├── 001_create_users.up.sql
└── 001_create_users.down.sql
```

**Новый формат:**
```
migrations/
└── 001_create_users.sql
```

**Содержимое `001_create_users.sql`:**
```sql
-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);

-- +goose Down
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
```

### Обработка edge cases

#### Комментарии в миграциях

Комментарии сохраняются как есть:

```sql
-- +goose Up
-- Миграция для создания таблицы пользователей
CREATE TABLE users (...);

-- +goose Down
-- Удаление таблицы пользователей
DROP TABLE users;
```

#### Сложные SQL с функциями и триггерами

Все SQL команды сохраняются без изменений:

```sql
-- +goose Up
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trigger_update_updated_at ON users;
DROP FUNCTION IF EXISTS update_updated_at() CASCADE;
```

## Миграция таблицы истории

Goose использует таблицу `goose_db_version` вместо `schema_migrations`. Для сохранения истории применения миграций выполните SQL скрипт:

```bash
psql -d your_database -f scripts/migrate_history.sql
```

Скрипт:
1. Создает таблицу `goose_db_version` если не существует
2. Мигрирует данные из `schema_migrations` в `goose_db_version`
3. Создает backup старой таблицы в `schema_migrations_backup`
4. Выводит статистику мигрированных записей

### Ручная миграция истории

Если вы предпочитаете сделать это вручную:

```sql
-- Создаем таблицу goose
CREATE TABLE IF NOT EXISTS goose_db_version (
    id serial PRIMARY KEY,
    version_id bigint NOT NULL,
    is_applied boolean NOT NULL,
    tstamp timestamp DEFAULT now()
);

-- Мигрируем данные
INSERT INTO goose_db_version (version_id, is_applied, tstamp)
SELECT 
    CAST(version AS bigint),
    true,
    applied_at
FROM schema_migrations
WHERE NOT EXISTS (
    SELECT 1 FROM goose_db_version 
    WHERE version_id = CAST(schema_migrations.version AS bigint)
)
ORDER BY version;

-- Создаем backup
CREATE TABLE IF NOT EXISTS schema_migrations_backup AS 
SELECT * FROM schema_migrations;
```

## Обновление кода

### Замена импортов

**Старый код:**
```go
import "potter/framework/migrations"

migrator := migrations.NewMigrator(migrations.NewPostgresMigrationDB(dsn))
migrator.RegisterFromFiles("migrations")
err := migrator.Up(ctx)
```

**Новый код:**
```go
import (
    "database/sql"
    "potter/framework/migrations"
    _ "github.com/jackc/pgx/v5/stdlib"
)

db, err := sql.Open("pgx", dsn)
if err != nil {
    return err
}
defer db.Close()

err = migrations.RunMigrations(db, "./migrations")
```

### Обновление программных вызовов

**Старый API:**
```go
// Получение статуса
statuses, err := migrator.Status(ctx)

// Получение версии
version, err := migrator.Version(ctx)

// Откат миграций
err := migrator.Down(ctx, 1)
```

**Новый API:**
```go
// Получение статуса
statuses, err := migrations.GetMigrationStatus(db, "./migrations")

// Получение версии
version, err := migrations.GetCurrentVersion(db)

// Откат миграций
err := migrations.RollbackMigration(db, "./migrations")
// или для N миграций
err := migrations.RollbackMigrations(db, "./migrations", 3)
```

### Примеры до/после

#### Пример 1: Применение миграций при старте приложения

**До:**
```go
func initMigrations(dsn string) error {
    ctx := context.Background()
    migrator := migrations.NewMigrator(migrations.NewPostgresMigrationDB(dsn))
    if err := migrator.RegisterFromFiles("./migrations"); err != nil {
        return err
    }
    return migrator.Up(ctx)
}
```

**После:**
```go
import (
    "database/sql"
    "potter/framework/migrations"
    _ "github.com/jackc/pgx/v5/stdlib"
)

func initMigrations(dsn string) error {
    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return err
    }
    defer db.Close()
    
    return migrations.RunMigrations(db, "./migrations")
}
```

#### Пример 2: Проверка статуса миграций

**До:**
```go
func checkMigrations(dsn string) error {
    ctx := context.Background()
    migrator := migrations.NewMigrator(migrations.NewPostgresMigrationDB(dsn))
    migrator.RegisterFromFiles("./migrations")
    
    statuses, err := migrator.Status(ctx)
    if err != nil {
        return err
    }
    
    for _, status := range statuses {
        if status.Status == "pending" {
            log.Printf("Pending migration: %s", status.Name)
        }
    }
    return nil
}
```

**После:**
```go
func checkMigrations(dsn string) error {
    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return err
    }
    defer db.Close()
    
    statuses, err := migrations.GetMigrationStatus(db, "./migrations")
    if err != nil {
        return err
    }
    
    for _, status := range statuses {
        if status.Status == "pending" {
            log.Printf("Pending migration: %d - %s", status.Version, status.Name)
        }
    }
    return nil
}
```

## Обновление CI/CD

### GitHub Actions

**До:**
```yaml
- name: Run migrations
  run: |
    potter-migrate up \
      --database-url ${{ secrets.DATABASE_URL }} \
      --migrations-dir ./migrations
```

**После:**
```yaml
- name: Install goose
  run: go install github.com/pressly/goose/v3/cmd/goose@latest

- name: Run migrations
  run: |
    goose -dir migrations postgres "${{ secrets.DATABASE_URL }}" up
```

### GitLab CI

**До:**
```yaml
migrate:
  script:
    - potter-migrate up --database-url $DATABASE_URL --migrations-dir ./migrations
```

**После:**
```yaml
migrate:
  before_script:
    - go install github.com/pressly/goose/v3/cmd/goose@latest
  script:
    - goose -dir migrations postgres "$DATABASE_URL" up
```

### Docker

**До:**
```dockerfile
FROM golang:1.25.0-alpine AS builder
RUN go install ./cmd/potter-migrate

FROM alpine
COPY --from=builder /go/bin/potter-migrate /usr/local/bin/
```

**После:**
```dockerfile
FROM golang:1.25.0-alpine AS builder
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

FROM alpine
COPY --from=builder /go/bin/goose /usr/local/bin/
```

## Troubleshooting

### Проблема: "goose: command not found"

**Решение:**
```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Убедитесь, что `$GOPATH/bin` или `$HOME/go/bin` добавлен в `$PATH`.

### Проблема: "no such file or directory" при применении миграций

**Причина:** Неправильный путь к директории с миграциями.

**Решение:** Убедитесь, что путь указан относительно текущей директории или используйте абсолютный путь:
```bash
goose -dir ./migrations postgres "$DATABASE_URL" up
```

### Проблема: "relation 'goose_db_version' does not exist"

**Причина:** Таблица истории не создана.

**Решение:** Goose создаст таблицу автоматически при первом запуске. Если это не произошло, создайте вручную:
```sql
CREATE TABLE IF NOT EXISTS goose_db_version (
    id serial PRIMARY KEY,
    version_id bigint NOT NULL,
    is_applied boolean NOT NULL,
    tstamp timestamp DEFAULT now()
);
```

### Проблема: Миграции не применяются после конвертации

**Причина:** Версии миграций не совпадают между старой и новой системой.

**Решение:**
1. Проверьте, что все миграции конвертированы правильно
2. Убедитесь, что миграция истории выполнена
3. Проверьте статус: `goose -dir migrations postgres "$DATABASE_URL" status`

### Проблема: Ошибка "duplicate key value violates unique constraint"

**Причина:** Попытка применить уже примененную миграцию.

**Решение:** Используйте `goose fix` для исправления состояния:
```bash
goose -dir migrations postgres "$DATABASE_URL" fix
```

## Rollback план

Если вам нужно откатиться на старую версию Potter:

1. **Восстановите старые миграции** из backup (если использовали скрипт конвертации)
2. **Восстановите таблицу истории** из `schema_migrations_backup`:
   ```sql
   DROP TABLE IF EXISTS goose_db_version;
   CREATE TABLE schema_migrations AS SELECT * FROM schema_migrations_backup;
   ```
3. **Откатите код** на версию v1.3.x
4. **Используйте старый potter-migrate** из v1.3.x

## Дополнительные ресурсы

- [Документация goose](https://github.com/pressly/goose)
- [framework/migrations/README.md](README.md) - документация по использованию миграций
- [Примеры миграций](../../examples/) - примеры использования в проектах

## Поддержка

Если у вас возникли проблемы с миграцией, создайте issue в репозитории проекта с описанием проблемы и шагами для воспроизведения.

