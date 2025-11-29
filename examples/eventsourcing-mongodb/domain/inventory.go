package domain

import (
	"errors"
	"potter/framework/events"
	"potter/framework/eventsourcing"
)

// Inventory Event Sourced агрегат для инвентаря
type Inventory struct {
	*eventsourcing.EventSourcedAggregate
	productID  string
	warehouseID string
	quantity   int
}

// NewInventory создает новый инвентарь
func NewInventory(inventoryID, productID, warehouseID string, quantity int) *Inventory {
	inventory := &Inventory{
		EventSourcedAggregate: eventsourcing.NewEventSourcedAggregate(inventoryID),
	}
	inventory.SetApplier(inventory)
	event := &ItemAddedEvent{
		BaseEvent:   events.NewBaseEvent("inventory.item.added", inventoryID),
		ProductID:   productID,
		WarehouseID:  warehouseID,
		Quantity:    quantity,
	}
	inventory.RaiseEvent(event)
	return inventory
}

// Reserve резервирует товар
func (i *Inventory) Reserve(quantity int) error {
	if i.quantity < quantity {
		return errors.New("insufficient stock")
	}
	event := &ItemReservedEvent{
		BaseEvent: events.NewBaseEvent("inventory.item.reserved", i.ID()),
		Quantity:  quantity,
	}
	i.RaiseEvent(event)
	return nil
}

// Apply применяет события
func (i *Inventory) Apply(event events.Event) error {
	switch e := event.(type) {
	case *ItemAddedEvent:
		i.productID = e.ProductID
		i.warehouseID = e.WarehouseID
		i.quantity += e.Quantity
	case *ItemReservedEvent:
		i.quantity -= e.Quantity
	}
	return nil
}

// GetQuantity возвращает количество
func (i *Inventory) GetQuantity() int {
	return i.quantity
}

