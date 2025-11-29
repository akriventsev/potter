// Package saga предоставляет события для отслеживания жизненного цикла саг.
package saga

import (
	"time"

	"github.com/akriventsev/potter/framework/events"
)

// SagaStartedEvent событие начала выполнения саги
type SagaStartedEvent struct {
	*events.BaseEvent
	SagaID        string
	DefinitionName string
	Timestamp     time.Time
	CorrelationID string
}

// SagaCompletedEvent событие успешного завершения саги
type SagaCompletedEvent struct {
	*events.BaseEvent
	SagaID         string
	Duration       time.Duration
	StepsCompleted int
	Timestamp      time.Time
}

// SagaFailedEvent событие неудачного завершения саги
type SagaFailedEvent struct {
	*events.BaseEvent
	SagaID    string
	Error     string
	FailedStep string
	Timestamp time.Time
}

// SagaCompensatingEvent событие начала компенсации саги
type SagaCompensatingEvent struct {
	*events.BaseEvent
	SagaID    string
	Reason    string
	Timestamp time.Time
}

// SagaCompensatedEvent событие завершения компенсации саги
type SagaCompensatedEvent struct {
	*events.BaseEvent
	SagaID          string
	CompensatedSteps int
	Timestamp       time.Time
}

// StepStartedEvent событие начала выполнения шага
type StepStartedEvent struct {
	*events.BaseEvent
	SagaID    string
	StepName  string
	Timestamp time.Time
}

// StepCompletedEvent событие успешного завершения шага
type StepCompletedEvent struct {
	*events.BaseEvent
	SagaID    string
	StepName  string
	Duration  time.Duration
	Timestamp time.Time
}

// StepFailedEvent событие неудачного завершения шага
type StepFailedEvent struct {
	*events.BaseEvent
	SagaID      string
	StepName    string
	Error       string
	RetryAttempt int
	Timestamp   time.Time
}

// StepCompensatingEvent событие начала компенсации шага
type StepCompensatingEvent struct {
	*events.BaseEvent
	SagaID    string
	StepName  string
	Timestamp time.Time
}

// StepCompensatedEvent событие завершения компенсации шага
type StepCompensatedEvent struct {
	*events.BaseEvent
	SagaID    string
	StepName  string
	Timestamp time.Time
}

