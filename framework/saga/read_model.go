// Package saga предоставляет механизмы для работы с сагами.
package saga

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SagaReadModelStore интерфейс для read model store
type SagaReadModelStore interface {
	GetSagaStatus(ctx context.Context, sagaID string) (*SagaStatusResponse, error)
	ListSagas(ctx context.Context, filter SagaFilter) (*SagaListResponse, error)
	GetMetrics(ctx context.Context, filter MetricsFilter) (*SagaMetricsResponse, error)
	UpsertSagaReadModel(ctx context.Context, rm *SagaReadModel) error
	UpsertSagaStepReadModel(ctx context.Context, step *SagaStepReadModel) error
}

// SagaFilter фильтр для запросов саг
type SagaFilter struct {
	Status         *SagaStatus
	DefinitionName *string
	CorrelationID  *string
	StartedAfter   *time.Time
	StartedBefore  *time.Time
	Limit          int
	Offset         int
}

// MetricsFilter фильтр для метрик
type MetricsFilter struct {
	DefinitionName *string
	StartedAfter   *time.Time
	StartedBefore  *time.Time
	GroupBy        string // "hour", "day", "week", "month"
}

// InMemorySagaReadModelStore реализация read model store в памяти для тестирования
type InMemorySagaReadModelStore struct {
	models map[string]*SagaReadModel
}

// NewInMemorySagaReadModelStore создает новый InMemorySagaReadModelStore
func NewInMemorySagaReadModelStore() *InMemorySagaReadModelStore {
	return &InMemorySagaReadModelStore{
		models: make(map[string]*SagaReadModel),
	}
}

func (s *InMemorySagaReadModelStore) GetSagaStatus(ctx context.Context, sagaID string) (*SagaStatusResponse, error) {
	model, ok := s.models[sagaID]
	if !ok {
		return nil, fmt.Errorf("saga not found: %s", sagaID)
	}

	response := &SagaStatusResponse{
		SagaID:        model.SagaID,
		DefinitionName: model.DefinitionName,
		Status:        model.Status,
		CurrentStep:   model.CurrentStep,
		TotalSteps:    model.TotalSteps,
		CompletedSteps: model.CompletedSteps,
		FailedSteps:   model.FailedSteps,
		StartedAt:     model.StartedAt,
		CompletedAt:   model.CompletedAt,
		Duration:      model.Duration,
		CorrelationID: model.CorrelationID,
		Context:       model.Context,
		RetryCount:    model.RetryCount,
	}

	if model.LastError != nil {
		errMsg := *model.LastError
		response.LastError = &errMsg
	}

	return response, nil
}

func (s *InMemorySagaReadModelStore) ListSagas(ctx context.Context, filter SagaFilter) (*SagaListResponse, error) {
	var summaries []SagaSummary

	for _, model := range s.models {
		// Применяем фильтры
		if filter.Status != nil && model.Status != *filter.Status {
			continue
		}
		if filter.DefinitionName != nil && model.DefinitionName != *filter.DefinitionName {
			continue
		}
		if filter.CorrelationID != nil && model.CorrelationID != *filter.CorrelationID {
			continue
		}
		if filter.StartedAfter != nil && model.StartedAt.Before(*filter.StartedAfter) {
			continue
		}
		if filter.StartedBefore != nil && model.StartedAt.After(*filter.StartedBefore) {
			continue
		}

		summary := SagaSummary{
			SagaID:        model.SagaID,
			DefinitionName: model.DefinitionName,
			Status:        model.Status,
			CurrentStep:   model.CurrentStep,
			StartedAt:     model.StartedAt,
			CompletedAt:   model.CompletedAt,
			CorrelationID: model.CorrelationID,
		}
		summaries = append(summaries, summary)
	}

	// Применяем пагинацию
	total := len(summaries)
	start := filter.Offset
	if start > total {
		start = total
	}
	end := start + filter.Limit
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
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

func (s *InMemorySagaReadModelStore) UpsertSagaReadModel(ctx context.Context, model *SagaReadModel) error {
	s.models[model.SagaID] = model
	return nil
}

func (s *InMemorySagaReadModelStore) UpsertSagaStepReadModel(ctx context.Context, step *SagaStepReadModel) error {
	// Для in-memory store шаги хранятся в самой саге через GetHistory
	// Можно расширить структуру для хранения шагов отдельно если нужно
	return nil
}

func (s *InMemorySagaReadModelStore) GetMetrics(ctx context.Context, filter MetricsFilter) (*SagaMetricsResponse, error) {
	var total, completed, failed, compensated int
	var totalDuration time.Duration
	var sagaCount int

	for _, model := range s.models {
		// Применяем фильтры
		if filter.DefinitionName != nil && model.DefinitionName != *filter.DefinitionName {
			continue
		}
		if filter.StartedAfter != nil && model.StartedAt.Before(*filter.StartedAfter) {
			continue
		}
		if filter.StartedBefore != nil && model.StartedAt.After(*filter.StartedBefore) {
			continue
		}

		total++
		switch model.Status {
		case SagaStatusCompleted:
			completed++
		case SagaStatusFailed:
			failed++
		case SagaStatusCompensated:
			compensated++
		}

		if model.CompletedAt != nil && model.Duration != nil {
			totalDuration += *model.Duration
			sagaCount++
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
		Throughput:       0,
	}, nil
}

// SagaReadModel денормализованное представление саги
type SagaReadModel struct {
	SagaID        string
	DefinitionName string
	Status        SagaStatus
	CurrentStep   string
	TotalSteps    int
	CompletedSteps int
	FailedSteps   int
	StartedAt     time.Time
	CompletedAt   *time.Time
	Duration      *time.Duration
	CorrelationID string
	Context       map[string]interface{}
	LastError     *string
	RetryCount    int
	UpdatedAt     time.Time
}

// SagaStepReadModel денормализованное представление шага саги для истории
type SagaStepReadModel struct {
	SagaID        string
	StepName      string
	Status        string // "started", "completed", "failed", "compensated"
	StartedAt     time.Time
	CompletedAt   *time.Time
	Duration      *time.Duration
	RetryAttempt  int
	Error         *string
	UpdatedAt     time.Time
}

// PostgresSagaReadModelStore реализация read model store для PostgreSQL
type PostgresSagaReadModelStore struct {
	conn *pgx.Conn
}

// NewPostgresSagaReadModelStore создает новый PostgresSagaReadModelStore
func NewPostgresSagaReadModelStore(dsn string) (*PostgresSagaReadModelStore, error) {
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	store := &PostgresSagaReadModelStore{conn: conn}
	if err := store.ensureTable(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure table: %w", err)
	}

	return store, nil
}

func (s *PostgresSagaReadModelStore) ensureTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS saga_read_models (
			saga_id VARCHAR(255) PRIMARY KEY,
			definition_name VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			current_step VARCHAR(255),
			total_steps INTEGER,
			completed_steps INTEGER,
			failed_steps INTEGER,
			started_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP,
			duration_ms INTEGER,
			correlation_id VARCHAR(255),
			context JSONB,
			last_error TEXT,
			retry_count INTEGER,
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		
		CREATE TABLE IF NOT EXISTS saga_step_read_models (
			saga_id VARCHAR(255) NOT NULL,
			step_name VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			started_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP,
			duration_ms INTEGER,
			retry_attempt INTEGER DEFAULT 0,
			error TEXT,
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (saga_id, step_name, started_at)
		);
		
		CREATE INDEX IF NOT EXISTS idx_saga_rm_status ON saga_read_models(status);
		CREATE INDEX IF NOT EXISTS idx_saga_rm_definition ON saga_read_models(definition_name);
		CREATE INDEX IF NOT EXISTS idx_saga_rm_correlation ON saga_read_models(correlation_id);
		CREATE INDEX IF NOT EXISTS idx_saga_rm_started_at ON saga_read_models(started_at);
		CREATE INDEX IF NOT EXISTS idx_saga_step_rm_saga_id ON saga_step_read_models(saga_id);
		CREATE INDEX IF NOT EXISTS idx_saga_step_rm_status ON saga_step_read_models(status);
	`
	_, err := s.conn.Exec(ctx, query)
	return err
}

func (s *PostgresSagaReadModelStore) GetSagaStatus(ctx context.Context, sagaID string) (*SagaStatusResponse, error) {
	query := `
		SELECT saga_id, definition_name, status, current_step, total_steps,
		       completed_steps, failed_steps, started_at, completed_at, duration_ms,
		       correlation_id, context, last_error, retry_count
		FROM saga_read_models
		WHERE saga_id = $1
	`

	var model SagaReadModel
	var durationMs *int64
	err := s.conn.QueryRow(ctx, query, sagaID).Scan(
		&model.SagaID,
		&model.DefinitionName,
		&model.Status,
		&model.CurrentStep,
		&model.TotalSteps,
		&model.CompletedSteps,
		&model.FailedSteps,
		&model.StartedAt,
		&model.CompletedAt,
		&durationMs,
		&model.CorrelationID,
		&model.Context,
		&model.LastError,
		&model.RetryCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get saga status: %w", err)
	}

	if durationMs != nil {
		duration := time.Duration(*durationMs) * time.Millisecond
		model.Duration = &duration
	}

	response := &SagaStatusResponse{
		SagaID:        model.SagaID,
		DefinitionName: model.DefinitionName,
		Status:        model.Status,
		CurrentStep:   model.CurrentStep,
		TotalSteps:    model.TotalSteps,
		CompletedSteps: model.CompletedSteps,
		FailedSteps:   model.FailedSteps,
		StartedAt:     model.StartedAt,
		CompletedAt:   model.CompletedAt,
		Duration:      model.Duration,
		CorrelationID: model.CorrelationID,
		Context:       model.Context,
		LastError:     model.LastError,
		RetryCount:    model.RetryCount,
	}

	return response, nil
}

func (s *PostgresSagaReadModelStore) UpsertSagaReadModel(ctx context.Context, model *SagaReadModel) error {
	var durationMs *int64
	if model.Duration != nil {
		ms := int64(model.Duration.Milliseconds())
		durationMs = &ms
	}

	query := `
		INSERT INTO saga_read_models (
			saga_id, definition_name, status, current_step, total_steps,
			completed_steps, failed_steps, started_at, completed_at, duration_ms,
			correlation_id, context, last_error, retry_count, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (saga_id) DO UPDATE SET
			definition_name = EXCLUDED.definition_name,
			status = EXCLUDED.status,
			current_step = EXCLUDED.current_step,
			total_steps = EXCLUDED.total_steps,
			completed_steps = EXCLUDED.completed_steps,
			failed_steps = EXCLUDED.failed_steps,
			started_at = EXCLUDED.started_at,
			completed_at = EXCLUDED.completed_at,
			duration_ms = EXCLUDED.duration_ms,
			correlation_id = EXCLUDED.correlation_id,
			context = EXCLUDED.context,
			last_error = EXCLUDED.last_error,
			retry_count = EXCLUDED.retry_count,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.conn.Exec(ctx, query,
		model.SagaID,
		model.DefinitionName,
		string(model.Status),
		model.CurrentStep,
		model.TotalSteps,
		model.CompletedSteps,
		model.FailedSteps,
		model.StartedAt,
		model.CompletedAt,
		durationMs,
		model.CorrelationID,
		model.Context,
		model.LastError,
		model.RetryCount,
		model.UpdatedAt,
	)
	return err
}

func (s *PostgresSagaReadModelStore) UpsertSagaStepReadModel(ctx context.Context, step *SagaStepReadModel) error {
	var durationMs *int64
	if step.Duration != nil {
		ms := int64(step.Duration.Milliseconds())
		durationMs = &ms
	}

	query := `
		INSERT INTO saga_step_read_models (
			saga_id, step_name, status, started_at, completed_at, duration_ms,
			retry_attempt, error, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (saga_id, step_name, started_at) DO UPDATE SET
			status = EXCLUDED.status,
			completed_at = EXCLUDED.completed_at,
			duration_ms = EXCLUDED.duration_ms,
			retry_attempt = EXCLUDED.retry_attempt,
			error = EXCLUDED.error,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.conn.Exec(ctx, query,
		step.SagaID,
		step.StepName,
		step.Status,
		step.StartedAt,
		step.CompletedAt,
		durationMs,
		step.RetryAttempt,
		step.Error,
		step.UpdatedAt,
	)
	return err
}

func (s *PostgresSagaReadModelStore) ListSagas(ctx context.Context, filter SagaFilter) (*SagaListResponse, error) {
	query := `SELECT saga_id, definition_name, status, current_step, started_at, completed_at, correlation_id
	          FROM saga_read_models WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, string(*filter.Status))
		argIndex++
	}
	if filter.DefinitionName != nil {
		query += fmt.Sprintf(" AND definition_name = $%d", argIndex)
		args = append(args, *filter.DefinitionName)
		argIndex++
	}
	if filter.CorrelationID != nil {
		query += fmt.Sprintf(" AND correlation_id = $%d", argIndex)
		args = append(args, *filter.CorrelationID)
		argIndex++
	}
	if filter.StartedAfter != nil {
		query += fmt.Sprintf(" AND started_at >= $%d", argIndex)
		args = append(args, *filter.StartedAfter)
		argIndex++
	}
	if filter.StartedBefore != nil {
		query += fmt.Sprintf(" AND started_at <= $%d", argIndex)
		args = append(args, *filter.StartedBefore)
		argIndex++
	}

	query += " ORDER BY started_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sagas: %w", err)
	}
	defer rows.Close()

	var summaries []SagaSummary
	for rows.Next() {
		var summary SagaSummary
		if err := rows.Scan(
			&summary.SagaID,
			&summary.DefinitionName,
			&summary.Status,
			&summary.CurrentStep,
			&summary.StartedAt,
			&summary.CompletedAt,
			&summary.CorrelationID,
		); err != nil {
			continue
		}
		summaries = append(summaries, summary)
	}

	// Получаем общее количество
	countQuery := `SELECT COUNT(*) FROM saga_read_models WHERE 1=1`
	countArgs := args[:len(args)-2] // Убираем LIMIT и OFFSET
	if filter.Offset > 0 {
		countArgs = countArgs[:len(countArgs)-1]
	}

	var total int
	if err := s.conn.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		total = len(summaries)
	}

	return &SagaListResponse{
		Sagas:  summaries,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

func (s *PostgresSagaReadModelStore) GetMetrics(ctx context.Context, filter MetricsFilter) (*SagaMetricsResponse, error) {
	query := `SELECT 
		COUNT(*) as total,
		COUNT(*) FILTER (WHERE status = 'completed') as completed,
		COUNT(*) FILTER (WHERE status = 'failed') as failed,
		COUNT(*) FILTER (WHERE status = 'compensated') as compensated,
		AVG(duration_ms) as avg_duration_ms
		FROM saga_read_models WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filter.DefinitionName != nil {
		query += fmt.Sprintf(" AND definition_name = $%d", argIndex)
		args = append(args, *filter.DefinitionName)
		argIndex++
	}
	if filter.StartedAfter != nil {
		query += fmt.Sprintf(" AND started_at >= $%d", argIndex)
		args = append(args, *filter.StartedAfter)
		argIndex++
	}
	if filter.StartedBefore != nil {
		query += fmt.Sprintf(" AND started_at <= $%d", argIndex)
		args = append(args, *filter.StartedBefore)
		argIndex++
	}

	var total, completed, failed, compensated int
	var avgDurationMs *float64

	err := s.conn.QueryRow(ctx, query, args...).Scan(
		&total,
		&completed,
		&failed,
		&compensated,
		&avgDurationMs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	var successRate float64
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}

	var avgDuration time.Duration
	if avgDurationMs != nil {
		avgDuration = time.Duration(*avgDurationMs) * time.Millisecond
	}

	return &SagaMetricsResponse{
		TotalSagas:       total,
		CompletedSagas:   completed,
		FailedSagas:      failed,
		CompensatedSagas: compensated,
		SuccessRate:      successRate,
		AvgDuration:      avgDuration,
		Throughput:       0,
	}, nil
}

// MongoSagaReadModelStore реализация read model store для MongoDB
type MongoSagaReadModelStore struct {
	collection *mongo.Collection
}

// NewMongoSagaReadModelStore создает новый MongoSagaReadModelStore
func NewMongoSagaReadModelStore(uri, database string) (*MongoSagaReadModelStore, error) {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	collection := client.Database(database).Collection("saga_read_models")
	store := &MongoSagaReadModelStore{collection: collection}
	if err := store.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure indexes: %w", err)
	}

	return store, nil
}

func (s *MongoSagaReadModelStore) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "definition_name", Value: 1}}},
		{Keys: bson.D{{Key: "correlation_id", Value: 1}}},
		{Keys: bson.D{{Key: "started_at", Value: 1}}},
	}
	_, err := s.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (s *MongoSagaReadModelStore) GetSagaStatus(ctx context.Context, sagaID string) (*SagaStatusResponse, error) {
	var model SagaReadModel
	err := s.collection.FindOne(ctx, bson.M{"_id": sagaID}).Decode(&model)
	if err != nil {
		return nil, fmt.Errorf("failed to get saga status: %w", err)
	}

	response := &SagaStatusResponse{
		SagaID:        model.SagaID,
		DefinitionName: model.DefinitionName,
		Status:        model.Status,
		CurrentStep:   model.CurrentStep,
		TotalSteps:    model.TotalSteps,
		CompletedSteps: model.CompletedSteps,
		FailedSteps:   model.FailedSteps,
		StartedAt:     model.StartedAt,
		CompletedAt:   model.CompletedAt,
		Duration:      model.Duration,
		CorrelationID: model.CorrelationID,
		Context:       model.Context,
		LastError:     model.LastError,
		RetryCount:    model.RetryCount,
	}

	return response, nil
}

func (s *MongoSagaReadModelStore) UpsertSagaReadModel(ctx context.Context, model *SagaReadModel) error {
	var durationMs *int64
	if model.Duration != nil {
		ms := int64(model.Duration.Milliseconds())
		durationMs = &ms
	}

	doc := bson.M{
		"_id":            model.SagaID,
		"definition_name": model.DefinitionName,
		"status":         string(model.Status),
		"current_step":   model.CurrentStep,
		"total_steps":    model.TotalSteps,
		"completed_steps": model.CompletedSteps,
		"failed_steps":   model.FailedSteps,
		"started_at":     model.StartedAt,
		"completed_at":   model.CompletedAt,
		"duration_ms":    durationMs,
		"correlation_id": model.CorrelationID,
		"context":        model.Context,
		"last_error":     model.LastError,
		"retry_count":    model.RetryCount,
		"updated_at":     model.UpdatedAt,
	}

	opts := options.Update().SetUpsert(true)
	_, err := s.collection.UpdateOne(ctx, bson.M{"_id": model.SagaID}, bson.M{"$set": doc}, opts)
	return err
}

func (s *MongoSagaReadModelStore) UpsertSagaStepReadModel(ctx context.Context, step *SagaStepReadModel) error {
	stepCollection := s.collection.Database().Collection("saga_step_read_models")
	
	var durationMs *int64
	if step.Duration != nil {
		ms := int64(step.Duration.Milliseconds())
		durationMs = &ms
	}

	doc := bson.M{
		"saga_id":       step.SagaID,
		"step_name":     step.StepName,
		"status":        step.Status,
		"started_at":    step.StartedAt,
		"completed_at":  step.CompletedAt,
		"duration_ms":   durationMs,
		"retry_attempt": step.RetryAttempt,
		"error":         step.Error,
		"updated_at":    step.UpdatedAt,
	}

	// Используем составной ключ для уникальности
	filter := bson.M{
		"saga_id":    step.SagaID,
		"step_name":  step.StepName,
		"started_at": step.StartedAt,
	}

	opts := options.Update().SetUpsert(true)
	_, err := stepCollection.UpdateOne(ctx, filter, bson.M{"$set": doc}, opts)
	return err
}

func (s *MongoSagaReadModelStore) ListSagas(ctx context.Context, filter SagaFilter) (*SagaListResponse, error) {
	mongoFilter := bson.M{}
	if filter.Status != nil {
		mongoFilter["status"] = string(*filter.Status)
	}
	if filter.DefinitionName != nil {
		mongoFilter["definition_name"] = *filter.DefinitionName
	}
	if filter.CorrelationID != nil {
		mongoFilter["correlation_id"] = *filter.CorrelationID
	}
	if filter.StartedAfter != nil {
		mongoFilter["started_at"] = bson.M{"$gte": *filter.StartedAfter}
	}
	if filter.StartedBefore != nil {
		if startedAt, ok := mongoFilter["started_at"].(bson.M); ok {
			startedAt["$lte"] = *filter.StartedBefore
		} else {
			mongoFilter["started_at"] = bson.M{"$lte": *filter.StartedBefore}
		}
	}

	opts := options.Find().SetSort(bson.D{{Key: "started_at", Value: -1}})
	if filter.Limit > 0 {
		opts.SetLimit(int64(filter.Limit))
	}
	if filter.Offset > 0 {
		opts.SetSkip(int64(filter.Offset))
	}

	cursor, err := s.collection.Find(ctx, mongoFilter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list sagas: %w", err)
	}
	defer cursor.Close(ctx)

	var summaries []SagaSummary
	if err := cursor.All(ctx, &summaries); err != nil {
		return nil, fmt.Errorf("failed to decode sagas: %w", err)
	}

	total, _ := s.collection.CountDocuments(ctx, mongoFilter)

	return &SagaListResponse{
		Sagas:  summaries,
		Total:  int(total),
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

func (s *MongoSagaReadModelStore) GetMetrics(ctx context.Context, filter MetricsFilter) (*SagaMetricsResponse, error) {
	mongoFilter := bson.M{}
	if filter.DefinitionName != nil {
		mongoFilter["definition_name"] = *filter.DefinitionName
	}
	if filter.StartedAfter != nil {
		mongoFilter["started_at"] = bson.M{"$gte": *filter.StartedAfter}
	}
	if filter.StartedBefore != nil {
		if startedAt, ok := mongoFilter["started_at"].(bson.M); ok {
			startedAt["$lte"] = *filter.StartedBefore
		} else {
			mongoFilter["started_at"] = bson.M{"$lte": *filter.StartedBefore}
		}
	}

	pipeline := []bson.M{
		{"$match": mongoFilter},
		{"$group": bson.M{
			"_id": nil,
			"total": bson.M{"$sum": 1},
			"completed": bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$status", "completed"}}, 1, 0}}},
			"failed": bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$status", "failed"}}, 1, 0}}},
			"compensated": bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$status", "compensated"}}, 1, 0}}},
			"avg_duration_ms": bson.M{"$avg": "$duration_ms"},
		}},
	}

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}
	defer cursor.Close(ctx)

	var result struct {
		Total       int     `bson:"total"`
		Completed   int     `bson:"completed"`
		Failed      int     `bson:"failed"`
		Compensated int     `bson:"compensated"`
		AvgDurationMs *float64 `bson:"avg_duration_ms"`
	}

	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode metrics: %w", err)
		}
	}

	var successRate float64
	if result.Total > 0 {
		successRate = float64(result.Completed) / float64(result.Total) * 100
	}

	var avgDuration time.Duration
	if result.AvgDurationMs != nil {
		avgDuration = time.Duration(*result.AvgDurationMs) * time.Millisecond
	}

	return &SagaMetricsResponse{
		TotalSagas:       result.Total,
		CompletedSagas:   result.Completed,
		FailedSagas:      result.Failed,
		CompensatedSagas: result.Compensated,
		SuccessRate:      successRate,
		AvgDuration:      avgDuration,
		Throughput:       0,
	}, nil
}

