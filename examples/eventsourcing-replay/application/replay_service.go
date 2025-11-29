package application

import (
	"context"
	"fmt"
	"potter/examples/eventsourcing-replay/projections"
	"potter/framework/eventsourcing"
)

// ProjectionHandler обработчик событий для проекций
type ProjectionHandler struct {
	OrderSummary  *projections.OrderSummaryProjection
	CustomerStats *projections.CustomerStatsProjection
}

// HandleEvent обрабатывает событие для обновления проекций (реализация ReplayHandler)
func (h *ProjectionHandler) HandleEvent(ctx context.Context, event eventsourcing.StoredEvent) error {
	if h.OrderSummary != nil {
		if err := h.OrderSummary.HandleEvent(ctx, event); err != nil {
			return fmt.Errorf("order summary projection error: %w", err)
		}
	}

	if h.CustomerStats != nil {
		if err := h.CustomerStats.HandleEvent(ctx, event); err != nil {
			return fmt.Errorf("customer stats projection error: %w", err)
		}
	}

	return nil
}
