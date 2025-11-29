package domain

import (
	"potter/framework/events"
	"potter/framework/eventsourcing"
)

// Order Event Sourced агрегат для заказа
type Order struct {
	*eventsourcing.EventSourcedAggregate
	customerID string
	amount     float64
	status     string
}

// NewOrder создает новый заказ
func NewOrder(orderID, customerID string, amount float64) *Order {
	order := &Order{
		EventSourcedAggregate: eventsourcing.NewEventSourcedAggregate(orderID),
	}
	order.SetApplier(order)
	event := &OrderCreatedEvent{
		BaseEvent:  events.NewBaseEvent("order.created", orderID),
		CustomerID: customerID,
		Amount:     amount,
	}
	order.RaiseEvent(event)
	return order
}

// Apply применяет события
func (o *Order) Apply(event events.Event) error {
	switch e := event.(type) {
	case *OrderCreatedEvent:
		o.customerID = e.CustomerID
		o.amount = e.Amount
		o.status = "created"
	}
	return nil
}

// GetCustomerID возвращает ID клиента
func (o *Order) GetCustomerID() string {
	return o.customerID
}

// GetAmount возвращает сумму
func (o *Order) GetAmount() float64 {
	return o.amount
}

