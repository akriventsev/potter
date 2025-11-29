package application

import (
	"context"
	"fmt"
	"time"

	"github.com/akriventsev/potter/examples/saga-parallel/domain"
	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/saga"
	"github.com/akriventsev/potter/framework/transport"
)

// CheckCreditStep шаг проверки кредита
type CheckCreditStep struct {
	*saga.BaseStep
	asyncCommandBus *invoke.AsyncCommandBus
	eventAwaiter    *invoke.EventAwaiter
	invoker         *invoke.CommandInvoker[*CheckCreditCommand, *domain.CreditCheckCompletedEvent, *domain.CreditCheckFailedEvent]
}

// NewCheckCreditStep создает новый шаг проверки кредита
func NewCheckCreditStep(asyncCommandBus *invoke.AsyncCommandBus, eventAwaiter *invoke.EventAwaiter) *CheckCreditStep {
	step := &CheckCreditStep{
		BaseStep:        saga.NewBaseStep("check_credit"),
		asyncCommandBus: asyncCommandBus,
		eventAwaiter:    eventAwaiter,
	}

	step.invoker = invoke.NewCommandInvoker[*CheckCreditCommand, *domain.CreditCheckCompletedEvent, *domain.CreditCheckFailedEvent](
		asyncCommandBus,
		eventAwaiter,
		"credit.check.completed",
		"credit.check.failed",
	).WithTimeout(30 * time.Second)

	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		customerID := sagaCtx.GetString("customer_id")
		amount := sagaCtx.GetFloat64("amount")

		cmd := &CheckCreditCommand{
			BaseCommand: transport.NewBaseCommandSimple("check_credit", customerID),
			CustomerID:  customerID,
			Amount:      amount,
		}

		event, err := step.invoker.Invoke(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to check credit: %w", err)
		}

		sagaCtx.Set("credit_approved", event.Approved)
		sagaCtx.Set("credit_limit", event.Limit)

		return nil
	})

	return step
}

// ReserveInventoryStep шаг резервирования товара
type ReserveInventoryStep struct {
	*saga.BaseStep
	asyncCommandBus *invoke.AsyncCommandBus
	eventAwaiter    *invoke.EventAwaiter
	invoker         *invoke.CommandInvoker[*ReserveInventoryCommand, *domain.InventoryReservedEvent, *domain.ReservationFailedEvent]
}

// NewReserveInventoryStep создает новый шаг резервирования товара
func NewReserveInventoryStep(asyncCommandBus *invoke.AsyncCommandBus, eventAwaiter *invoke.EventAwaiter) *ReserveInventoryStep {
	step := &ReserveInventoryStep{
		BaseStep:        saga.NewBaseStep("reserve_inventory"),
		asyncCommandBus: asyncCommandBus,
		eventAwaiter:    eventAwaiter,
	}

	step.invoker = invoke.NewCommandInvoker[*ReserveInventoryCommand, *domain.InventoryReservedEvent, *domain.ReservationFailedEvent](
		asyncCommandBus,
		eventAwaiter,
		"inventory.reserved",
		"reservation.failed",
	).WithTimeout(30 * time.Second)

	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		items := sagaCtx.Get("items")
		if items == nil {
			return fmt.Errorf("items not found in saga context")
		}

		cmd := &ReserveInventoryCommand{
			BaseCommand: transport.NewBaseCommandSimple("reserve_inventory", orderID),
			OrderID:     orderID,
			Items:       items.([]domain.OrderItem),
		}

		event, err := step.invoker.Invoke(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to reserve inventory: %w", err)
		}

		sagaCtx.Set("inventory_reserved", true)
		sagaCtx.Set("reservation_id", event.ReservationID)

		return nil
	})

	return step
}

// CalculateShippingStep шаг расчета доставки
type CalculateShippingStep struct {
	*saga.BaseStep
	asyncCommandBus *invoke.AsyncCommandBus
	eventAwaiter    *invoke.EventAwaiter
	invoker         *invoke.CommandInvoker[*CalculateShippingCommand, *domain.ShippingCalculatedEvent, *domain.ShippingCalculationFailedEvent]
}

// NewCalculateShippingStep создает новый шаг расчета доставки
func NewCalculateShippingStep(asyncCommandBus *invoke.AsyncCommandBus, eventAwaiter *invoke.EventAwaiter) *CalculateShippingStep {
	step := &CalculateShippingStep{
		BaseStep:        saga.NewBaseStep("calculate_shipping"),
		asyncCommandBus: asyncCommandBus,
		eventAwaiter:    eventAwaiter,
	}

	step.invoker = invoke.NewCommandInvoker[*CalculateShippingCommand, *domain.ShippingCalculatedEvent, *domain.ShippingCalculationFailedEvent](
		asyncCommandBus,
		eventAwaiter,
		"shipping.calculated",
		"shipping.calculation.failed",
	).WithTimeout(30 * time.Second)

	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		customerID := sagaCtx.GetString("customer_id")
		items := sagaCtx.Get("items")
		if items == nil {
			return fmt.Errorf("items not found in saga context")
		}

		cmd := &CalculateShippingCommand{
			BaseCommand: transport.NewBaseCommandSimple("calculate_shipping", orderID),
			OrderID:     orderID,
			CustomerID:  customerID,
			Items:       items.([]domain.OrderItem),
		}

		event, err := step.invoker.Invoke(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to calculate shipping: %w", err)
		}

		sagaCtx.Set("shipping_cost", event.ShippingCost)
		sagaCtx.Set("estimated_days", event.EstimatedDays)

		return nil
	})

	return step
}

