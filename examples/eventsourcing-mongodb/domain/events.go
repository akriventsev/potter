package domain

import "github.com/akriventsev/potter/framework/events"

// ItemAddedEvent событие добавления товара
type ItemAddedEvent struct {
	*events.BaseEvent
	ProductID  string
	WarehouseID string
	Quantity   int
}

// ItemReservedEvent событие резервирования товара
type ItemReservedEvent struct {
	*events.BaseEvent
	Quantity int
}

