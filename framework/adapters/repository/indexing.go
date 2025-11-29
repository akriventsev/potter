// Package repository предоставляет generic адаптеры для работы с различными storage backends.
package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IndexType тип индекса
type IndexType string

const (
	// PostgreSQL index types
	IndexTypeBTree IndexType = "btree"
	IndexTypeHash IndexType = "hash"
	IndexTypeGIN  IndexType = "gin"
	IndexTypeGiST IndexType = "gist"

	// MongoDB index types
	IndexTypeSingle    IndexType = "single"
	IndexTypeCompound  IndexType = "compound"
	IndexTypeText      IndexType = "text"
	IndexTypeGeo2D     IndexType = "2d"
	IndexTypeGeo2DSphere IndexType = "2dsphere"
)

// IndexSpec спецификация индекса
type IndexSpec struct {
	Name         string
	Fields       []string
	Type         IndexType
	Unique       bool
	Sparse       bool
	PartialFilter map[string]interface{} // для MongoDB partial indexes
}

// IndexInfo информация об индексе
type IndexInfo struct {
	Name      string
	Fields    []string
	Type      IndexType
	Size      int64
	UsageStats *IndexUsageStats
}

// IndexUsageStats статистика использования индекса
type IndexUsageStats struct {
	Scans       int64
	TuplesRead  int64
	TuplesFetched int64
}

// IndexRecommendation рекомендация по созданию индекса
type IndexRecommendation struct {
	Fields               []string
	Reason               string
	EstimatedImprovement string
	Priority             int // 1-10, где 10 - высший приоритет
}

// IndexManager интерфейс для управления индексами
type IndexManager interface {
	CreateIndex(ctx context.Context, spec IndexSpec) error
	DropIndex(ctx context.Context, name string) error
	ListIndexes(ctx context.Context) ([]IndexInfo, error)
	AnalyzeQueries(ctx context.Context) ([]IndexRecommendation, error)
}

// PostgresIndexManager реализация IndexManager для PostgreSQL
type PostgresIndexManager[T Entity] struct {
	db     *pgx.Conn
	config PostgresConfig
}

// NewPostgresIndexManager создает новый PostgresIndexManager
func NewPostgresIndexManager[T Entity](db *pgx.Conn, config PostgresConfig) *PostgresIndexManager[T] {
	return &PostgresIndexManager[T]{
		db:     db,
		config: config,
	}
}

// CreateIndex создает индекс
func (m *PostgresIndexManager[T]) CreateIndex(ctx context.Context, spec IndexSpec) error {
	tableName := fmt.Sprintf("%s.%s", m.config.SchemaName, m.config.TableName)
	
	var indexType string
	if spec.Type == "" {
		indexType = string(IndexTypeBTree)
	} else {
		indexType = string(spec.Type)
	}

	var uniqueClause string
	if spec.Unique {
		uniqueClause = "UNIQUE "
	}

	fields := strings.Join(spec.Fields, ", ")
	indexName := spec.Name
	if indexName == "" {
		indexName = fmt.Sprintf("idx_%s_%s", m.config.TableName, strings.Join(spec.Fields, "_"))
	}

	query := fmt.Sprintf(
		"CREATE %sINDEX IF NOT EXISTS %s ON %s USING %s (%s)",
		uniqueClause,
		indexName,
		tableName,
		indexType,
		fields,
	)

	_, err := m.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// DropIndex удаляет индекс
func (m *PostgresIndexManager[T]) DropIndex(ctx context.Context, name string) error {
	query := fmt.Sprintf("DROP INDEX IF EXISTS %s", name)
	_, err := m.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop index: %w", err)
	}
	return nil
}

// ListIndexes возвращает список всех индексов
func (m *PostgresIndexManager[T]) ListIndexes(ctx context.Context) ([]IndexInfo, error) {
	query := `
		SELECT 
			i.indexname,
			array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) as fields,
			am.amname as type
		FROM pg_indexes i
		JOIN pg_index ix ON i.indexname = (SELECT relname FROM pg_class WHERE oid = ix.indexrelid)
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_am am ON am.oid = (SELECT relam FROM pg_class WHERE oid = ix.indexrelid)
		LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE i.schemaname = $1 AND i.tablename = $2
		GROUP BY i.indexname, am.amname
	`

	rows, err := m.db.Query(ctx, query, m.config.SchemaName, m.config.TableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var info IndexInfo
		var fields []string
		var indexType string

		if err := rows.Scan(&info.Name, &fields, &indexType); err != nil {
			continue
		}

		info.Fields = fields
		info.Type = IndexType(indexType)
		indexes = append(indexes, info)
	}

	return indexes, nil
}

// AnalyzeQueries анализирует query patterns и возвращает рекомендации
func (m *PostgresIndexManager[T]) AnalyzeQueries(ctx context.Context) ([]IndexRecommendation, error) {
	var recommendations []IndexRecommendation
	tableName := fmt.Sprintf("%s.%s", m.config.SchemaName, m.config.TableName)

	// Проверяем доступность pg_stat_statements
	var pgStatStatementsExists bool
	checkQuery := `
		SELECT EXISTS (
			SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'
		)
	`
	if err := m.db.QueryRow(ctx, checkQuery).Scan(&pgStatStatementsExists); err != nil {
		// Если не можем проверить, продолжаем без pg_stat_statements
		pgStatStatementsExists = false
	}

	// Анализируем через pg_stat_statements если доступен
	if pgStatStatementsExists {
		query := `
			SELECT 
				regexp_split_to_table(query, E'\\s+') as field,
				SUM(calls) as total_calls,
				SUM(total_exec_time) as total_time
			FROM pg_stat_statements
			WHERE query LIKE '%' || $1 || '%'
				AND query LIKE '%WHERE%'
			GROUP BY field
			HAVING SUM(calls) > 10
			ORDER BY SUM(calls) DESC
			LIMIT 20
		`
		rows, err := m.db.Query(ctx, query, tableName)
		if err == nil {
			defer rows.Close()
			fieldUsage := make(map[string]int64)
			for rows.Next() {
				var field string
				var calls int64
				var totalTime float64
				if err := rows.Scan(&field, &calls, &totalTime); err != nil {
					continue
				}
				// Извлекаем имена полей из WHERE условий
				if strings.Contains(field, "=") || strings.Contains(field, ">") || strings.Contains(field, "<") {
					// Упрощенная логика: ищем имена полей
					parts := strings.Fields(field)
					for _, part := range parts {
						// Убираем операторы и кавычки
						cleanPart := strings.Trim(part, "=<>!()'\"")
						if cleanPart != "" && !strings.Contains(cleanPart, "$") {
							fieldUsage[cleanPart] += calls
						}
					}
				}
			}
			// Создаем рекомендации на основе использования
			for field, calls := range fieldUsage {
				// Проверяем, есть ли уже индекс на это поле
				indexes, err := m.ListIndexes(ctx)
				if err == nil {
					hasIndex := false
					for _, idx := range indexes {
						for _, idxField := range idx.Fields {
							if idxField == field {
								hasIndex = true
								break
							}
						}
						if hasIndex {
							break
						}
					}
					if !hasIndex && calls > 100 {
						recommendations = append(recommendations, IndexRecommendation{
							Fields:               []string{field},
							Reason:               fmt.Sprintf("Field used in %d queries without index", calls),
							EstimatedImprovement: "High - frequent WHERE conditions",
							Priority:             8,
						})
					}
				}
			}
		}
	}

	// Анализируем через pg_stat_user_indexes для неиспользуемых индексов
	indexUsageQuery := `
		SELECT 
			i.indexname,
			COALESCE(idx_scan, 0) as scans,
			COALESCE(idx_tup_read, 0) as tuples_read,
			COALESCE(idx_tup_fetch, 0) as tuples_fetched
		FROM pg_stat_user_indexes i
		WHERE schemaname = $1 AND tablename = $2
		ORDER BY idx_scan ASC
	`
	rows, err := m.db.Query(ctx, indexUsageQuery, m.config.SchemaName, m.config.TableName)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var indexName string
			var scans, tuplesRead, tuplesFetched int64
			if err := rows.Scan(&indexName, &scans, &tuplesRead, &tuplesFetched); err != nil {
				continue
			}
			// Рекомендуем удалить неиспользуемые индексы (но не primary key)
			if scans == 0 && !strings.HasPrefix(indexName, "pk_") && indexName != "PRIMARY" {
				recommendations = append(recommendations, IndexRecommendation{
					Fields:               []string{indexName},
					Reason:               fmt.Sprintf("Index %s has 0 scans and may be unused", indexName),
					EstimatedImprovement: "Medium - reduces write overhead",
					Priority:             5,
				})
			}
		}
	}

	// Анализируем частые WHERE поля через pg_stat_user_tables
	// (упрощенный анализ - ищем поля, которые часто используются в WHERE)
	tableStatsQuery := `
		SELECT 
			seq_scan,
			seq_tup_read,
			idx_scan,
			idx_tup_fetch
		FROM pg_stat_user_tables
		WHERE schemaname = $1 AND relname = $2
	`
	var seqScan, seqTupRead, idxScan, idxTupFetch int64
	if err := m.db.QueryRow(ctx, tableStatsQuery, m.config.SchemaName, m.config.TableName).Scan(
		&seqScan, &seqTupRead, &idxScan, &idxTupFetch,
	); err == nil {
		// Если много sequential scans, возможно нужны индексы
		if seqScan > idxScan*2 && seqScan > 100 {
			recommendations = append(recommendations, IndexRecommendation{
				Fields:               []string{"*"},
				Reason:               fmt.Sprintf("High sequential scan ratio (%d seq vs %d idx)", seqScan, idxScan),
				EstimatedImprovement: "High - consider indexing frequently queried fields",
				Priority:             7,
			})
		}
	}

	return recommendations, nil
}

// MongoIndexManager реализация IndexManager для MongoDB
type MongoIndexManager[T Entity] struct {
	collection *mongo.Collection
	config     MongoConfig
}

// NewMongoIndexManager создает новый MongoIndexManager
func NewMongoIndexManager[T Entity](collection *mongo.Collection, config MongoConfig) *MongoIndexManager[T] {
	return &MongoIndexManager[T]{
		collection: collection,
		config:     config,
	}
}

// CreateIndex создает индекс
func (m *MongoIndexManager[T]) CreateIndex(ctx context.Context, spec IndexSpec) error {
	var keys bson.D
	for _, field := range spec.Fields {
		keys = append(keys, bson.E{Key: field, Value: 1})
	}

	indexModel := mongo.IndexModel{
		Keys: keys,
		Options: options.Index().
			SetName(spec.Name).
			SetUnique(spec.Unique).
			SetSparse(spec.Sparse),
	}

	// Добавляем partial filter если указан
	if len(spec.PartialFilter) > 0 {
		indexModel.Options.SetPartialFilterExpression(spec.PartialFilter)
	}

	// Специальная обработка для text индекса
	if spec.Type == IndexTypeText {
		textKeys := bson.D{}
		for _, field := range spec.Fields {
			textKeys = append(textKeys, bson.E{Key: field, Value: "text"})
		}
		indexModel.Keys = textKeys
	}

	_, err := m.collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// DropIndex удаляет индекс
func (m *MongoIndexManager[T]) DropIndex(ctx context.Context, name string) error {
	_, err := m.collection.Indexes().DropOne(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to drop index: %w", err)
	}
	return nil
}

// ListIndexes возвращает список всех индексов
func (m *MongoIndexManager[T]) ListIndexes(ctx context.Context) ([]IndexInfo, error) {
	cursor, err := m.collection.Indexes().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}
	defer cursor.Close(ctx)

	var indexes []IndexInfo
	for cursor.Next(ctx) {
		var indexDoc bson.M
		if err := cursor.Decode(&indexDoc); err != nil {
			continue
		}

		info := IndexInfo{
			Name: indexDoc["name"].(string),
		}

		// Извлекаем поля из ключей
		if keys, ok := indexDoc["key"].(bson.M); ok {
			var fields []string
			for field := range keys {
				fields = append(fields, field)
			}
			info.Fields = fields
		}

		indexes = append(indexes, info)
	}

	return indexes, nil
}

// AnalyzeQueries анализирует query patterns и возвращает рекомендации
func (m *MongoIndexManager[T]) AnalyzeQueries(ctx context.Context) ([]IndexRecommendation, error) {
	var recommendations []IndexRecommendation

	// Получаем статистику индексов
	indexes, err := m.ListIndexes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	// Анализируем использование индексов через $indexStats (MongoDB 3.2+)
	indexStatsPipeline := []bson.M{
		{"$indexStats": bson.M{}},
	}
	cursor, err := m.collection.Aggregate(ctx, indexStatsPipeline)
	if err == nil {
		defer cursor.Close(ctx)
		indexUsage := make(map[string]int64)
		for cursor.Next(ctx) {
			var stat bson.M
			if err := cursor.Decode(&stat); err != nil {
				continue
			}
			if name, ok := stat["name"].(string); ok {
				if access, ok := stat["accesses"].(bson.M); ok {
					if ops, ok := access["ops"].(int64); ok {
						indexUsage[name] = ops
					}
				}
			}
		}
		// Рекомендуем удалить неиспользуемые индексы
		for _, idx := range indexes {
			if idx.Name == "_id_" {
				continue // Пропускаем primary key
			}
			usage := indexUsage[idx.Name]
			if usage == 0 {
				recommendations = append(recommendations, IndexRecommendation{
					Fields:               idx.Fields,
					Reason:               fmt.Sprintf("Index %s has 0 operations and may be unused", idx.Name),
					EstimatedImprovement: "Medium - reduces write overhead",
					Priority:             5,
				})
			}
		}
	}

	// Анализируем через explain на примере запросов
	// Создаем тестовый запрос для анализа
	testFilter := bson.M{}
	explainResult := m.collection.FindOne(ctx, testFilter, options.FindOne().SetHint(bson.M{}))
	if explainResult.Err() == nil || explainResult.Err() == mongo.ErrNoDocuments {
		// Выполняем explain для анализа плана выполнения
		explainCmd := bson.D{
			{Key: "find", Value: m.collection.Name()},
			{Key: "filter", Value: testFilter},
		}
		var explainResult bson.M
		if err := m.collection.Database().RunCommand(ctx, bson.D{{Key: "explain", Value: explainCmd}}).Decode(&explainResult); err == nil {
			if execStats, ok := explainResult["executionStats"].(bson.M); ok {
				if stage, ok := execStats["executionStages"].(bson.M); ok {
					if stageType, ok := stage["stage"].(string); ok {
						if stageType == "COLLSCAN" {
							// Full collection scan - нужны индексы
							recommendations = append(recommendations, IndexRecommendation{
								Fields:               []string{"*"},
								Reason:               "Collection scan detected - consider adding indexes for frequently queried fields",
								EstimatedImprovement: "High - indexes can significantly improve query performance",
								Priority:             9,
							})
						}
					}
				}
			}
		}
	}

	// Анализируем через profiler если включен
	// (упрощенная версия - в реальности нужен доступ к system.profile)
	profileCollection := m.collection.Database().Collection("system.profile")
	if profileCollection != nil {
		profileQuery := bson.M{
			"ns": m.collection.Database().Name() + "." + m.collection.Name(),
			"op": bson.M{"$in": []string{"query", "find"}},
		}
		cursor, err := profileCollection.Find(ctx, profileQuery, options.Find().SetLimit(100))
		if err == nil {
			defer cursor.Close(ctx)
			fieldUsage := make(map[string]int64)
			for cursor.Next(ctx) {
				var profileDoc bson.M
				if err := cursor.Decode(&profileDoc); err != nil {
					continue
				}
				if command, ok := profileDoc["command"].(bson.M); ok {
					if filter, ok := command["filter"].(bson.M); ok {
						for field := range filter {
							fieldUsage[field]++
						}
					}
				}
			}
			// Создаем рекомендации для часто используемых полей без индексов
			for field, count := range fieldUsage {
				hasIndex := false
				for _, idx := range indexes {
					for _, idxField := range idx.Fields {
						if idxField == field {
							hasIndex = true
							break
						}
					}
					if hasIndex {
						break
					}
				}
				if !hasIndex && count > 10 {
					recommendations = append(recommendations, IndexRecommendation{
						Fields:               []string{field},
						Reason:               fmt.Sprintf("Field used in %d profiled queries without index", count),
						EstimatedImprovement: "High - frequent query field",
						Priority:             8,
					})
				}
			}
		}
	}

	return recommendations, nil
}

// AutoIndexManager автоматический менеджер индексов
type AutoIndexManager struct {
	indexManager IndexManager
	policy       IndexPolicy
	queryPatterns map[string]int64 // поле -> количество использований
}

// IndexPolicy политика автоматического управления индексами
type IndexPolicy struct {
	AutoCreate        bool
	AutoDrop          bool
	MinUsageThreshold int64
	MaxIndexes        int
}

// DefaultIndexPolicy возвращает политику по умолчанию
func DefaultIndexPolicy() IndexPolicy {
	return IndexPolicy{
		AutoCreate:        false, // по умолчанию отключено для безопасности
		AutoDrop:          false,
		MinUsageThreshold: 100,
		MaxIndexes:        10,
	}
}

// NewAutoIndexManager создает новый AutoIndexManager
func NewAutoIndexManager(indexManager IndexManager, policy IndexPolicy) *AutoIndexManager {
	return &AutoIndexManager{
		indexManager:  indexManager,
		policy:        policy,
		queryPatterns: make(map[string]int64),
	}
}

// RecordQueryPattern записывает паттерн запроса для анализа
func (a *AutoIndexManager) RecordQueryPattern(field string) {
	a.queryPatterns[field]++
}

// AnalyzeAndOptimize анализирует паттерны и оптимизирует индексы
func (a *AutoIndexManager) AnalyzeAndOptimize(ctx context.Context) error {
	if !a.policy.AutoCreate && !a.policy.AutoDrop {
		return nil
	}

	recommendations, err := a.indexManager.AnalyzeQueries(ctx)
	if err != nil {
		return fmt.Errorf("failed to analyze queries: %w", err)
	}

	// Создаем индексы на основе рекомендаций
	if a.policy.AutoCreate {
		for _, rec := range recommendations {
			if rec.Priority >= 7 { // высокий приоритет
				spec := IndexSpec{
					Name:   fmt.Sprintf("auto_idx_%s", strings.Join(rec.Fields, "_")),
					Fields: rec.Fields,
					Type:   IndexTypeBTree,
				}
				if err := a.indexManager.CreateIndex(ctx, spec); err != nil {
					// Логируем ошибку, но продолжаем
					continue
				}
			}
		}
	}

	// Удаляем неиспользуемые индексы
	if a.policy.AutoDrop {
		indexes, err := a.indexManager.ListIndexes(ctx)
		if err != nil {
			return fmt.Errorf("failed to list indexes: %w", err)
		}

		for _, idx := range indexes {
			// Проверяем использование индекса
			usage := int64(0)
			for _, field := range idx.Fields {
				if count, ok := a.queryPatterns[field]; ok {
					usage += count
				}
			}

			if usage < a.policy.MinUsageThreshold {
				// Удаляем неиспользуемый индекс (кроме primary key)
				if idx.Name != "_id_" && !strings.HasPrefix(idx.Name, "pk_") {
					_ = a.indexManager.DropIndex(ctx, idx.Name)
				}
			}
		}
	}

	return nil
}

