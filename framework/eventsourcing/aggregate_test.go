package eventsourcing

import (
	"testing"

	"github.com/akriventsev/potter/framework/events"
)

// TestAggregate тестовый агрегат для проверки функциональности
type TestAggregate struct {
	*EventSourcedAggregate
	name  string
	value int
}

// NewTestAggregate создает новый тестовый агрегат
func NewTestAggregate(id string) *TestAggregate {
	agg := &TestAggregate{
		EventSourcedAggregate: NewEventSourcedAggregate(id),
	}
	// Устанавливаем applier для применения событий
	agg.SetApplier(agg)
	return agg
}

// Apply применяет событие к агрегату
func (a *TestAggregate) Apply(event events.Event) error {
	switch e := event.(type) {
	case *TestCreatedEvent:
		a.name = e.Name
		a.value = e.Value
	case *TestUpdatedEvent:
		a.value = e.Value
	}
	return nil
}

// TestCreatedEvent событие создания
type TestCreatedEvent struct {
	*events.BaseEvent
	Name  string
	Value int
}

// TestUpdatedEvent событие обновления
type TestUpdatedEvent struct {
	*events.BaseEvent
	Value int
}

func TestEventSourcedAggregate_RaiseEvent(t *testing.T) {
	agg := NewTestAggregate("test-1")
	event := &TestCreatedEvent{
		BaseEvent: events.NewBaseEvent("test.created", "test-1"),
		Name:      "Test",
		Value:     10,
	}

	initialVersion := agg.Version()
	agg.RaiseEvent(event)

	uncommitted := agg.GetUncommittedEvents()
	if len(uncommitted) != 1 {
		t.Errorf("Expected 1 uncommitted event, got %d", len(uncommitted))
	}

	if agg.name != "Test" {
		t.Errorf("Expected name 'Test', got '%s'", agg.name)
	}

	if agg.value != 10 {
		t.Errorf("Expected value 10, got %d", agg.value)
	}

	// Проверяем, что версия увеличилась
	if agg.Version() != initialVersion+1 {
		t.Errorf("Expected version %d, got %d", initialVersion+1, agg.Version())
	}
}

func TestEventSourcedAggregate_LoadFromHistory(t *testing.T) {
	agg := NewTestAggregate("test-1")
	events := []events.Event{
		&TestCreatedEvent{
			BaseEvent: events.NewBaseEvent("test.created", "test-1"),
			Name:      "Test",
			Value:     10,
		},
		&TestUpdatedEvent{
			BaseEvent: events.NewBaseEvent("test.updated", "test-1"),
			Value:     20,
		},
	}

	err := agg.LoadFromHistory(events)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if agg.name != "Test" {
		t.Errorf("Expected name 'Test', got '%s'", agg.name)
	}

	if agg.value != 20 {
		t.Errorf("Expected value 20, got %d", agg.value)
	}

	if agg.Version() != 2 {
		t.Errorf("Expected version 2, got %d", agg.Version())
	}
}

func TestEventSourcedAggregate_ApplyEvent(t *testing.T) {
	agg := NewTestAggregate("test-1")
	event := &TestCreatedEvent{
		BaseEvent: events.NewBaseEvent("test.created", "test-1"),
		Name:      "Test",
		Value:     10,
	}

	initialVersion := agg.Version()
	err := agg.ApplyEvent(event)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if agg.name != "Test" {
		t.Errorf("Expected name 'Test', got '%s'", agg.name)
	}

	// ApplyEvent не увеличивает версию автоматически (это делает RaiseEvent)
	// Но мы можем проверить, что состояние изменилось
	if agg.value != 10 {
		t.Errorf("Expected value 10, got %d", agg.value)
	}

	// Версия не должна измениться при ApplyEvent (только при RaiseEvent)
	if agg.Version() != initialVersion {
		t.Errorf("Expected version to remain %d after ApplyEvent, got %d", initialVersion, agg.Version())
	}
}

func TestEventSourcedAggregate_GetUncommittedEvents(t *testing.T) {
	agg := NewTestAggregate("test-1")

	if len(agg.GetUncommittedEvents()) != 0 {
		t.Error("Expected no uncommitted events initially")
	}

	event1 := &TestCreatedEvent{
		BaseEvent: events.NewBaseEvent("test.created", "test-1"),
		Name:      "Test",
		Value:     10,
	}
	agg.RaiseEvent(event1)

	event2 := &TestUpdatedEvent{
		BaseEvent: events.NewBaseEvent("test.updated", "test-1"),
		Value:     20,
	}
	agg.RaiseEvent(event2)

	uncommitted := agg.GetUncommittedEvents()
	if len(uncommitted) != 2 {
		t.Errorf("Expected 2 uncommitted events, got %d", len(uncommitted))
	}
}

func TestEventSourcedAggregate_MarkEventsAsCommitted(t *testing.T) {
	agg := NewTestAggregate("test-1")
	event := &TestCreatedEvent{
		BaseEvent: events.NewBaseEvent("test.created", "test-1"),
		Name:      "Test",
		Value:     10,
	}
	agg.RaiseEvent(event)

	agg.MarkEventsAsCommitted()

	uncommitted := agg.GetUncommittedEvents()
	if len(uncommitted) != 0 {
		t.Errorf("Expected no uncommitted events after marking as committed, got %d", len(uncommitted))
	}
}

func TestEventSourcedAggregate_Version(t *testing.T) {
	agg := NewTestAggregate("test-1")

	if agg.Version() != 0 {
		t.Errorf("Expected initial version 0, got %d", agg.Version())
	}

	events := []events.Event{
		&TestCreatedEvent{
			BaseEvent: events.NewBaseEvent("test.created", "test-1"),
			Name:      "Test",
			Value:     10,
		},
	}
	agg.LoadFromHistory(events)

	if agg.Version() != 1 {
		t.Errorf("Expected version 1 after loading 1 event, got %d", agg.Version())
	}
}

func TestEventSourcedAggregate_ReplayOrder(t *testing.T) {
	agg := NewTestAggregate("test-1")
	events := []events.Event{
		&TestCreatedEvent{
			BaseEvent: events.NewBaseEvent("test.created", "test-1"),
			Name:      "Test",
			Value:     10,
		},
		&TestUpdatedEvent{
			BaseEvent: events.NewBaseEvent("test.updated", "test-1"),
			Value:     20,
		},
		&TestUpdatedEvent{
			BaseEvent: events.NewBaseEvent("test.updated", "test-1"),
			Value:     30,
		},
	}

	err := agg.LoadFromHistory(events)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Последнее значение должно быть применено
	if agg.value != 30 {
		t.Errorf("Expected value 30 after replay, got %d", agg.value)
	}
}

func TestEventSourcedAggregate_EmptyHistory(t *testing.T) {
	agg := NewTestAggregate("test-1")
	err := agg.LoadFromHistory([]events.Event{})
	if err != nil {
		t.Fatalf("Expected no error with empty history, got %v", err)
	}

	if agg.Version() != 0 {
		t.Errorf("Expected version 0 with empty history, got %d", agg.Version())
	}
}

