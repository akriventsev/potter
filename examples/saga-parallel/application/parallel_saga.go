package application

import (
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/saga"
)

// NewParallelSagaDefinition создает новое определение саги с параллельными шагами
func NewParallelSagaDefinition(
	asyncCommandBus *invoke.AsyncCommandBus,
	eventAwaiter *invoke.EventAwaiter,
) saga.SagaDefinition {
	builder := saga.NewSagaBuilder("parallel_order_saga")

	// Создаем отдельные шаги
	checkCreditStep := NewCheckCreditStep(asyncCommandBus, eventAwaiter)
	reserveInventoryStep := NewReserveInventoryStep(asyncCommandBus, eventAwaiter)
	calculateShippingStep := NewCalculateShippingStep(asyncCommandBus, eventAwaiter)

	// Создаем параллельный шаг, который выполняет все три операции одновременно
	parallelStep := saga.NewParallelStep(
		"parallel_checks",
		checkCreditStep,
		reserveInventoryStep,
		calculateShippingStep,
	)

	// Добавляем параллельный шаг в сагу
	builder.AddStep(parallelStep)

	// Устанавливаем общий timeout (5 минут)
	builder.WithTimeout(5 * 60 * time.Second)

	definition, err := builder.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build parallel saga: %v", err))
	}

	return definition
}

