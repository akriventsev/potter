// Package saga предоставляет адаптеры для интеграции Saga с существующими компонентами фреймворка.
package saga

import (
	"context"
	"fmt"

	"potter/framework/events"
	"potter/framework/invoke"
	"potter/framework/transport"
)

// CommandBusAdapter адаптер для выполнения команд через CommandBus в SagaStep
type CommandBusAdapter struct {
	commandBus transport.CommandBus
	invoker    *invoke.CommandInvoker[transport.Command, events.Event, invoke.ErrorEvent]
}

// NewCommandBusAdapter создает новый адаптер CommandBus
func NewCommandBusAdapter(commandBus transport.CommandBus) *CommandBusAdapter {
	return &CommandBusAdapter{
		commandBus: commandBus,
	}
}

// ExecuteCommand выполняет команду через CommandBus
func (a *CommandBusAdapter) ExecuteCommand(ctx context.Context, cmd transport.Command, sagaCtx SagaContext) error {
	// Добавляем correlation ID в контекст
	if correlationID := sagaCtx.CorrelationID(); correlationID != "" {
		ctx = invoke.WithCorrelationID(ctx, correlationID)
	}

	return a.commandBus.Send(ctx, cmd)
}

// EventBusAdapter адаптер для публикации saga events через EventBus
type EventBusAdapter struct {
	eventBus events.EventBus
}

// NewEventBusAdapter создает новый адаптер EventBus
func NewEventBusAdapter(eventBus events.EventBus) *EventBusAdapter {
	return &EventBusAdapter{
		eventBus: eventBus,
	}
}

// PublishEvent публикует событие через EventBus
func (a *EventBusAdapter) PublishEvent(ctx context.Context, event events.Event, sagaCtx SagaContext) error {
	// Добавляем correlation ID в событие
	if correlationID := sagaCtx.CorrelationID(); correlationID != "" {
		if baseEvent, ok := event.(*events.BaseEvent); ok {
			baseEvent.WithCorrelationID(correlationID)
		}
	}

	return a.eventBus.Publish(ctx, event)
}

// SubscribeToEvents подписывается на события для координации между сагами
func (a *EventBusAdapter) SubscribeToEvents(eventType string, handler events.EventHandler) error {
	return a.eventBus.Subscribe(eventType, handler)
}

// TwoPhaseCommitAdapter адаптер для интеграции 2PC координатора как SagaStep
type TwoPhaseCommitAdapter struct {
	coordinator TwoPhaseCommitCoordinator
}

// NewTwoPhaseCommitAdapter создает новый адаптер 2PC
func NewTwoPhaseCommitAdapter(coordinator TwoPhaseCommitCoordinator) *TwoPhaseCommitAdapter {
	return &TwoPhaseCommitAdapter{
		coordinator: coordinator,
	}
}

// Execute2PC выполняет 2PC транзакцию через координатор
func (a *TwoPhaseCommitAdapter) Execute2PC(
	ctx context.Context,
	transactionID string,
	participants []TwoPhaseCommitParticipant,
	sagaCtx SagaContext,
) error {
	return a.coordinator.Execute(ctx, transactionID, participants)
}

// SagaCommandHandler generic command handler для запуска саг через CommandBus
type SagaCommandHandler struct {
	orchestrator SagaOrchestrator
	registry     *SagaRegistry
}

// NewSagaCommandHandler создает новый command handler для саг
func NewSagaCommandHandler(orchestrator SagaOrchestrator, registry *SagaRegistry) *SagaCommandHandler {
	return &SagaCommandHandler{
		orchestrator: orchestrator,
		registry:     registry,
	}
}

// Handle обрабатывает команду запуска саги
func (h *SagaCommandHandler) Handle(ctx context.Context, cmd transport.Command) error {
	// Извлекаем имя саги из команды
	// В реальности команда должна содержать информацию о саге
	sagaName := cmd.CommandName()
	
	// Получаем definition из registry
	definition, err := h.registry.GetSaga(sagaName)
	if err != nil {
		return fmt.Errorf("failed to get saga definition: %w", err)
	}

	// Создаем контекст саги
	sagaCtx := NewSagaContext()
	if correlationID := invoke.ExtractCorrelationID(ctx); correlationID != "" {
		sagaCtx.SetCorrelationID(correlationID)
	}

	// Создаем instance саги
	instance := definition.CreateInstance(ctx, sagaCtx)

	// Запускаем выполнение через orchestrator
	return h.orchestrator.Execute(ctx, instance)
}

// CommandName возвращает имя команды
func (h *SagaCommandHandler) CommandName() string {
	return "StartSaga"
}

// SagaQueryHandler для получения статуса и истории саг через CQRS QueryBus.
// 
// Пример использования:
//   queryHandler := saga.NewSagaQueryHandler(persistence, readModelStore)
//   queryBus.RegisterHandler("GetSagaStatus", queryHandler)
//   
//   query := &saga.GetSagaStatusQuery{SagaID: "saga-123"}
//   result, err := queryBus.Send(ctx, query)
//   status := result.(*saga.SagaStatusResponse)
// 
// См. framework/saga/query_handler.go для полного API.

