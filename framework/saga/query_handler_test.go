package saga

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockSagaPersistence mock реализация SagaPersistence для тестов
type mockSagaPersistence struct {
	sagas   map[string]Saga
	history map[string][]SagaHistory
}

func newMockSagaPersistence() *mockSagaPersistence {
	return &mockSagaPersistence{
		sagas:   make(map[string]Saga),
		history: make(map[string][]SagaHistory),
	}
}

func (m *mockSagaPersistence) Save(ctx context.Context, saga Saga) error {
	m.sagas[saga.ID()] = saga
	return nil
}

func (m *mockSagaPersistence) Load(ctx context.Context, sagaID string) (Saga, error) {
	saga, exists := m.sagas[sagaID]
	if !exists {
		return nil, fmt.Errorf("saga %s not found", sagaID)
	}
	return saga, nil
}

func (m *mockSagaPersistence) LoadAll(ctx context.Context, status SagaStatus) ([]Saga, error) {
	var result []Saga
	for _, saga := range m.sagas {
		if saga.Status() == status {
			result = append(result, saga)
		}
	}
	return result, nil
}

func (m *mockSagaPersistence) Delete(ctx context.Context, sagaID string) error {
	delete(m.sagas, sagaID)
	delete(m.history, sagaID)
	return nil
}

func (m *mockSagaPersistence) GetHistory(ctx context.Context, sagaID string) ([]SagaHistory, error) {
	history, exists := m.history[sagaID]
	if !exists {
		return nil, fmt.Errorf("history for saga %s not found", sagaID)
	}
	return history, nil
}

func (m *mockSagaPersistence) setHistory(sagaID string, history []SagaHistory) {
	m.history[sagaID] = history
}

func TestSagaQueryHandler_GetStatus_WithReadModelStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemorySagaReadModelStore()
	persistence := newMockSagaPersistence()

	handler := NewSagaQueryHandler(persistence, store)

	// Создаем тестовый read model
	model := &SagaReadModel{
		SagaID:        "test-saga-1",
		DefinitionName: "test_saga",
		Status:        SagaStatusRunning,
		CurrentStep:   "step1",
		TotalSteps:    3,
		CompletedSteps: 1,
		FailedSteps:   0,
		StartedAt:     time.Now(),
		CorrelationID: "corr-123",
		Context:       map[string]interface{}{"key": "value"},
		UpdatedAt:     time.Now(),
	}

	if err := store.UpsertSagaReadModel(ctx, model); err != nil {
		t.Fatalf("Failed to upsert read model: %v", err)
	}

	query := &GetSagaStatusQuery{SagaID: "test-saga-1"}
	result, err := handler.Handle(ctx, query)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	status, ok := result.(*SagaStatusResponse)
	if !ok {
		t.Fatalf("Expected *SagaStatusResponse, got %T", result)
	}

	if status.SagaID != "test-saga-1" {
		t.Errorf("Expected saga ID test-saga-1, got %s", status.SagaID)
	}

	if status.Status != SagaStatusRunning {
		t.Errorf("Expected status running, got %s", status.Status)
	}

	if status.CurrentStep != "step1" {
		t.Errorf("Expected current step step1, got %s", status.CurrentStep)
	}

	if status.TotalSteps != 3 {
		t.Errorf("Expected total steps 3, got %d", status.TotalSteps)
	}

	if status.CompletedSteps != 1 {
		t.Errorf("Expected completed steps 1, got %d", status.CompletedSteps)
	}
}

func TestSagaQueryHandler_GetStatus_WithPersistenceFallback(t *testing.T) {
	ctx := context.Background()
	persistence := newMockSagaPersistence()

	// Создаем сагу для теста
	definition := NewBaseSagaDefinition("test_saga")
	step := NewBaseStep("step1")
	step.WithExecute(func(ctx context.Context, sagaCtx SagaContext) error {
		return nil
	})
	definition.AddStep(step)

	sagaCtx := NewSagaContext()
	sagaCtx.SetCorrelationID("corr-123")
	saga, err := NewBaseSaga("test-saga-1", definition, sagaCtx, persistence)
	if err != nil {
		t.Fatalf("Failed to create saga: %v", err)
	}

	// Сохраняем сагу
	if err := persistence.Save(ctx, saga); err != nil {
		t.Fatalf("Failed to save saga: %v", err)
	}

	// Создаем handler без read model store (fallback на persistence)
	handler := NewSagaQueryHandler(persistence, nil)

	query := &GetSagaStatusQuery{SagaID: "test-saga-1"}
	result, err := handler.Handle(ctx, query)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	status, ok := result.(*SagaStatusResponse)
	if !ok {
		t.Fatalf("Expected *SagaStatusResponse, got %T", result)
	}

	if status.SagaID != "test-saga-1" {
		t.Errorf("Expected saga ID test-saga-1, got %s", status.SagaID)
	}
}

func TestSagaQueryHandler_GetStatus_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemorySagaReadModelStore()
	persistence := newMockSagaPersistence()

	handler := NewSagaQueryHandler(persistence, store)

	query := &GetSagaStatusQuery{SagaID: "non-existent"}
	_, err := handler.Handle(ctx, query)
	if err == nil {
		t.Error("Expected error for non-existent saga")
	}
}

func TestSagaQueryHandler_GetHistory(t *testing.T) {
	ctx := context.Background()
	persistence := newMockSagaPersistence()

	// Создаем тестовую историю
	history := []SagaHistory{
		{
			StepName:     "step1",
			Status:       StepStatusCompleted,
			StartedAt:    time.Now().Add(-2 * time.Minute),
			CompletedAt:  func() *time.Time { t := time.Now().Add(-1 * time.Minute); return &t }(),
			RetryAttempt: 0,
		},
		{
			StepName:     "step2",
			Status:       StepStatusRunning,
			StartedAt:    time.Now().Add(-1 * time.Minute),
			RetryAttempt: 0,
		},
	}
	persistence.setHistory("test-saga-1", history)

	handler := NewSagaQueryHandler(persistence, nil)

	query := &GetSagaHistoryQuery{SagaID: "test-saga-1"}
	result, err := handler.Handle(ctx, query)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	historyResponse, ok := result.(*SagaHistoryResponse)
	if !ok {
		t.Fatalf("Expected *SagaHistoryResponse, got %T", result)
	}

	if historyResponse.SagaID != "test-saga-1" {
		t.Errorf("Expected saga ID test-saga-1, got %s", historyResponse.SagaID)
	}

	if len(historyResponse.History) != 2 {
		t.Errorf("Expected 2 history entries, got %d", len(historyResponse.History))
	}

	if historyResponse.History[0].StepName != "step1" {
		t.Errorf("Expected first step step1, got %s", historyResponse.History[0].StepName)
	}

	if historyResponse.History[0].Status != "completed" {
		t.Errorf("Expected first step status completed, got %s", historyResponse.History[0].Status)
	}
}

func TestSagaQueryHandler_ListSagas_WithReadModelStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemorySagaReadModelStore()
	persistence := newMockSagaPersistence()

	handler := NewSagaQueryHandler(persistence, store)

	// Создаем несколько read models
	for i := 0; i < 5; i++ {
		status := SagaStatusRunning
		if i >= 3 {
			status = SagaStatusCompleted
		}

		model := &SagaReadModel{
			SagaID:        fmt.Sprintf("test-saga-%d", i+1),
			DefinitionName: "test_saga",
			Status:        status,
			StartedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := store.UpsertSagaReadModel(ctx, model); err != nil {
			t.Fatalf("Failed to upsert read model: %v", err)
		}
	}

	runningStatus := SagaStatusRunning
	query := &ListSagasQuery{
		Status: &runningStatus,
		Limit:  10,
		Offset: 0,
	}

	result, err := handler.Handle(ctx, query)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	listResponse, ok := result.(*SagaListResponse)
	if !ok {
		t.Fatalf("Expected *SagaListResponse, got %T", result)
	}

	if listResponse.Total != 3 {
		t.Errorf("Expected 3 running sagas, got %d", listResponse.Total)
	}
}

func TestSagaQueryHandler_ListSagas_WithPagination(t *testing.T) {
	ctx := context.Background()
	store := NewInMemorySagaReadModelStore()
	persistence := newMockSagaPersistence()

	handler := NewSagaQueryHandler(persistence, store)

	// Создаем 10 саг
	for i := 0; i < 10; i++ {
		model := &SagaReadModel{
			SagaID:        fmt.Sprintf("test-saga-%d", i+1),
			DefinitionName: "test_saga",
			Status:        SagaStatusRunning,
			StartedAt:     time.Now().Add(-time.Duration(i) * time.Minute),
			UpdatedAt:     time.Now(),
		}
		if err := store.UpsertSagaReadModel(ctx, model); err != nil {
			t.Fatalf("Failed to upsert read model: %v", err)
		}
	}

	runningStatus := SagaStatusRunning
	query := &ListSagasQuery{
		Status: &runningStatus,
		Limit:  5,
		Offset: 0,
	}

	result, err := handler.Handle(ctx, query)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	listResponse, ok := result.(*SagaListResponse)
	if !ok {
		t.Fatalf("Expected *SagaListResponse, got %T", result)
	}

	if len(listResponse.Sagas) != 5 {
		t.Errorf("Expected 5 sagas in page, got %d", len(listResponse.Sagas))
	}

	if listResponse.Total != 10 {
		t.Errorf("Expected total 10 sagas, got %d", listResponse.Total)
	}
}

func TestSagaQueryHandler_GetMetrics_WithReadModelStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemorySagaReadModelStore()
	persistence := newMockSagaPersistence()

	handler := NewSagaQueryHandler(persistence, store)

	// Создаем саги с разными статусами
	statuses := []SagaStatus{
		SagaStatusCompleted,
		SagaStatusCompleted,
		SagaStatusFailed,
		SagaStatusCompensated,
		SagaStatusRunning,
	}

	now := time.Now()
	for i, status := range statuses {
		model := &SagaReadModel{
			SagaID:        fmt.Sprintf("test-saga-%d", i+1),
			DefinitionName: "test_saga",
			Status:        status,
			StartedAt:     now.Add(-time.Duration(i+1) * time.Hour),
			UpdatedAt:     now,
		}

		if status == SagaStatusCompleted {
			completedAt := now.Add(-time.Duration(i) * time.Hour)
			duration := completedAt.Sub(model.StartedAt)
			model.CompletedAt = &completedAt
			model.Duration = &duration
		}

		if err := store.UpsertSagaReadModel(ctx, model); err != nil {
			t.Fatalf("Failed to upsert read model: %v", err)
		}
	}

	query := &GetSagaMetricsQuery{}
	result, err := handler.Handle(ctx, query)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	metrics, ok := result.(*SagaMetricsResponse)
	if !ok {
		t.Fatalf("Expected *SagaMetricsResponse, got %T", result)
	}

	if metrics.TotalSagas != 5 {
		t.Errorf("Expected 5 total sagas, got %d", metrics.TotalSagas)
	}

	if metrics.CompletedSagas != 2 {
		t.Errorf("Expected 2 completed sagas, got %d", metrics.CompletedSagas)
	}

	if metrics.FailedSagas != 1 {
		t.Errorf("Expected 1 failed saga, got %d", metrics.FailedSagas)
	}

	if metrics.CompensatedSagas != 1 {
		t.Errorf("Expected 1 compensated saga, got %d", metrics.CompensatedSagas)
	}

	expectedSuccessRate := 40.0 // 2 completed / 5 total * 100
	if metrics.SuccessRate != expectedSuccessRate {
		t.Errorf("Expected success rate %.1f, got %.1f", expectedSuccessRate, metrics.SuccessRate)
	}
}

func TestSagaQueryHandler_GetStatus_InMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemorySagaReadModelStore()

	// Создаем тестовый read model
	model := &SagaReadModel{
		SagaID:        "test-saga-1",
		DefinitionName: "test_saga",
		Status:        SagaStatusRunning,
		CurrentStep:   "step1",
		TotalSteps:    3,
		CompletedSteps: 1,
		FailedSteps:   0,
		StartedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := store.UpsertSagaReadModel(ctx, model); err != nil {
		t.Fatalf("Failed to upsert read model: %v", err)
	}

	status, err := store.GetSagaStatus(ctx, "test-saga-1")
	if err != nil {
		t.Fatalf("Failed to get saga status: %v", err)
	}

	if status.SagaID != "test-saga-1" {
		t.Errorf("Expected saga ID test-saga-1, got %s", status.SagaID)
	}

	if status.Status != SagaStatusRunning {
		t.Errorf("Expected status running, got %s", status.Status)
	}
}

func TestSagaQueryHandler_ListSagas_InMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemorySagaReadModelStore()

	// Создаем несколько read models
	for i := 0; i < 5; i++ {
		model := &SagaReadModel{
			SagaID:        fmt.Sprintf("test-saga-%d", i+1),
			DefinitionName: "test_saga",
			Status:        SagaStatusRunning,
			StartedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := store.UpsertSagaReadModel(ctx, model); err != nil {
			t.Fatalf("Failed to upsert read model: %v", err)
		}
	}

	runningStatus := SagaStatusRunning
	filter := SagaFilter{
		Status: &runningStatus,
		Limit:  10,
		Offset: 0,
	}

	result, err := store.ListSagas(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list sagas: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("Expected 5 sagas, got %d", result.Total)
	}
}

func TestSagaQueryHandler_GetMetrics_InMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemorySagaReadModelStore()

	// Создаем саги с разными статусами
	statuses := []SagaStatus{SagaStatusCompleted, SagaStatusFailed, SagaStatusCompensated, SagaStatusRunning}
	now := time.Now()
	for i, status := range statuses {
		model := &SagaReadModel{
			SagaID:        fmt.Sprintf("test-saga-%d", i+1),
			DefinitionName: "test_saga",
			Status:        status,
			StartedAt:     now.Add(-time.Duration(i+1) * time.Hour),
			UpdatedAt:     now,
		}

		if status == SagaStatusCompleted {
			completedAt := now.Add(-time.Duration(i) * time.Hour)
			duration := completedAt.Sub(model.StartedAt)
			model.CompletedAt = &completedAt
			model.Duration = &duration
		}

		if err := store.UpsertSagaReadModel(ctx, model); err != nil {
			t.Fatalf("Failed to upsert read model: %v", err)
		}
	}

	filter := MetricsFilter{}
	metrics, err := store.GetMetrics(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.TotalSagas != 4 {
		t.Errorf("Expected 4 total sagas, got %d", metrics.TotalSagas)
	}

	if metrics.CompletedSagas != 1 {
		t.Errorf("Expected 1 completed saga, got %d", metrics.CompletedSagas)
	}
}
