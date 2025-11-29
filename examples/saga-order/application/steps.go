package application

import (
	"context"
	"fmt"
	"time"

	"github.com/akriventsev/potter/examples/saga-order/domain"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/eventsourcing"
	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/saga"
	"github.com/akriventsev/potter/framework/transport"
)

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

	// Создаем invoker для типизированных команд и событий
	step.invoker = invoke.NewCommandInvoker[*ReserveInventoryCommand, *domain.InventoryReservedEvent, *domain.ReservationFailedEvent](
		asyncCommandBus,
		eventAwaiter,
		"inventory.reserved",
		"reservation.failed",
	).WithTimeout(30 * time.Second)

	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		if orderID == "" {
			return fmt.Errorf("order_id not found in saga context")
		}

		items := sagaCtx.Get("items")
		if items == nil {
			return fmt.Errorf("items not found in saga context")
		}

		// Создаем команду резервирования
		cmd := &ReserveInventoryCommand{
			BaseCommand: transport.NewBaseCommandSimple("reserve_inventory", orderID),
			OrderID:     orderID,
			Items:       items.([]domain.OrderItem),
		}

		// Отправляем команду и ждем события через invoker
		event, err := step.invoker.Invoke(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to reserve inventory: %w", err)
		}

		// Сохраняем результат в контекст
		sagaCtx.Set("inventory_reserved", true)
		sagaCtx.Set("reservation_id", event.ReservationID)
		sagaCtx.Set("reserved_items", event.Items)

		return nil
	})

	step.WithCompensate(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		reservationID := sagaCtx.GetString("reservation_id")
		if reservationID == "" {
			return nil // Нет что откатывать
		}

		orderID := sagaCtx.GetString("order_id")
		items := sagaCtx.Get("reserved_items")
		if items == nil {
			return nil
		}

		// Создаем команду освобождения резерва
		cmd := &ReleaseInventoryCommand{
			BaseCommand: transport.NewBaseCommandSimple("release_inventory", orderID),
			OrderID:     orderID,
			ReservationID: reservationID,
			Items:       items.([]domain.OrderItem),
		}

		// Отправляем команду асинхронно (без ожидания результата)
		metadata := transport.NewBaseCommandMetadata("", sagaCtx.CorrelationID(), "")
		return step.asyncCommandBus.SendAsync(ctx, cmd, metadata)
	})

	step.WithTimeout(30 * time.Second)
	step.WithRetry(saga.ExponentialBackoff(3, 1*time.Second, 2.0))

	return step
}

// ProcessPaymentStep шаг обработки платежа
type ProcessPaymentStep struct {
	*saga.BaseStep
	asyncCommandBus *invoke.AsyncCommandBus
	eventAwaiter    *invoke.EventAwaiter
	invoker         *invoke.CommandInvoker[*ProcessPaymentCommand, *domain.PaymentProcessedEvent, *domain.PaymentFailedEvent]
}

// NewProcessPaymentStep создает новый шаг обработки платежа
func NewProcessPaymentStep(asyncCommandBus *invoke.AsyncCommandBus, eventAwaiter *invoke.EventAwaiter) *ProcessPaymentStep {
	step := &ProcessPaymentStep{
		BaseStep:        saga.NewBaseStep("process_payment"),
		asyncCommandBus: asyncCommandBus,
		eventAwaiter:    eventAwaiter,
	}

	// Создаем invoker для типизированных команд и событий
	step.invoker = invoke.NewCommandInvoker[*ProcessPaymentCommand, *domain.PaymentProcessedEvent, *domain.PaymentFailedEvent](
		asyncCommandBus,
		eventAwaiter,
		"payment.processed",
		"payment.failed",
	).WithTimeout(30 * time.Second)

	step.WithGuard(func(ctx context.Context, sagaCtx saga.SagaContext) bool {
		// Проверяем, что товар был зарезервирован
		return sagaCtx.GetBool("inventory_reserved")
	})

	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		customerID := sagaCtx.GetString("customer_id")
		items := sagaCtx.Get("items")
		if items == nil {
			return fmt.Errorf("items not found in saga context")
		}

		// Вычисляем сумму
		totalAmount := 0.0
		for _, item := range items.([]domain.OrderItem) {
			totalAmount += item.Price * float64(item.Quantity)
		}

		// Создаем команду обработки платежа
		cmd := &ProcessPaymentCommand{
			BaseCommand: transport.NewBaseCommandSimple("process_payment", orderID),
			OrderID:     orderID,
			CustomerID:  customerID,
			Amount:      totalAmount,
		}

		// Отправляем команду и ждем события через invoker
		event, err := step.invoker.Invoke(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to process payment: %w", err)
		}

		// Сохраняем payment ID
		sagaCtx.Set("payment_id", event.PaymentID)

		return nil
	})

	step.WithCompensate(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		paymentID := sagaCtx.GetString("payment_id")
		if paymentID == "" {
			return nil // Нет что откатывать
		}

		// Создаем команду возврата платежа
		cmd := &RefundPaymentCommand{
			BaseCommand: transport.NewBaseCommandSimple("refund_payment", paymentID),
			PaymentID:   paymentID,
		}

		// Отправляем команду асинхронно
		metadata := transport.NewBaseCommandMetadata("", sagaCtx.CorrelationID(), "")
		return step.asyncCommandBus.SendAsync(ctx, cmd, metadata)
	})

	step.WithTimeout(30 * time.Second)
	step.WithRetry(saga.SimpleRetry(2))

	return step
}

// CreateShipmentStep шаг создания доставки
type CreateShipmentStep struct {
	*saga.BaseStep
	asyncCommandBus *invoke.AsyncCommandBus
	eventAwaiter    *invoke.EventAwaiter
	invoker         *invoke.CommandInvoker[*CreateShipmentCommand, *domain.ShipmentCreatedEvent, *domain.ShipmentFailedEvent]
}

// NewCreateShipmentStep создает новый шаг создания доставки
func NewCreateShipmentStep(asyncCommandBus *invoke.AsyncCommandBus, eventAwaiter *invoke.EventAwaiter) *CreateShipmentStep {
	step := &CreateShipmentStep{
		BaseStep:        saga.NewBaseStep("create_shipment"),
		asyncCommandBus: asyncCommandBus,
		eventAwaiter:    eventAwaiter,
	}

	// Создаем invoker для типизированных команд и событий
	step.invoker = invoke.NewCommandInvoker[*CreateShipmentCommand, *domain.ShipmentCreatedEvent, *domain.ShipmentFailedEvent](
		asyncCommandBus,
		eventAwaiter,
		"shipment.created",
		"shipment.failed",
	).WithTimeout(30 * time.Second)

	step.WithGuard(func(ctx context.Context, sagaCtx saga.SagaContext) bool {
		// Проверяем, что платеж был обработан
		return sagaCtx.GetString("payment_id") != ""
	})

	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		customerID := sagaCtx.GetString("customer_id")
		items := sagaCtx.Get("items")
		if items == nil {
			return fmt.Errorf("items not found in saga context")
		}

		// Создаем команду создания доставки
		cmd := &CreateShipmentCommand{
			BaseCommand: transport.NewBaseCommandSimple("create_shipment", orderID),
			OrderID:     orderID,
			CustomerID:  customerID,
			Items:       items.([]domain.OrderItem),
		}

		// Отправляем команду и ждем события через invoker
		event, err := step.invoker.Invoke(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to create shipment: %w", err)
		}

		// Сохраняем shipment ID
		sagaCtx.Set("shipment_id", event.ShipmentID)

		return nil
	})

	step.WithCompensate(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		shipmentID := sagaCtx.GetString("shipment_id")
		if shipmentID == "" {
			return nil // Нет что откатывать
		}

		// Создаем команду отмены доставки
		cmd := &CancelShipmentCommand{
			BaseCommand: transport.NewBaseCommandSimple("cancel_shipment", shipmentID),
			ShipmentID:  shipmentID,
		}

		// Отправляем команду асинхронно
		metadata := transport.NewBaseCommandMetadata("", sagaCtx.CorrelationID(), "")
		return step.asyncCommandBus.SendAsync(ctx, cmd, metadata)
	})

	step.WithTimeout(30 * time.Second)

	return step
}

// CompleteOrderStep шаг завершения заказа
type CompleteOrderStep struct {
	*saga.BaseStep
	orderRepo *eventsourcing.EventSourcedRepository[*domain.Order]
	eventBus  events.EventBus
}

// NewCompleteOrderStep создает новый шаг завершения заказа
func NewCompleteOrderStep(orderRepo *eventsourcing.EventSourcedRepository[*domain.Order], eventBus events.EventBus) *CompleteOrderStep {
	step := &CompleteOrderStep{
		BaseStep:  saga.NewBaseStep("complete_order"),
		orderRepo:  orderRepo,
		eventBus:  eventBus,
	}

	step.WithGuard(func(ctx context.Context, sagaCtx saga.SagaContext) bool {
		// Проверяем, что shipment был создан
		return sagaCtx.GetString("shipment_id") != ""
	})

	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		if orderID == "" {
			return fmt.Errorf("order_id not found in saga context")
		}

		paymentID := sagaCtx.GetString("payment_id")
		shipmentID := sagaCtx.GetString("shipment_id")

		// Загружаем заказ из репозитория
		order, err := step.orderRepo.GetByID(ctx, orderID)
		if err != nil {
			return fmt.Errorf("failed to load order: %w", err)
		}

		// Подтверждаем платеж
		if err := order.ConfirmPayment(paymentID); err != nil {
			return fmt.Errorf("failed to confirm payment: %w", err)
		}

		// Создаем доставку
		if err := order.CreateShipment(shipmentID); err != nil {
			return fmt.Errorf("failed to create shipment: %w", err)
		}

		// Завершаем заказ
		if err := order.Complete(); err != nil {
			return fmt.Errorf("failed to complete order: %w", err)
		}

		// Сохраняем заказ
		if err := step.orderRepo.Save(ctx, order); err != nil {
			return fmt.Errorf("failed to save order: %w", err)
		}

		// Публикуем событие завершения заказа
		event := &domain.OrderCompletedEvent{
			BaseEvent: events.NewBaseEvent("order.completed", orderID),
			OrderID:   orderID,
		}
		event.WithCorrelationID(sagaCtx.CorrelationID())

		return step.eventBus.Publish(ctx, event)
	})

	step.WithCompensate(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		if orderID == "" {
			return nil
		}

		// Загружаем заказ
		order, err := step.orderRepo.GetByID(ctx, orderID)
		if err != nil {
			return fmt.Errorf("failed to load order for compensation: %w", err)
		}

		// Отменяем заказ
		if err := order.Cancel(); err != nil {
			return fmt.Errorf("failed to cancel order: %w", err)
		}

		// Сохраняем заказ
		return step.orderRepo.Save(ctx, order)
	})

	return step
}
