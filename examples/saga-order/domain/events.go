package domain

import (
	"fmt"

	"potter/framework/events"
	"potter/framework/invoke"
)

// OrderCreatedEvent событие создания заказа
type OrderCreatedEvent struct {
	*events.BaseEvent
	CustomerID  string
	Items       []OrderItem
	TotalAmount float64
}

// InventoryReservedEvent событие резервирования товара
type InventoryReservedEvent struct {
	*events.BaseEvent
	OrderID       string
	ReservationID string
	Items         []OrderItem
}

// PaymentProcessedEvent событие обработки платежа
type PaymentProcessedEvent struct {
	*events.BaseEvent
	OrderID   string
	PaymentID string
}

// PaymentFailedEvent событие ошибки платежа
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
	return false
}

// OriginalCommand возвращает исходную команду, вызвавшую ошибку
func (e *PaymentFailedEvent) OriginalCommand() interface{} {
	return nil
}

// ShipmentCreatedEvent событие создания доставки
type ShipmentCreatedEvent struct {
	*events.BaseEvent
	OrderID    string
	ShipmentID string
}

// OrderCompletedEvent событие завершения заказа
type OrderCompletedEvent struct {
	*events.BaseEvent
	OrderID string
}

// OrderCancelledEvent событие отмены заказа
type OrderCancelledEvent struct {
	*events.BaseEvent
	OrderID string
	Reason  string
}

// ReservationFailedEvent событие ошибки резервирования
type ReservationFailedEvent struct {
	*events.BaseEvent
	OrderID      string
	errorMessage string
}

// Error возвращает ошибку, связанную с событием
func (e *ReservationFailedEvent) Error() error {
	if e.errorMessage != "" {
		return fmt.Errorf(e.errorMessage)
	}
	return fmt.Errorf("reservation failed for order %s", e.OrderID)
}

// ErrorCode возвращает код ошибки
func (e *ReservationFailedEvent) ErrorCode() string {
	return "RESERVATION_FAILED"
}

// ErrorMessage возвращает сообщение об ошибке
func (e *ReservationFailedEvent) ErrorMessage() string {
	return e.errorMessage
}

// SetErrorMessage устанавливает сообщение об ошибке
func (e *ReservationFailedEvent) SetErrorMessage(msg string) {
	e.errorMessage = msg
}

// IsRetryable указывает, можно ли повторить операцию
func (e *ReservationFailedEvent) IsRetryable() bool {
	return true
}

// OriginalCommand возвращает исходную команду, вызвавшую ошибку
func (e *ReservationFailedEvent) OriginalCommand() interface{} {
	return nil
}

// ShipmentFailedEvent событие ошибки создания доставки
type ShipmentFailedEvent struct {
	*events.BaseEvent
	OrderID      string
	errorMessage string
}

// Error возвращает ошибку, связанную с событием
func (e *ShipmentFailedEvent) Error() error {
	if e.errorMessage != "" {
		return fmt.Errorf(e.errorMessage)
	}
	return fmt.Errorf("shipment failed for order %s", e.OrderID)
}

// ErrorCode возвращает код ошибки
func (e *ShipmentFailedEvent) ErrorCode() string {
	return "SHIPMENT_FAILED"
}

// ErrorMessage возвращает сообщение об ошибке
func (e *ShipmentFailedEvent) ErrorMessage() string {
	return e.errorMessage
}

// SetErrorMessage устанавливает сообщение об ошибке
func (e *ShipmentFailedEvent) SetErrorMessage(msg string) {
	e.errorMessage = msg
}

// IsRetryable указывает, можно ли повторить операцию
func (e *ShipmentFailedEvent) IsRetryable() bool {
	return false
}

// OriginalCommand возвращает исходную команду, вызвавшую ошибку
func (e *ShipmentFailedEvent) OriginalCommand() interface{} {
	return nil
}

// Ensure events implement invoke.ErrorEvent
var _ invoke.ErrorEvent = (*PaymentFailedEvent)(nil)
var _ invoke.ErrorEvent = (*ReservationFailedEvent)(nil)
var _ invoke.ErrorEvent = (*ShipmentFailedEvent)(nil)

