package domain

import (
	"errors"
	"potter/framework/events"
	"potter/framework/eventsourcing"
)

// Product Event Sourced агрегат для продукта
type Product struct {
	*eventsourcing.EventSourcedAggregate
	name        string
	price       float64
	description string
	stock       int
}

// NewProduct создает новый продукт
func NewProduct(productID, name string, price float64) *Product {
	product := &Product{
		EventSourcedAggregate: eventsourcing.NewEventSourcedAggregate(productID),
	}
	product.SetApplier(product)
	event := &ProductCreatedEvent{
		BaseEvent: events.NewBaseEvent("product.created", productID),
		Name:      name,
		Price:     price,
	}
	product.RaiseEvent(event)
	return product
}

// UpdatePrice обновляет цену продукта
func (p *Product) UpdatePrice(newPrice float64) error {
	if newPrice <= 0 {
		return errors.New("price must be positive")
	}
	event := &PriceUpdatedEvent{
		BaseEvent: events.NewBaseEvent("product.price.updated", p.ID()),
		NewPrice:   newPrice,
		OldPrice:   p.price,
	}
	p.RaiseEvent(event)
	return nil
}

// UpdateStock обновляет остаток
func (p *Product) UpdateStock(quantity int) error {
	event := &StockUpdatedEvent{
		BaseEvent: events.NewBaseEvent("product.stock.updated", p.ID()),
		Quantity:   quantity,
	}
	p.RaiseEvent(event)
	return nil
}

// Apply применяет события
func (p *Product) Apply(event events.Event) error {
	switch e := event.(type) {
	case *ProductCreatedEvent:
		p.name = e.Name
		p.price = e.Price
		p.stock = 0
	case *PriceUpdatedEvent:
		p.price = e.NewPrice
	case *StockUpdatedEvent:
		p.stock += e.Quantity
	}
	return nil
}

// GetName возвращает название
func (p *Product) GetName() string {
	return p.name
}

// GetPrice возвращает цену
func (p *Product) GetPrice() float64 {
	return p.price
}

// GetStock возвращает остаток
func (p *Product) GetStock() int {
	return p.stock
}

