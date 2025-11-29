// Package examples содержит примеры использования адаптеров.
package examples

import (
	"context"
	"fmt"
	"log"
	"time"

	transportadapters "github.com/akriventsev/potter/framework/adapters/transport"
	"github.com/akriventsev/potter/framework/transport"
)

// ExampleRESTAdapter демонстрирует использование REST Transport адаптера
func ExampleRESTAdapter() {
	// Создание command и query buses
	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()

	// Создание конфигурации
	config := transportadapters.RESTConfig{
		Port:         8080,
		BasePath:     "/api/v1",
		EnableMetrics: true,
	}

	// Создание адаптера
	adapter, err := transportadapters.NewRESTAdapter(config, commandBus, queryBus)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Регистрация command
	adapter.RegisterCommand("POST", "/users", &CreateUserCommand{})

	// Регистрация query
	adapter.RegisterQuery("GET", "/users/:id", &GetUserQuery{})

	// Запуск сервера
	if err := adapter.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := adapter.Stop(ctx); err != nil {
			log.Printf("Failed to stop adapter: %v", err)
		}
	}()

	fmt.Println("REST server started on :8080")
	time.Sleep(5 * time.Second)
}

// CreateUserCommand пример команды
type CreateUserCommand struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (c *CreateUserCommand) CommandName() string {
	return "create_user"
}

// GetUserQuery пример запроса
type GetUserQuery struct {
	ID string `json:"id"`
}

func (q *GetUserQuery) QueryName() string {
	return "get_user"
}

