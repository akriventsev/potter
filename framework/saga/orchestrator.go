// Package saga предоставляет SagaOrchestrator для координации выполнения саг.
package saga

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/metrics"
)

// SagaOrchestrator интерфейс оркестратора саг
type SagaOrchestrator interface {
	// Execute запускает выполнение саги
	Execute(ctx context.Context, saga Saga) error
	// Compensate запускает компенсацию саги
	Compensate(ctx context.Context, saga Saga) error
	// Resume возобновляет выполнение саги после сбоя
	Resume(ctx context.Context, sagaID string) error
	// GetStatus возвращает статус саги
	GetStatus(ctx context.Context, sagaID string) (SagaStatus, error)
	// Cancel отменяет выполнение саги
	Cancel(ctx context.Context, sagaID string) error
}

// DefaultOrchestrator реализация оркестратора по умолчанию
type DefaultOrchestrator struct {
	mu          sync.RWMutex
	persistence SagaPersistence
	eventBus    events.EventBus
	metrics     *metrics.Metrics
	registry    *SagaRegistry
	runningSagas map[string]context.CancelFunc
}

// NewDefaultOrchestrator создает новый оркестратор
func NewDefaultOrchestrator(persistence SagaPersistence, eventBus events.EventBus) *DefaultOrchestrator {
	return &DefaultOrchestrator{
		persistence:  persistence,
		eventBus:     eventBus,
		registry:     NewSagaRegistry(),
		runningSagas: make(map[string]context.CancelFunc),
	}
}

// WithRegistry устанавливает реестр саг
func (o *DefaultOrchestrator) WithRegistry(registry *SagaRegistry) *DefaultOrchestrator {
	o.registry = registry
	return o
}

// RegisterSaga регистрирует определение саги в реестре
func (o *DefaultOrchestrator) RegisterSaga(name string, definition SagaDefinition) error {
	if o.registry == nil {
		o.registry = NewSagaRegistry()
	}
	return o.registry.RegisterSaga(name, definition)
}

// StartSaga convenience-метод для запуска саги по имени definition
// Автоматически получает definition из registry, создает instance и запускает выполнение
func (o *DefaultOrchestrator) StartSaga(ctx context.Context, definitionName string, sagaCtx SagaContext) (Saga, error) {
	if o.registry == nil {
		return nil, fmt.Errorf("registry not configured")
	}

	// Получаем definition из registry
	definition, err := o.registry.GetSaga(definitionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get saga definition: %w", err)
	}

	// Создаем instance с persistence и eventBus
	var instance Saga
	if baseDef, ok := definition.(*BaseSagaDefinition); ok {
		instance = baseDef.CreateInstanceWithPersistenceAndEventBus(ctx, sagaCtx, o.persistence, o.eventBus)
	} else {
		instance = definition.CreateInstance(ctx, sagaCtx)
	}

	if instance == nil {
		return nil, fmt.Errorf("failed to create saga instance")
	}

	sagaID := instance.ID()

	// Создаем контекст с отменой для саги заранее
	sagaContext, cancel := context.WithCancel(ctx)
	o.mu.Lock()
	o.runningSagas[sagaID] = cancel
	o.mu.Unlock()

	// Запускаем выполнение в горутине для асинхронности
	go func() {
		if err := o.Execute(sagaContext, instance); err != nil {
			// Ошибка уже залогирована в Execute
		}
	}()

	return instance, nil
}

// RegisterDefinition алиас для RegisterSaga для обратной совместимости
func (o *DefaultOrchestrator) RegisterDefinition(definition SagaDefinition) error {
	return o.RegisterSaga(definition.Name(), definition)
}

// WithMetrics добавляет метрики к оркестратору
func (o *DefaultOrchestrator) WithMetrics(m *metrics.Metrics) *DefaultOrchestrator {
	o.metrics = m
	return o
}

func (o *DefaultOrchestrator) Execute(ctx context.Context, saga Saga) error {
	sagaID := saga.ID()

	// Устанавливаем eventBus в сагу, если она поддерживает это
	if baseSaga, ok := saga.(*BaseSaga); ok && baseSaga.eventBus == nil && o.eventBus != nil {
		baseSaga.mu.Lock()
		baseSaga.eventBus = o.eventBus
		baseSaga.mu.Unlock()
	}

	// Проверяем, есть ли уже контекст с отменой для этой саги
	o.mu.Lock()
	_, exists := o.runningSagas[sagaID]
	o.mu.Unlock()

	var sagaCtx context.Context
	if exists {
		// Контекст уже создан в StartSaga, используем переданный контекст
		sagaCtx = ctx
	} else {
		// Создаем контекст с отменой для саги (для прямых вызовов Execute)
		var cancel context.CancelFunc
		sagaCtx, cancel = context.WithCancel(ctx)
		o.mu.Lock()
		o.runningSagas[sagaID] = cancel
		o.mu.Unlock()
	}

	// Публикуем событие начала саги
	if o.eventBus != nil {
		startedEvent := &SagaStartedEvent{
			BaseEvent: events.NewBaseEvent("SagaStarted", sagaID),
			SagaID:    sagaID,
			DefinitionName: saga.Definition().Name(),
			Timestamp: time.Now(),
			CorrelationID: saga.Context().CorrelationID(),
		}
		startedEvent.WithCorrelationID(saga.Context().CorrelationID())
		_ = o.eventBus.Publish(ctx, startedEvent)
	}

	// Записываем метрику
	if o.metrics != nil {
		o.metrics.RecordEvent(ctx, "saga.started")
	}

	// Выполняем сагу
	err := saga.Execute(sagaCtx)

	// Удаляем из running sagas
	o.mu.Lock()
	delete(o.runningSagas, sagaID)
	o.mu.Unlock()

	// Публикуем событие завершения
	if o.eventBus != nil {
		if err != nil {
			failedEvent := &SagaFailedEvent{
				BaseEvent: events.NewBaseEvent("SagaFailed", sagaID),
				SagaID:    sagaID,
				Error:     err.Error(),
				FailedStep: saga.CurrentStep(),
				Timestamp: time.Now(),
			}
			failedEvent.WithCorrelationID(saga.Context().CorrelationID())
			_ = o.eventBus.Publish(ctx, failedEvent)

			if o.metrics != nil {
				o.metrics.RecordEvent(ctx, "saga.failed")
			}
		} else {
			metadata := saga.Context().Metadata()
			duration := time.Duration(0)
			if !metadata.CreatedAt.IsZero() {
				duration = time.Since(metadata.CreatedAt)
			}
			
			completedEvent := &SagaCompletedEvent{
				BaseEvent: events.NewBaseEvent("SagaCompleted", sagaID),
				SagaID:    sagaID,
				Duration:  duration,
				StepsCompleted: len(saga.GetHistory()),
				Timestamp: time.Now(),
			}
			completedEvent.WithCorrelationID(saga.Context().CorrelationID())
			_ = o.eventBus.Publish(ctx, completedEvent)

			if o.metrics != nil {
				o.metrics.RecordEvent(ctx, "saga.completed")
			}
		}
	}

	// Сохраняем финальное состояние
	if o.persistence != nil {
		_ = o.persistence.Save(ctx, saga)
	}

	return err
}

func (o *DefaultOrchestrator) Compensate(ctx context.Context, saga Saga) error {
	sagaID := saga.ID()

	// Публикуем событие начала компенсации
	if o.eventBus != nil {
		compensatingEvent := &SagaCompensatingEvent{
			BaseEvent: events.NewBaseEvent("SagaCompensating", sagaID),
			SagaID:    sagaID,
			Reason:    "manual_compensation",
			Timestamp: time.Now(),
		}
		compensatingEvent.WithCorrelationID(saga.Context().CorrelationID())
		_ = o.eventBus.Publish(ctx, compensatingEvent)
	}

	// Выполняем компенсацию
	err := saga.Compensate(ctx)

	// Публикуем событие завершения компенсации
	if o.eventBus != nil {
		if err != nil {
			// Компенсация не удалась
			if o.metrics != nil {
				o.metrics.RecordEvent(ctx, "saga.compensation.failed")
			}
		} else {
			compensatedEvent := &SagaCompensatedEvent{
				BaseEvent: events.NewBaseEvent("SagaCompensated", sagaID),
				SagaID:    sagaID,
				CompensatedSteps: len(saga.GetHistory()),
				Timestamp: time.Now(),
			}
			compensatedEvent.WithCorrelationID(saga.Context().CorrelationID())
			_ = o.eventBus.Publish(ctx, compensatedEvent)

			if o.metrics != nil {
				o.metrics.RecordEvent(ctx, "saga.compensated")
			}
		}
	}

	// Сохраняем финальное состояние
	if o.persistence != nil {
		_ = o.persistence.Save(ctx, saga)
	}

	return err
}

func (o *DefaultOrchestrator) Resume(ctx context.Context, sagaID string) error {
	// Загружаем сагу из persistence
	if o.persistence == nil {
		return fmt.Errorf("persistence not configured, cannot resume saga")
	}

	saga, err := o.persistence.Load(ctx, sagaID)
	if err != nil {
		return fmt.Errorf("failed to load saga %s: %w", sagaID, err)
	}

	// Проверяем статус
	status := saga.Status()
	if status != SagaStatusRunning && status != SagaStatusPending {
		return fmt.Errorf("saga %s cannot be resumed, current status: %s", sagaID, status)
	}

	// Возобновляем выполнение
	return o.Execute(ctx, saga)
}

func (o *DefaultOrchestrator) GetStatus(ctx context.Context, sagaID string) (SagaStatus, error) {
	if o.persistence == nil {
		return "", fmt.Errorf("persistence not configured, cannot get saga status")
	}

	saga, err := o.persistence.Load(ctx, sagaID)
	if err != nil {
		return "", fmt.Errorf("failed to load saga %s: %w", sagaID, err)
	}

	return saga.Status(), nil
}

func (o *DefaultOrchestrator) Cancel(ctx context.Context, sagaID string) error {
	// Отменяем выполнение через cancel функцию
	o.mu.Lock()
	cancel, exists := o.runningSagas[sagaID]
	o.mu.Unlock()

	if !exists {
		return fmt.Errorf("saga %s is not running", sagaID)
	}

	// Отменяем контекст
	cancel()

	// Загружаем сагу и помечаем как отмененную
	if o.persistence != nil {
		saga, err := o.persistence.Load(ctx, sagaID)
		if err == nil {
			// Можно добавить специальный статус для отмененных саг
			// Пока просто сохраняем текущее состояние
			_ = o.persistence.Save(ctx, saga)
		}
	}

	// Удаляем из running sagas
	o.mu.Lock()
	delete(o.runningSagas, sagaID)
	o.mu.Unlock()

	return nil
}

