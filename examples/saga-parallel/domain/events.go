package domain

import "potter/framework/events"

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

