// Package saga предоставляет определения шагов для Saga Pattern.
package saga

import (
	"context"
	"fmt"
	"time"

	"potter/framework/events"
	"potter/framework/invoke"
	"potter/framework/transport"
)

// SagaStep интерфейс шага саги
type SagaStep interface {
	// Name возвращает имя шага
	Name() string
	// Execute выполняет forward action (основное действие)
	Execute(ctx context.Context, sagaCtx SagaContext) error
	// Compensate выполняет compensating action (откат изменений)
	Compensate(ctx context.Context, sagaCtx SagaContext) error
	// CanExecute проверяет возможность выполнения шага (guard)
	CanExecute(ctx context.Context, sagaCtx SagaContext) bool
	// Timeout возвращает таймаут выполнения шага
	Timeout() time.Duration
	// RetryPolicy возвращает политику повторов
	RetryPolicy() *RetryPolicy
}

// RetryPolicy политика повторов для шага
type RetryPolicy struct {
	MaxAttempts    int
	InitialDelay   time.Duration
	Backoff        float64
	RetryableErrors []error
}

// ShouldRetry определяет, нужно ли повторить попытку
func (p *RetryPolicy) ShouldRetry(err error, attempt int) bool {
	if attempt >= p.MaxAttempts {
		return false
	}

	// Если указаны retryable errors, проверяем соответствие
	if len(p.RetryableErrors) > 0 {
		for _, retryableErr := range p.RetryableErrors {
			if err == retryableErr || fmt.Sprintf("%v", err) == fmt.Sprintf("%v", retryableErr) {
				return true
			}
		}
		return false
	}

	// По умолчанию повторяем для всех ошибок
	return true
}

// CalculateDelay вычисляет задержку перед повтором
func (p *RetryPolicy) CalculateDelay(attempt int) time.Duration {
	delay := time.Duration(float64(p.InitialDelay) * float64(attempt+1) * p.Backoff)
	return delay
}

// NoRetry создает политику без повторов
func NoRetry() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  1,
		InitialDelay: 0,
		Backoff:      1.0,
	}
}

// SimpleRetry создает простую политику повторов
func SimpleRetry(maxAttempts int) *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  maxAttempts,
		InitialDelay: time.Second,
		Backoff:      1.0,
	}
}

// ExponentialBackoff создает политику с экспоненциальной задержкой
func ExponentialBackoff(maxAttempts int, initialDelay time.Duration, backoff float64) *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  maxAttempts,
		InitialDelay: initialDelay,
		Backoff:      backoff,
	}
}

// BaseStep базовая реализация SagaStep
type BaseStep struct {
	name            string
	executeAction   func(ctx context.Context, sagaCtx SagaContext) error
	compensateAction func(ctx context.Context, sagaCtx SagaContext) error
	guard           func(ctx context.Context, sagaCtx SagaContext) bool
	timeout         time.Duration
	retryPolicy     *RetryPolicy
	metadata        map[string]interface{}
}

// NewBaseStep создает новый базовый шаг
func NewBaseStep(name string) *BaseStep {
	return &BaseStep{
		name:     name,
		metadata: make(map[string]interface{}),
	}
}

func (s *BaseStep) Name() string {
	return s.name
}

func (s *BaseStep) Execute(ctx context.Context, sagaCtx SagaContext) error {
	if s.executeAction == nil {
		return fmt.Errorf("execute action not set for step %s", s.name)
	}
	return s.executeAction(ctx, sagaCtx)
}

func (s *BaseStep) Compensate(ctx context.Context, sagaCtx SagaContext) error {
	if s.compensateAction == nil {
		// Если компенсация не задана, это не ошибка (no-op)
		return nil
	}
	return s.compensateAction(ctx, sagaCtx)
}

func (s *BaseStep) CanExecute(ctx context.Context, sagaCtx SagaContext) bool {
	if s.guard == nil {
		return true
	}
	return s.guard(ctx, sagaCtx)
}

func (s *BaseStep) Timeout() time.Duration {
	return s.timeout
}

func (s *BaseStep) RetryPolicy() *RetryPolicy {
	return s.retryPolicy
}

// WithExecute устанавливает execute action
func (s *BaseStep) WithExecute(action func(ctx context.Context, sagaCtx SagaContext) error) *BaseStep {
	s.executeAction = action
	return s
}

// WithCompensate устанавливает compensate action
func (s *BaseStep) WithCompensate(action func(ctx context.Context, sagaCtx SagaContext) error) *BaseStep {
	s.compensateAction = action
	return s
}

// WithGuard устанавливает guard функцию
func (s *BaseStep) WithGuard(guard func(ctx context.Context, sagaCtx SagaContext) bool) *BaseStep {
	s.guard = guard
	return s
}

// WithTimeout устанавливает timeout
func (s *BaseStep) WithTimeout(timeout time.Duration) *BaseStep {
	s.timeout = timeout
	return s
}

// WithRetry устанавливает retry policy
func (s *BaseStep) WithRetry(policy *RetryPolicy) *BaseStep {
	s.retryPolicy = policy
	return s
}

// WithMetadata добавляет метаданные
func (s *BaseStep) WithMetadata(key string, value interface{}) *BaseStep {
	if s.metadata == nil {
		s.metadata = make(map[string]interface{})
	}
	s.metadata[key] = value
	return s
}

// CommandStep шаг для выполнения команды через CommandBus
type CommandStep struct {
	*BaseStep
	commandBus     transport.CommandBus
	forwardCommand transport.Command
	compensateCommand transport.Command
	invoker        *invoke.CommandInvoker[transport.Command, events.Event, invoke.ErrorEvent]
}

// NewCommandStep создает новый CommandStep
func NewCommandStep(
	name string,
	commandBus transport.CommandBus,
	forwardCommand transport.Command,
	compensateCommand transport.Command,
) *CommandStep {
	step := &CommandStep{
		BaseStep:         NewBaseStep(name),
		commandBus:       commandBus,
		forwardCommand:   forwardCommand,
		compensateCommand: compensateCommand,
	}

	// Устанавливаем execute action
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		// Добавляем correlation ID в контекст
		if correlationID := sagaCtx.CorrelationID(); correlationID != "" {
			ctx = invoke.WithCorrelationID(ctx, correlationID)
		}

		// Отправляем команду через CommandBus
		return commandBus.Send(ctx, forwardCommand)
	})

	// Устанавливаем compensate action
	if compensateCommand != nil {
		step.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
			if correlationID := sagaCtx.CorrelationID(); correlationID != "" {
				ctx = invoke.WithCorrelationID(ctx, correlationID)
			}
			return commandBus.Send(ctx, compensateCommand)
		})
	}

	return step
}

// EventStep шаг для публикации события через EventBus
type EventStep struct {
	*BaseStep
	eventBus      events.EventBus
	event         events.Event
	compensateEvent events.Event
}

// NewEventStep создает новый EventStep
func NewEventStep(
	name string,
	eventBus events.EventBus,
	event events.Event,
) *EventStep {
	step := &EventStep{
		BaseStep: NewBaseStep(name),
		eventBus: eventBus,
		event:    event,
	}

	// Устанавливаем execute action
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		// Добавляем correlation ID в событие
		if correlationID := sagaCtx.CorrelationID(); correlationID != "" {
			if baseEvent, ok := event.(*events.BaseEvent); ok {
				baseEvent.WithCorrelationID(correlationID)
			}
		}

		return eventBus.Publish(ctx, event)
	})

	// Compensate для EventStep обычно no-op, но можно опубликовать compensate event
	step.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		if step.compensateEvent != nil {
			if correlationID := sagaCtx.CorrelationID(); correlationID != "" {
				if baseEvent, ok := step.compensateEvent.(*events.BaseEvent); ok {
					baseEvent.WithCorrelationID(correlationID)
				}
			}
			return eventBus.Publish(ctx, step.compensateEvent)
		}
		return nil
	})

	return step
}

// WithCompensateEvent устанавливает событие компенсации
func (s *EventStep) WithCompensateEvent(event events.Event) *EventStep {
	s.compensateEvent = event
	return s
}

// TwoPhaseCommitStep шаг для интеграции с 2PC координатором
type TwoPhaseCommitStep struct {
	*BaseStep
	coordinator   TwoPhaseCommitCoordinator
	participants  func(ctx context.Context, sagaCtx SagaContext) []TwoPhaseCommitParticipant
	transactionID string
}

// TwoPhaseCommitCoordinator интерфейс координатора 2PC
type TwoPhaseCommitCoordinator interface {
	Execute(ctx context.Context, transactionID string, participants []TwoPhaseCommitParticipant) error
}

// TwoPhaseCommitParticipant интерфейс участника 2PC
type TwoPhaseCommitParticipant interface {
	Prepare(ctx context.Context, transactionID string) error
	Commit(ctx context.Context, transactionID string) error
	Abort(ctx context.Context, transactionID string) error
}

// NewTwoPhaseCommitStep создает новый TwoPhaseCommitStep
func NewTwoPhaseCommitStep(
	name string,
	coordinator TwoPhaseCommitCoordinator,
	participants func(ctx context.Context, sagaCtx SagaContext) []TwoPhaseCommitParticipant,
) *TwoPhaseCommitStep {
	step := &TwoPhaseCommitStep{
		BaseStep:     NewBaseStep(name),
		coordinator:  coordinator,
		participants: participants,
	}

	// Устанавливаем execute action
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		// Генерируем transaction ID если его нет
		transactionID := sagaCtx.GetString("transaction_id")
		if transactionID == "" {
			transactionID = fmt.Sprintf("txn-%d", time.Now().UnixNano())
			sagaCtx.Set("transaction_id", transactionID)
		}

		// Получаем participants
		participantsList := participants(ctx, sagaCtx)
		if len(participantsList) == 0 {
			return fmt.Errorf("no participants for 2PC transaction")
		}

		// Выполняем 2PC через координатор
		return coordinator.Execute(ctx, transactionID, participantsList)
	})

	// Устанавливаем compensate action (abort)
	step.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		transactionID := sagaCtx.GetString("transaction_id")
		if transactionID == "" {
			return nil // Нет транзакции для отката
		}

		participantsList := participants(ctx, sagaCtx)
		// Выполняем abort для всех participants
		for _, participant := range participantsList {
			if err := participant.Abort(ctx, transactionID); err != nil {
				return fmt.Errorf("failed to abort participant: %w", err)
			}
		}

		return nil
	})

	return step
}

// ParallelStep шаг для параллельного выполнения нескольких шагов
type ParallelStep struct {
	*BaseStep
	steps []SagaStep
}

// NewParallelStep создает новый ParallelStep
func NewParallelStep(name string, steps ...SagaStep) *ParallelStep {
	parallelStep := &ParallelStep{
		BaseStep: NewBaseStep(name),
		steps:    steps,
	}

	// Устанавливаем execute action для параллельного выполнения
	parallelStep.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		type stepResult struct {
			index int
			err   error
		}

		resultCh := make(chan stepResult, len(steps))
		
		// Запускаем все шаги параллельно
		for i, step := range steps {
			go func(idx int, st SagaStep) {
				err := st.Execute(ctx, sagaCtx)
				resultCh <- stepResult{index: idx, err: err}
			}(i, step)
		}

		// Собираем результаты
		var errors []error
		for i := 0; i < len(steps); i++ {
			result := <-resultCh
			if result.err != nil {
				errors = append(errors, fmt.Errorf("step %s failed: %w", steps[result.index].Name(), result.err))
			}
		}

		if len(errors) > 0 {
			return fmt.Errorf("parallel execution failed: %v", errors)
		}

		return nil
	})

	// Устанавливаем compensate action для параллельной компенсации
	parallelStep.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		type stepResult struct {
			index int
			err   error
		}

		resultCh := make(chan stepResult, len(steps))
		
		// Компенсируем все шаги параллельно (в обратном порядке)
		for i := len(steps) - 1; i >= 0; i-- {
			go func(idx int, st SagaStep) {
				err := st.Compensate(ctx, sagaCtx)
				resultCh <- stepResult{index: idx, err: err}
			}(i, steps[i])
		}

		// Собираем результаты
		var errors []error
		for i := 0; i < len(steps); i++ {
			result := <-resultCh
			if result.err != nil {
				errors = append(errors, fmt.Errorf("compensation for step %s failed: %w", steps[result.index].Name(), result.err))
			}
		}

		if len(errors) > 0 {
			return fmt.Errorf("parallel compensation failed: %v", errors)
		}

		return nil
	})

	return parallelStep
}

// ConditionalStep шаг с условным выполнением
type ConditionalStep struct {
	*BaseStep
	condition func(ctx context.Context, sagaCtx SagaContext) bool
	step      SagaStep
}

// NewConditionalStep создает новый ConditionalStep
func NewConditionalStep(
	name string,
	condition func(ctx context.Context, sagaCtx SagaContext) bool,
	step SagaStep,
) *ConditionalStep {
	conditionalStep := &ConditionalStep{
		BaseStep:  NewBaseStep(name),
		condition: condition,
		step:      step,
	}

	// Устанавливаем guard
	conditionalStep.WithGuard(condition)

	// Устанавливаем execute action
	conditionalStep.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		if !condition(ctx, sagaCtx) {
			return nil // Условие не выполнено, пропускаем шаг
		}
		return step.Execute(ctx, sagaCtx)
	})

	// Устанавливаем compensate action
	conditionalStep.WithCompensate(func(ctx context.Context, sagaCtx SagaContext) error {
		if !condition(ctx, sagaCtx) {
			return nil // Условие не выполнено, компенсация не нужна
		}
		return step.Compensate(ctx, sagaCtx)
	})

	return conditionalStep
}

