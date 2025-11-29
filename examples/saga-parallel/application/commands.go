package application

import (
	"potter/examples/saga-parallel/domain"
	"potter/framework/transport"
)

// CheckCreditCommand команда проверки кредита
type CheckCreditCommand struct {
	*transport.BaseCommand
	CustomerID string
	Amount     float64
}

// CommandName возвращает имя команды
func (c *CheckCreditCommand) CommandName() string {
	return "check_credit"
}

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

// CalculateShippingCommand команда расчета доставки
type CalculateShippingCommand struct {
	*transport.BaseCommand
	OrderID    string
	CustomerID string
	Items      []domain.OrderItem
}

// CommandName возвращает имя команды
func (c *CalculateShippingCommand) CommandName() string {
	return "calculate_shipping"
}

