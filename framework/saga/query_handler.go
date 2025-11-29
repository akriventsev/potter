// Package saga предоставляет механизмы для работы с сагами.
package saga

import (
	"context"
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/transport"
)

// GetSagaStatusQuery запрос для получения статуса саги
type GetSagaStatusQuery struct {
	SagaID string
}

func (q *GetSagaStatusQuery) QueryName() string {
	return "GetSagaStatus"
}

// GetSagaHistoryQuery запрос для получения истории саги
type GetSagaHistoryQuery struct {
	SagaID string
}

func (q *GetSagaHistoryQuery) QueryName() string {
	return "GetSagaHistory"
}

// ListSagasQuery запрос для получения списка саг
type ListSagasQuery struct {
	Status         *SagaStatus
	DefinitionName *string
	CorrelationID  *string
	StartedAfter   *time.Time
	StartedBefore  *time.Time
	Limit          int
	Offset         int
}

func (q *ListSagasQuery) QueryName() string {
	return "ListSagas"
}

// GetSagaMetricsQuery запрос для получения метрик саг
type GetSagaMetricsQuery struct {
	DefinitionName *string
	StartedAfter   *time.Time
	StartedBefore  *time.Time
}

func (q *GetSagaMetricsQuery) QueryName() string {
	return "GetSagaMetrics"
}

// SagaStatusResponse ответ со статусом саги
type SagaStatusResponse struct {
	SagaID         string
	DefinitionName string
	Status         SagaStatus
	CurrentStep    string
	TotalSteps     int
	CompletedSteps int
	FailedSteps    int
	StartedAt      time.Time
	CompletedAt    *time.Time
	Duration       *time.Duration
	CorrelationID  string
	Context        map[string]interface{}
	LastError      *string
	RetryCount     int
}

// SagaHistoryResponse ответ с историей саги
type SagaHistoryResponse struct {
	SagaID  string
	History []SagaStepHistory
}

// SagaStepHistory история шага саги
type SagaStepHistory struct {
	StepName     string
	Status       string
	StartedAt    time.Time
	CompletedAt  *time.Time
	Duration     *time.Duration
	RetryAttempt int
	Error        *string
}

// SagaListResponse ответ со списком саг
type SagaListResponse struct {
	Sagas  []SagaSummary
	Total  int
	Limit  int
	Offset int
}

// SagaSummary краткая информация о саге
type SagaSummary struct {
	SagaID         string
	DefinitionName string
	Status         SagaStatus
	CurrentStep    string
	StartedAt      time.Time
	CompletedAt    *time.Time
	CorrelationID  string
}

// SagaMetricsResponse ответ с метриками саг
type SagaMetricsResponse struct {
	TotalSagas       int
	CompletedSagas   int
	FailedSagas      int
	CompensatedSagas int
	SuccessRate      float64
	AvgDuration      time.Duration
	Throughput       float64 // саг в час
}

// SagaQueryHandler обработчик запросов о сагах
type SagaQueryHandler struct {
	persistence    SagaPersistence
	readModelStore SagaReadModelStore
}

// NewSagaQueryHandler создает новый SagaQueryHandler
func NewSagaQueryHandler(persistence SagaPersistence, readModelStore SagaReadModelStore) *SagaQueryHandler {
	return &SagaQueryHandler{
		persistence:    persistence,
		readModelStore: readModelStore,
	}
}

// Handle обрабатывает запрос
func (h *SagaQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	switch query := q.(type) {
	case *GetSagaStatusQuery:
		return h.handleGetStatus(ctx, query)
	case *GetSagaHistoryQuery:
		return h.handleGetHistory(ctx, query)
	case *ListSagasQuery:
		return h.handleListSagas(ctx, query)
	case *GetSagaMetricsQuery:
		return h.handleGetMetrics(ctx, query)
	default:
		return nil, fmt.Errorf("unknown query type: %T", q)
	}
}

// QueryName возвращает имя запроса
func (h *SagaQueryHandler) QueryName() string {
	return "SagaQuery"
}

func (h *SagaQueryHandler) handleGetStatus(ctx context.Context, query *GetSagaStatusQuery) (*SagaStatusResponse, error) {
	// Используем read model store если доступен
	if h.readModelStore != nil {
		return h.readModelStore.GetSagaStatus(ctx, query.SagaID)
	}

	// Иначе загружаем из persistence
	saga, err := h.persistence.Load(ctx, query.SagaID)
	if err != nil {
		return nil, fmt.Errorf("failed to load saga: %w", err)
	}

	definition := saga.Definition()
	context := saga.Context()
	history := saga.GetHistory()

	response := &SagaStatusResponse{
		SagaID:         saga.ID(),
		DefinitionName: definition.Name(),
		Status:         saga.Status(),
		CurrentStep:    saga.CurrentStep(),
		CorrelationID:  context.CorrelationID(),
		Context:        context.ToMap(),
		TotalSteps:     len(definition.Steps()),
	}

	// Подсчитываем completed и failed steps из истории
	completedSteps := 0
	failedSteps := 0
	for _, h := range history {
		if h.Status == StepStatusCompleted {
			completedSteps++
		} else if h.Status == StepStatusFailed {
			failedSteps++
		}
	}
	response.CompletedSteps = completedSteps
	response.FailedSteps = failedSteps

	// Получаем startedAt из первой записи истории
	if len(history) > 0 {
		response.StartedAt = history[0].StartedAt
	}

	// Получаем completedAt из последней записи истории
	if len(history) > 0 {
		lastEntry := history[len(history)-1]
		if lastEntry.CompletedAt != nil {
			response.CompletedAt = lastEntry.CompletedAt
			duration := lastEntry.CompletedAt.Sub(response.StartedAt)
			response.Duration = &duration
		}
		// Получаем последнюю ошибку
		if lastEntry.Error != nil {
			errMsg := lastEntry.Error.Error()
			response.LastError = &errMsg
		}
		response.RetryCount = lastEntry.RetryAttempt
	}

	return response, nil
}

func (h *SagaQueryHandler) handleGetHistory(ctx context.Context, query *GetSagaHistoryQuery) (*SagaHistoryResponse, error) {
	history, err := h.persistence.GetHistory(ctx, query.SagaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	stepHistory := make([]SagaStepHistory, len(history))
	for i, h := range history {
		stepHistory[i] = SagaStepHistory{
			StepName:     h.StepName,
			Status:       string(h.Status),
			StartedAt:    h.StartedAt,
			RetryAttempt: h.RetryAttempt,
		}
		if h.CompletedAt != nil {
			stepHistory[i].CompletedAt = h.CompletedAt
			duration := h.CompletedAt.Sub(h.StartedAt)
			stepHistory[i].Duration = &duration
		}
		if h.Error != nil {
			errMsg := h.Error.Error()
			stepHistory[i].Error = &errMsg
		}
	}

	return &SagaHistoryResponse{
		SagaID:  query.SagaID,
		History: stepHistory,
	}, nil
}

func (h *SagaQueryHandler) handleListSagas(ctx context.Context, query *ListSagasQuery) (*SagaListResponse, error) {
	if h.readModelStore != nil {
		filter := SagaFilter{
			Status:         query.Status,
			DefinitionName: query.DefinitionName,
			CorrelationID:  query.CorrelationID,
			StartedAfter:   query.StartedAfter,
			StartedBefore:  query.StartedBefore,
			Limit:          query.Limit,
			Offset:         query.Offset,
		}
		return h.readModelStore.ListSagas(ctx, filter)
	}

	// Базовая реализация через persistence
	var status SagaStatus
	if query.Status != nil {
		status = *query.Status
	} else {
		status = SagaStatusRunning
	}

	sagas, err := h.persistence.LoadAll(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to load sagas: %w", err)
	}

	summaries := make([]SagaSummary, 0, len(sagas))
	for _, saga := range sagas {
		definition := saga.Definition()
		context := saga.Context()
		history := saga.GetHistory()

		// Применяем фильтры
		if query.DefinitionName != nil && definition.Name() != *query.DefinitionName {
			continue
		}
		if query.CorrelationID != nil && context.CorrelationID() != *query.CorrelationID {
			continue
		}

		summary := SagaSummary{
			SagaID:         saga.ID(),
			DefinitionName: definition.Name(),
			Status:         saga.Status(),
			CurrentStep:    saga.CurrentStep(),
			CorrelationID:  context.CorrelationID(),
		}

		// Получаем startedAt и completedAt из истории
		if len(history) > 0 {
			summary.StartedAt = history[0].StartedAt
			lastEntry := history[len(history)-1]
			if lastEntry.CompletedAt != nil {
				summary.CompletedAt = lastEntry.CompletedAt
			}
		}

		summaries = append(summaries, summary)
	}

	// Применяем пагинацию
	total := len(summaries)
	start := query.Offset
	if start > total {
		start = total
	}
	end := start + query.Limit
	if end > total {
		end = total
	}
	if start < end {
		summaries = summaries[start:end]
	} else {
		summaries = []SagaSummary{}
	}

	return &SagaListResponse{
		Sagas:  summaries,
		Total:  total,
		Limit:  query.Limit,
		Offset: query.Offset,
	}, nil
}

func (h *SagaQueryHandler) handleGetMetrics(ctx context.Context, query *GetSagaMetricsQuery) (*SagaMetricsResponse, error) {
	if h.readModelStore != nil {
		filter := MetricsFilter{
			DefinitionName: query.DefinitionName,
			StartedAfter:   query.StartedAfter,
			StartedBefore:  query.StartedBefore,
		}
		return h.readModelStore.GetMetrics(ctx, filter)
	}

	// Базовая реализация через persistence
	allStatuses := []SagaStatus{SagaStatusRunning, SagaStatusCompleted, SagaStatusFailed, SagaStatusCompensated}

	var total, completed, failed, compensated int
	var totalDuration time.Duration
	var sagaCount int

	for _, status := range allStatuses {
		sagas, err := h.persistence.LoadAll(ctx, status)
		if err != nil {
			continue
		}

		for _, saga := range sagas {
			definition := saga.Definition()
			history := saga.GetHistory()

			// Применяем фильтры
			if query.DefinitionName != nil && definition.Name() != *query.DefinitionName {
				continue
			}
			var startedAt time.Time
			if len(history) > 0 {
				startedAt = history[0].StartedAt
			}
			if query.StartedAfter != nil && startedAt.Before(*query.StartedAfter) {
				continue
			}
			if query.StartedBefore != nil && startedAt.After(*query.StartedBefore) {
				continue
			}

			total++
			switch saga.Status() {
			case SagaStatusCompleted:
				completed++
			case SagaStatusFailed:
				failed++
			case SagaStatusCompensated:
				compensated++
			}

			if len(history) > 0 {
				lastEntry := history[len(history)-1]
				if lastEntry.CompletedAt != nil {
					duration := lastEntry.CompletedAt.Sub(startedAt)
					totalDuration += duration
					sagaCount++
				}
			}
		}
	}

	var successRate float64
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}

	var avgDuration time.Duration
	if sagaCount > 0 {
		avgDuration = totalDuration / time.Duration(sagaCount)
	}

	return &SagaMetricsResponse{
		TotalSagas:       total,
		CompletedSagas:   completed,
		FailedSagas:      failed,
		CompensatedSagas: compensated,
		SuccessRate:      successRate,
		AvgDuration:      avgDuration,
		Throughput:       0, // Требует дополнительных данных
	}, nil
}
