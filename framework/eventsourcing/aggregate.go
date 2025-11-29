package eventsourcing

import (
	"fmt"

	"potter/framework/events"
)

// EventSourcedAggregate базовый класс для агрегатов с Event Sourcing
type EventSourcedAggregate struct {
	id               string
	version          int64
	uncommittedEvents []events.Event
	applier          EventApplier
}

// NewEventSourcedAggregate создает новый Event Sourced агрегат
func NewEventSourcedAggregate(id string) *EventSourcedAggregate {
	return &EventSourcedAggregate{
		id:                id,
		version:           0,
		uncommittedEvents: make([]events.Event, 0),
		applier:           nil,
	}
}

// NewEventSourcedAggregateWithApplier создает новый Event Sourced агрегат с EventApplier
func NewEventSourcedAggregateWithApplier(id string, applier EventApplier) *EventSourcedAggregate {
	return &EventSourcedAggregate{
		id:                id,
		version:           0,
		uncommittedEvents: make([]events.Event, 0),
		applier:           applier,
	}
}

// SetApplier устанавливает EventApplier для агрегата
func (a *EventSourcedAggregate) SetApplier(applier EventApplier) {
	a.applier = applier
}

// ID возвращает идентификатор агрегата
func (a *EventSourcedAggregate) ID() string {
	return a.id
}

// Version возвращает текущую версию агрегата
func (a *EventSourcedAggregate) Version() int64 {
	return a.version
}

// RaiseEvent добавляет новое событие в uncommitted события
func (a *EventSourcedAggregate) RaiseEvent(event events.Event) {
	a.uncommittedEvents = append(a.uncommittedEvents, event)
	// Применяем событие сразу для обновления состояния
	if err := a.ApplyEvent(event); err != nil {
		// В production здесь должна быть более серьезная обработка ошибок
		panic(fmt.Sprintf("failed to apply event: %v", err))
	}
	// Увеличиваем версию после успешного применения
	a.version++
}

// ApplyEvent применяет событие к состоянию агрегата
func (a *EventSourcedAggregate) ApplyEvent(event events.Event) error {
	if a.applier == nil {
		return fmt.Errorf("EventApplier not set for aggregate %s", a.id)
	}
	return a.applier.Apply(event)
}

// LoadFromHistory восстанавливает состояние агрегата из истории событий
func (a *EventSourcedAggregate) LoadFromHistory(events []events.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Применяем события последовательно
	for i, event := range events {
		if err := a.ApplyEvent(event); err != nil {
			return fmt.Errorf("failed to apply event at index %d: %w", i, err)
		}
		// Увеличиваем версию после каждого успешного применения
		a.version++
	}

	return nil
}

// GetUncommittedEvents возвращает несохраненные события
func (a *EventSourcedAggregate) GetUncommittedEvents() []events.Event {
	return a.uncommittedEvents
}

// MarkEventsAsCommitted очищает uncommitted события после сохранения
func (a *EventSourcedAggregate) MarkEventsAsCommitted() {
	a.uncommittedEvents = make([]events.Event, 0)
}

// SetVersion устанавливает версию агрегата (используется при загрузке)
func (a *EventSourcedAggregate) SetVersion(version int64) {
	a.version = version
}

// Apply применяет событие к агрегату (реализация AggregateInterface)
func (a *EventSourcedAggregate) Apply(event events.Event) error {
	return a.ApplyEvent(event)
}

// EventApplier интерфейс для агрегатов, которые могут применять события
type EventApplier interface {
	// Apply применяет конкретное событие к состоянию агрегата
	Apply(event events.Event) error
}

// ValidateEventVersion проверяет корректность версии события
func ValidateEventVersion(currentVersion, expectedVersion int64) error {
	if expectedVersion < 0 {
		return ErrInvalidVersion
	}
	if expectedVersion != currentVersion {
		return ErrConcurrencyConflict
	}
	return nil
}

// ApplyEventsToAggregate применяет события к агрегату через EventApplier
func ApplyEventsToAggregate(aggregate EventApplier, events []events.Event) error {
	for i, event := range events {
		if err := aggregate.Apply(event); err != nil {
			return fmt.Errorf("failed to apply event at index %d: %w", i, err)
		}
	}
	return nil
}

