package eventsourcing

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/akriventsev/potter/framework/events"
)

// AggregateInterface интерфейс для Event Sourced агрегатов
type AggregateInterface interface {
	ID() string
	Version() int64
	GetUncommittedEvents() []events.Event
	MarkEventsAsCommitted()
	SetVersion(int64)
	Apply(events.Event) error
}

// AggregateFactory фабричная функция для создания агрегатов
type AggregateFactory[T AggregateInterface] func(id string) T

// RepositoryConfig конфигурация для Event Sourced репозитория
type RepositoryConfig struct {
	SnapshotFrequency int
	UseSnapshots      bool
	SnapshotStrategy  SnapshotStrategy
	Serializer        SnapshotSerializer
}

// DefaultRepositoryConfig возвращает конфигурацию по умолчанию
func DefaultRepositoryConfig() RepositoryConfig {
	return RepositoryConfig{
		SnapshotFrequency: 100,
		UseSnapshots:      true,
		SnapshotStrategy:  NewFrequencySnapshotStrategy(100),
		Serializer:        NewJSONSnapshotSerializer(),
	}
}

// EventSourcedRepository generic репозиторий для Event Sourced агрегатов
type EventSourcedRepository[T AggregateInterface] struct {
	eventStore    EventStore
	snapshotStore SnapshotStore
	config        RepositoryConfig
	factory       AggregateFactory[T]
}

// NewEventSourcedRepository создает новый Event Sourced репозиторий
func NewEventSourcedRepository[T AggregateInterface](
	eventStore EventStore,
	snapshotStore SnapshotStore,
	config RepositoryConfig,
	factory AggregateFactory[T],
) *EventSourcedRepository[T] {
	if config.Serializer == nil {
		config.Serializer = NewJSONSnapshotSerializer()
	}
	if config.SnapshotStrategy == nil {
		config.SnapshotStrategy = NewFrequencySnapshotStrategy(int64(config.SnapshotFrequency))
	}

	return &EventSourcedRepository[T]{
		eventStore:    eventStore,
		snapshotStore:  snapshotStore,
		config:        config,
		factory:       factory,
	}
}

// Save сохраняет агрегат, добавляя uncommitted события в EventStore
func (r *EventSourcedRepository[T]) Save(ctx context.Context, aggregate T) error {
	uncommittedEvents := aggregate.GetUncommittedEvents()
	if len(uncommittedEvents) == 0 {
		return nil
	}

	expectedVersion := aggregate.Version() - int64(len(uncommittedEvents))
	if expectedVersion < 0 {
		expectedVersion = 0
	}

	// Сохраняем события в EventStore
	err := r.eventStore.AppendEvents(ctx, aggregate.ID(), expectedVersion, uncommittedEvents)
	if err != nil {
		return fmt.Errorf("failed to append events: %w", err)
	}

	// Создаем снапшот если нужно
	if r.config.UseSnapshots && r.snapshotStore != nil {
		eventCount := aggregate.Version()
		// Передаем агрегат как интерфейс для стратегии
		if r.config.SnapshotStrategy.ShouldCreateSnapshot(aggregate, eventCount) {
			if err := r.createSnapshot(ctx, aggregate); err != nil {
				// Логируем ошибку, но не прерываем сохранение
				// В production здесь должно быть логирование
			}
		}
	}

	// Помечаем события как сохраненные
	aggregate.MarkEventsAsCommitted()
	return nil
}

// GetByID загружает агрегат по ID, восстанавливая состояние из событий
func (r *EventSourcedRepository[T]) GetByID(ctx context.Context, aggregateID string) (T, error) {
	var zero T

	if r.factory == nil {
		return zero, fmt.Errorf("aggregate factory not set")
	}

	// Пытаемся загрузить из снапшота
	var fromVersion int64 = 0
	if r.config.UseSnapshots && r.snapshotStore != nil {
		snapshot, err := r.snapshotStore.GetSnapshot(ctx, aggregateID)
		if err == nil && snapshot != nil {
			// Создаем новый агрегат через фабрику
			aggregate := r.factory(aggregateID)
			
			// Десериализуем состояние из снапшота
			if err := r.config.Serializer.Deserialize(snapshot.State, aggregate); err != nil {
				// Если не удалось десериализовать, загружаем с начала
				fromVersion = 0
			} else {
				aggregate.SetVersion(snapshot.Version)
				fromVersion = snapshot.Version + 1
			}

			// Загружаем события после снапшота
			storedEvents, err := r.eventStore.GetEvents(ctx, aggregateID, fromVersion)
			if err != nil && err != ErrStreamNotFound {
				return zero, fmt.Errorf("failed to get events: %w", err)
			}

			// Применяем события для восстановления состояния
			if len(storedEvents) > 0 {
				eventList := make([]events.Event, 0, len(storedEvents))
				for _, stored := range storedEvents {
					if stored.EventData != nil {
						eventList = append(eventList, stored.EventData)
					}
				}
				// Применяем события последовательно
				for _, event := range eventList {
					if err := aggregate.Apply(event); err != nil {
						return zero, fmt.Errorf("failed to apply event: %w", err)
					}
					aggregate.SetVersion(aggregate.Version() + 1)
				}
			}

			return aggregate, nil
		}
	}

	// Загружаем все события с начала
	storedEvents, err := r.eventStore.GetEvents(ctx, aggregateID, 0)
	if err != nil {
		if err == ErrStreamNotFound {
			return zero, fmt.Errorf("aggregate not found: %s", aggregateID)
		}
		return zero, fmt.Errorf("failed to get events: %w", err)
	}

	// Создаем новый агрегат через фабрику
	aggregate := r.factory(aggregateID)

	// Применяем события для восстановления состояния
	if len(storedEvents) > 0 {
		eventList := make([]events.Event, 0, len(storedEvents))
		for _, stored := range storedEvents {
			if stored.EventData != nil {
				eventList = append(eventList, stored.EventData)
			}
		}
		// Применяем события последовательно
		for _, event := range eventList {
			if err := aggregate.Apply(event); err != nil {
				return zero, fmt.Errorf("failed to apply event: %w", err)
			}
			aggregate.SetVersion(aggregate.Version() + 1)
		}
	}

	return aggregate, nil
}

// GetVersion возвращает текущую версию агрегата
func (r *EventSourcedRepository[T]) GetVersion(ctx context.Context, aggregateID string) (int64, error) {
	events, err := r.eventStore.GetEvents(ctx, aggregateID, 0)
	if err != nil {
		if err == ErrStreamNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get events: %w", err)
	}

	if len(events) == 0 {
		return 0, nil
	}

	return events[len(events)-1].Version, nil
}

// Exists проверяет существование агрегата
func (r *EventSourcedRepository[T]) Exists(ctx context.Context, aggregateID string) (bool, error) {
	events, err := r.eventStore.GetEvents(ctx, aggregateID, 0)
	if err != nil {
		if err == ErrStreamNotFound {
			return false, nil
		}
		return false, err
	}
	return len(events) > 0, nil
}

// createSnapshot создает снапшот агрегата
func (r *EventSourcedRepository[T]) createSnapshot(ctx context.Context, aggregate T) error {
	state, err := r.config.Serializer.Serialize(aggregate)
	if err != nil {
		return fmt.Errorf("failed to serialize aggregate: %w", err)
	}

	snapshot := Snapshot{
		AggregateID:  aggregate.ID(),
		AggregateType: getAggregateTypeName(aggregate),
		Version:      aggregate.Version(),
		State:        state,
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
	}

	return r.snapshotStore.SaveSnapshot(ctx, snapshot)
}

// getAggregateTypeName получает имя типа агрегата
func getAggregateTypeName(aggregate interface{}) string {
	if aggregate == nil {
		return "aggregate"
	}

	t := reflect.TypeOf(aggregate)
	// Обрабатываем указатели
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Возвращаем полное имя типа с пакетом для уникальности
	if t.PkgPath() != "" {
		return t.PkgPath() + "." + t.Name()
	}
	return t.Name()
}

