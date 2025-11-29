// Package saga предоставляет механизмы для работы с сагами.
package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/eventsourcing"
)

// SagaReadModelProjection проекция для обновления read model из событий саги
type SagaReadModelProjection struct {
	store SagaReadModelStore
}

// NewSagaReadModelProjection создает новую проекцию для read model саг
func NewSagaReadModelProjection(store SagaReadModelStore) *SagaReadModelProjection {
	return &SagaReadModelProjection{
		store: store,
	}
}

// Name возвращает имя проекции
func (p *SagaReadModelProjection) Name() string {
	return "SagaReadModelProjection"
}

// HandleEvent обрабатывает событие из EventStore (реализация eventsourcing.Projection)
func (p *SagaReadModelProjection) HandleEvent(ctx context.Context, event eventsourcing.StoredEvent) error {
	// Извлекаем событие саги из StoredEvent
	// События саги могут иметь AggregateType = "saga" или AggregateID = sagaID
	sagaID := event.AggregateID
	
	// Проверяем, что это событие саги (по AggregateType или по типу события)
	if event.AggregateType != "saga" && !p.isSagaEventType(event.EventType) {
		return nil // Игнорируем события не саг
	}
	
	// Создаем eventData из метаданных или EventData
	eventData := make(map[string]interface{})
	if event.Metadata != nil {
		for k, v := range event.Metadata {
			eventData[k] = v
		}
	}
	eventData["saga_id"] = sagaID
	
	// Если EventData доступно, пытаемся извлечь данные из него
	if event.EventData != nil {
		eventJSON, err := json.Marshal(event.EventData)
		if err == nil {
			var dataFromEvent map[string]interface{}
			if err := json.Unmarshal(eventJSON, &dataFromEvent); err == nil {
				// Объединяем данные из EventData с метаданными
				for k, v := range dataFromEvent {
					if _, exists := eventData[k]; !exists {
						eventData[k] = v
					}
				}
			}
		}
	}

	// Добавляем timestamp из StoredEvent
	if !event.OccurredAt.IsZero() {
		eventData["timestamp"] = event.OccurredAt.Format(time.RFC3339)
	}

	// Обрабатываем события по типу
	switch event.EventType {
	case "SagaStarted":
		return p.handleSagaStartedFromMap(ctx, eventData)
	case "SagaStateChanged":
		// Для SagaStateChanged используем метаданные
		if status, ok := eventData["status"].(string); ok {
			if status == "running" && !p.hasReadModel(ctx, sagaID) {
				// Если сага только начинается, обрабатываем как SagaStarted
				return p.handleSagaStartedFromMap(ctx, eventData)
			}
			// Обновляем статус существующей саги
			return p.handleSagaStateChangedFromMap(ctx, eventData)
		}
	case "StepStarted":
		return p.handleStepStartedFromMap(ctx, eventData)
	case "StepCompleted":
		return p.handleStepCompletedFromMap(ctx, eventData)
	case "StepFailed":
		return p.handleStepFailedFromMap(ctx, eventData)
	case "StepCompensated":
		return p.handleStepCompensatedFromMap(ctx, eventData)
	case "SagaCompleted":
		return p.handleSagaCompletedFromMap(ctx, eventData)
	case "SagaFailed":
		return p.handleSagaFailedFromMap(ctx, eventData)
	case "SagaCompensated":
		return p.handleSagaCompensatedFromMap(ctx, eventData)
	}

	return nil // Игнорируем неизвестные типы событий
}

// isSagaEventType проверяет, является ли тип события событием саги
func (p *SagaReadModelProjection) isSagaEventType(eventType string) bool {
	sagaEventTypes := []string{
		"SagaStarted", "SagaStateChanged", "SagaCompleted", "SagaFailed", "SagaCompensated",
		"StepStarted", "StepCompleted", "StepFailed", "StepCompensated",
	}
	for _, t := range sagaEventTypes {
		if eventType == t {
			return true
		}
	}
	return false
}

// hasReadModel проверяет, существует ли read model для саги
func (p *SagaReadModelProjection) hasReadModel(ctx context.Context, sagaID string) bool {
	_, err := p.store.GetSagaStatus(ctx, sagaID)
	return err == nil
}

// handleSagaStateChangedFromMap обрабатывает изменение состояния саги
func (p *SagaReadModelProjection) handleSagaStateChangedFromMap(ctx context.Context, eventData map[string]interface{}) error {
	sagaID, _ := eventData["saga_id"].(string)
	status, _ := eventData["status"].(string)
	currentStep, _ := eventData["step"].(string)

	model, err := p.getOrCreateReadModel(ctx, sagaID)
	if err != nil {
		return err
	}

	model.Status = SagaStatus(status)
	if currentStep != "" {
		model.CurrentStep = currentStep
	}
	model.UpdatedAt = time.Now()

	return p.saveReadModel(ctx, model)
}

// Reset сбрасывает состояние проекции (удаляет все read models)
func (p *SagaReadModelProjection) Reset(ctx context.Context) error {
	// Для reset нужно удалить все read models
	// Это зависит от реализации store, пока возвращаем nil
	// В реальной реализации можно добавить метод ClearAll в SagaReadModelStore
	return nil
}

// HandleSagaStarted обрабатывает событие начала саги
func (p *SagaReadModelProjection) HandleSagaStarted(ctx context.Context, event *SagaStartedEvent) error {
	if p.store == nil {
		return nil
	}

	// Получаем или создаем read model
	model, err := p.getOrCreateReadModel(ctx, event.SagaID)
	if err != nil {
		return fmt.Errorf("failed to get or create read model: %w", err)
	}

	// Обновляем поля
	model.DefinitionName = event.DefinitionName
	model.Status = SagaStatusRunning
	model.StartedAt = event.Timestamp
	model.CorrelationID = event.CorrelationID
	model.UpdatedAt = time.Now()

	return p.saveReadModel(ctx, model)
}

// HandleStepStarted обрабатывает событие начала шага
func (p *SagaReadModelProjection) HandleStepStarted(ctx context.Context, event *StepStartedEvent) error {
	if p.store == nil {
		return nil
	}

	model, err := p.getOrCreateReadModel(ctx, event.SagaID)
	if err != nil {
		return fmt.Errorf("failed to get read model: %w", err)
	}

	model.CurrentStep = event.StepName
	model.UpdatedAt = time.Now()

	return p.saveReadModel(ctx, model)
}

// HandleStepCompleted обрабатывает событие успешного завершения шага
func (p *SagaReadModelProjection) HandleStepCompleted(ctx context.Context, event *StepCompletedEvent) error {
	if p.store == nil {
		return nil
	}

	model, err := p.getOrCreateReadModel(ctx, event.SagaID)
	if err != nil {
		return fmt.Errorf("failed to get read model: %w", err)
	}

	model.CompletedSteps++
	model.UpdatedAt = time.Now()

	return p.saveReadModel(ctx, model)
}

// HandleStepFailed обрабатывает событие неудачного завершения шага
func (p *SagaReadModelProjection) HandleStepFailed(ctx context.Context, event *StepFailedEvent) error {
	if p.store == nil {
		return nil
	}

	model, err := p.getOrCreateReadModel(ctx, event.SagaID)
	if err != nil {
		return fmt.Errorf("failed to get read model: %w", err)
	}

	model.FailedSteps++
	model.LastError = &event.Error
	model.RetryCount = event.RetryAttempt
	model.UpdatedAt = time.Now()

	return p.saveReadModel(ctx, model)
}

// HandleSagaCompleted обрабатывает событие успешного завершения саги
func (p *SagaReadModelProjection) HandleSagaCompleted(ctx context.Context, event *SagaCompletedEvent) error {
	if p.store == nil {
		return nil
	}

	model, err := p.getOrCreateReadModel(ctx, event.SagaID)
	if err != nil {
		return fmt.Errorf("failed to get read model: %w", err)
	}

	model.Status = SagaStatusCompleted
	now := time.Now()
	model.CompletedAt = &now
	duration := event.Duration
	model.Duration = &duration
	model.UpdatedAt = now

	return p.saveReadModel(ctx, model)
}

// HandleSagaFailed обрабатывает событие неудачного завершения саги
func (p *SagaReadModelProjection) HandleSagaFailed(ctx context.Context, event *SagaFailedEvent) error {
	if p.store == nil {
		return nil
	}

	model, err := p.getOrCreateReadModel(ctx, event.SagaID)
	if err != nil {
		return fmt.Errorf("failed to get read model: %w", err)
	}

	model.Status = SagaStatusFailed
	now := time.Now()
	model.CompletedAt = &now
	if model.StartedAt != (time.Time{}) {
		duration := now.Sub(model.StartedAt)
		model.Duration = &duration
	}
	model.LastError = &event.Error
	model.UpdatedAt = now

	return p.saveReadModel(ctx, model)
}

// getOrCreateReadModel получает или создает read model
func (p *SagaReadModelProjection) getOrCreateReadModel(ctx context.Context, sagaID string) (*SagaReadModel, error) {
	// Пытаемся получить существующий read model
	status, err := p.store.GetSagaStatus(ctx, sagaID)
	if err == nil && status != nil {
		// Конвертируем SagaStatusResponse в SagaReadModel
		model := &SagaReadModel{
			SagaID:        status.SagaID,
			DefinitionName: status.DefinitionName,
			Status:        status.Status,
			CurrentStep:   status.CurrentStep,
			TotalSteps:    status.TotalSteps,
			CompletedSteps: status.CompletedSteps,
			FailedSteps:   status.FailedSteps,
			StartedAt:     status.StartedAt,
			CompletedAt:   status.CompletedAt,
			Duration:      status.Duration,
			CorrelationID: status.CorrelationID,
			Context:       status.Context,
			LastError:     status.LastError,
			RetryCount:    status.RetryCount,
			UpdatedAt:     time.Now(),
		}
		return model, nil
	}

	// Если не найдено, создаем новый read model
	// (err != nil означает что сага не найдена, это нормально)
	return &SagaReadModel{
		SagaID:        sagaID,
		Status:        SagaStatusRunning,
		TotalSteps:    0,
		CompletedSteps: 0,
		FailedSteps:   0,
		StartedAt:     time.Now(),
		Context:       make(map[string]interface{}),
		UpdatedAt:     time.Now(),
	}, nil
}

// saveReadModel сохраняет read model
func (p *SagaReadModelProjection) saveReadModel(ctx context.Context, model *SagaReadModel) error {
	return p.store.UpsertSagaReadModel(ctx, model)
}

// Вспомогательные методы для обработки событий из map (для HandleEvent)
func (p *SagaReadModelProjection) handleSagaStartedFromMap(ctx context.Context, eventData map[string]interface{}) error {
	sagaID, _ := eventData["saga_id"].(string)
	definitionName, _ := eventData["definition_name"].(string)
	correlationID, _ := eventData["correlation_id"].(string)
	
	var timestamp time.Time
	if ts, ok := eventData["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			timestamp = t
		}
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	model := &SagaReadModel{
		SagaID:        sagaID,
		DefinitionName: definitionName,
		Status:        SagaStatusRunning,
		StartedAt:     timestamp,
		CorrelationID: correlationID,
		Context:       make(map[string]interface{}),
		UpdatedAt:     time.Now(),
	}

	return p.saveReadModel(ctx, model)
}

func (p *SagaReadModelProjection) handleStepStartedFromMap(ctx context.Context, eventData map[string]interface{}) error {
	sagaID, _ := eventData["saga_id"].(string)
	stepName, _ := eventData["step_name"].(string)
	
	var timestamp time.Time
	if ts, ok := eventData["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			timestamp = t
		}
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	model, err := p.getOrCreateReadModel(ctx, sagaID)
	if err != nil {
		return err
	}

	model.CurrentStep = stepName
	model.UpdatedAt = time.Now()

	// Сохраняем шаг в истории
	stepModel := &SagaStepReadModel{
		SagaID:    sagaID,
		StepName:  stepName,
		Status:    "started",
		StartedAt: timestamp,
		UpdatedAt: time.Now(),
	}
	if err := p.store.UpsertSagaStepReadModel(ctx, stepModel); err != nil {
		return fmt.Errorf("failed to save step read model: %w", err)
	}

	return p.saveReadModel(ctx, model)
}

func (p *SagaReadModelProjection) handleStepCompletedFromMap(ctx context.Context, eventData map[string]interface{}) error {
	sagaID, _ := eventData["saga_id"].(string)
	stepName, _ := eventData["step_name"].(string)
	
	var timestamp time.Time
	if ts, ok := eventData["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			timestamp = t
		}
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	var duration time.Duration
	if d, ok := eventData["duration"].(string); ok {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	model, err := p.getOrCreateReadModel(ctx, sagaID)
	if err != nil {
		return err
	}

	model.CompletedSteps++
	model.UpdatedAt = time.Now()

	// Обновляем шаг в истории
	stepModel := &SagaStepReadModel{
		SagaID:      sagaID,
		StepName:    stepName,
		Status:      "completed",
		StartedAt:   timestamp.Add(-duration), // Приблизительное время начала
		CompletedAt: &timestamp,
		Duration:    &duration,
		UpdatedAt:   time.Now(),
	}
	if err := p.store.UpsertSagaStepReadModel(ctx, stepModel); err != nil {
		return fmt.Errorf("failed to save step read model: %w", err)
	}

	return p.saveReadModel(ctx, model)
}

func (p *SagaReadModelProjection) handleStepFailedFromMap(ctx context.Context, eventData map[string]interface{}) error {
	sagaID, _ := eventData["saga_id"].(string)
	stepName, _ := eventData["step_name"].(string)
	errorMsg, _ := eventData["error"].(string)
	retryAttempt := 0
	if ra, ok := eventData["retry_attempt"].(float64); ok {
		retryAttempt = int(ra)
	}
	
	var timestamp time.Time
	if ts, ok := eventData["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			timestamp = t
		}
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	model, err := p.getOrCreateReadModel(ctx, sagaID)
	if err != nil {
		return err
	}

	model.FailedSteps++
	model.LastError = &errorMsg
	model.RetryCount = retryAttempt
	model.UpdatedAt = time.Now()

	// Обновляем шаг в истории
	stepModel := &SagaStepReadModel{
		SagaID:      sagaID,
		StepName:    stepName,
		Status:      "failed",
		StartedAt:   timestamp,
		CompletedAt: &timestamp,
		RetryAttempt: retryAttempt,
		Error:       &errorMsg,
		UpdatedAt:   time.Now(),
	}
	if err := p.store.UpsertSagaStepReadModel(ctx, stepModel); err != nil {
		return fmt.Errorf("failed to save step read model: %w", err)
	}

	return p.saveReadModel(ctx, model)
}

func (p *SagaReadModelProjection) handleStepCompensatedFromMap(ctx context.Context, eventData map[string]interface{}) error {
	sagaID, _ := eventData["saga_id"].(string)

	model, err := p.getOrCreateReadModel(ctx, sagaID)
	if err != nil {
		return err
	}

	model.UpdatedAt = time.Now()

	return p.saveReadModel(ctx, model)
}

func (p *SagaReadModelProjection) handleSagaCompletedFromMap(ctx context.Context, eventData map[string]interface{}) error {
	sagaID, _ := eventData["saga_id"].(string)
	
	var duration time.Duration
	if d, ok := eventData["duration"].(string); ok {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	model, err := p.getOrCreateReadModel(ctx, sagaID)
	if err != nil {
		return err
	}

	model.Status = SagaStatusCompleted
	now := time.Now()
	model.CompletedAt = &now
	model.Duration = &duration
	model.UpdatedAt = now

	return p.saveReadModel(ctx, model)
}

func (p *SagaReadModelProjection) handleSagaFailedFromMap(ctx context.Context, eventData map[string]interface{}) error {
	sagaID, _ := eventData["saga_id"].(string)
	errorMsg, _ := eventData["error"].(string)

	model, err := p.getOrCreateReadModel(ctx, sagaID)
	if err != nil {
		return err
	}

	model.Status = SagaStatusFailed
	now := time.Now()
	model.CompletedAt = &now
	if model.StartedAt != (time.Time{}) {
		duration := now.Sub(model.StartedAt)
		model.Duration = &duration
	}
	model.LastError = &errorMsg
	model.UpdatedAt = now

	return p.saveReadModel(ctx, model)
}

func (p *SagaReadModelProjection) handleSagaCompensatedFromMap(ctx context.Context, eventData map[string]interface{}) error {
	sagaID, _ := eventData["saga_id"].(string)

	model, err := p.getOrCreateReadModel(ctx, sagaID)
	if err != nil {
		return err
	}

	model.Status = SagaStatusCompensated
	now := time.Now()
	model.CompletedAt = &now
	if model.StartedAt != (time.Time{}) {
		duration := now.Sub(model.StartedAt)
		model.Duration = &duration
	}
	model.UpdatedAt = now

	return p.saveReadModel(ctx, model)
}

