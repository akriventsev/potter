package domain

import "github.com/akriventsev/potter/framework/events"

// ProductCreatedEvent событие создания продукта
type ProductCreatedEvent struct {
	*events.BaseEvent
	Name  string
	Price float64
}

// PriceUpdatedEvent событие обновления цены
type PriceUpdatedEvent struct {
	*events.BaseEvent
	NewPrice float64
	OldPrice float64
}

// StockUpdatedEvent событие обновления остатка
type StockUpdatedEvent struct {
	*events.BaseEvent
	Quantity int
}

