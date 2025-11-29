package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/eventsourcing"
)

// OrderStatus статус заказа
type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "pending"
	OrderStatusReserved   OrderStatus = "reserved"
	OrderStatusPaid       OrderStatus = "paid"
	OrderStatusShipped    OrderStatus = "shipped"
	OrderStatusCompleted  OrderStatus = "completed"
	OrderStatusCancelled  OrderStatus = "cancelled"
)

// OrderItem элемент заказа
type OrderItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

// Order доменная сущность заказа (EventSourced)
type Order struct {
	*eventsourcing.EventSourcedAggregate
	customerID  string
	items       []OrderItem
	status      OrderStatus
	paymentID   string
	shipmentID  string
	totalAmount float64
	createdAt   time.Time
	updatedAt   time.Time
}

// NewOrder создает новый заказ
func NewOrder(customerID string, items []OrderItem) *Order {
	orderID := uuid.New().String()
	
	// Вычисляем общую сумму
	totalAmount := 0.0
	for _, item := range items {
		totalAmount += item.Price * float64(item.Quantity)
	}

	order := &Order{
		EventSourcedAggregate: eventsourcing.NewEventSourcedAggregateWithApplier(orderID, nil),
		customerID:             customerID,
		items:                  items,
		status:                 OrderStatusPending,
		totalAmount:            totalAmount,
		createdAt:              time.Now(),
		updatedAt:              time.Now(),
	}
	
	// Устанавливаем applier после создания
	order.SetApplier(order)

	// Создаем событие создания заказа
	event := &OrderCreatedEvent{
		BaseEvent:  events.NewBaseEvent("order.created", orderID),
		CustomerID: customerID,
		Items:      items,
		TotalAmount: totalAmount,
	}
	
	order.RaiseEvent(event)

	return order
}

// NewOrderWithID создает заказ с указанным ID (для восстановления из БД)
func NewOrderWithID(orderID string) *Order {
	order := &Order{
		EventSourcedAggregate: eventsourcing.NewEventSourcedAggregateWithApplier(orderID, nil),
		status:                 OrderStatusPending,
		createdAt:              time.Now(),
		updatedAt:              time.Now(),
	}
	order.SetApplier(order)
	return order
}

// CustomerID возвращает ID клиента
func (o *Order) CustomerID() string {
	return o.customerID
}

// Items возвращает элементы заказа
func (o *Order) Items() []OrderItem {
	return o.items
}

// Status возвращает статус заказа
func (o *Order) Status() OrderStatus {
	return o.status
}

// PaymentID возвращает ID платежа
func (o *Order) PaymentID() string {
	return o.paymentID
}

// ShipmentID возвращает ID доставки
func (o *Order) ShipmentID() string {
	return o.shipmentID
}

// TotalAmount возвращает общую сумму заказа
func (o *Order) TotalAmount() float64 {
	return o.totalAmount
}

// ReserveInventory резервирует товар на складе
func (o *Order) ReserveInventory(reservationID string) error {
	if o.status != OrderStatusPending {
		return fmt.Errorf("order %s is not in pending status", o.ID())
	}
	
	event := &InventoryReservedEvent{
		BaseEvent:     events.NewBaseEvent("inventory.reserved", o.ID()),
		OrderID:       o.ID(),
		ReservationID: reservationID,
		Items:         o.items,
	}
	
	o.RaiseEvent(event)
	o.updatedAt = time.Now()
	
	return nil
}

// ConfirmPayment подтверждает оплату
func (o *Order) ConfirmPayment(paymentID string) error {
	if o.status != OrderStatusReserved {
		return fmt.Errorf("order %s is not in reserved status", o.ID())
	}
	
	event := &PaymentProcessedEvent{
		BaseEvent: events.NewBaseEvent("payment.processed", o.ID()),
		OrderID:   o.ID(),
		PaymentID: paymentID,
	}
	
	o.RaiseEvent(event)
	o.updatedAt = time.Now()
	
	return nil
}

// CreateShipment создает доставку
func (o *Order) CreateShipment(shipmentID string) error {
	if o.status != OrderStatusPaid {
		return fmt.Errorf("order %s is not in paid status", o.ID())
	}
	
	event := &ShipmentCreatedEvent{
		BaseEvent:  events.NewBaseEvent("shipment.created", o.ID()),
		OrderID:    o.ID(),
		ShipmentID: shipmentID,
	}
	
	o.RaiseEvent(event)
	o.updatedAt = time.Now()
	
	return nil
}

// Complete завершает заказ
func (o *Order) Complete() error {
	if o.status != OrderStatusShipped {
		return fmt.Errorf("order %s is not in shipped status", o.ID())
	}
	
	event := &OrderCompletedEvent{
		BaseEvent: events.NewBaseEvent("order.completed", o.ID()),
		OrderID:   o.ID(),
	}
	
	o.RaiseEvent(event)
	o.updatedAt = time.Now()
	
	return nil
}

// Cancel отменяет заказ
func (o *Order) Cancel() error {
	if o.status == OrderStatusCompleted || o.status == OrderStatusCancelled {
		return fmt.Errorf("order %s cannot be cancelled", o.ID())
	}
	
	event := &OrderCancelledEvent{
		BaseEvent: events.NewBaseEvent("order.cancelled", o.ID()),
		OrderID:   o.ID(),
		Reason:    "manual_cancellation",
	}
	
	o.RaiseEvent(event)
	o.updatedAt = time.Now()
	
	return nil
}

// Apply применяет событие к агрегату (реализация EventApplier)
func (o *Order) Apply(event events.Event) error {
	switch e := event.(type) {
	case *OrderCreatedEvent:
		o.customerID = e.CustomerID
		o.items = e.Items
		o.status = OrderStatusPending
		o.totalAmount = e.TotalAmount
		if o.createdAt.IsZero() {
			o.createdAt = time.Now()
		}
	case *InventoryReservedEvent:
		o.status = OrderStatusReserved
	case *PaymentProcessedEvent:
		o.status = OrderStatusPaid
		o.paymentID = e.PaymentID
	case *ShipmentCreatedEvent:
		o.status = OrderStatusShipped
		o.shipmentID = e.ShipmentID
	case *OrderCompletedEvent:
		o.status = OrderStatusCompleted
	case *OrderCancelledEvent:
		o.status = OrderStatusCancelled
	default:
		return fmt.Errorf("unknown event type: %T", event)
	}
	o.updatedAt = time.Now()
	return nil
}

