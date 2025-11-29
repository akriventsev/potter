package domain

import (
	"errors"

	"potter/framework/events"
	"potter/framework/invoke"
)

// CreditCheckCompletedEvent событие завершения проверки кредита
type CreditCheckCompletedEvent struct {
	*events.BaseEvent
	CustomerID string
	Approved   bool
	Limit      float64
}

// CreditCheckFailedEvent событие ошибки проверки кредита
type CreditCheckFailedEvent struct {
	*events.BaseEvent
	CustomerID string
	Reason     string
}

// Error возвращает ошибку, связанную с событием
func (c *CreditCheckFailedEvent) Error() error {
	return errors.New(c.Reason)
}

// ErrorCode возвращает код ошибки
func (c *CreditCheckFailedEvent) ErrorCode() string {
	return "CREDIT_CHECK_FAILED"
}

// ErrorMessage возвращает сообщение об ошибке
func (c *CreditCheckFailedEvent) ErrorMessage() string {
	return c.Reason
}

// IsRetryable указывает, можно ли повторить операцию
func (c *CreditCheckFailedEvent) IsRetryable() bool {
	return false
}

// OriginalCommand возвращает исходную команду, вызвавшую ошибку
func (c *CreditCheckFailedEvent) OriginalCommand() interface{} {
	return nil
}

// Ensure CreditCheckFailedEvent implements invoke.ErrorEvent
var _ invoke.ErrorEvent = (*CreditCheckFailedEvent)(nil)

// InventoryReservedEvent событие резервирования товара
type InventoryReservedEvent struct {
	*events.BaseEvent
	OrderID       string
	ReservationID string
	Items         []OrderItem
}

// ReservationFailedEvent событие ошибки резервирования
type ReservationFailedEvent struct {
	*events.BaseEvent
	OrderID string
	Reason  string
}

// Error возвращает ошибку, связанную с событием
func (r *ReservationFailedEvent) Error() error {
	return errors.New(r.Reason)
}

// ErrorCode возвращает код ошибки
func (r *ReservationFailedEvent) ErrorCode() string {
	return "RESERVATION_FAILED"
}

// ErrorMessage возвращает сообщение об ошибке
func (r *ReservationFailedEvent) ErrorMessage() string {
	return r.Reason
}

// IsRetryable указывает, можно ли повторить операцию
func (r *ReservationFailedEvent) IsRetryable() bool {
	return true
}

// OriginalCommand возвращает исходную команду, вызвавшую ошибку
func (r *ReservationFailedEvent) OriginalCommand() interface{} {
	return nil
}

// Ensure ReservationFailedEvent implements invoke.ErrorEvent
var _ invoke.ErrorEvent = (*ReservationFailedEvent)(nil)

// ShippingCalculatedEvent событие расчета доставки
type ShippingCalculatedEvent struct {
	*events.BaseEvent
	OrderID     string
	ShippingCost float64
	EstimatedDays int
}

// ShippingCalculationFailedEvent событие ошибки расчета доставки
type ShippingCalculationFailedEvent struct {
	*events.BaseEvent
	OrderID string
	Reason  string
}

// Error возвращает ошибку, связанную с событием
func (s *ShippingCalculationFailedEvent) Error() error {
	return errors.New(s.Reason)
}

// ErrorCode возвращает код ошибки
func (s *ShippingCalculationFailedEvent) ErrorCode() string {
	return "SHIPPING_CALCULATION_FAILED"
}

// ErrorMessage возвращает сообщение об ошибке
func (s *ShippingCalculationFailedEvent) ErrorMessage() string {
	return s.Reason
}

// IsRetryable указывает, можно ли повторить операцию
func (s *ShippingCalculationFailedEvent) IsRetryable() bool {
	return true
}

// OriginalCommand возвращает исходную команду, вызвавшую ошибку
func (s *ShippingCalculationFailedEvent) OriginalCommand() interface{} {
	return nil
}

// Ensure ShippingCalculationFailedEvent implements invoke.ErrorEvent
var _ invoke.ErrorEvent = (*ShippingCalculationFailedEvent)(nil)

