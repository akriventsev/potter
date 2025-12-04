// Package saga предоставляет реализацию Saga Pattern через FSM для оркестрации долгоживущих транзакций.
package saga

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/fsm"
	"github.com/akriventsev/potter/framework/invoke"

	"github.com/google/uuid"
)

// SagaStatus статус выполнения саги
type SagaStatus string

const (
	SagaStatusPending      SagaStatus = "pending"
	SagaStatusRunning      SagaStatus = "running"
	SagaStatusCompleted    SagaStatus = "completed"
	SagaStatusCompensating SagaStatus = "compensating"
	SagaStatusCompensated  SagaStatus = "compensated"
	SagaStatusFailed       SagaStatus = "failed"
)

// Saga основной интерфейс саги
type Saga interface {
	// ID возвращает уникальный идентификатор саги
	ID() string
	// CurrentStep возвращает текущий шаг выполнения
	CurrentStep() string
	// Status возвращает текущий статус саги
	Status() SagaStatus
	// Execute запускает выполнение саги
	Execute(ctx context.Context) error
	// Compensate запускает компенсацию саги
	Compensate(ctx context.Context) error
	// GetHistory возвращает историю выполнения шагов
	GetHistory() []SagaHistory
	// Definition возвращает определение саги
	Definition() SagaDefinition
	// Context возвращает контекст саги
	Context() SagaContext
}

// SagaDefinition определение саги с шагами
type SagaDefinition interface {
	// Name возвращает имя определения саги
	Name() string
	// Steps возвращает все шаги саги
	Steps() []SagaStep
	// AddStep добавляет шаг в сагу
	AddStep(step SagaStep) SagaDefinition
	// Build строит FSM из определения
	Build() (*fsm.FSM, error)
	// CreateInstance создает экземпляр саги
	CreateInstance(ctx context.Context, sagaCtx SagaContext) (Saga, error)
}

// SagaContext контекст выполнения саги с данными и метаданными
type SagaContext interface {
	// Get получает значение по ключу
	Get(key string) interface{}
	// Set устанавливает значение по ключу
	Set(key string, value interface{})
	// GetString получает строковое значение
	GetString(key string) string
	// GetInt получает целочисленное значение
	GetInt(key string) int
	// GetBool получает булево значение
	GetBool(key string) bool
	// GetFloat64 получает значение float64
	GetFloat64(key string) float64
	// GetStringSlice получает слайс строк
	GetStringSlice(key string) []string
	// Metadata возвращает указатель на копию метаданных (snapshot).
	// Метаданные являются read-only snapshot и не должны изменяться напрямую.
	// Для изменения метаданных используйте методы SetTimeout, SetRetryPolicy, SetCustomValue.
	Metadata() *SagaMetadata
	// SetTimeout устанавливает timeout для саги
	SetTimeout(timeout time.Duration)
	// SetRetryPolicy устанавливает политику повторов для саги
	SetRetryPolicy(policy *RetryPolicy)
	// SetCustomValue устанавливает кастомное значение в метаданных
	SetCustomValue(key string, value interface{})
	// CorrelationID возвращает correlation ID
	CorrelationID() string
	// SetCorrelationID устанавливает correlation ID
	SetCorrelationID(id string)
	// ToMap преобразует контекст в map
	ToMap() map[string]interface{}
	// FromMap восстанавливает контекст из map
	FromMap(data map[string]interface{}) error
}

// SagaMetadata метаданные саги
type SagaMetadata struct {
	Timeout       time.Duration
	RetryPolicy   *RetryPolicy
	CorrelationID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Custom        map[string]interface{}
}

// SagaHistory запись истории выполнения шага
type SagaHistory struct {
	StepName     string
	Status       StepStatus
	StartedAt    time.Time
	CompletedAt  *time.Time
	Error        error
	RetryAttempt int
}

// StepStatus статус выполнения шага
type StepStatus string

const (
	StepStatusPending      StepStatus = "pending"
	StepStatusRunning      StepStatus = "running"
	StepStatusCompleted    StepStatus = "completed"
	StepStatusFailed       StepStatus = "failed"
	StepStatusCompensating StepStatus = "compensating"
	StepStatusCompensated  StepStatus = "compensated"
)

// BaseSaga базовая реализация саги
type BaseSaga struct {
	mu          sync.RWMutex
	id          string
	definition  SagaDefinition
	fsm         *fsm.FSM
	context     SagaContext
	status      SagaStatus
	history     []SagaHistory
	persistence SagaPersistence
	eventBus    events.EventBus
	currentStep string
	startedAt   time.Time
	completedAt *time.Time
}

// NewBaseSaga создает новую базовую сагу
func NewBaseSaga(id string, definition SagaDefinition, sagaCtx SagaContext, persistence SagaPersistence) (*BaseSaga, error) {
	return NewBaseSagaWithEventBus(id, definition, sagaCtx, persistence, nil)
}

// NewBaseSagaWithEventBus создает новую базовую сагу с EventBus
func NewBaseSagaWithEventBus(id string, definition SagaDefinition, sagaCtx SagaContext, persistence SagaPersistence, eventBus events.EventBus) (*BaseSaga, error) {
	fsmInstance, err := definition.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build FSM for saga definition: %w", err)
	}

	now := time.Now()

	// Заполняем метаданные контекста
	if ctxImpl, ok := sagaCtx.(*SagaContextImpl); ok {
		ctxImpl.mu.Lock()
		if ctxImpl.metadata.CreatedAt.IsZero() {
			ctxImpl.metadata.CreatedAt = now
		}
		ctxImpl.metadata.UpdatedAt = now
		ctxImpl.mu.Unlock()
	}

	return &BaseSaga{
		id:          id,
		definition:  definition,
		fsm:         fsmInstance,
		context:     sagaCtx,
		status:      SagaStatusPending,
		history:     make([]SagaHistory, 0),
		persistence: persistence,
		eventBus:    eventBus,
		startedAt:   now,
	}, nil
}

func (s *BaseSaga) ID() string {
	return s.id
}

func (s *BaseSaga) CurrentStep() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentStep
}

func (s *BaseSaga) Status() SagaStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *BaseSaga) Definition() SagaDefinition {
	return s.definition
}

func (s *BaseSaga) Context() SagaContext {
	return s.context
}

func (s *BaseSaga) GetHistory() []SagaHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SagaHistory, len(s.history))
	copy(result, s.history)
	return result
}

func (s *BaseSaga) Execute(ctx context.Context) error {
	s.mu.Lock()
	if s.status != SagaStatusPending {
		s.mu.Unlock()
		return fmt.Errorf("saga %s is not in pending status, current: %s", s.id, s.status)
	}
	now := time.Now()
	s.status = SagaStatusRunning
	s.startedAt = now
	s.mu.Unlock()

	// Обновляем метаданные
	if ctxImpl, ok := s.context.(*SagaContextImpl); ok {
		ctxImpl.mu.Lock()
		ctxImpl.metadata.UpdatedAt = now
		ctxImpl.mu.Unlock()
	}

	// Сохраняем начальное состояние
	if s.persistence != nil {
		if err := s.persistence.Save(ctx, s); err != nil {
			return fmt.Errorf("failed to save saga state: %w", err)
		}
	}

	// Инициализируем FSM - переходим из initial в первый шаг
	if s.fsm != nil {
		initialEvent := fsm.NewEvent("start", nil)
		if err := s.fsm.Trigger(ctx, initialEvent); err != nil {
			// Если FSM не поддерживает start event, игнорируем ошибку
			// Это может быть нормально, если FSM уже в нужном состоянии
		}
	}

	// Выполняем шаги последовательно
	steps := s.definition.Steps()
	for i, step := range steps {
		s.mu.Lock()
		s.currentStep = step.Name()
		s.mu.Unlock()

		// Триггерим событие FSM для перехода к шагу
		if s.fsm != nil {
			stepEventName := fmt.Sprintf("execute_%s", step.Name())
			stepEvent := fsm.NewEvent(stepEventName, map[string]interface{}{
				"step_name": step.Name(),
				"saga_id":   s.id,
			})
			if err := s.fsm.Trigger(ctx, stepEvent); err != nil {
				// Если переход не удался, логируем, но продолжаем выполнение
				// Это может быть нормально, если FSM уже в нужном состоянии
			}
		}

		// Добавляем запись в историю
		stepStartedAt := time.Now()
		historyEntry := SagaHistory{
			StepName:     step.Name(),
			Status:       StepStatusRunning,
			StartedAt:    stepStartedAt,
			RetryAttempt: 0,
		}
		s.addHistory(historyEntry)

		// Публикуем событие начала шага
		if s.eventBus != nil {
			stepStartedEvent := &StepStartedEvent{
				BaseEvent: events.NewBaseEvent("StepStarted", s.id),
				SagaID:    s.id,
				StepName:  step.Name(),
				Timestamp: stepStartedAt,
			}
			stepStartedEvent.WithCorrelationID(s.context.CorrelationID())
			_ = s.eventBus.Publish(ctx, stepStartedEvent)
		}

		// Проверяем guard
		if !step.CanExecute(ctx, s.context) {
			s.mu.Lock()
			s.status = SagaStatusFailed
			s.mu.Unlock()
			historyEntry.Status = StepStatusFailed
			historyEntry.Error = fmt.Errorf("step %s guard check failed", step.Name())
			now := time.Now()
			historyEntry.CompletedAt = &now
			s.updateHistory(historyEntry)

			// Триггерим событие ошибки в FSM
			if s.fsm != nil {
				errorEvent := fsm.NewEvent("step_failed", map[string]interface{}{
					"step_name": step.Name(),
					"error":     "guard check failed",
				})
				_ = s.fsm.Trigger(ctx, errorEvent)
			}

			return fmt.Errorf("step %s guard check failed", step.Name())
		}

		// Выполняем шаг с retry
		var stepErr error
		retryPolicy := step.RetryPolicy()
		if retryPolicy == nil {
			retryPolicy = NoRetry()
		}

		for attempt := 0; attempt < retryPolicy.MaxAttempts; attempt++ {
			historyEntry.RetryAttempt = attempt

			// Создаем контекст с timeout если задан
			stepCtx := ctx
			var cancel context.CancelFunc
			if timeout := step.Timeout(); timeout > 0 {
				stepCtx, cancel = context.WithTimeout(ctx, timeout)
			}

			stepErr = step.Execute(stepCtx, s.context)

			// Явно отменяем контекст после выполнения шага
			if cancel != nil {
				cancel()
			}

			if stepErr == nil {
				break
			}

			// Проверяем, нужно ли повторять
			if !retryPolicy.ShouldRetry(stepErr, attempt) {
				break
			}

			// Ждем перед повтором
			if attempt < retryPolicy.MaxAttempts-1 {
				delay := retryPolicy.CalculateDelay(attempt)
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		if stepErr != nil {
			// Ошибка выполнения шага - запускаем компенсацию
			stepFailedAt := time.Now()
			historyEntry.Status = StepStatusFailed
			historyEntry.Error = stepErr
			historyEntry.CompletedAt = &stepFailedAt
			s.updateHistory(historyEntry)

			// Публикуем событие ошибки шага
			if s.eventBus != nil {
				stepFailedEvent := &StepFailedEvent{
					BaseEvent:    events.NewBaseEvent("StepFailed", s.id),
					SagaID:       s.id,
					StepName:     step.Name(),
					Error:        stepErr.Error(),
					RetryAttempt: historyEntry.RetryAttempt,
					Timestamp:    stepFailedAt,
				}
				stepFailedEvent.WithCorrelationID(s.context.CorrelationID())
				_ = s.eventBus.Publish(ctx, stepFailedEvent)
			}

			// Триггерим событие ошибки в FSM
			if s.fsm != nil {
				errorEvent := fsm.NewEvent("step_failed", map[string]interface{}{
					"step_name": step.Name(),
					"error":     stepErr.Error(),
				})
				_ = s.fsm.Trigger(ctx, errorEvent)
			}

			// Компенсируем выполненные шаги в обратном порядке
			compensateErr := s.compensateSteps(ctx, i-1)
			if compensateErr != nil {
				s.mu.Lock()
				s.status = SagaStatusFailed
				s.mu.Unlock()
				return fmt.Errorf("step %s failed: %w, compensation also failed: %w", step.Name(), stepErr, compensateErr)
			}

			s.mu.Lock()
			s.status = SagaStatusCompensated
			now2 := time.Now()
			s.completedAt = &now2
			s.mu.Unlock()

			if s.persistence != nil {
				_ = s.persistence.Save(ctx, s)
			}

			return fmt.Errorf("step %s failed: %w", step.Name(), stepErr)
		}

		// Шаг выполнен успешно
		stepCompletedAt := time.Now()
		historyEntry.Status = StepStatusCompleted
		historyEntry.CompletedAt = &stepCompletedAt
		s.updateHistory(historyEntry)

		// Публикуем событие успешного завершения шага
		if s.eventBus != nil {
			duration := stepCompletedAt.Sub(stepStartedAt)
			stepCompletedEvent := &StepCompletedEvent{
				BaseEvent: events.NewBaseEvent("StepCompleted", s.id),
				SagaID:    s.id,
				StepName:  step.Name(),
				Duration:  duration,
				Timestamp: stepCompletedAt,
			}
			stepCompletedEvent.WithCorrelationID(s.context.CorrelationID())
			_ = s.eventBus.Publish(ctx, stepCompletedEvent)
		}

		// Триггерим событие успешного завершения шага в FSM
		if s.fsm != nil {
			completedEvent := fsm.NewEvent("step_completed", map[string]interface{}{
				"step_name": step.Name(),
			})
			_ = s.fsm.Trigger(ctx, completedEvent)
		}

		// Обновляем метаданные
		if ctxImpl, ok := s.context.(*SagaContextImpl); ok {
			ctxImpl.mu.Lock()
			ctxImpl.metadata.UpdatedAt = stepCompletedAt
			ctxImpl.mu.Unlock()
		}

		// Сохраняем состояние после каждого шага
		if s.persistence != nil {
			if err := s.persistence.Save(ctx, s); err != nil {
				return fmt.Errorf("failed to save saga state after step %s: %w", step.Name(), err)
			}
		}
	}

	// Все шаги выполнены успешно
	s.mu.Lock()
	s.status = SagaStatusCompleted
	now = time.Now()
	s.completedAt = &now
	s.mu.Unlock()

	// Триггерим событие завершения саги в FSM
	if s.fsm != nil {
		completedEvent := fsm.NewEvent("saga_completed", nil)
		_ = s.fsm.Trigger(ctx, completedEvent)
	}

	// Обновляем метаданные
	if ctxImpl, ok := s.context.(*SagaContextImpl); ok {
		ctxImpl.mu.Lock()
		ctxImpl.metadata.UpdatedAt = now
		ctxImpl.mu.Unlock()
	}

	if s.persistence != nil {
		_ = s.persistence.Save(ctx, s)
	}

	return nil
}

func (s *BaseSaga) Compensate(ctx context.Context) error {
	s.mu.Lock()
	if s.status != SagaStatusRunning && s.status != SagaStatusCompleted {
		s.mu.Unlock()
		return fmt.Errorf("saga %s cannot be compensated, current status: %s", s.id, s.status)
	}
	now := time.Now()
	s.status = SagaStatusCompensating
	s.mu.Unlock()

	// Обновляем метаданные
	if ctxImpl, ok := s.context.(*SagaContextImpl); ok {
		ctxImpl.mu.Lock()
		ctxImpl.metadata.UpdatedAt = now
		ctxImpl.mu.Unlock()
	}

	steps := s.definition.Steps()
	return s.compensateSteps(ctx, len(steps)-1)
}

// compensateSteps компенсирует шаги в обратном порядке
func (s *BaseSaga) compensateSteps(ctx context.Context, lastStepIndex int) error {
	steps := s.definition.Steps()

	// Получаем копию истории под блокировкой
	s.mu.RLock()
	historyCopy := make([]SagaHistory, len(s.history))
	copy(historyCopy, s.history)
	s.mu.RUnlock()

	for i := lastStepIndex; i >= 0; i-- {
		step := steps[i]

		// Проверяем, был ли шаг выполнен
		wasExecuted := false
		for _, hist := range historyCopy {
			if hist.StepName == step.Name() && hist.Status == StepStatusCompleted {
				wasExecuted = true
				break
			}
		}

		if !wasExecuted {
			continue
		}

		s.mu.Lock()
		s.currentStep = step.Name()
		s.mu.Unlock()

		// Добавляем запись в историю
		stepCompensatingAt := time.Now()
		historyEntry := SagaHistory{
			StepName:     step.Name(),
			Status:       StepStatusCompensating,
			StartedAt:    stepCompensatingAt,
			RetryAttempt: 0,
		}
		s.addHistory(historyEntry)

		// Публикуем событие начала компенсации шага
		if s.eventBus != nil {
			stepCompensatingEvent := &StepCompensatingEvent{
				BaseEvent: events.NewBaseEvent("StepCompensating", s.id),
				SagaID:    s.id,
				StepName:  step.Name(),
				Timestamp: stepCompensatingAt,
			}
			stepCompensatingEvent.WithCorrelationID(s.context.CorrelationID())
			_ = s.eventBus.Publish(ctx, stepCompensatingEvent)
		}

		// Выполняем компенсацию
		compensateErr := step.Compensate(ctx, s.context)
		if compensateErr != nil {
			historyEntry.Status = StepStatusFailed
			historyEntry.Error = compensateErr
			now := time.Now()
			historyEntry.CompletedAt = &now
			s.updateHistory(historyEntry)

			s.mu.Lock()
			s.status = SagaStatusFailed
			s.mu.Unlock()

			if s.persistence != nil {
				_ = s.persistence.Save(ctx, s)
			}

			return fmt.Errorf("compensation failed for step %s: %w", step.Name(), compensateErr)
		}

		// Компенсация успешна
		stepCompensatedAt := time.Now()
		historyEntry.Status = StepStatusCompensated
		historyEntry.CompletedAt = &stepCompensatedAt
		s.updateHistory(historyEntry)

		// Публикуем событие завершения компенсации шага
		if s.eventBus != nil {
			stepCompensatedEvent := &StepCompensatedEvent{
				BaseEvent: events.NewBaseEvent("StepCompensated", s.id),
				SagaID:    s.id,
				StepName:  step.Name(),
				Timestamp: stepCompensatedAt,
			}
			stepCompensatedEvent.WithCorrelationID(s.context.CorrelationID())
			_ = s.eventBus.Publish(ctx, stepCompensatedEvent)
		}

		if s.persistence != nil {
			_ = s.persistence.Save(ctx, s)
		}
	}

	s.mu.Lock()
	s.status = SagaStatusCompensated
	now := time.Now()
	s.completedAt = &now
	s.mu.Unlock()

	// Обновляем метаданные
	if ctxImpl, ok := s.context.(*SagaContextImpl); ok {
		ctxImpl.mu.Lock()
		ctxImpl.metadata.UpdatedAt = now
		ctxImpl.mu.Unlock()
	}

	if s.persistence != nil {
		_ = s.persistence.Save(ctx, s)
	}

	return nil
}

func (s *BaseSaga) addHistory(entry SagaHistory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, entry)
}

func (s *BaseSaga) updateHistory(entry SagaHistory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.history) - 1; i >= 0; i-- {
		if s.history[i].StepName == entry.StepName && s.history[i].StartedAt.Equal(entry.StartedAt) {
			s.history[i] = entry
			return
		}
	}
}

// SagaContextImpl реализация SagaContext
type SagaContextImpl struct {
	mu            sync.RWMutex
	data          map[string]interface{}
	metadata      SagaMetadata
	correlationID string
}

// NewSagaContext создает новый контекст саги
func NewSagaContext() SagaContext {
	return &SagaContextImpl{
		data:     make(map[string]interface{}),
		metadata: SagaMetadata{Custom: make(map[string]interface{})},
	}
}

// NewSagaContextWithCorrelationID создает контекст с correlation ID
func NewSagaContextWithCorrelationID(correlationID string) SagaContext {
	ctx := NewSagaContext()
	ctx.SetCorrelationID(correlationID)
	return ctx
}

func (c *SagaContextImpl) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[key]
}

func (c *SagaContextImpl) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		c.data = make(map[string]interface{})
	}
	c.data[key] = value
}

func (c *SagaContextImpl) GetString(key string) string {
	val := c.Get(key)
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

func (c *SagaContextImpl) GetInt(key string) int {
	val := c.Get(key)
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func (c *SagaContextImpl) GetBool(key string) bool {
	val := c.Get(key)
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}

func (c *SagaContextImpl) GetFloat64(key string) float64 {
	val := c.Get(key)
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0.0
}

func (c *SagaContextImpl) GetStringSlice(key string) []string {
	val := c.Get(key)
	if val == nil {
		return nil
	}

	// Прямое приведение
	if strSlice, ok := val.([]string); ok {
		return strSlice
	}

	// Преобразование из []interface{}
	if interfaceSlice, ok := val.([]interface{}); ok {
		result := make([]string, 0, len(interfaceSlice))
		for _, item := range interfaceSlice {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	return nil
}

func (c *SagaContextImpl) Metadata() *SagaMetadata {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Возвращаем указатель на копию метаданных (snapshot) для безопасности
	// Изменения должны выполняться через специальные методы SetTimeout, SetRetryPolicy, SetCustomValue
	metadataCopy := c.metadata
	// Копируем вложенные структуры
	if metadataCopy.RetryPolicy != nil {
		retryPolicyCopy := *metadataCopy.RetryPolicy
		metadataCopy.RetryPolicy = &retryPolicyCopy
	}
	if metadataCopy.Custom != nil {
		customCopy := make(map[string]interface{})
		for k, v := range metadataCopy.Custom {
			customCopy[k] = v
		}
		metadataCopy.Custom = customCopy
	}
	return &metadataCopy
}

func (c *SagaContextImpl) SetTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata.Timeout = timeout
	c.metadata.UpdatedAt = time.Now()
}

func (c *SagaContextImpl) SetRetryPolicy(policy *RetryPolicy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata.RetryPolicy = policy
	c.metadata.UpdatedAt = time.Now()
}

func (c *SagaContextImpl) SetCustomValue(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.metadata.Custom == nil {
		c.metadata.Custom = make(map[string]interface{})
	}
	c.metadata.Custom[key] = value
	c.metadata.UpdatedAt = time.Now()
}

func (c *SagaContextImpl) CorrelationID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.correlationID
}

func (c *SagaContextImpl) SetCorrelationID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.correlationID = id
	c.metadata.CorrelationID = id
}

func (c *SagaContextImpl) ToMap() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range c.data {
		result[k] = v
	}
	result["correlation_id"] = c.correlationID
	return result
}

func (c *SagaContextImpl) FromMap(data map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]interface{})
	for k, v := range data {
		if k == "correlation_id" {
			if id, ok := v.(string); ok {
				c.correlationID = id
				c.metadata.CorrelationID = id
			}
		} else {
			c.data[k] = v
		}
	}
	return nil
}

// BaseSagaDefinition базовая реализация SagaDefinition
type BaseSagaDefinition struct {
	name  string
	steps []SagaStep
}

// NewBaseSagaDefinition создает новое определение саги
func NewBaseSagaDefinition(name string) *BaseSagaDefinition {
	return &BaseSagaDefinition{
		name:  name,
		steps: make([]SagaStep, 0),
	}
}

func (d *BaseSagaDefinition) Name() string {
	return d.name
}

func (d *BaseSagaDefinition) Steps() []SagaStep {
	return d.steps
}

func (d *BaseSagaDefinition) AddStep(step SagaStep) SagaDefinition {
	d.steps = append(d.steps, step)
	return d
}

func (d *BaseSagaDefinition) Build() (*fsm.FSM, error) {
	if len(d.steps) == 0 {
		return nil, fmt.Errorf("saga definition must have at least one step")
	}

	// Создаем начальное состояние
	initialState := fsm.NewBaseState("initial")
	fsmInstance := fsm.NewFSM(initialState)

	// Создаем состояния для каждого шага
	states := make([]fsm.State, len(d.steps)+1)
	states[0] = initialState

	for i, step := range d.steps {
		stateName := fmt.Sprintf("step_%s", step.Name())
		states[i+1] = fsm.NewBaseState(stateName)
		if err := fsmInstance.AddState(states[i+1]); err != nil {
			return nil, fmt.Errorf("failed to add state for step %s: %w", step.Name(), err)
		}
	}

	// Создаем переходы между шагами
	for i := 0; i < len(d.steps); i++ {
		fromState := states[i]
		toState := states[i+1]
		eventName := fmt.Sprintf("execute_%s", d.steps[i].Name())

		transition := fsm.NewTransition(fromState, toState, eventName)
		if err := fsmInstance.AddTransition(transition); err != nil {
			return nil, fmt.Errorf("failed to add transition for step %s: %w", d.steps[i].Name(), err)
		}
	}

	return fsmInstance, nil
}

func (d *BaseSagaDefinition) CreateInstance(ctx context.Context, sagaCtx SagaContext) (Saga, error) {
	return d.CreateInstanceWithPersistence(ctx, sagaCtx, nil)
}

// CreateInstanceWithPersistence создает экземпляр саги с persistence
func (d *BaseSagaDefinition) CreateInstanceWithPersistence(ctx context.Context, sagaCtx SagaContext, persistence SagaPersistence) (Saga, error) {
	return d.CreateInstanceWithPersistenceAndEventBus(ctx, sagaCtx, persistence, nil)
}

// CreateInstanceWithPersistenceAndEventBus создает экземпляр саги с persistence и eventBus
func (d *BaseSagaDefinition) CreateInstanceWithPersistenceAndEventBus(ctx context.Context, sagaCtx SagaContext, persistence SagaPersistence, eventBus events.EventBus) (Saga, error) {
	if sagaCtx == nil {
		sagaCtx = NewSagaContext()
	}

	// Генерируем correlation ID если его нет
	if sagaCtx.CorrelationID() == "" {
		correlationID := invoke.GenerateCorrelationID()
		sagaCtx.SetCorrelationID(correlationID)
	}

	sagaID := uuid.New().String()
	saga, err := NewBaseSagaWithEventBus(sagaID, d, sagaCtx, persistence, eventBus)
	if err != nil {
		return nil, fmt.Errorf("failed to create saga instance: %w", err)
	}
	return saga, nil
}
