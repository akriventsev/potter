// Package fsm предоставляет определения действий для FSM.
package fsm

import (
	"context"
	"fmt"
	"time"
)

// Action представляет действие, которое может быть выполнено в процессе работы FSM
type Action interface {
	// Execute выполняет действие
	Execute(ctx context.Context, event Event) error
	// Name возвращает имя действия
	Name() string
}

// ActionFunc функция, реализующая Action
type ActionFunc func(ctx context.Context, event Event) error

// NamedAction действие с именем
type NamedAction struct {
	name   string
	action ActionFunc
}

// NewNamedAction создает новое именованное действие
func NewNamedAction(name string, action ActionFunc) *NamedAction {
	return &NamedAction{
		name:   name,
		action: action,
	}
}

func (a *NamedAction) Name() string {
	return a.name
}

func (a *NamedAction) Execute(ctx context.Context, event Event) error {
	return a.action(ctx, event)
}

// AsyncAction асинхронное действие
type AsyncAction struct {
	*NamedAction
	timeout time.Duration
}

// NewAsyncAction создает новое асинхронное действие
func NewAsyncAction(name string, action ActionFunc, timeout time.Duration) *AsyncAction {
	return &AsyncAction{
		NamedAction: NewNamedAction(name, action),
		timeout:     timeout,
	}
}

func (a *AsyncAction) Execute(ctx context.Context, event Event) error {
	done := make(chan error, 1)
	go func() {
		done <- a.action(ctx, event)
	}()

	if a.timeout > 0 {
		select {
		case err := <-done:
			return err
		case <-time.After(a.timeout):
			return fmt.Errorf("action %s timeout", a.name)
		}
	}

	return <-done
}

// ActionWithRetry действие с повторами
type ActionWithRetry struct {
	*NamedAction
	maxAttempts int
	delay       time.Duration
	backoff     time.Duration
}

// NewActionWithRetry создает действие с повторами
func NewActionWithRetry(name string, action ActionFunc, maxAttempts int, delay, backoff time.Duration) *ActionWithRetry {
	return &ActionWithRetry{
		NamedAction: NewNamedAction(name, action),
		maxAttempts: maxAttempts,
		delay:       delay,
		backoff:     backoff,
	}
}

func (a *ActionWithRetry) Execute(ctx context.Context, event Event) error {
	var lastErr error
	delay := a.delay

	for attempt := 0; attempt < a.maxAttempts; attempt++ {
		if err := a.action(ctx, event); err == nil {
			return nil
		} else {
			lastErr = err
			if attempt < a.maxAttempts-1 {
				time.Sleep(delay)
				delay += a.backoff
			}
		}
	}

	return lastErr
}

// CompositeAction составное действие, выполняющее несколько действий последовательно
type CompositeAction struct {
	name    string
	actions []Action
}

// NewCompositeAction создает новое составное действие
func NewCompositeAction(name string, actions ...Action) *CompositeAction {
	return &CompositeAction{
		name:    name,
		actions: actions,
	}
}

func (a *CompositeAction) Name() string {
	return a.name
}

func (a *CompositeAction) Execute(ctx context.Context, event Event) error {
	for _, action := range a.actions {
		if err := action.Execute(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// ConditionalAction действие, выполняемое только при выполнении условия
type ConditionalAction struct {
	name      string
	condition func(ctx context.Context, event Event) bool
	action    Action
}

// NewConditionalAction создает новое условное действие
func NewConditionalAction(name string, condition func(ctx context.Context, event Event) bool, action Action) *ConditionalAction {
	return &ConditionalAction{
		name:      name,
		condition: condition,
		action:    action,
	}
}

func (a *ConditionalAction) Name() string {
	return a.name
}

func (a *ConditionalAction) Execute(ctx context.Context, event Event) error {
	if a.condition(ctx, event) {
		return a.action.Execute(ctx, event)
	}
	return nil
}

