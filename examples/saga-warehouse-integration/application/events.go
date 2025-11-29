package application

import (
	"fmt"

	"potter/framework/events"
	"potter/framework/invoke"
)

// PaymentProcessedEvent событие успешной обработки платежа
type PaymentProcessedEvent struct {
	*events.BaseEvent
	OrderID   string
	PaymentID string
	Amount    float64
}

// PaymentFailedEvent событие ошибки обработки платежа
type PaymentFailedEvent struct {
	*events.BaseEvent
	OrderID      string
	errorMessage string
}

// Error возвращает ошибку, связанную с событием
func (e *PaymentFailedEvent) Error() error {
	if e.errorMessage != "" {
		return fmt.Errorf(e.errorMessage)
	}
	return fmt.Errorf("payment failed for order %s", e.OrderID)
}

// ErrorCode возвращает код ошибки
func (e *PaymentFailedEvent) ErrorCode() string {
	return "PAYMENT_FAILED"
}

// ErrorMessage возвращает сообщение об ошибке
func (e *PaymentFailedEvent) ErrorMessage() string {
	return e.errorMessage
}

// SetErrorMessage устанавливает сообщение об ошибке
func (e *PaymentFailedEvent) SetErrorMessage(msg string) {
	e.errorMessage = msg
}

// IsRetryable указывает, можно ли повторить операцию
func (e *PaymentFailedEvent) IsRetryable() bool {
	return false // Платежи обычно не повторяются автоматически
}

// OriginalCommand возвращает исходную команду, вызвавшую ошибку
func (e *PaymentFailedEvent) OriginalCommand() interface{} {
	return nil
}

// Ensure PaymentFailedEvent implements invoke.ErrorEvent
var _ invoke.ErrorEvent = (*PaymentFailedEvent)(nil)

