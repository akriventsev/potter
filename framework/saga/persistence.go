// Package saga предоставляет механизмы persistence для сохранения состояния саг.
package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"potter/framework/events"
	"potter/framework/eventsourcing"
)

// SagaPersistence интерфейс для сохранения состояния саг
type SagaPersistence interface {
	// Save сохраняет состояние саги
	Save(ctx context.Context, saga Saga) error
	// Load загружает сагу по ID
	Load(ctx context.Context, sagaID string) (Saga, error)
	// LoadAll загружает все саги с определенным статусом
	LoadAll(ctx context.Context, status SagaStatus) ([]Saga, error)
	// Delete удаляет сагу
	Delete(ctx context.Context, sagaID string) error
	// GetHistory возвращает историю выполнения саги
	GetHistory(ctx context.Context, sagaID string) ([]SagaHistory, error)
}

// InMemoryPersistence реализация persistence в памяти для тестирования
type InMemoryPersistence struct {
	mu    sync.RWMutex
	sagas map[string]Saga
}

// NewInMemoryPersistence создает новую in-memory persistence
func NewInMemoryPersistence() *InMemoryPersistence {
	return &InMemoryPersistence{
		sagas: make(map[string]Saga),
	}
}

func (p *InMemoryPersistence) Save(ctx context.Context, saga Saga) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sagas[saga.ID()] = saga
	return nil
}

func (p *InMemoryPersistence) Load(ctx context.Context, sagaID string) (Saga, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	saga, exists := p.sagas[sagaID]
	if !exists {
		return nil, fmt.Errorf("saga %s not found", sagaID)
	}
	return saga, nil
}

func (p *InMemoryPersistence) LoadAll(ctx context.Context, status SagaStatus) ([]Saga, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []Saga
	for _, saga := range p.sagas {
		if saga.Status() == status {
			result = append(result, saga)
		}
	}
	return result, nil
}

func (p *InMemoryPersistence) Delete(ctx context.Context, sagaID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.sagas, sagaID)
	return nil
}

func (p *InMemoryPersistence) GetHistory(ctx context.Context, sagaID string) ([]SagaHistory, error) {
	saga, err := p.Load(ctx, sagaID)
	if err != nil {
		return nil, err
	}
	return saga.GetHistory(), nil
}

// EventStorePersistence реализация persistence через EventStore
type EventStorePersistence struct {
	eventStore    eventsourcing.EventStore
	snapshotStore eventsourcing.SnapshotStore
	serializer    eventsourcing.SnapshotSerializer
	snapshotFreq  int // частота создания snapshots (каждые N шагов)
	registry      *SagaRegistry // реестр для восстановления определений саг
}

// NewEventStorePersistence создает новую EventStore persistence
func NewEventStorePersistence(
	eventStore eventsourcing.EventStore,
	snapshotStore eventsourcing.SnapshotStore,
) *EventStorePersistence {
	return &EventStorePersistence{
		eventStore:    eventStore,
		snapshotStore: snapshotStore,
		serializer:    eventsourcing.NewJSONSnapshotSerializer(),
		snapshotFreq:  10, // по умолчанию каждые 10 шагов
		registry:      NewSagaRegistry(),
	}
}

// WithRegistry устанавливает реестр саг
func (p *EventStorePersistence) WithRegistry(registry *SagaRegistry) *EventStorePersistence {
	p.registry = registry
	return p
}

// WithSnapshotFrequency устанавливает частоту создания snapshots
func (p *EventStorePersistence) WithSnapshotFrequency(freq int) *EventStorePersistence {
	p.snapshotFreq = freq
	return p
}

func (p *EventStorePersistence) Save(ctx context.Context, saga Saga) error {
	sagaID := saga.ID()

	// Создаем события для каждого изменения состояния
	// Используем events.BaseEvent для совместимости с EventStore
	stateEvent := events.NewBaseEvent("SagaStateChanged", sagaID)
	stateEvent.WithMetadata("status", string(saga.Status()))
	stateEvent.WithMetadata("step", saga.CurrentStep())
	stateEvent.WithMetadata("context", saga.Context().ToMap())
	stateEvent.WithMetadata("definition_name", saga.Definition().Name())
	stateEvent.WithCorrelationID(saga.Context().CorrelationID())
	
	eventsList := []events.Event{stateEvent}

	// Добавляем события шагов из истории
	history := saga.GetHistory()
	for _, hist := range history {
		var baseEvent *events.BaseEvent
		
		switch hist.Status {
		case StepStatusRunning:
			// Событие начала шага
			baseEvent = events.NewBaseEvent("StepStarted", sagaID)
			baseEvent.WithMetadata("step_name", hist.StepName)
			baseEvent.WithMetadata("started_at", hist.StartedAt.Format(time.RFC3339))
		case StepStatusCompleted:
			// Событие завершения шага
			baseEvent = events.NewBaseEvent("StepCompleted", sagaID)
			baseEvent.WithMetadata("step_name", hist.StepName)
			baseEvent.WithMetadata("started_at", hist.StartedAt.Format(time.RFC3339))
			if hist.CompletedAt != nil {
				baseEvent.WithMetadata("completed_at", hist.CompletedAt.Format(time.RFC3339))
				duration := hist.CompletedAt.Sub(hist.StartedAt)
				baseEvent.WithMetadata("duration_ms", duration.Milliseconds())
			}
			baseEvent.WithMetadata("retry_attempt", hist.RetryAttempt)
		case StepStatusFailed:
			// Событие ошибки шага
			baseEvent = events.NewBaseEvent("StepFailed", sagaID)
			baseEvent.WithMetadata("step_name", hist.StepName)
			baseEvent.WithMetadata("started_at", hist.StartedAt.Format(time.RFC3339))
			if hist.CompletedAt != nil {
				baseEvent.WithMetadata("completed_at", hist.CompletedAt.Format(time.RFC3339))
			}
			if hist.Error != nil {
				baseEvent.WithMetadata("error", hist.Error.Error())
				baseEvent.WithMetadata("error_message", hist.Error.Error())
			}
			baseEvent.WithMetadata("retry_attempt", hist.RetryAttempt)
		case StepStatusCompensating:
			// Событие начала компенсации шага
			baseEvent = events.NewBaseEvent("StepCompensating", sagaID)
			baseEvent.WithMetadata("step_name", hist.StepName)
			baseEvent.WithMetadata("started_at", hist.StartedAt.Format(time.RFC3339))
		case StepStatusCompensated:
			// Событие завершения компенсации шага
			baseEvent = events.NewBaseEvent("StepCompensated", sagaID)
			baseEvent.WithMetadata("step_name", hist.StepName)
			baseEvent.WithMetadata("started_at", hist.StartedAt.Format(time.RFC3339))
			if hist.CompletedAt != nil {
				baseEvent.WithMetadata("completed_at", hist.CompletedAt.Format(time.RFC3339))
			}
		}
		
		if baseEvent != nil {
			baseEvent.WithCorrelationID(saga.Context().CorrelationID())
			eventsList = append(eventsList, baseEvent)
		}
	}

	// Сохраняем события
	if err := p.eventStore.AppendEvents(ctx, sagaID, 0, eventsList); err != nil {
		return fmt.Errorf("failed to append events: %w", err)
	}

	// Создаем snapshot если нужно
	if len(history) > 0 && len(history)%p.snapshotFreq == 0 {
		snapshot := eventsourcing.Snapshot{
			AggregateID:  sagaID,
			AggregateType: "saga",
			Version:      int64(len(history)),
			Metadata:     saga.Context().ToMap(),
			CreatedAt:    time.Now(),
		}

		// Сериализуем состояние саги
		stateData, err := p.serializeSagaState(saga)
		if err != nil {
			return fmt.Errorf("failed to serialize saga state: %w", err)
		}
		snapshot.State = stateData

		if err := p.snapshotStore.SaveSnapshot(ctx, snapshot); err != nil {
			// Логируем ошибку, но не прерываем сохранение
			_ = err
		}
	}

	return nil
}

func (p *EventStorePersistence) Load(ctx context.Context, sagaID string) (Saga, error) {
	// Пытаемся загрузить snapshot
	snapshot, err := p.snapshotStore.GetSnapshot(ctx, sagaID)
	if err == nil && snapshot != nil {
		// Восстанавливаем из snapshot
		saga, err := p.deserializeSagaState(snapshot.State)
		if err == nil {
			return saga, nil
		}
	}

	// Загружаем все события и восстанавливаем состояние
	storedEvents, err := p.eventStore.GetEvents(ctx, sagaID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	if len(storedEvents) == 0 {
		return nil, fmt.Errorf("saga %s not found", sagaID)
	}

	// Восстанавливаем состояние из событий
	var definitionName string
	var sagaStatus SagaStatus
	var currentStep string
	var sagaCtx SagaContext
	var history []SagaHistory

	// Извлекаем информацию из последнего события состояния
	for _, storedEvent := range storedEvents {
		if storedEvent.EventType == "SagaStateChanged" {
			// Извлекаем метаданные из события
			if statusVal, ok := storedEvent.Metadata["status"]; ok {
				if statusStr, ok := statusVal.(string); ok {
					sagaStatus = SagaStatus(statusStr)
				}
			}
			if stepVal, ok := storedEvent.Metadata["step"]; ok {
				if stepStr, ok := stepVal.(string); ok {
					currentStep = stepStr
				}
			}
			if contextVal, ok := storedEvent.Metadata["context"]; ok {
				if contextMap, ok := contextVal.(map[string]interface{}); ok {
					sagaCtx = NewSagaContext()
					sagaCtx.FromMap(contextMap)
				}
			}
		}
	}

	// Если не удалось восстановить из событий, создаем базовый контекст
	if sagaCtx == nil {
		sagaCtx = NewSagaContext()
	}

	// Пытаемся получить definition из метаданных snapshot или первого события
	if snapshot != nil && snapshot.Metadata != nil {
		if defName, ok := snapshot.Metadata["definition_name"].(string); ok {
			definitionName = defName
		}
	}

	// Если definition не найден, пытаемся найти его в событиях
	if definitionName == "" {
		for _, storedEvent := range storedEvents {
			if defName, ok := storedEvent.Metadata["definition_name"].(string); ok {
				definitionName = defName
				break
			}
		}
	}

	// Если definition все еще не найден, возвращаем ошибку
	if definitionName == "" {
		return nil, fmt.Errorf("cannot determine saga definition for saga %s", sagaID)
	}

	// Получаем definition из registry
	definition, err := p.registry.GetSaga(definitionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get saga definition %s: %w", definitionName, err)
	}

	// Восстанавливаем историю из событий шагов
	stepHistoryMap := make(map[string]*SagaHistory) // step_name -> history entry
	
	for _, storedEvent := range storedEvents {
		stepName, _ := storedEvent.Metadata["step_name"].(string)
		if stepName == "" {
			continue
		}

		// Получаем или создаем запись истории для этого шага
		hist, exists := stepHistoryMap[stepName]
		if !exists {
			hist = &SagaHistory{
				StepName: stepName,
			}
			stepHistoryMap[stepName] = hist
		}

		switch storedEvent.EventType {
		case "StepStarted":
			if startedAtStr, ok := storedEvent.Metadata["started_at"].(string); ok {
				if t, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
					hist.StartedAt = t
				} else {
					hist.StartedAt = storedEvent.OccurredAt
				}
			} else {
				hist.StartedAt = storedEvent.OccurredAt
			}
			hist.Status = StepStatusRunning
			
		case "StepCompleted":
			hist.Status = StepStatusCompleted
			if completedAtStr, ok := storedEvent.Metadata["completed_at"].(string); ok {
				if t, err := time.Parse(time.RFC3339, completedAtStr); err == nil {
					hist.CompletedAt = &t
				} else {
					hist.CompletedAt = &storedEvent.OccurredAt
				}
			} else {
				hist.CompletedAt = &storedEvent.OccurredAt
			}
			if retryAttempt, ok := storedEvent.Metadata["retry_attempt"].(int); ok {
				hist.RetryAttempt = retryAttempt
			} else if retryAttemptFloat, ok := storedEvent.Metadata["retry_attempt"].(float64); ok {
				hist.RetryAttempt = int(retryAttemptFloat)
			}
			
		case "StepFailed":
			hist.Status = StepStatusFailed
			if completedAtStr, ok := storedEvent.Metadata["completed_at"].(string); ok {
				if t, err := time.Parse(time.RFC3339, completedAtStr); err == nil {
					hist.CompletedAt = &t
				} else {
					hist.CompletedAt = &storedEvent.OccurredAt
				}
			} else {
				hist.CompletedAt = &storedEvent.OccurredAt
			}
			// Восстанавливаем ошибку из метаданных (поддерживаем оба варианта: error и error_message)
			if errorStr, ok := storedEvent.Metadata["error"].(string); ok && errorStr != "" {
				hist.Error = fmt.Errorf(errorStr)
			} else if errorMsg, ok := storedEvent.Metadata["error_message"].(string); ok && errorMsg != "" {
				hist.Error = fmt.Errorf(errorMsg)
			}
			if retryAttempt, ok := storedEvent.Metadata["retry_attempt"].(int); ok {
				hist.RetryAttempt = retryAttempt
			} else if retryAttemptFloat, ok := storedEvent.Metadata["retry_attempt"].(float64); ok {
				hist.RetryAttempt = int(retryAttemptFloat)
			}
			
		case "StepCompensating":
			hist.Status = StepStatusCompensating
			if hist.StartedAt.IsZero() {
				if startedAtStr, ok := storedEvent.Metadata["started_at"].(string); ok {
					if t, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
						hist.StartedAt = t
					} else {
						hist.StartedAt = storedEvent.OccurredAt
					}
				} else {
					hist.StartedAt = storedEvent.OccurredAt
				}
			}
			
		case "StepCompensated":
			hist.Status = StepStatusCompensated
			if completedAtStr, ok := storedEvent.Metadata["completed_at"].(string); ok {
				if t, err := time.Parse(time.RFC3339, completedAtStr); err == nil {
					hist.CompletedAt = &t
				} else {
					hist.CompletedAt = &storedEvent.OccurredAt
				}
			} else {
				hist.CompletedAt = &storedEvent.OccurredAt
			}
			if hist.StartedAt.IsZero() {
				if startedAtStr, ok := storedEvent.Metadata["started_at"].(string); ok {
					if t, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
						hist.StartedAt = t
					} else {
						hist.StartedAt = storedEvent.OccurredAt
					}
				} else {
					hist.StartedAt = storedEvent.OccurredAt
				}
			}
		}
	}

	// Преобразуем map в slice, сортируя по времени начала
	for _, hist := range stepHistoryMap {
		history = append(history, *hist)
	}

	// Создаем экземпляр саги (eventBus будет nil, так как он не сохраняется в persistence)
	saga, err := NewBaseSagaWithEventBus(sagaID, definition, sagaCtx, p, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create saga instance: %w", err)
	}
	
	// Восстанавливаем состояние
	saga.mu.Lock()
	saga.status = sagaStatus
	saga.currentStep = currentStep
	saga.history = history
	if len(history) > 0 {
		saga.startedAt = history[0].StartedAt
		if lastHist := history[len(history)-1]; lastHist.CompletedAt != nil {
			saga.completedAt = lastHist.CompletedAt
		}
	}
	saga.mu.Unlock()

	return saga, nil
}

func (p *EventStorePersistence) LoadAll(ctx context.Context, status SagaStatus) ([]Saga, error) {
	// EventStore не поддерживает прямую загрузку по статусу
	// Загружаем события типа SagaStateChanged и фильтруем по статусу
	storedEvents, err := p.eventStore.GetEventsByType(ctx, "SagaStateChanged", time.Time{})
	if err != nil {
		return nil, fmt.Errorf("failed to get events by type: %w", err)
	}

	// Группируем события по sagaID
	sagaIDs := make(map[string]bool)
	for _, storedEvent := range storedEvents {
		if statusVal, ok := storedEvent.Metadata["status"]; ok {
			if statusStr, ok := statusVal.(string); ok && SagaStatus(statusStr) == status {
				sagaIDs[storedEvent.AggregateID] = true
			}
		}
	}

	// Загружаем каждую сагу
	var sagas []Saga
	for sagaID := range sagaIDs {
		saga, err := p.Load(ctx, sagaID)
		if err != nil {
			// Пропускаем ошибки загрузки отдельных саг
			continue
		}
		sagas = append(sagas, saga)
	}

	return sagas, nil
}

func (p *EventStorePersistence) Delete(ctx context.Context, sagaID string) error {
	// EventStore обычно не поддерживает удаление событий
	return fmt.Errorf("Delete not supported for EventStorePersistence")
}

func (p *EventStorePersistence) GetHistory(ctx context.Context, sagaID string) ([]SagaHistory, error) {
	saga, err := p.Load(ctx, sagaID)
	if err != nil {
		return nil, err
	}
	return saga.GetHistory(), nil
}

func (p *EventStorePersistence) serializeSagaState(saga Saga) ([]byte, error) {
	// Сериализуем историю с явной обработкой ошибок
	history := saga.GetHistory()
	historyData := make([]map[string]interface{}, len(history))
	for i, hist := range history {
		histMap := map[string]interface{}{
			"step_name":     hist.StepName,
			"status":        string(hist.Status),
			"started_at":    hist.StartedAt.Format(time.RFC3339),
			"retry_attempt": hist.RetryAttempt,
		}
		if hist.CompletedAt != nil {
			histMap["completed_at"] = hist.CompletedAt.Format(time.RFC3339)
		}
		// Явная сериализация ошибки в строковое поле
		if hist.Error != nil {
			histMap["error_message"] = hist.Error.Error()
		}
		historyData[i] = histMap
	}

	state := map[string]interface{}{
		"id":             saga.ID(),
		"status":         string(saga.Status()),
		"step":           saga.CurrentStep(),
		"context":        saga.Context().ToMap(),
		"history":        historyData,
		"definition":     saga.Definition().Name(),
		"correlation_id": saga.Context().CorrelationID(),
	}
	return json.Marshal(state)
}

func (p *EventStorePersistence) deserializeSagaState(data []byte) (Saga, error) {
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal saga state: %w", err)
	}

	// Извлекаем основные поля
	sagaID, _ := state["id"].(string)
	definitionName, _ := state["definition"].(string)
	statusStr, _ := state["status"].(string)
	currentStep, _ := state["step"].(string)

	if sagaID == "" || definitionName == "" {
		return nil, fmt.Errorf("invalid saga state: missing id or definition")
	}

	// Получаем definition из registry
	definition, err := p.registry.GetSaga(definitionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get saga definition %s: %w", definitionName, err)
	}

	// Восстанавливаем контекст
	sagaCtx := NewSagaContext()
	if contextData, ok := state["context"].(map[string]interface{}); ok {
		if err := sagaCtx.FromMap(contextData); err != nil {
			return nil, fmt.Errorf("failed to restore context: %w", err)
		}
	}

	// Создаем экземпляр саги (eventBus будет nil, так как он не сохраняется в persistence)
	saga, err := NewBaseSagaWithEventBus(sagaID, definition, sagaCtx, p, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create saga instance: %w", err)
	}

	// Восстанавливаем состояние
	saga.mu.Lock()
	saga.status = SagaStatus(statusStr)
	saga.currentStep = currentStep

	// Восстанавливаем историю с явной обработкой ошибок
	if historyData, ok := state["history"].([]interface{}); ok {
		history := make([]SagaHistory, 0, len(historyData))
		for _, histItem := range historyData {
			if histMap, ok := histItem.(map[string]interface{}); ok {
				hist := SagaHistory{}
				if stepName, ok := histMap["step_name"].(string); ok {
					hist.StepName = stepName
				}
				if statusStr, ok := histMap["status"].(string); ok {
					hist.Status = StepStatus(statusStr)
				}
				if startedAtStr, ok := histMap["started_at"].(string); ok {
					if t, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
						hist.StartedAt = t
					}
				}
				if completedAtStr, ok := histMap["completed_at"].(string); ok {
					if t, err := time.Parse(time.RFC3339, completedAtStr); err == nil {
						hist.CompletedAt = &t
					}
				}
				if retryAttempt, ok := histMap["retry_attempt"].(int); ok {
					hist.RetryAttempt = retryAttempt
				} else if retryAttemptFloat, ok := histMap["retry_attempt"].(float64); ok {
					hist.RetryAttempt = int(retryAttemptFloat)
				}
				// Восстанавливаем ошибку из error_message
				if errorMsg, ok := histMap["error_message"].(string); ok && errorMsg != "" {
					hist.Error = fmt.Errorf(errorMsg)
				}
				history = append(history, hist)
			}
		}
		saga.history = history
		if len(history) > 0 {
			saga.startedAt = history[0].StartedAt
			if lastHist := history[len(history)-1]; lastHist.CompletedAt != nil {
				saga.completedAt = lastHist.CompletedAt
			}
		}
	}
	saga.mu.Unlock()

	return saga, nil
}


// PostgresPersistence реализация persistence через PostgreSQL
type PostgresPersistence struct {
	conn     *pgx.Conn
	dsn      string
	registry *SagaRegistry // реестр для восстановления определений саг
}

// NewPostgresPersistence создает новую PostgreSQL persistence
func NewPostgresPersistence(dsn string) (*PostgresPersistence, error) {
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	return &PostgresPersistence{
		conn:     conn,
		dsn:      dsn,
		registry: NewSagaRegistry(),
	}, nil
}

// WithRegistry устанавливает реестр саг
func (p *PostgresPersistence) WithRegistry(registry *SagaRegistry) *PostgresPersistence {
	p.registry = registry
	return p
}

func (p *PostgresPersistence) Save(ctx context.Context, saga Saga) error {
	sagaID := saga.ID()
	definitionName := saga.Definition().Name()
	status := string(saga.Status())
	currentStep := saga.CurrentStep()
	contextJSON, err := json.Marshal(saga.Context().ToMap())
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	correlationID := saga.Context().CorrelationID()
	now := time.Now()

	// Сохраняем или обновляем сагу
	query := `
		INSERT INTO saga_instances (id, definition_name, status, context, correlation_id, current_step, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			status = $3,
			context = $4,
			current_step = $6,
			updated_at = $8
	`
	_, err = p.conn.Exec(ctx, query,
		sagaID, definitionName, status, contextJSON, correlationID, currentStep, now, now)
	if err != nil {
		return fmt.Errorf("failed to save saga: %w", err)
	}

	// Сохраняем историю шагов
	history := saga.GetHistory()
	for _, hist := range history {
		histQuery := `
			INSERT INTO saga_history (id, saga_id, step_name, status, error, retry_attempt, started_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO UPDATE SET
				status = $4,
				error = $5,
				completed_at = $8
		`
		histID := uuid.New().String()
		errorStr := ""
		if hist.Error != nil {
			errorStr = hist.Error.Error()
		}
		_, err = p.conn.Exec(ctx, histQuery,
			histID, sagaID, hist.StepName, string(hist.Status), errorStr, hist.RetryAttempt, hist.StartedAt, hist.CompletedAt)
		if err != nil {
			// Логируем ошибку, но не прерываем сохранение
			_ = err
		}
	}

	return nil
}

func (p *PostgresPersistence) Load(ctx context.Context, sagaID string) (Saga, error) {
	query := `
		SELECT id, definition_name, status, context, correlation_id, current_step, created_at, updated_at, completed_at
		FROM saga_instances
		WHERE id = $1
	`
	var id, definitionName, statusStr, currentStep, correlationID string
	var contextJSON []byte
	var createdAt, updatedAt time.Time
	var completedAt *time.Time

	err := p.conn.QueryRow(ctx, query, sagaID).Scan(
		&id, &definitionName, &statusStr, &contextJSON, &correlationID, &currentStep, &createdAt, &updatedAt, &completedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to load saga: %w", err)
	}

	// Получаем definition из registry
	definition, err := p.registry.GetSaga(definitionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get saga definition %s: %w", definitionName, err)
	}

	// Восстанавливаем контекст
	sagaCtx := NewSagaContext()
	var contextData map[string]interface{}
	if err := json.Unmarshal(contextJSON, &contextData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context: %w", err)
	}
	sagaCtx.FromMap(contextData)
	
	// Восстанавливаем correlation ID
	if correlationID != "" {
		sagaCtx.SetCorrelationID(correlationID)
	}

	// Восстанавливаем метаданные
	if ctxImpl, ok := sagaCtx.(*SagaContextImpl); ok {
		ctxImpl.mu.Lock()
		ctxImpl.metadata.CreatedAt = createdAt
		ctxImpl.metadata.UpdatedAt = updatedAt
		ctxImpl.mu.Unlock()
	}

	// Загружаем историю
	history, err := p.GetHistory(ctx, sagaID)
	if err != nil {
		return nil, fmt.Errorf("failed to load history: %w", err)
	}

	// Создаем экземпляр саги
	saga, err := NewBaseSaga(id, definition, sagaCtx, p)
	if err != nil {
		return nil, fmt.Errorf("failed to create saga instance: %w", err)
	}

	// Восстанавливаем состояние
	saga.mu.Lock()
	saga.status = SagaStatus(statusStr)
	saga.currentStep = currentStep
	saga.history = history
	saga.startedAt = createdAt
	saga.completedAt = completedAt
	saga.mu.Unlock()

	return saga, nil
}

func (p *PostgresPersistence) LoadAll(ctx context.Context, status SagaStatus) ([]Saga, error) {
	query := `
		SELECT id, definition_name, status, context, correlation_id, current_step, created_at, updated_at, completed_at
		FROM saga_instances
		WHERE status = $1
		ORDER BY created_at DESC
	`
	rows, err := p.conn.Query(ctx, query, string(status))
	if err != nil {
		return nil, fmt.Errorf("failed to query sagas: %w", err)
	}
	defer rows.Close()

	var sagas []Saga
	for rows.Next() {
		var id, definitionName, statusStr, currentStep, correlationID string
		var contextJSON []byte
		var createdAt, updatedAt time.Time
		var completedAt *time.Time

		if err := rows.Scan(&id, &definitionName, &statusStr, &contextJSON, &correlationID, &currentStep, &createdAt, &updatedAt, &completedAt); err != nil {
			continue
		}

		// Получаем definition из registry
		definition, err := p.registry.GetSaga(definitionName)
		if err != nil {
			// Пропускаем саги с неизвестными определениями
			continue
		}

		// Восстанавливаем контекст
		sagaCtx := NewSagaContext()
		var contextData map[string]interface{}
		if err := json.Unmarshal(contextJSON, &contextData); err != nil {
			continue
		}
		sagaCtx.FromMap(contextData)
		
		if correlationID != "" {
			sagaCtx.SetCorrelationID(correlationID)
		}

		// Восстанавливаем метаданные
		if ctxImpl, ok := sagaCtx.(*SagaContextImpl); ok {
			ctxImpl.mu.Lock()
			ctxImpl.metadata.CreatedAt = createdAt
			ctxImpl.metadata.UpdatedAt = updatedAt
			ctxImpl.mu.Unlock()
		}

		// Загружаем историю
		history, err := p.GetHistory(ctx, id)
		if err != nil {
			// Продолжаем без истории
			history = []SagaHistory{}
		}

		// Создаем экземпляр саги (eventBus будет nil)
		saga, err := NewBaseSagaWithEventBus(id, definition, sagaCtx, p, nil)
		if err != nil {
			// Пропускаем саги с ошибками создания
			continue
		}

		// Восстанавливаем состояние
		saga.mu.Lock()
		saga.status = SagaStatus(statusStr)
		saga.currentStep = currentStep
		saga.history = history
		saga.startedAt = createdAt
		saga.completedAt = completedAt
		saga.mu.Unlock()

		sagas = append(sagas, saga)
	}

	return sagas, nil
}

func (p *PostgresPersistence) Delete(ctx context.Context, sagaID string) error {
	query := `DELETE FROM saga_instances WHERE id = $1`
	_, err := p.conn.Exec(ctx, query, sagaID)
	return err
}

func (p *PostgresPersistence) GetHistory(ctx context.Context, sagaID string) ([]SagaHistory, error) {
	query := `
		SELECT step_name, status, error, retry_attempt, started_at, completed_at
		FROM saga_history
		WHERE saga_id = $1
		ORDER BY started_at ASC
	`
	rows, err := p.conn.Query(ctx, query, sagaID)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	var history []SagaHistory
	for rows.Next() {
		var stepName, statusStr, errorStr string
		var retryAttempt int
		var startedAt time.Time
		var completedAt *time.Time

		if err := rows.Scan(&stepName, &statusStr, &errorStr, &retryAttempt, &startedAt, &completedAt); err != nil {
			continue
		}

		var err error
		if errorStr != "" {
			err = fmt.Errorf(errorStr)
		}

		history = append(history, SagaHistory{
			StepName:     stepName,
			Status:       StepStatus(statusStr),
			StartedAt:    startedAt,
			CompletedAt:  completedAt,
			Error:        err,
			RetryAttempt: retryAttempt,
		})
	}

	return history, nil
}

// Close закрывает соединение
func (p *PostgresPersistence) Close(ctx context.Context) error {
	return p.conn.Close(ctx)
}

