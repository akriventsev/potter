// Package examples содержит примеры использования адаптеров.
package examples

import (
	"context"
	"fmt"
	"log"

	"potter/framework/adapters/repository"
)

// User пример entity
type User struct {
	IDField string
	Name    string
	Email   string
}

func (u User) ID() string {
	return u.IDField
}

// UserMapper пример mapper для User
type UserMapper struct{}

func (m *UserMapper) ToRow(entity User) (map[string]interface{}, error) {
	return map[string]interface{}{
		"id":    entity.IDField,
		"name":  entity.Name,
		"email": entity.Email,
	}, nil
}

func (m *UserMapper) FromRow(row map[string]interface{}) (User, error) {
	return User{
		IDField: row["id"].(string),
		Name:    row["name"].(string),
		Email:   row["email"].(string),
	}, nil
}

// ExamplePostgresRepository демонстрирует использование PostgreSQL Repository
func ExamplePostgresRepository() {
	// Создание конфигурации
	config := repository.PostgresConfig{
		DSN:        "postgres://user:password@localhost/dbname",
		TableName:  "users",
		SchemaName: "public",
	}

	// Создание mapper
	mapper := &UserMapper{}

	// Создание репозитория
	repo, err := repository.NewPostgresRepository[User](config, mapper)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Запуск репозитория
	if err := repo.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := repo.Stop(ctx); err != nil {
			log.Printf("Failed to stop repository: %v", err)
		}
	}()

	// Сохранение entity
	user := User{
		IDField: "user-123",
		Name:    "John Doe",
		Email:   "john@example.com",
	}

	err = repo.Save(ctx, user)
	if err != nil {
		log.Fatal(err)
	}

	// Поиск по ID
	found, err := repo.FindByID(ctx, "user-123")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found user: %+v\n", found)
}

