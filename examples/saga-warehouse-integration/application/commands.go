package application

import "potter/framework/transport"

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

