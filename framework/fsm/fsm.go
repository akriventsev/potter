// Package fsm предоставляет реализацию конечного автомата для саг и оркестрации.
package fsm

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// FSM конечный автомат
type FSM struct {
	mu           sync.RWMutex
	id           string
	currentState State
	states       map[string]State
	transitions  map[string][]Transition // key: "fromState:eventName"
	initialState State
	history      []StateHistory
	maxHistory   int
	persistence  StatePersistence
	eventQueue   []Event
}

// StateHistory запись истории состояний
type StateHistory struct {
	State     State
	Timestamp int64
	Event     Event
}

// StatePersistence интерфейс для сохранения состояния FSM
type StatePersistence interface {
	Save(ctx context.Context, fsmID string, state State) error
	Load(ctx context.Context, fsmID string) (State, error)
}

// Config конфигурация FSM
type Config struct {
	MaxHistory int
	Persistence StatePersistence
}

// NewFSM создает новый конечный автомат
func NewFSM(initialState State, config ...Config) *FSM {
	cfg := Config{}
	if len(config) > 0 {
		cfg = config[0]
	}

	fsm := &FSM{
		id:           uuid.New().String(),
		currentState: initialState,
		states:       make(map[string]State),
		transitions:  make(map[string][]Transition),
		initialState: initialState,
		history:      make([]StateHistory, 0),
		maxHistory:   cfg.MaxHistory,
		persistence:  cfg.Persistence,
		eventQueue:   make([]Event, 0),
	}

	fsm.states[initialState.Name()] = initialState

	return fsm
}

// CurrentState возвращает текущее состояние
func (f *FSM) CurrentState() State {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.currentState
}

// AddState добавляет состояние в автомат
func (f *FSM) AddState(state State) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.states[state.Name()]; exists {
		return fmt.Errorf("state %s already exists", state.Name())
	}

	f.states[state.Name()] = state
	return nil
}

// GetState получает состояние по имени
func (f *FSM) GetState(name string) (State, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	state, exists := f.states[name]
	return state, exists
}

// AddTransition добавляет переход в автомат
func (f *FSM) AddTransition(transition Transition) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	from := transition.From()
	to := transition.To()

	// Проверка существования состояний
	if _, exists := f.states[from.Name()]; !exists {
		return fmt.Errorf("from state %s does not exist", from.Name())
	}
	if _, exists := f.states[to.Name()]; !exists {
		// Автоматически добавляем состояние, если его нет
		f.states[to.Name()] = to
	}

	event := transition.Event()
	key := fmt.Sprintf("%s:%s", from.Name(), event.Name())

	f.transitions[key] = append(f.transitions[key], transition)
	return nil
}

// GetTransitions получает все переходы из указанного состояния для события
func (f *FSM) GetTransitions(from State, eventName string) []Transition {
	f.mu.RLock()
	defer f.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", from.Name(), eventName)
	return f.transitions[key]
}

// CanTransition проверяет возможность перехода из текущего состояния по событию
func (f *FSM) CanTransition(ctx context.Context, eventName string) (bool, error) {
	f.mu.RLock()
	current := f.currentState
	f.mu.RUnlock()

	transitions := f.GetTransitions(current, eventName)
	if len(transitions) == 0 {
		return false, nil
	}

	// Проверяем хотя бы один переход
	for _, transition := range transitions {
		if can, err := transition.CanTransition(ctx); err != nil {
			return false, err
		} else if can {
			return true, nil
		}
	}

	return false, nil
}

// Trigger запускает событие и выполняет переход, если возможно
func (f *FSM) Trigger(ctx context.Context, event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	current := f.currentState
	transitions := f.GetTransitions(current, event.Name())

	if len(transitions) == 0 {
		// Добавляем в очередь, если переход не найден
		f.eventQueue = append(f.eventQueue, event)
		return fmt.Errorf("no transition found from state %s for event %s", current.Name(), event.Name())
	}

	// Ищем первый разрешенный переход
	var executedTransition Transition
	for _, transition := range transitions {
		if can, err := transition.CanTransition(ctx); err != nil {
			return fmt.Errorf("guard check failed: %w", err)
		} else if can {
			executedTransition = transition
			break
		}
	}

	if executedTransition == nil {
		return fmt.Errorf("no allowed transition from state %s for event %s", current.Name(), event.Name())
	}

	// Выполняем переход
	if err := executedTransition.Execute(ctx); err != nil {
		return fmt.Errorf("transition execution failed: %w", err)
	}

	// Обновляем текущее состояние
	previousState := f.currentState
	f.currentState = executedTransition.To()

	// Сохраняем в историю
	f.addHistory(previousState, f.currentState, event)

	// Сохраняем состояние если есть persistence
	if f.persistence != nil {
		_ = f.persistence.Save(ctx, f.id, f.currentState)
	}

	return nil
}

// addHistory добавляет запись в историю
func (f *FSM) addHistory(from, to State, event Event) {
	if f.maxHistory <= 0 {
		return
	}

	history := StateHistory{
		State:     to,
		Timestamp: event.Timestamp().Unix(),
		Event:     event,
	}

	f.history = append(f.history, history)

	// Ограничиваем размер истории
	if len(f.history) > f.maxHistory {
		f.history = f.history[len(f.history)-f.maxHistory:]
	}
}

// History возвращает историю состояний
func (f *FSM) History() []StateHistory {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]StateHistory, len(f.history))
	copy(result, f.history)
	return result
}

// Reset сбрасывает FSM в начальное состояние
func (f *FSM) Reset(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.currentState != nil {
		event := NewEvent("reset", nil)
		if err := f.currentState.OnExit(ctx, event); err != nil {
			return fmt.Errorf("onExit failed during reset: %w", err)
		}
	}

	f.currentState = f.initialState
	f.history = make([]StateHistory, 0)

	event := NewEvent("reset", nil)
	if err := f.initialState.OnEnter(ctx, event); err != nil {
		return fmt.Errorf("onEnter failed during reset: %w", err)
	}

	return nil
}

