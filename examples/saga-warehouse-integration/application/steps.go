package application

import (
	"context"
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/saga"
	"github.com/akriventsev/potter/framework/transport"
)

// NewValidateOrderStep создает новый шаг валидации заказа
func NewValidateOrderStep() saga.SagaStep {
	step := saga.NewBaseStep("validate_order")

	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		items := sagaCtx.Get("items")

		// Валидация заказа
		if orderID == "" {
			return fmt.Errorf("order_id is required")
		}
		if items == nil {
			return fmt.Errorf("items are required")
		}

		// Сохраняем warehouse_ids для использования в 2PC шаге
		warehouseIDs := sagaCtx.Get("warehouse_ids")
		if warehouseIDs == nil {
			// По умолчанию используем все доступные склады
			warehouseIDs = []string{"warehouse-1", "warehouse-2"}
			sagaCtx.Set("warehouse_ids", warehouseIDs)
		}

		return nil
	})

	step.WithTimeout(10 * time.Second)

	return step
}

// ProcessPaymentStep шаг обработки платежа
func NewProcessPaymentStep(asyncCommandBus *invoke.AsyncCommandBus, eventAwaiter *invoke.EventAwaiter) saga.SagaStep {
	step := saga.NewBaseStep("process_payment")

	// Создаем invoker для типизированных команд и событий
	invoker := invoke.NewCommandInvoker[*ProcessPaymentCommand, *PaymentProcessedEvent, *PaymentFailedEvent](
		asyncCommandBus,
		eventAwaiter,
		"payment.processed",
		"payment.failed",
	).WithTimeout(30 * time.Second)

	step.WithGuard(func(ctx context.Context, sagaCtx saga.SagaContext) bool {
		// Проверяем, что 2PC был выполнен успешно
		// Transaction ID устанавливается в TwoPhaseCommitStep
		return sagaCtx.GetString("transaction_id") != ""
	})

	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		orderID := sagaCtx.GetString("order_id")
		customerID := sagaCtx.GetString("customer_id")
		amountVal := sagaCtx.Get("amount")
		var amount float64
		if amountVal != nil {
			switch v := amountVal.(type) {
			case float64:
				amount = v
			case int:
				amount = float64(v)
			case int64:
				amount = float64(v)
			}
		}

		// Создаем команду обработки платежа
		cmd := &ProcessPaymentCommand{
			BaseCommand: transport.NewBaseCommandSimple("process_payment", orderID),
			OrderID:     orderID,
			CustomerID:  customerID,
			Amount:      amount,
		}

		// Отправляем команду и ждем события через invoker
		event, err := invoker.Invoke(ctx, cmd)
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
			return nil
		}

		// Создаем команду возврата платежа
		cmd := &RefundPaymentCommand{
			BaseCommand: transport.NewBaseCommandSimple("refund_payment", paymentID),
			PaymentID:   paymentID,
		}

		// Отправляем команду асинхронно
		metadata := transport.NewBaseCommandMetadata("", sagaCtx.CorrelationID(), "")
		return asyncCommandBus.SendAsync(ctx, cmd, metadata)
	})

	step.WithTimeout(30 * time.Second)

	return step
}

// WarehouseStockParticipant участник 2PC для warehouse stock
type WarehouseStockParticipant struct {
	warehouseID string
	orderID     string
	items       interface{}
	// participant *twopc.StockParticipant // Реальный participant из warehouse (пакет не реализован)
}

// WarehouseStockParticipantAdapter адаптер для WarehouseStockParticipant
type WarehouseStockParticipantAdapter struct {
	*WarehouseStockParticipant
}

// NewWarehouseStockParticipant создает нового участника 2PC для warehouse
func NewWarehouseStockParticipant(warehouseID, orderID string, items interface{}) saga.TwoPhaseCommitParticipant {
	// В реальном приложении здесь нужно создать twopc.StockParticipant
	// с правильными репозиториями из warehouse
	return &WarehouseStockParticipantAdapter{
		WarehouseStockParticipant: &WarehouseStockParticipant{
			warehouseID: warehouseID,
			orderID:     orderID,
			items:       items,
		},
	}
}

// Prepare подготавливает транзакцию
func (a *WarehouseStockParticipantAdapter) Prepare(ctx context.Context, transactionID string) error {
	// Для примера просто возвращаем успех
	// В реальном приложении здесь должна быть логика подготовки транзакции
	return nil
}

// Commit подтверждает транзакцию
func (a *WarehouseStockParticipantAdapter) Commit(ctx context.Context, transactionID string) error {
	// Для примера просто возвращаем успех
	// В реальном приложении здесь должна быть логика подтверждения транзакции
	return nil
}

// Abort отменяет транзакцию
func (a *WarehouseStockParticipantAdapter) Abort(ctx context.Context, transactionID string) error {
	// Для примера просто возвращаем успех
	// В реальном приложении здесь должна быть логика отмены транзакции
	return nil
}

