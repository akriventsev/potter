// Package saga предоставляет фабрики для упрощенного создания компонентов Saga.
package saga

import (
	"context"
	"fmt"

	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/eventsourcing"
	"github.com/akriventsev/potter/framework/transport"
)

// OrchestratorConfig конфигурация оркестратора
type OrchestratorConfig struct {
	Persistence SagaPersistence
	EventBus   events.EventBus
	Metrics    interface{} // *metrics.Metrics
	Timeout    int        // в секундах
}

// OrchestratorFactory фабрика для создания оркестраторов
type OrchestratorFactory struct{}

// NewOrchestratorFactory создает новую фабрику оркестраторов
func NewOrchestratorFactory() *OrchestratorFactory {
	return &OrchestratorFactory{}
}

// NewDefaultOrchestrator создает orchestrator с дефолтными настройками
func (f *OrchestratorFactory) NewDefaultOrchestrator(
	persistence SagaPersistence,
	eventBus events.EventBus,
) SagaOrchestrator {
	return NewDefaultOrchestrator(persistence, eventBus)
}

// NewOrchestratorWithConfig создает orchestrator с кастомной конфигурацией
func (f *OrchestratorFactory) NewOrchestratorWithConfig(config OrchestratorConfig) (SagaOrchestrator, error) {
	if config.Persistence == nil {
		return nil, fmt.Errorf("persistence is required")
	}
	if config.EventBus == nil {
		return nil, fmt.Errorf("eventBus is required")
	}

	orchestrator := NewDefaultOrchestrator(config.Persistence, config.EventBus)

	// Применяем метрики если заданы
	// if config.Metrics != nil {
	// 	orchestrator.WithMetrics(config.Metrics)
	// }

	return orchestrator, nil
}

// PersistenceFactory фабрика для создания persistence
type PersistenceFactory struct{}

// NewPersistenceFactory создает новую фабрику persistence
func NewPersistenceFactory() *PersistenceFactory {
	return &PersistenceFactory{}
}

// NewInMemoryPersistence создает in-memory persistence для тестирования
func (f *PersistenceFactory) NewInMemoryPersistence() SagaPersistence {
	return NewInMemoryPersistence()
}

// NewEventStorePersistence создает EventStore persistence для production
func (f *PersistenceFactory) NewEventStorePersistence(
	eventStore eventsourcing.EventStore,
	snapshotStore eventsourcing.SnapshotStore,
) SagaPersistence {
	return NewEventStorePersistence(eventStore, snapshotStore)
}

// NewEventStorePersistenceWithRegistry создает EventStore persistence с реестром
func (f *PersistenceFactory) NewEventStorePersistenceWithRegistry(
	eventStore eventsourcing.EventStore,
	snapshotStore eventsourcing.SnapshotStore,
	registry *SagaRegistry,
) SagaPersistence {
	return NewEventStorePersistence(eventStore, snapshotStore).WithRegistry(registry)
}

// NewPostgresPersistence создает PostgreSQL persistence для production
func (f *PersistenceFactory) NewPostgresPersistence(dsn string) (SagaPersistence, error) {
	return NewPostgresPersistence(dsn)
}

// NewPostgresPersistenceWithRegistry создает PostgreSQL persistence с реестром
func (f *PersistenceFactory) NewPostgresPersistenceWithRegistry(dsn string, registry *SagaRegistry) (SagaPersistence, error) {
	p, err := NewPostgresPersistence(dsn)
	if err != nil {
		return nil, err
	}
	return p.WithRegistry(registry), nil
}

// StepFactory фабрика для создания различных типов шагов
type StepFactory struct{}

// NewStepFactory создает новую фабрику шагов
func NewStepFactory() *StepFactory {
	return &StepFactory{}
}

// NewCommandStep создает шаг с командами
func (f *StepFactory) NewCommandStep(
	name string,
	commandBus transport.CommandBus,
	forwardCmd transport.Command,
	compensateCmd transport.Command,
) SagaStep {
	return NewCommandStep(name, commandBus, forwardCmd, compensateCmd)
}

// NewEventStep создает шаг с публикацией события
func (f *StepFactory) NewEventStep(
	name string,
	eventBus events.EventBus,
	event events.Event,
) SagaStep {
	return NewEventStep(name, eventBus, event)
}

// NewTwoPhaseCommitStep создает шаг с 2PC
func (f *StepFactory) NewTwoPhaseCommitStep(
	name string,
	coordinator TwoPhaseCommitCoordinator,
	participants func(ctx context.Context, sagaCtx SagaContext) []TwoPhaseCommitParticipant,
) SagaStep {
	return NewTwoPhaseCommitStep(name, coordinator, participants)
}

// NewParallelStep создает шаг с параллельным выполнением
func (f *StepFactory) NewParallelStep(name string, steps ...SagaStep) SagaStep {
	return NewParallelStep(name, steps...)
}

// NewConditionalStep создает шаг с условным выполнением
func (f *StepFactory) NewConditionalStep(
	name string,
	condition func(ctx context.Context, sagaCtx SagaContext) bool,
	step SagaStep,
) SagaStep {
	return NewConditionalStep(name, condition, step)
}

// SagaRegistry реестр для регистрации saga definitions
type SagaRegistry struct {
	definitions map[string]SagaDefinition
}

// NewSagaRegistry создает новый реестр саг
func NewSagaRegistry() *SagaRegistry {
	return &SagaRegistry{
		definitions: make(map[string]SagaDefinition),
	}
}

// RegisterSaga регистрирует saga definition
func (r *SagaRegistry) RegisterSaga(name string, definition SagaDefinition) error {
	if r.definitions == nil {
		r.definitions = make(map[string]SagaDefinition)
	}
	r.definitions[name] = definition
	return nil
}

// GetSaga получает definition по имени
func (r *SagaRegistry) GetSaga(name string) (SagaDefinition, error) {
	definition, exists := r.definitions[name]
	if !exists {
		return nil, fmt.Errorf("saga definition %s not found", name)
	}
	return definition, nil
}

// CreateInstance создает instance саги
func (r *SagaRegistry) CreateInstance(ctx context.Context, name string, sagaCtx SagaContext) (Saga, error) {
	return r.CreateInstanceWithPersistence(ctx, name, sagaCtx, nil)
}

// CreateInstanceWithPersistence создает instance саги с persistence
func (r *SagaRegistry) CreateInstanceWithPersistence(ctx context.Context, name string, sagaCtx SagaContext, persistence SagaPersistence) (Saga, error) {
	definition, err := r.GetSaga(name)
	if err != nil {
		return nil, err
	}
	if baseDef, ok := definition.(*BaseSagaDefinition); ok {
		return baseDef.CreateInstanceWithPersistence(ctx, sagaCtx, persistence)
	}
	return definition.CreateInstance(ctx, sagaCtx)
}

// ListSagas возвращает список всех зарегистрированных саг
func (r *SagaRegistry) ListSagas() []string {
	names := make([]string, 0, len(r.definitions))
	for name := range r.definitions {
		names = append(names, name)
	}
	return names
}

