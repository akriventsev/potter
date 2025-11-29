package projections

import (
	"context"
	"encoding/json"
	"fmt"
	"potter/framework/eventsourcing"
)

// OrderSummaryProjection проекция для сводки заказов
type OrderSummaryProjection struct {
	orders map[string]*OrderSummary
}

// OrderSummary сводка по заказу
type OrderSummary struct {
	OrderID    string  `json:"order_id"`
	CustomerID string  `json:"customer_id"`
	Amount     float64 `json:"amount"`
	Status     string  `json:"status"`
	EventCount int     `json:"event_count"`
}

// NewOrderSummaryProjection создает новую проекцию
func NewOrderSummaryProjection() *OrderSummaryProjection {
	return &OrderSummaryProjection{
		orders: make(map[string]*OrderSummary),
	}
}

// HandleEvent обрабатывает событие для обновления проекции
func (p *OrderSummaryProjection) HandleEvent(ctx context.Context, event eventsourcing.StoredEvent) error {
	orderID := event.AggregateID

	summary, exists := p.orders[orderID]
	if !exists {
		summary = &OrderSummary{
			OrderID:    orderID,
			EventCount: 0,
		}
		p.orders[orderID] = summary
	}

	summary.EventCount++

	// Парсим данные события (EventData уже является events.Event)
	// Используем метаданные или сериализуем событие обратно в JSON для парсинга
	eventJSON, err := json.Marshal(event.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}
	var eventData map[string]interface{}
	if err := json.Unmarshal(eventJSON, &eventData); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	switch event.EventType {
	case "order.created":
		if customerID, ok := eventData["customer_id"].(string); ok {
			summary.CustomerID = customerID
		}
		if amount, ok := eventData["amount"].(float64); ok {
			summary.Amount = amount
		}
		summary.Status = "created"
	case "order.completed":
		summary.Status = "completed"
	case "order.cancelled":
		summary.Status = "cancelled"
	}

	return nil
}

// GetOrderSummary возвращает сводку по заказу
func (p *OrderSummaryProjection) GetOrderSummary(orderID string) (*OrderSummary, bool) {
	summary, exists := p.orders[orderID]
	return summary, exists
}

// GetAllSummaries возвращает все сводки
func (p *OrderSummaryProjection) GetAllSummaries() map[string]*OrderSummary {
	return p.orders
}
