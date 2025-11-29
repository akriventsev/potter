package application

import (
	"github.com/akriventsev/potter/examples/saga-order/domain"
	"github.com/akriventsev/potter/framework/transport"
)

// ReserveInventoryCommand команда резервирования товара
type ReserveInventoryCommand struct {
	*transport.BaseCommand
	OrderID string
	Items   []domain.OrderItem
}

// CommandName возвращает имя команды
func (c *ReserveInventoryCommand) CommandName() string {
	return "reserve_inventory"
}

// ReleaseInventoryCommand команда освобождения резерва
type ReleaseInventoryCommand struct {
	*transport.BaseCommand
	OrderID       string
	ReservationID string
	Items         []domain.OrderItem
}

// CommandName возвращает имя команды
func (c *ReleaseInventoryCommand) CommandName() string {
	return "release_inventory"
}

// ProcessPaymentCommand команда обработки платежа
type ProcessPaymentCommand struct {
	*transport.BaseCommand
	OrderID    string
	CustomerID string
	Amount     float64
}

// CommandName возвращает имя команды
func (c *ProcessPaymentCommand) CommandName() string {
	return "process_payment"
}

// RefundPaymentCommand команда возврата платежа
type RefundPaymentCommand struct {
	*transport.BaseCommand
	PaymentID string
}

// CommandName возвращает имя команды
func (c *RefundPaymentCommand) CommandName() string {
	return "refund_payment"
}

// CreateShipmentCommand команда создания доставки
type CreateShipmentCommand struct {
	*transport.BaseCommand
	OrderID    string
	CustomerID string
	Items      []domain.OrderItem
}

// CommandName возвращает имя команды
func (c *CreateShipmentCommand) CommandName() string {
	return "create_shipment"
}

// CancelShipmentCommand команда отмены доставки
type CancelShipmentCommand struct {
	*transport.BaseCommand
	ShipmentID string
}

// CommandName возвращает имя команды
func (c *CancelShipmentCommand) CommandName() string {
	return "cancel_shipment"
}

