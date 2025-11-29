package examples

import (
	"context"
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/adapters/messagebus"
	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/transport"
)

// GetUserQuery запрос для получения пользователя
type GetUserQuery struct {
	UserID string `json:"user_id"`
}

func (q GetUserQuery) QueryName() string {
	return "get_user"
}

// GetUserResponse ответ на запрос пользователя
type GetUserResponse struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// GetUserQueryHandler обработчик запроса пользователя
type GetUserQueryHandler struct {
	// В реальном приложении здесь будет репозиторий
	users map[string]*GetUserResponse
}

func NewGetUserQueryHandler() *GetUserQueryHandler {
	return &GetUserQueryHandler{
		users: map[string]*GetUserResponse{
			"user-1": {UserID: "user-1", Email: "alice@example.com", Name: "Alice", Active: true},
			"user-2": {UserID: "user-2", Email: "bob@example.com", Name: "Bob", Active: true},
		},
	}
}

func (h *GetUserQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	query := q.(GetUserQuery)

	user, exists := h.users[query.UserID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", query.UserID)
	}

	return user, nil
}

func (h *GetUserQueryHandler) QueryName() string {
	return "get_user"
}

// ExampleQueryInvokerWithNATS демонстрирует использование QueryInvoker с NATS Request-Reply
func ExampleQueryInvokerWithNATS() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Инициализация NATS адаптера
	natsAdapter, err := messagebus.NewNATSAdapter("nats://localhost:4222")
	if err != nil {
		fmt.Printf("Failed to create NATS adapter: %v\n", err)
		return
	}

	if err := natsAdapter.Start(ctx); err != nil {
		fmt.Printf("Failed to start NATS adapter: %v\n", err)
		return
	}
	defer func() {
		_ = natsAdapter.Stop(ctx)
	}()

	// 2. Создание InMemoryQueryBus
	queryBus := transport.NewInMemoryQueryBus()

	// 3. Регистрация query handler в QueryBus
	handler := NewGetUserQueryHandler()
	if err := queryBus.Register(handler); err != nil {
		fmt.Printf("Failed to register handler: %v\n", err)
		return
	}

	// Регистрация NATS responder для обработки запросов через NATS Request-Reply
	querySubject := "queries.get_user"
	serializer := invoke.NewJSONSerializer()

	if err := natsAdapter.Respond(ctx, querySubject, func(ctx context.Context, request *transport.Message) (*transport.Message, error) {
		var query GetUserQuery
		if err := serializer.Deserialize(request.Data, &query); err != nil {
			return nil, fmt.Errorf("failed to deserialize query: %w", err)
		}

		result, err := queryBus.Ask(ctx, query)
		if err != nil {
			return nil, err
		}

		resultData, err := serializer.Serialize(result)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize result: %w", err)
		}

		return &transport.Message{
			Subject: request.Subject,
			Data:    resultData,
			Headers: map[string]string{
				"status": "success",
			},
		}, nil
	}); err != nil {
		fmt.Printf("Failed to register responder: %v\n", err)
		return
	}

	// 4. Создание QueryInvoker с типами запроса/результата
	// Для работы через NATS нам нужно обернуть QueryBus
	natsQueryBus := &natsRequestReplyQueryBus{
		adapter:       natsAdapter,
		subject:       querySubject,
		serializer:    serializer,
		timeout:       5 * time.Second,
	}

	invoker := invoke.NewQueryInvoker[GetUserQuery, GetUserResponse](natsQueryBus).
		WithTimeout(5 * time.Second)

	// 5. Отправка запроса через Invoke() с timeout
	fmt.Println("=== Запрос пользователя ===")
	query := GetUserQuery{UserID: "user-1"}

	result, err := invoker.Invoke(ctx, query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("User found: %+v\n", result)
	}

	// 6. Получение типизированного результата
	fmt.Println("\n=== Типизированный результат ===")
	user, err := invoker.Invoke(ctx, GetUserQuery{UserID: "user-2"})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Email: %s, Name: %s, Active: %v\n", user.Email, user.Name, user.Active)
	}

	// 7. Демонстрация InvokeBatch() для пакетных запросов
	fmt.Println("\n=== Пакетные запросы ===")
	queries := []GetUserQuery{
		{UserID: "user-1"},
		{UserID: "user-2"},
	}

	results, err := invoker.InvokeBatch(ctx, queries)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		for i, user := range results {
			fmt.Printf("Result %d: %s - %s\n", i+1, user.Name, user.Email)
		}
	}

	// 8. Использование WithValidator() для валидации результата
	fmt.Println("\n=== Валидация результата ===")
	validatorInvoker := invoker.WithValidator(func(user GetUserResponse) error {
		if !user.Active {
			return fmt.Errorf("user is not active")
		}
		return nil
	})

	validUser, err := validatorInvoker.Invoke(ctx, GetUserQuery{UserID: "user-1"})
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
	} else {
		fmt.Printf("Valid user: %s\n", validUser.Name)
	}

	// 9. Обработка ошибок (timeout, invalid type, validation failed)
	fmt.Println("\n=== Обработка ошибок ===")
	// Запрос несуществующего пользователя
	_, err = invoker.Invoke(ctx, GetUserQuery{UserID: "user-999"})
	if err != nil {
		fmt.Printf("Expected error for non-existent user: %v\n", err)
	}

	// Output:
	// === Запрос пользователя ===
	// User found: &{UserID:user-1 Email:alice@example.com Name:Alice Active:true}
	// === Типизированный результат ===
	// Email: bob@example.com, Name: Bob, Active: true
	// === Пакетные запросы ===
	// Result 1: Alice - alice@example.com
	// Result 2: Bob - bob@example.com
	// === Валидация результата ===
	// Valid user: Alice
	// === Обработка ошибок ===
	// Expected error for non-existent user: user not found: user-999
}

// natsRequestReplyQueryBus обертка QueryBus для работы через NATS Request-Reply
type natsRequestReplyQueryBus struct {
	adapter    transport.RequestReply
	subject    string
	serializer transport.MessageSerializer
	timeout    time.Duration
}

func (b *natsRequestReplyQueryBus) Ask(ctx context.Context, q transport.Query) (interface{}, error) {
	// Сериализуем запрос
	data, err := b.serializer.Serialize(q)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query: %w", err)
	}

	// Отправляем запрос через NATS Request-Reply
	reply, err := b.adapter.Request(ctx, b.subject, data, b.timeout)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Десериализуем результат
	var result GetUserResponse
	if err := b.serializer.Deserialize(reply.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to deserialize result: %w", err)
	}

	return result, nil
}

func (b *natsRequestReplyQueryBus) Register(handler transport.QueryHandler) error {
	// Регистрация уже выполнена в примере
	return nil
}

