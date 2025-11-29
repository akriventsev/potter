package domain

import "potter/framework/events"

// OrderCreatedEvent событие создания заказа
type OrderCreatedEvent struct {
	*events.BaseEvent
	CustomerID string  `json:"customer_id"`
	Amount     float64 `json:"amount"`
}

// OrderCompletedEvent событие завершения заказа
type OrderCompletedEvent struct {
	*events.BaseEvent
	OrderID string `json:"order_id"`
}

// OrderCancelledEvent событие отмены заказа
type OrderCancelledEvent struct {
	*events.BaseEvent
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}
