package application

import (
	"fmt"
	"time"

	"potter/examples/saga-order/domain"
	"potter/framework/events"
	"potter/framework/eventsourcing"
	"potter/framework/invoke"
	"potter/framework/saga"
)

// NewOrderSagaDefinition создает новое определение саги заказа
func NewOrderSagaDefinition(
	asyncCommandBus *invoke.AsyncCommandBus,
	eventAwaiter *invoke.EventAwaiter,
	eventBus events.EventBus,
	orderRepo *eventsourcing.EventSourcedRepository[*domain.Order],
) saga.SagaDefinition {
	builder := saga.NewSagaBuilder("order_saga")

	// Добавляем шаги
	builder.AddStep(NewReserveInventoryStep(asyncCommandBus, eventAwaiter))
	builder.AddStep(NewProcessPaymentStep(asyncCommandBus, eventAwaiter))
	builder.AddStep(NewCreateShipmentStep(asyncCommandBus, eventAwaiter))
	builder.AddStep(NewCompleteOrderStep(orderRepo, eventBus))

	// Устанавливаем общий timeout (5 минут)
	builder.WithTimeout(5 * 60 * time.Second)

	definition, err := builder.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build order saga: %v", err))
	}

	return definition
}

