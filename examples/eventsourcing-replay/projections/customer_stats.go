package projections

import (
	"context"
	"encoding/json"
	"fmt"
	"potter/framework/eventsourcing"
)

// CustomerStatsProjection проекция для статистики клиентов
type CustomerStatsProjection struct {
	customers map[string]*CustomerStats
}

// CustomerStats статистика по клиенту
type CustomerStats struct {
	CustomerID    string  `json:"customer_id"`
	TotalOrders   int     `json:"total_orders"`
	TotalAmount   float64 `json:"total_amount"`
	AverageAmount float64 `json:"average_amount"`
}

// NewCustomerStatsProjection создает новую проекцию
func NewCustomerStatsProjection() *CustomerStatsProjection {
	return &CustomerStatsProjection{
		customers: make(map[string]*CustomerStats),
	}
}

// HandleEvent обрабатывает событие для обновления проекции
func (p *CustomerStatsProjection) HandleEvent(ctx context.Context, event eventsourcing.StoredEvent) error {
	// Парсим данные события (EventData уже является events.Event)
	eventJSON, err := json.Marshal(event.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}
	var eventData map[string]interface{}
	if err := json.Unmarshal(eventJSON, &eventData); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	if event.EventType == "order.created" {
		customerID, ok := eventData["customer_id"].(string)
		if !ok {
			return nil
		}

		stats, exists := p.customers[customerID]
		if !exists {
			stats = &CustomerStats{
				CustomerID: customerID,
			}
			p.customers[customerID] = stats
		}

		stats.TotalOrders++
		if amount, ok := eventData["amount"].(float64); ok {
			stats.TotalAmount += amount
			stats.AverageAmount = stats.TotalAmount / float64(stats.TotalOrders)
		}
	}

	return nil
}

// GetCustomerStats возвращает статистику по клиенту
func (p *CustomerStatsProjection) GetCustomerStats(customerID string) (*CustomerStats, bool) {
	stats, exists := p.customers[customerID]
	return stats, exists
}

// GetAllStats возвращает всю статистику
func (p *CustomerStatsProjection) GetAllStats() map[string]*CustomerStats {
	return p.customers
}

