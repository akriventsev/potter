package eventsourcing

import (
	"context"
	"fmt"
	"time"
)

// ReplayHandler интерфейс для обработчиков replay событий
type ReplayHandler interface {
	// HandleEvent обрабатывает событие при replay
	HandleEvent(ctx context.Context, event StoredEvent) error
}

// ReplayProgress содержит информацию о прогрессе replay
type ReplayProgress struct {
	ProcessedEvents int64
	TotalEvents     int64
	CurrentPosition int64
	StartTime       time.Time
	ElapsedTime     time.Duration
}

// ReplayOptions опции для replay операций
type ReplayOptions struct {
	BatchSize   int
	Parallel    bool
	StopOnError bool
}

// DefaultReplayOptions возвращает опции по умолчанию
func DefaultReplayOptions() ReplayOptions {
	return ReplayOptions{
		BatchSize:   1000,
		Parallel:    false,
		StopOnError: true,
	}
}

// EventReplayer интерфейс для replay событий
type EventReplayer interface {
	// ReplayAggregate воспроизводит события для конкретного агрегата
	ReplayAggregate(ctx context.Context, aggregateID string, toVersion int64) error

	// ReplayAll воспроизводит все события с указанной позиции
	ReplayAll(ctx context.Context, handler ReplayHandler, fromPosition int64, options ReplayOptions) error

	// ReplayByType воспроизводит события определенного типа
	ReplayByType(ctx context.Context, eventType string, handler ReplayHandler, fromTimestamp time.Time, options ReplayOptions) error
}

// DefaultEventReplayer реализация EventReplayer
type DefaultEventReplayer struct {
	eventStore    EventStore
	snapshotStore SnapshotStore
}

// NewDefaultEventReplayer создает новый Event Replayer
func NewDefaultEventReplayer(eventStore EventStore, snapshotStore SnapshotStore) *DefaultEventReplayer {
	return &DefaultEventReplayer{
		eventStore:    eventStore,
		snapshotStore: snapshotStore,
	}
}

// ReplayAggregate воспроизводит события для конкретного агрегата
func (r *DefaultEventReplayer) ReplayAggregate(ctx context.Context, aggregateID string, toVersion int64) error {
	events, err := r.eventStore.GetEvents(ctx, aggregateID, 0)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	for _, event := range events {
		if toVersion > 0 && event.Version > toVersion {
			break
		}
		// В реальной реализации здесь применяется событие к агрегату
	}

	return nil
}

// ReplayAll воспроизводит все события с указанной позиции
func (r *DefaultEventReplayer) ReplayAll(ctx context.Context, handler ReplayHandler, fromPosition int64, options ReplayOptions) error {
	eventChan, err := r.eventStore.GetAllEvents(ctx, fromPosition)
	if err != nil {
		return fmt.Errorf("failed to get all events: %w", err)
	}

	batch := make([]StoredEvent, 0, options.BatchSize)
	for event := range eventChan {
		batch = append(batch, event)

		if len(batch) >= options.BatchSize {
			if err := r.processBatch(ctx, handler, batch, options); err != nil {
				if options.StopOnError {
					return err
				}
			}
			batch = batch[:0]
		}
	}

	// Обрабатываем оставшиеся события
	if len(batch) > 0 {
		if err := r.processBatch(ctx, handler, batch, options); err != nil {
			if options.StopOnError {
				return err
			}
		}
	}

	return nil
}

// ReplayByType воспроизводит события определенного типа
func (r *DefaultEventReplayer) ReplayByType(ctx context.Context, eventType string, handler ReplayHandler, fromTimestamp time.Time, options ReplayOptions) error {
	events, err := r.eventStore.GetEventsByType(ctx, eventType, fromTimestamp)
	if err != nil {
		return fmt.Errorf("failed to get events by type: %w", err)
	}

	batch := make([]StoredEvent, 0, options.BatchSize)
	for _, event := range events {
		batch = append(batch, event)

		if len(batch) >= options.BatchSize {
			if err := r.processBatch(ctx, handler, batch, options); err != nil {
				if options.StopOnError {
					return err
				}
			}
			batch = batch[:0]
		}
	}

	// Обрабатываем оставшиеся события
	if len(batch) > 0 {
		if err := r.processBatch(ctx, handler, batch, options); err != nil {
			if options.StopOnError {
				return err
			}
		}
	}

	return nil
}

// processBatch обрабатывает батч событий
func (r *DefaultEventReplayer) processBatch(ctx context.Context, handler ReplayHandler, batch []StoredEvent, options ReplayOptions) error {
	if options.Parallel {
		// Параллельная обработка
		errChan := make(chan error, len(batch))
		for _, event := range batch {
			go func(e StoredEvent) {
				if err := handler.HandleEvent(ctx, e); err != nil {
					errChan <- err
				} else {
					errChan <- nil
				}
			}(event)
		}

		// Собираем ошибки
		for i := 0; i < len(batch); i++ {
			if err := <-errChan; err != nil {
				if options.StopOnError {
					return err
				}
			}
		}
	} else {
		// Последовательная обработка
		for _, event := range batch {
			if err := handler.HandleEvent(ctx, event); err != nil {
				if options.StopOnError {
					return err
				}
			}
		}
	}

	return nil
}

// ReplayWithProgress воспроизводит события с отслеживанием прогресса
func (r *DefaultEventReplayer) ReplayWithProgress(
	ctx context.Context,
	handler ReplayHandler,
	fromPosition int64,
	options ReplayOptions,
	progressCallback func(progress ReplayProgress),
) error {
	progress := ReplayProgress{
		StartTime: time.Now(),
	}

	eventChan, err := r.eventStore.GetAllEvents(ctx, fromPosition)
	if err != nil {
		return fmt.Errorf("failed to get all events: %w", err)
	}

	batch := make([]StoredEvent, 0, options.BatchSize)
	for event := range eventChan {
		batch = append(batch, event)
		progress.TotalEvents++

		if len(batch) >= options.BatchSize {
			if err := r.processBatch(ctx, handler, batch, options); err != nil {
				if options.StopOnError {
					return err
				}
			}
			progress.ProcessedEvents += int64(len(batch))
			progress.CurrentPosition = event.Position
			progress.ElapsedTime = time.Since(progress.StartTime)

			if progressCallback != nil {
				progressCallback(progress)
			}

			batch = batch[:0]
		}
	}

	// Обрабатываем оставшиеся события
	if len(batch) > 0 {
		if err := r.processBatch(ctx, handler, batch, options); err != nil {
			if options.StopOnError {
				return err
			}
		}
		progress.ProcessedEvents += int64(len(batch))
		if len(batch) > 0 {
			progress.CurrentPosition = batch[len(batch)-1].Position
		}
		progress.ElapsedTime = time.Since(progress.StartTime)

		if progressCallback != nil {
			progressCallback(progress)
		}
	}

	return nil
}

