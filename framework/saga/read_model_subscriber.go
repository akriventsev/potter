// Package saga предоставляет механизмы для работы с сагами.
package saga

import (
	"context"
	"fmt"

	"github.com/akriventsev/potter/framework/events"
)

// SagaReadModelSubscriber подписчик на события саги для обновления read model
type SagaReadModelSubscriber struct {
	projection *SagaReadModelProjection
}

// NewSagaReadModelSubscriber создает новый подписчик на события саги
func NewSagaReadModelSubscriber(projection *SagaReadModelProjection) *SagaReadModelSubscriber {
	return &SagaReadModelSubscriber{
		projection: projection,
	}
}

// EventType возвращает тип события, которое обрабатывает подписчик
func (s *SagaReadModelSubscriber) EventType() string {
	return "SagaEvent" // общий тип для всех событий саги
}

// Handle обрабатывает события саги
func (s *SagaReadModelSubscriber) Handle(ctx context.Context, event events.Event) error {
	switch e := event.(type) {
	case *SagaStartedEvent:
		return s.projection.HandleSagaStarted(ctx, e)
	case *StepStartedEvent:
		return s.projection.HandleStepStarted(ctx, e)
	case *StepCompletedEvent:
		return s.projection.HandleStepCompleted(ctx, e)
	case *StepFailedEvent:
		return s.projection.HandleStepFailed(ctx, e)
	case *SagaCompletedEvent:
		return s.projection.HandleSagaCompleted(ctx, e)
	case *SagaFailedEvent:
		return s.projection.HandleSagaFailed(ctx, e)
	default:
		// Игнорируем неизвестные события
		return nil
	}
}

// EventTypes возвращает типы событий, на которые подписывается подписчик
func (s *SagaReadModelSubscriber) EventTypes() []string {
	return []string{
		"SagaStarted",
		"StepStarted",
		"StepCompleted",
		"StepFailed",
		"SagaCompleted",
		"SagaFailed",
	}
}

// RegisterSubscriber регистрирует подписчик на EventBus
func RegisterSagaReadModelSubscriber(eventBus events.EventBus, projection *SagaReadModelProjection) error {
	subscriber := NewSagaReadModelSubscriber(projection)
	
	// Подписываемся на все типы событий саги
	eventTypes := subscriber.EventTypes()
	for _, eventType := range eventTypes {
		if err := eventBus.Subscribe(eventType, subscriber); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", eventType, err)
		}
	}
	
	return nil
}

