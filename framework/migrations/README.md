# Database Migrations

Potter Framework использует [goose](https://github.com/pressly/goose) для управления миграциями схемы базы данных.

## Обзор

Goose - это инструмент для управления миграциями БД, который поддерживает:
- SQL миграции для PostgreSQL, MySQL, SQLite
- Go миграции для сложных сценариев (включая MongoDB)
- Out-of-order миграции
- Environment variable substitution
- Версионирование и rollback

## Формат файлов миграций

### SQL миграции

Миграции должны быть в едином файле с аннотациями `-- +goose Up` и `-- +goose Down`:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- +goose Down
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
```

### Go миграции (для MongoDB)

Для MongoDB рекомендуется использовать Go миграции:

```go
package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	goose.AddMigration(upInitCollections, downInitCollections)
}

func upInitCollections(tx *sql.Tx) error {
	// Получаем MongoDB клиент из контекста или глобальной переменной
	// В реальном приложении используйте dependency injection
	client := getMongoClient()
	
	ctx := context.Background()
	
	// Создаем коллекции
	db := client.Database("myapp")
	
	// Создаем коллекцию event_store
	if err := db.CreateCollection(ctx, "event_store"); err != nil {
		return err
	}
	
	// Создаем индексы
	eventStoreCollection := db.Collection("event_store")
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "aggregate_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "event_type", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "occurred_at", Value: 1}},
		},
	}
	
	_, err := eventStoreCollection.Indexes().CreateMany(ctx, indexes)
	return err
}

func downInitCollections(tx *sql.Tx) error {
	client := getMongoClient()
	ctx := context.Background()
	db := client.Database("myapp")
	
	// Удаляем коллекции
	if err := db.Collection("event_store").Drop(ctx); err != nil {
		return err
	}
	
	return nil
}
```

## Создание новых миграций

### Через CLI

```bash
# Установите goose CLI
go install github.com/pressly/goose/v3/cmd/goose@latest

# Создайте новую миграцию
goose -dir migrations create add_user_roles sql
```

### Через potter-migrate

```bash
potter-migrate create add_user_roles --migrations-dir ./migrations
```

### Программно

```go
import "github.com/akriventsev/potter/framework/migrations"

err := migrations.CreateMigration("./migrations", "add_user_roles")
```

## Применение миграций

### Через goose CLI

```bash
# Применить все pending миграции
goose -dir migrations postgres "postgres://user:pass@localhost/dbname?sslmode=disable" up

# Откатить последнюю миграцию
goose -dir migrations postgres "postgres://user:pass@localhost/dbname?sslmode=disable" down

# Показать статус
goose -dir migrations postgres "postgres://user:pass@localhost/dbname?sslmode=disable" status

# Откатить N миграций
goose -dir migrations postgres "postgres://user:pass@localhost/dbname?sslmode=disable" down-to VERSION
```

### Через potter-migrate

**PostgreSQL:**
```bash
# Применить все миграции
potter-migrate up --database-url postgres://user:pass@localhost/dbname --migrations-dir ./migrations

# Откатить миграции
potter-migrate down 1 --database-url postgres://user:pass@localhost/dbname --migrations-dir ./migrations

# Показать статус
potter-migrate status --database-url postgres://user:pass@localhost/dbname --migrations-dir ./migrations

# Показать текущую версию
potter-migrate version --database-url postgres://user:pass@localhost/dbname --migrations-dir ./migrations
```

**MySQL:** (обязательно укажите `--dialect mysql`)
```bash
potter-migrate up --database-url mysql://user:pass@tcp(localhost:3306)/dbname --dialect mysql --migrations-dir ./migrations
```

**SQLite:** (обязательно укажите `--dialect sqlite3`)
```bash
potter-migrate up --database-url sqlite3://./database.db --dialect sqlite3 --migrations-dir ./migrations
```

### Программно

```go
import (
	"database/sql"
	"github.com/akriventsev/potter/framework/migrations"
	
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	db, err := sql.Open("pgx", "postgres://user:pass@localhost/dbname?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Применить все миграции
	if err := migrations.RunMigrations(db, "./migrations"); err != nil {
		log.Fatal(err)
	}

	// Получить статус
	statuses, err := migrations.GetMigrationStatus(db, "./migrations")
	if err != nil {
		log.Fatal(err)
	}

	for _, status := range statuses {
		fmt.Printf("%s: %s\n", status.Name, status.Status)
	}
}
```

## Environment Variable Substitution

Goose поддерживает подстановку переменных окружения в миграциях:

```sql
-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Использование переменной окружения
INSERT INTO users (id, email) VALUES 
    ('00000000-0000-0000-0000-000000000001', '${ADMIN_EMAIL}');

-- +goose Down
DROP TABLE users;
```

## Out-of-Order Migrations

Goose поддерживает применение миграций вне порядка. Это полезно при работе в команде:

```bash
goose -dir migrations postgres "postgres://..." up
```

Goose автоматически определит и применит только те миграции, которые еще не были применены.

## Миграция с Potter v1.3.x

Если вы используете старую версию Potter с самописной системой миграций, выполните следующие шаги:

### 1. Установите goose

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### 2. Конвертируйте миграции

Используйте скрипт `scripts/convert_migrations.sh` для автоматической конвертации:

```bash
./scripts/convert_migrations.sh ./migrations
```

Или конвертируйте вручную:
- Объедините `.up.sql` и `.down.sql` файлы в один
- Добавьте аннотации `-- +goose Up` и `-- +goose Down`
- Переименуйте файлы в формат `YYYYMMDDHHMMSS_name.sql`

### 3. Мигрируйте историю миграций

Выполните SQL скрипт для переноса данных из `schema_migrations` в `goose_db_version`:

```bash
psql -d your_database -f scripts/migrate_history.sql
```

### 4. Обновите код

Замените использование старого API:

```go
// Старый код
migrator := migrations.NewMigrator(migrations.NewPostgresMigrationDB(dsn))
migrator.RegisterFromFiles("migrations")
err := migrator.Up(ctx)

// Новый код
import "github.com/akriventsev/potter/framework/migrations"
import _ "github.com/jackc/pgx/v5/stdlib"

db, _ := sql.Open("pgx", dsn)
err := migrations.RunMigrations(db, "migrations")
```

### 5. Обновите CI/CD

Замените `potter-migrate` на `goose` в скриптах сборки:

```yaml
# Старый вариант
- potter-migrate up --database-url $DATABASE_URL

# Новый вариант
- goose -dir migrations postgres "$DATABASE_URL" up
```

Подробные инструкции см. в [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md).

## Поддержка различных БД

**Важно:** Для не‑PostgreSQL БД (MySQL, SQLite) **обязательно** необходимо вызывать `migrations.SetDialect()` перед использованием функций миграций или передавать флаг `--dialect` в `potter-migrate`.

### PostgreSQL

```go
import _ "github.com/jackc/pgx/v5/stdlib"

db, err := sql.Open("pgx", "postgres://user:pass@localhost/dbname?sslmode=disable")
// Для PostgreSQL диалект устанавливается автоматически (postgres по умолчанию)
```

### MySQL

```go
import _ "github.com/go-sql-driver/mysql"

db, err := sql.Open("mysql", "user:pass@tcp(localhost:3306)/dbname")
// Обязательно установите диалект для MySQL
if err := migrations.SetDialect("mysql"); err != nil {
    log.Fatal(err)
}
err = migrations.RunMigrations(db, "./migrations")
```

Или через CLI:
```bash
potter-migrate up --database-url mysql://user:pass@tcp(localhost:3306)/dbname --dialect mysql
```

### SQLite

```go
import _ "github.com/mattn/go-sqlite3"

db, err := sql.Open("sqlite3", "./database.db")
// Обязательно установите диалект для SQLite
if err := migrations.SetDialect("sqlite3"); err != nil {
    log.Fatal(err)
}
err = migrations.RunMigrations(db, "./migrations")
```

Или через CLI:
```bash
potter-migrate up --database-url sqlite3://./database.db --dialect sqlite3
```

### MongoDB

Используйте Go миграции (см. пример выше).

## Лучшие практики

1. **Всегда создавайте Down миграции** - они необходимы для отката
2. **Используйте транзакции** - goose автоматически оборачивает каждую миграцию в транзакцию
3. **Не изменяйте примененные миграции** - создавайте новые миграции для изменений
4. **Используйте IF EXISTS/IF NOT EXISTS** - для идемпотентности
5. **Тестируйте миграции** - применяйте и откатывайте миграции в тестовой БД
6. **Версионируйте миграции** - используйте timestamp в имени файла

## Дополнительные ресурсы

- [Официальная документация goose](https://github.com/pressly/goose)
- [Примеры миграций в Potter Framework](../../examples/)
- [Руководство по миграции с v1.3.x](MIGRATION_GUIDE.md)

