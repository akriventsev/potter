// Package saga предоставляет fluent API builder для декларативного создания саг.
package saga

import (
	"context"
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/transport"
)

// SagaBuilder построитель саги
type SagaBuilder struct {
	name         string
	steps        []SagaStep
	timeout      time.Duration
	retryPolicy  *RetryPolicy
	persistence  SagaPersistence
	eventBus     events.EventBus
	commandBus   transport.CommandBus
	metadata     map[string]interface{}
}

// NewSagaBuilder создает новый построитель саги
func NewSagaBuilder(name string) *SagaBuilder {
	return &SagaBuilder{
		name:     name,
		steps:    make([]SagaStep, 0),
		metadata: make(map[string]interface{}),
	}
}

// AddStep добавляет шаг в сагу
func (b *SagaBuilder) AddStep(step SagaStep) *SagaBuilder {
	b.steps = append(b.steps, step)
	return b
}

// WithTimeout устанавливает общий таймаут для саги
func (b *SagaBuilder) WithTimeout(timeout time.Duration) *SagaBuilder {
	b.timeout = timeout
	return b
}

// WithRetryPolicy устанавливает политику повторов
func (b *SagaBuilder) WithRetryPolicy(policy *RetryPolicy) *SagaBuilder {
	b.retryPolicy = policy
	return b
}

// WithPersistence устанавливает persistence
func (b *SagaBuilder) WithPersistence(persistence SagaPersistence) *SagaBuilder {
	b.persistence = persistence
	return b
}

// WithEventBus устанавливает EventBus для публикации событий
func (b *SagaBuilder) WithEventBus(eventBus events.EventBus) *SagaBuilder {
	b.eventBus = eventBus
	return b
}

// WithCommandBus устанавливает CommandBus для выполнения команд
func (b *SagaBuilder) WithCommandBus(commandBus transport.CommandBus) *SagaBuilder {
	b.commandBus = commandBus
	return b
}

// WithMetadata добавляет метаданные
func (b *SagaBuilder) WithMetadata(key string, value interface{}) *SagaBuilder {
	if b.metadata == nil {
		b.metadata = make(map[string]interface{})
	}
	b.metadata[key] = value
	return b
}

// Build строит SagaDefinition
func (b *SagaBuilder) Build() (SagaDefinition, error) {
	// Валидация
	if len(b.steps) == 0 {
		return nil, fmt.Errorf("saga must have at least one step")
	}

	// Проверка уникальности имен шагов
	stepNames := make(map[string]bool)
	for _, step := range b.steps {
		if stepNames[step.Name()] {
			return nil, fmt.Errorf("duplicate step name: %s", step.Name())
		}
		stepNames[step.Name()] = true

		// Проверка наличия compensate action (предупреждение, не ошибка)
		// В реальности можно сделать опциональным
	}

	definition := &BaseSagaDefinition{
		name:  b.name,
		steps: b.steps,
	}

	// Применяем общие настройки к шагам
	for _, step := range b.steps {
		// Применяем общий timeout если задан и у шага нет своего
		if b.timeout > 0 && step.Timeout() == 0 {
			if baseStep, ok := step.(*BaseStep); ok {
				baseStep.WithTimeout(b.timeout)
			}
		}

		// Применяем общую retry policy если задана и у шага нет своей
		if b.retryPolicy != nil && step.RetryPolicy() == nil {
			if baseStep, ok := step.(*BaseStep); ok {
				baseStep.WithRetry(b.retryPolicy)
			}
		}
	}

	return definition, nil
}

// StepBuilder построитель шага
type StepBuilder struct {
	name            string
	executeAction   func(ctx context.Context, sagaCtx SagaContext) error
	compensateAction func(ctx context.Context, sagaCtx SagaContext) error
	guard           func(ctx context.Context, sagaCtx SagaContext) bool
	timeout         time.Duration
	retryPolicy     *RetryPolicy
	metadata        map[string]interface{}
}

// NewStepBuilder создает новый построитель шага
func NewStepBuilder(name string) *StepBuilder {
	return &StepBuilder{
		name:     name,
		metadata: make(map[string]interface{}),
	}
}

// WithExecute устанавливает execute action
func (b *StepBuilder) WithExecute(action func(ctx context.Context, sagaCtx SagaContext) error) *StepBuilder {
	b.executeAction = action
	return b
}

// WithCompensate устанавливает compensate action
func (b *StepBuilder) WithCompensate(action func(ctx context.Context, sagaCtx SagaContext) error) *StepBuilder {
	b.compensateAction = action
	return b
}

// WithGuard устанавливает guard функцию
func (b *StepBuilder) WithGuard(guard func(ctx context.Context, sagaCtx SagaContext) bool) *StepBuilder {
	b.guard = guard
	return b
}

// WithTimeout устанавливает timeout
func (b *StepBuilder) WithTimeout(timeout time.Duration) *StepBuilder {
	b.timeout = timeout
	return b
}

// WithRetry устанавливает retry policy
func (b *StepBuilder) WithRetry(policy *RetryPolicy) *StepBuilder {
	b.retryPolicy = policy
	return b
}

// WithMetadata добавляет метаданные
func (b *StepBuilder) WithMetadata(key string, value interface{}) *StepBuilder {
	if b.metadata == nil {
		b.metadata = make(map[string]interface{})
	}
	b.metadata[key] = value
	return b
}

// Build строит SagaStep
func (b *StepBuilder) Build() (SagaStep, error) {
	// Валидация
	if b.executeAction == nil {
		return nil, fmt.Errorf("execute action is required for step %s", b.name)
	}

	step := NewBaseStep(b.name)

	// Устанавливаем execute action
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return b.executeAction(ctx, sagaCtx)
	})

	// Устанавливаем compensate action если задан
	if b.compensateAction != nil {
		step.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
			return b.compensateAction(ctx, sagaCtx)
		})
	}

	// Устанавливаем guard если задан
	if b.guard != nil {
		step.WithGuard(func(ctx context.Context, sagaCtx SagaContext) bool {
			return b.guard(ctx, sagaCtx)
		})
	}

	// Устанавливаем timeout
	if b.timeout > 0 {
		step.WithTimeout(b.timeout)
	}

	// Устанавливаем retry policy
	if b.retryPolicy != nil {
		step.WithRetry(b.retryPolicy)
	}

	// Устанавливаем метаданные
	for k, v := range b.metadata {
		step.WithMetadata(k, v)
	}

	return step, nil
}

