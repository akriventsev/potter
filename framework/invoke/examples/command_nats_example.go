package examples

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/akriventsev/potter/framework/adapters/messagebus"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/transport"
)

// CreateUserCommand команда для создания пользователя
type CreateUserCommand struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (c CreateUserCommand) CommandName() string {
	return "create_user"
}

// UserCreatedEvent событие успешного создания пользователя
type UserCreatedEvent struct {
	*events.BaseEvent
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

func NewUserCreatedEvent(userID, email, name string) *UserCreatedEvent {
	return &UserCreatedEvent{
		BaseEvent: events.NewBaseEvent("user.created", userID),
		UserID:    userID,
		Email:     email,
		Name:      name,
	}
}

// UserCreationFailedEvent событие ошибки при создании пользователя
type UserCreationFailedEvent struct {
	*invoke.BaseErrorEvent
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

func NewUserCreationFailedEvent(email, reason string, err error) *UserCreationFailedEvent {
	return &UserCreationFailedEvent{
		BaseErrorEvent: invoke.NewBaseErrorEvent(
			"user.creation_failed",
			"",
			"VALIDATION_ERROR",
			reason,
			err,
			false,
		),
		Email:  email,
		Reason: reason,
	}
}

// UserCommandHandler обработчик команд пользователя
type UserCommandHandler struct {
	eventPublisher events.EventPublisher
}

func NewUserCommandHandler(eventPublisher events.EventPublisher) *UserCommandHandler {
	return &UserCommandHandler{
		eventPublisher: eventPublisher,
	}
}

func (h *UserCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	createCmd := cmd.(CreateUserCommand)

	// Извлекаем correlation ID из контекста
	correlationID := invoke.ExtractCorrelationID(ctx)
	if correlationID == "" {
		return fmt.Errorf("correlation ID not found in context")
	}

	// Валидация
	if createCmd.Email == "" {
		errorEvent := NewUserCreationFailedEvent(
			createCmd.Email,
			"Email is required",
			fmt.Errorf("email is required"),
		)
		errorEvent.WithCorrelationID(correlationID)
		_ = h.eventPublisher.Publish(ctx, errorEvent)
		return nil
	}

	// Имитация создания пользователя (в реальном приложении - сохранение в БД)
	userID := fmt.Sprintf("user-%d", time.Now().UnixNano())

	// Публикуем событие успешного создания с correlation ID
	successEvent := NewUserCreatedEvent(userID, createCmd.Email, createCmd.Name)
	successEvent.WithCorrelationID(correlationID)

	if err := h.eventPublisher.Publish(ctx, successEvent); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

func (h *UserCommandHandler) CommandName() string {
	return "create_user"
}

// ExampleCommandInvokerWithNATS демонстрирует использование CommandInvoker с NATS транспортом
func ExampleCommandInvokerWithNATS() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Инициализация NATS адаптера
	natsBuilder := messagebus.NewNATSAdapterBuilder().
		WithURL("nats://localhost:4222").
		WithMaxReconnects(10).
		WithReconnectWait(2 * time.Second).
		WithConnectionTimeout(5 * time.Second).
		WithMetrics(true)

	natsAdapter, err := natsBuilder.Build()
	if err != nil {
		fmt.Printf("Failed to create NATS adapter: %v\n", err)
		return
	}

	// Запуск адаптера
	if err := natsAdapter.Start(ctx); err != nil {
		fmt.Printf("Failed to start NATS adapter: %v\n", err)
		return
	}
	defer func() {
		_ = natsAdapter.Stop(ctx)
	}()

	// 2. Создание EventBus
	eventBus := events.NewInMemoryEventBus()

	// 3. Создание AsyncCommandBus с NATS publisher
	asyncBus := invoke.NewAsyncCommandBus(natsAdapter)
	asyncBus.WithSubjectPrefix("commands")

	// 4. Создание EventAwaiter из EventBus
	awaiter := invoke.NewEventAwaiterFromEventBus(eventBus)

	// 5. Создание CommandInvoker с типами команды/событий
	invoker := invoke.NewCommandInvoker[CreateUserCommand, UserCreatedEvent, UserCreationFailedEvent](
		asyncBus,
		awaiter,
		"user.created",          // тип успешного события
		"user.creation_failed",  // тип ошибочного события
	).WithTimeout(10 * time.Second)

	// 6. Создание handler и регистрация подписки на команды через NATS
	eventPublisher := eventBus // EventBus реализует EventPublisher
	handler := NewUserCommandHandler(eventPublisher)

	// Подписываемся на команды через NATS
	subjectResolver := invoke.NewDefaultSubjectResolver("commands", "events")
	if err := natsAdapter.Subscribe(ctx, subjectResolver.ResolveCommandSubject(createCmd()), func(ctx context.Context, msg *transport.Message) error {
		// Десериализуем команду
		var cmd CreateUserCommand
		serializer := invoke.NewJSONSerializer()
		if err := serializer.Deserialize(msg.Data, &cmd); err != nil {
			return fmt.Errorf("failed to deserialize command: %w", err)
		}

		// Извлекаем метаданные из headers
		if correlationID, ok := msg.Headers["correlation_id"]; ok {
			ctx = invoke.WithCorrelationID(ctx, correlationID)
		}
		if commandID, ok := msg.Headers["command_id"]; ok {
			ctx = invoke.WithCommandID(ctx, commandID)
		}

		// Обрабатываем команду
		return handler.Handle(ctx, cmd)
	}); err != nil {
		fmt.Printf("Failed to subscribe to commands: %v\n", err)
		return
	}

	// Подписываемся на события в EventBus (для EventAwaiter)
	_ = eventBus.Subscribe("user.created", &eventHandlerWrapper{eventBus: eventBus})
	_ = eventBus.Subscribe("user.creation_failed", &eventHandlerWrapper{eventBus: eventBus})

	// Подписываем EventBus на события из NATS (для демонстрации)
	// В реальном приложении события могут публиковаться напрямую в NATS
	// и затем подписываться через transport.Subscriber

	// 7. Отправка команды через Invoke() и получение события
	fmt.Println("=== Успешный сценарий ===")
	cmd := CreateUserCommand{
		Email:    "user@example.com",
		Name:     "John Doe",
		Password: "secret123",
	}

	event, err := invoker.Invoke(ctx, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("User created successfully: %+v\n", event)
	}

	// 8. Обработка ошибочного сценария
	fmt.Println("\n=== Ошибочный сценарий ===")
	invalidCmd := CreateUserCommand{
		Email: "", // пустой email
		Name:  "Jane Doe",
	}

	_, err = invoker.Invoke(ctx, invalidCmd)
	if err != nil {
		fmt.Printf("Expected error received: %v\n", err)
	} else {
		fmt.Println("No error received (unexpected)")
	}

	// 9. Демонстрация InvokeWithBothResults для детального анализа
	fmt.Println("\n=== Детальный анализ с InvokeWithBothResults ===")
	successEvent, errorEvent, err := invoker.InvokeWithBothResults(ctx, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		var zeroError UserCreationFailedEvent
		var zeroSuccess UserCreatedEvent
		if !reflect.DeepEqual(errorEvent, zeroError) {
			fmt.Printf("Error event received: %s\n", errorEvent.ErrorMessage())
		} else if !reflect.DeepEqual(successEvent, zeroSuccess) {
			fmt.Printf("Success event received: UserID=%s\n", successEvent.UserID)
		}
	}

	// 10. Graceful shutdown
	fmt.Println("\n=== Graceful shutdown ===")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := awaiter.Stop(shutdownCtx); err != nil {
		fmt.Printf("Error stopping awaiter: %v\n", err)
	}

	// Output:
	// === Успешный сценарий ===
	// User created successfully: &{BaseEvent:... UserID:user-... Email:user@example.com Name:John Doe}
	// === Ошибочный сценарий ===
	// Expected error received: ...
	// === Детальный анализ с InvokeWithBothResults ===
	// Success event received: UserID=user-...
	// === Graceful shutdown ===
}

// Вспомогательная функция для создания команды (для получения subject)
func createCmd() CreateUserCommand {
	return CreateUserCommand{}
}

// eventHandlerWrapper обертка для обработки событий из NATS в EventBus
type eventHandlerWrapper struct {
	eventBus events.EventBus
}

func (h *eventHandlerWrapper) Handle(ctx context.Context, event events.Event) error {
	// События уже обрабатываются EventBus, просто подтверждаем получение
	return nil
}

func (h *eventHandlerWrapper) EventType() string {
	return "" // используется для подписки на конкретный тип
}

