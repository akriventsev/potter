// Package fsm предоставляет определения состояний для FSM.
package fsm

import (
	"context"
	"time"
)

// State представляет состояние конечного автомата
type State interface {
	// Name возвращает имя состояния
	Name() string
	// OnEnter вызывается при входе в состояние
	OnEnter(ctx context.Context, event Event) error
	// OnExit вызывается при выходе из состояния
	OnExit(ctx context.Context, event Event) error
}

// StateMetadata метаданные состояния
type StateMetadata map[string]interface{}

// Get получает значение метаданных
func (m StateMetadata) Get(key string) (interface{}, bool) {
	val, ok := m[key]
	return val, ok
}

// Set устанавливает значение метаданных
func (m StateMetadata) Set(key string, value interface{}) {
	m[key] = value
}

// BaseState базовая реализация состояния с пустыми обработчиками
type BaseState struct {
	name     string
	metadata StateMetadata
	timeout  time.Duration
}

// NewBaseState создает новое базовое состояние
func NewBaseState(name string) *BaseState {
	return &BaseState{
		name:     name,
		metadata: make(StateMetadata),
	}
}

// WithMetadata добавляет метаданные к состоянию
func (s *BaseState) WithMetadata(key string, value interface{}) *BaseState {
	s.metadata.Set(key, value)
	return s
}

// WithTimeout устанавливает timeout для состояния
func (s *BaseState) WithTimeout(timeout time.Duration) *BaseState {
	s.timeout = timeout
	return s
}

func (s *BaseState) Name() string {
	return s.name
}

func (s *BaseState) OnEnter(ctx context.Context, event Event) error {
	return nil
}

func (s *BaseState) OnExit(ctx context.Context, event Event) error {
	return nil
}

// StateWithActions состояние с дополнительными действиями
type StateWithActions struct {
	*BaseState
	enterActions []Action
	exitActions  []Action
}

// NewStateWithActions создает состояние с действиями
func NewStateWithActions(name string, enterActions, exitActions []Action) *StateWithActions {
	return &StateWithActions{
		BaseState:    NewBaseState(name),
		enterActions: enterActions,
		exitActions:  exitActions,
	}
}

func (s *StateWithActions) OnEnter(ctx context.Context, event Event) error {
	for _, action := range s.enterActions {
		if err := action.Execute(ctx, event); err != nil {
			return err
		}
	}
	return s.BaseState.OnEnter(ctx, event)
}

func (s *StateWithActions) OnExit(ctx context.Context, event Event) error {
	for _, action := range s.exitActions {
		if err := action.Execute(ctx, event); err != nil {
			return err
		}
	}
	return s.BaseState.OnExit(ctx, event)
}

