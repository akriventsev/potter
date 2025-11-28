// Package fsm предоставляет определения переходов для FSM.
package fsm

import (
	"context"
	"fmt"
	"time"
)

// Transition представляет переход между состояниями
type Transition interface {
	// From возвращает исходное состояние
	From() State
	// To возвращает целевое состояние
	To() State
	// Event возвращает событие, вызывающее переход
	Event() Event
	// CanTransition проверяет возможность перехода
	CanTransition(ctx context.Context) (bool, error)
	// Execute выполняет переход
	Execute(ctx context.Context) error
}

// BaseTransition базовая реализация перехода
type BaseTransition struct {
	from       State
	to         State
	eventName  string
	guard      Guard
	actions    []Action
	beforeHook TransitionHook
	afterHook  TransitionHook
	timeout    time.Duration
}

// NewTransition создает новый переход
func NewTransition(from, to State, eventName string) *BaseTransition {
	return &BaseTransition{
		from:      from,
		to:        to,
		eventName: eventName,
		actions:   make([]Action, 0),
	}
}

// WithGuard добавляет охранник (guard) к переходу
func (t *BaseTransition) WithGuard(guard Guard) *BaseTransition {
	t.guard = guard
	return t
}

// WithActions добавляет действия к переходу
func (t *BaseTransition) WithActions(actions ...Action) *BaseTransition {
	t.actions = append(t.actions, actions...)
	return t
}

// WithBeforeHook добавляет хук, вызываемый до перехода
func (t *BaseTransition) WithBeforeHook(hook TransitionHook) *BaseTransition {
	t.beforeHook = hook
	return t
}

// WithAfterHook добавляет хук, вызываемый после перехода
func (t *BaseTransition) WithAfterHook(hook TransitionHook) *BaseTransition {
	t.afterHook = hook
	return t
}

// WithTimeout устанавливает timeout для перехода
func (t *BaseTransition) WithTimeout(timeout time.Duration) *BaseTransition {
	t.timeout = timeout
	return t
}

func (t *BaseTransition) From() State {
	return t.from
}

func (t *BaseTransition) To() State {
	return t.to
}

func (t *BaseTransition) Event() Event {
	return NewEvent(t.eventName, nil)
}

func (t *BaseTransition) CanTransition(ctx context.Context) (bool, error) {
	if t.guard == nil {
		return true, nil
	}

	// Создаем временное событие для проверки guard
	event := NewEvent(t.eventName, nil)
	return t.guard(ctx, t.from, t.to, event)
}

func (t *BaseTransition) Execute(ctx context.Context) error {
	event := NewEvent(t.eventName, nil)

	// Устанавливаем timeout если задан
	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	// Проверка guard
	if can, err := t.CanTransition(ctx); err != nil {
		return fmt.Errorf("guard check failed: %w", err)
	} else if !can {
		return fmt.Errorf("transition not allowed by guard: %s -> %s", t.from.Name(), t.to.Name())
	}

	// Вызов хука перед переходом
	if t.beforeHook != nil {
		if err := t.beforeHook(ctx, t.from, t.to, event); err != nil {
			return fmt.Errorf("before hook failed: %w", err)
		}
	}

	// Выход из текущего состояния
	if err := t.from.OnExit(ctx, event); err != nil {
		return fmt.Errorf("onExit failed for state %s: %w", t.from.Name(), err)
	}

	// Выполнение действий перехода
	for _, action := range t.actions {
		if err := action.Execute(ctx, event); err != nil {
			return fmt.Errorf("action %s failed: %w", action.Name(), err)
		}
	}

	// Вход в новое состояние
	if err := t.to.OnEnter(ctx, event); err != nil {
		return fmt.Errorf("onEnter failed for state %s: %w", t.to.Name(), err)
	}

	// Вызов хука после перехода
	if t.afterHook != nil {
		if err := t.afterHook(ctx, t.from, t.to, event); err != nil {
			return fmt.Errorf("after hook failed: %w", err)
		}
	}

	return nil
}

// TransitionBuilder построитель переходов
type TransitionBuilder struct {
	from       State
	to         State
	eventName  string
	guard      Guard
	actions    []Action
	beforeHook TransitionHook
	afterHook  TransitionHook
	timeout    time.Duration
}

// NewTransitionBuilder создает новый построитель переходов
func NewTransitionBuilder(from, to State, eventName string) *TransitionBuilder {
	return &TransitionBuilder{
		from:      from,
		to:        to,
		eventName: eventName,
		actions:   make([]Action, 0),
	}
}

// WithGuard добавляет охранник
func (tb *TransitionBuilder) WithGuard(guard Guard) *TransitionBuilder {
	tb.guard = guard
	return tb
}

// WithActions добавляет действия
func (tb *TransitionBuilder) WithActions(actions ...Action) *TransitionBuilder {
	tb.actions = append(tb.actions, actions...)
	return tb
}

// WithBeforeHook добавляет хук до перехода
func (tb *TransitionBuilder) WithBeforeHook(hook TransitionHook) *TransitionBuilder {
	tb.beforeHook = hook
	return tb
}

// WithAfterHook добавляет хук после перехода
func (tb *TransitionBuilder) WithAfterHook(hook TransitionHook) *TransitionBuilder {
	tb.afterHook = hook
	return tb
}

// WithTimeout устанавливает timeout
func (tb *TransitionBuilder) WithTimeout(timeout time.Duration) *TransitionBuilder {
	tb.timeout = timeout
	return tb
}

// Build создает переход
func (tb *TransitionBuilder) Build() *BaseTransition {
	return &BaseTransition{
		from:       tb.from,
		to:         tb.to,
		eventName:  tb.eventName,
		guard:      tb.guard,
		actions:    tb.actions,
		beforeHook: tb.beforeHook,
		afterHook:  tb.afterHook,
		timeout:    tb.timeout,
	}
}

