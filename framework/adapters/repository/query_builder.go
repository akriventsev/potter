// Package repository предоставляет generic адаптеры для работы с различными storage backends.
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// QueryOperator оператор для фильтрации
type QueryOperator string

const (
	Eq      QueryOperator = "="
	NotEq   QueryOperator = "!="
	Gt      QueryOperator = ">"
	Gte     QueryOperator = ">="
	Lt      QueryOperator = "<"
	Lte     QueryOperator = "<="
	In      QueryOperator = "IN"
	NotIn   QueryOperator = "NOT IN"
	Like    QueryOperator = "LIKE"
	Between QueryOperator = "BETWEEN"
	IsNull  QueryOperator = "IS NULL"
	IsNotNull QueryOperator = "IS NOT NULL"
)

// SortOrder порядок сортировки
type SortOrder string

const (
	Asc  SortOrder = "ASC"
	Desc SortOrder = "DESC"
)

// QueryBuilder интерфейс для построения запросов
type QueryBuilder[T Entity] interface {
	Where(field string, op QueryOperator, value interface{}) QueryBuilder[T]
	And() QueryBuilder[T]
	Or() QueryBuilder[T]
	Not() QueryBuilder[T]
	OrderBy(field string, order SortOrder) QueryBuilder[T]
	OrderByDesc(field string) QueryBuilder[T]
	Limit(limit int) QueryBuilder[T]
	Offset(offset int) QueryBuilder[T]
	Page(page, pageSize int) QueryBuilder[T]
	Execute(ctx context.Context) ([]T, error)
	Count(ctx context.Context) (int64, error)
	First(ctx context.Context) (T, error)
	Exists(ctx context.Context) (bool, error)
}

// QueryCondition условие запроса
type QueryCondition struct {
	Field    string
	Operator QueryOperator
	Value    interface{}
	Logical  string // AND, OR, NOT
}

// convertToInterfaceSlice безопасно конвертирует значение в []interface{}
// Поддерживает:
// - []interface{} - используется напрямую
// - reflect.Slice ([]string, []int, []time.Time и т.д.) - копирует элементы
// - другие типы - возвращает ошибку
func convertToInterfaceSlice(value interface{}) ([]interface{}, error) {
	if value == nil {
		return nil, fmt.Errorf("value cannot be nil")
	}

	// Если уже []interface{}, используем напрямую
	if slice, ok := value.([]interface{}); ok {
		return slice, nil
	}

	// Проверяем, является ли значение срезом через reflection
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("value must be a slice, got %T", value)
	}

	// Копируем элементы в новый []interface{}
	result := make([]interface{}, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		result[i] = rv.Index(i).Interface()
	}

	return result, nil
}

// PostgresQueryBuilder реализация QueryBuilder для PostgreSQL
type PostgresQueryBuilder[T Entity] struct {
	db              *pgx.Conn
	mapper          Mapper[T]
	config          PostgresConfig
	conditions      []QueryCondition
	orderBy         []string
	limitValue      *int
	offsetValue     *int
	joins           []string
	groupBy         []string
	having          []string
	args            []interface{}
	argIndex        int
	nextLogical     string // логический оператор для следующего условия (по умолчанию "AND")
	autoIndexManager *AutoIndexManager
}

// NewPostgresQueryBuilder создает новый PostgresQueryBuilder
func NewPostgresQueryBuilder[T Entity](db *pgx.Conn, mapper Mapper[T], config PostgresConfig) *PostgresQueryBuilder[T] {
	return &PostgresQueryBuilder[T]{
		db:              db,
		mapper:          mapper,
		config:          config,
		conditions:      make([]QueryCondition, 0),
		orderBy:         make([]string, 0),
		joins:           make([]string, 0),
		groupBy:         make([]string, 0),
		having:          make([]string, 0),
		args:            make([]interface{}, 0),
		argIndex:        1,
		nextLogical:     "AND", // по умолчанию AND
		autoIndexManager: nil,
	}
}

// SetAutoIndexManager устанавливает AutoIndexManager для записи паттернов запросов
func (q *PostgresQueryBuilder[T]) SetAutoIndexManager(manager *AutoIndexManager) {
	q.autoIndexManager = manager
}

// Where добавляет условие фильтрации
func (q *PostgresQueryBuilder[T]) Where(field string, op QueryOperator, value interface{}) QueryBuilder[T] {
	q.conditions = append(q.conditions, QueryCondition{
		Field:    field,
		Operator: op,
		Value:    value,
		Logical:  q.nextLogical, // используем текущий nextLogical
	})
	// Сбрасываем nextLogical в дефолт после использования
	q.nextLogical = "AND"
	
	// Записываем паттерн запроса для AutoIndexManager
	if q.autoIndexManager != nil {
		q.autoIndexManager.RecordQueryPattern(field)
	}
	
	return q
}

// And добавляет логический оператор AND для следующего условия
func (q *PostgresQueryBuilder[T]) And() QueryBuilder[T] {
	q.nextLogical = "AND"
	return q
}

// Or добавляет логический оператор OR для следующего условия
func (q *PostgresQueryBuilder[T]) Or() QueryBuilder[T] {
	q.nextLogical = "OR"
	return q
}

// Not добавляет логический оператор NOT для следующего условия
// NOT будет обернут вокруг следующего условия: NOT (condition)
func (q *PostgresQueryBuilder[T]) Not() QueryBuilder[T] {
	q.nextLogical = "NOT"
	return q
}

// OrderBy добавляет сортировку
func (q *PostgresQueryBuilder[T]) OrderBy(field string, order SortOrder) QueryBuilder[T] {
	q.orderBy = append(q.orderBy, fmt.Sprintf("%s %s", field, order))
	
	// Записываем паттерн запроса для AutoIndexManager
	if q.autoIndexManager != nil {
		q.autoIndexManager.RecordQueryPattern(field)
	}
	
	return q
}

// OrderByDesc добавляет сортировку по убыванию
func (q *PostgresQueryBuilder[T]) OrderByDesc(field string) QueryBuilder[T] {
	return q.OrderBy(field, Desc)
}

// Limit устанавливает лимит результатов
func (q *PostgresQueryBuilder[T]) Limit(limit int) QueryBuilder[T] {
	q.limitValue = &limit
	return q
}

// Offset устанавливает смещение
func (q *PostgresQueryBuilder[T]) Offset(offset int) QueryBuilder[T] {
	q.offsetValue = &offset
	return q
}

// Page устанавливает пагинацию
func (q *PostgresQueryBuilder[T]) Page(page, pageSize int) QueryBuilder[T] {
	offset := (page - 1) * pageSize
	q.Limit(pageSize)
	q.Offset(offset)
	return q
}

// InnerJoin добавляет INNER JOIN
func (q *PostgresQueryBuilder[T]) InnerJoin(table, on string) *PostgresQueryBuilder[T] {
	q.joins = append(q.joins, fmt.Sprintf("INNER JOIN %s ON %s", table, on))
	return q
}

// LeftJoin добавляет LEFT JOIN
func (q *PostgresQueryBuilder[T]) LeftJoin(table, on string) *PostgresQueryBuilder[T] {
	q.joins = append(q.joins, fmt.Sprintf("LEFT JOIN %s ON %s", table, on))
	return q
}

// RightJoin добавляет RIGHT JOIN
func (q *PostgresQueryBuilder[T]) RightJoin(table, on string) *PostgresQueryBuilder[T] {
	q.joins = append(q.joins, fmt.Sprintf("RIGHT JOIN %s ON %s", table, on))
	return q
}

// GroupBy добавляет группировку
func (q *PostgresQueryBuilder[T]) GroupBy(field string) *PostgresQueryBuilder[T] {
	q.groupBy = append(q.groupBy, field)
	return q
}

// Having добавляет условие HAVING
// Плейсхолдеры будут пересчитаны в buildQuery() после WHERE
func (q *PostgresQueryBuilder[T]) Having(field string, op QueryOperator, value interface{}) *PostgresQueryBuilder[T] {
	// Сохраняем условие без плейсхолдера, он будет добавлен позже
	q.having = append(q.having, fmt.Sprintf("%s %s $PLACEHOLDER", field, op))
	q.args = append(q.args, value)
	return q
}

// buildWhereClause строит WHERE clause
// Использует и обновляет q.argIndex для унификации с HAVING
func (q *PostgresQueryBuilder[T]) buildWhereClause() (string, []interface{}, error) {
	if len(q.conditions) == 0 {
		return "", nil, nil
	}

	var parts []string
	args := make([]interface{}, 0)
	// Используем q.argIndex вместо локального счетчика
	argIndex := q.argIndex

	for i, cond := range q.conditions {
		var part string
		logical := cond.Logical

		// Формируем условие
		var conditionPart string
		switch cond.Operator {
		case IsNull, IsNotNull:
			conditionPart = fmt.Sprintf("%s %s", cond.Field, cond.Operator)
		case Between:
			values, err := convertToInterfaceSlice(cond.Value)
			if err != nil {
				return "", nil, fmt.Errorf("BETWEEN requires a slice with 2 elements, got %T: %w", cond.Value, err)
			}
			if len(values) != 2 {
				return "", nil, fmt.Errorf("BETWEEN requires exactly 2 values, got %d", len(values))
			}
			conditionPart = fmt.Sprintf("%s BETWEEN $%d AND $%d", cond.Field, argIndex, argIndex+1)
			args = append(args, values[0], values[1])
			argIndex += 2
		case In, NotIn:
			values, err := convertToInterfaceSlice(cond.Value)
			if err != nil {
				return "", nil, fmt.Errorf("IN/NOT IN requires a slice, got %T: %w", cond.Value, err)
			}
			if len(values) == 0 {
				return "", nil, fmt.Errorf("IN/NOT IN requires at least one value")
			}
			placeholders := make([]string, len(values))
			for j := range values {
				placeholders[j] = fmt.Sprintf("$%d", argIndex)
				args = append(args, values[j])
				argIndex++
			}
			conditionPart = fmt.Sprintf("%s %s (%s)", cond.Field, cond.Operator, strings.Join(placeholders, ", "))
		case Like:
			conditionPart = fmt.Sprintf("%s LIKE $%d", cond.Field, argIndex)
			args = append(args, cond.Value)
			argIndex++
		default:
			conditionPart = fmt.Sprintf("%s %s $%d", cond.Field, cond.Operator, argIndex)
			args = append(args, cond.Value)
			argIndex++
		}

		// Применяем NOT если нужно
		if logical == "NOT" {
			conditionPart = fmt.Sprintf("NOT (%s)", conditionPart)
			logical = "" // NOT уже применен, оператор AND/OR не нужен
		}

		// Добавляем логический оператор перед условием (кроме первого)
		if logical != "" && i > 0 {
			part = fmt.Sprintf("%s %s", logical, conditionPart)
		} else {
			part = conditionPart
		}

		parts = append(parts, part)
	}

	// Обновляем q.argIndex для использования в HAVING
	q.argIndex = argIndex

	return "WHERE " + strings.Join(parts, " "), args, nil
}

// BuildQuery строит SQL запрос (экспортирован для тестирования)
func (q *PostgresQueryBuilder[T]) BuildQuery() (string, []interface{}, error) {
	return q.buildQuery()
}

// buildQuery строит SQL запрос
func (q *PostgresQueryBuilder[T]) buildQuery() (string, []interface{}, error) {
	tableName := fmt.Sprintf("%s.%s", q.config.SchemaName, q.config.TableName)
	
	var parts []string
	args := make([]interface{}, 0)

	// SELECT
	parts = append(parts, "SELECT data FROM", tableName)

	// JOINs
	if len(q.joins) > 0 {
		parts = append(parts, strings.Join(q.joins, " "))
	}

	// WHERE
	whereClause, whereArgs, err := q.buildWhereClause()
	if err != nil {
		return "", nil, err
	}
	if whereClause != "" {
		parts = append(parts, whereClause)
		args = append(args, whereArgs...)
	}

	// GROUP BY
	if len(q.groupBy) > 0 {
		parts = append(parts, "GROUP BY", strings.Join(q.groupBy, ", "))
	}

	// HAVING
	if len(q.having) > 0 {
		// Пересчитываем плейсхолдеры для HAVING с учетом WHERE
		havingParts := make([]string, len(q.having))
		for i, having := range q.having {
			// Заменяем $PLACEHOLDER на правильный номер
			havingParts[i] = strings.Replace(having, "$PLACEHOLDER", fmt.Sprintf("$%d", q.argIndex), 1)
			q.argIndex++
		}
		parts = append(parts, "HAVING", strings.Join(havingParts, " AND "))
		args = append(args, q.args...)
	}

	// ORDER BY
	if len(q.orderBy) > 0 {
		parts = append(parts, "ORDER BY", strings.Join(q.orderBy, ", "))
	}

	// LIMIT
	if q.limitValue != nil {
		parts = append(parts, fmt.Sprintf("LIMIT %d", *q.limitValue))
	}

	// OFFSET
	if q.offsetValue != nil {
		parts = append(parts, fmt.Sprintf("OFFSET %d", *q.offsetValue))
	}

	return strings.Join(parts, " "), args, nil
}

// Execute выполняет запрос и возвращает результаты
func (q *PostgresQueryBuilder[T]) Execute(ctx context.Context) ([]T, error) {
	query, args, err := q.buildQuery()
	if err != nil {
		return nil, err
	}
	
	rows, err := q.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		var dataJSON []byte
		if err := rows.Scan(&dataJSON); err != nil {
			continue
		}

		var row map[string]interface{}
		if err := json.Unmarshal(dataJSON, &row); err != nil {
			continue
		}

		entity, err := q.mapper.FromRow(row)
		if err != nil {
			continue
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

// Count возвращает количество записей
func (q *PostgresQueryBuilder[T]) Count(ctx context.Context) (int64, error) {
	tableName := fmt.Sprintf("%s.%s", q.config.SchemaName, q.config.TableName)
	
	var parts []string
	args := make([]interface{}, 0)

	parts = append(parts, "SELECT COUNT(*) FROM", tableName)

	// JOINs
	if len(q.joins) > 0 {
		parts = append(parts, strings.Join(q.joins, " "))
	}

	// WHERE
	whereClause, whereArgs, err := q.buildWhereClause()
	if err != nil {
		return 0, err
	}
	if whereClause != "" {
		parts = append(parts, whereClause)
		args = append(args, whereArgs...)
	}

	query := strings.Join(parts, " ")
	
	var count int64
	err = q.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count: %w", err)
	}

	return count, nil
}

// First возвращает первую запись
func (q *PostgresQueryBuilder[T]) First(ctx context.Context) (T, error) {
	var zero T
	q.Limit(1)
	results, err := q.Execute(ctx)
	if err != nil {
		return zero, err
	}
	if len(results) == 0 {
		return zero, fmt.Errorf("no results found")
	}
	return results[0], nil
}

// Exists проверяет существование записей
func (q *PostgresQueryBuilder[T]) Exists(ctx context.Context) (bool, error) {
	count, err := q.Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MongoQueryBuilder реализация QueryBuilder для MongoDB
type MongoQueryBuilder[T Entity] struct {
	collection  *mongo.Collection
	config      MongoConfig
	filter      bson.M
	sort        bson.D
	limitValue  *int64
	skipValue   *int64
	pipeline    []bson.D
}

// NewMongoQueryBuilder создает новый MongoQueryBuilder
func NewMongoQueryBuilder[T Entity](collection *mongo.Collection, config MongoConfig) *MongoQueryBuilder[T] {
	return &MongoQueryBuilder[T]{
		collection: collection,
		config:     config,
		filter:     bson.M{},
		sort:       bson.D{},
		pipeline:   make([]bson.D, 0),
	}
}

// Where добавляет условие фильтрации
func (q *MongoQueryBuilder[T]) Where(field string, op QueryOperator, value interface{}) QueryBuilder[T] {
	switch op {
	case Eq:
		q.filter[field] = value
	case NotEq:
		q.filter[field] = bson.M{"$ne": value}
	case Gt:
		q.filter[field] = bson.M{"$gt": value}
	case Gte:
		q.filter[field] = bson.M{"$gte": value}
	case Lt:
		q.filter[field] = bson.M{"$lt": value}
	case Lte:
		q.filter[field] = bson.M{"$lte": value}
	case In:
		q.filter[field] = bson.M{"$in": value}
	case NotIn:
		q.filter[field] = bson.M{"$nin": value}
	case Like:
		q.filter[field] = bson.M{"$regex": value, "$options": "i"}
	}
	return q
}

// And добавляет логический оператор AND (не используется в MongoDB, все условия по умолчанию AND)
func (q *MongoQueryBuilder[T]) And() QueryBuilder[T] {
	return q
}

// Or добавляет логический оператор OR
func (q *MongoQueryBuilder[T]) Or() QueryBuilder[T] {
	// Для OR нужно использовать $or оператор, что требует рефакторинга структуры
	// Пока оставляем базовую реализацию
	return q
}

// Not добавляет логический оператор NOT
func (q *MongoQueryBuilder[T]) Not() QueryBuilder[T] {
	// Для NOT нужно использовать $not оператор
	return q
}

// OrderBy добавляет сортировку
func (q *MongoQueryBuilder[T]) OrderBy(field string, order SortOrder) QueryBuilder[T] {
	direction := 1
	if order == Desc {
		direction = -1
	}
	q.sort = append(q.sort, bson.E{Key: field, Value: direction})
	return q
}

// OrderByDesc добавляет сортировку по убыванию
func (q *MongoQueryBuilder[T]) OrderByDesc(field string) QueryBuilder[T] {
	return q.OrderBy(field, Desc)
}

// Limit устанавливает лимит результатов
func (q *MongoQueryBuilder[T]) Limit(limit int) QueryBuilder[T] {
	limit64 := int64(limit)
	q.limitValue = &limit64
	return q
}

// Offset устанавливает смещение
func (q *MongoQueryBuilder[T]) Offset(offset int) QueryBuilder[T] {
	offset64 := int64(offset)
	q.skipValue = &offset64
	return q
}

// Page устанавливает пагинацию
func (q *MongoQueryBuilder[T]) Page(page, pageSize int) QueryBuilder[T] {
	offset := (page - 1) * pageSize
	q.Limit(pageSize)
	q.Offset(offset)
	return q
}

// Match добавляет stage в aggregation pipeline
func (q *MongoQueryBuilder[T]) Match(filter bson.M) *MongoQueryBuilder[T] {
	q.pipeline = append(q.pipeline, bson.D{{Key: "$match", Value: filter}})
	return q
}

// Project добавляет projection stage
func (q *MongoQueryBuilder[T]) Project(projection bson.M) *MongoQueryBuilder[T] {
	q.pipeline = append(q.pipeline, bson.D{{Key: "$project", Value: projection}})
	return q
}

// Group добавляет group stage
func (q *MongoQueryBuilder[T]) Group(group bson.M) *MongoQueryBuilder[T] {
	q.pipeline = append(q.pipeline, bson.D{{Key: "$group", Value: group}})
	return q
}

// TextSearch добавляет полнотекстовый поиск
func (q *MongoQueryBuilder[T]) TextSearch(search string) *MongoQueryBuilder[T] {
	q.filter["$text"] = bson.M{"$search": search}
	return q
}

// Execute выполняет запрос и возвращает результаты
func (q *MongoQueryBuilder[T]) Execute(ctx context.Context) ([]T, error) {
	opts := options.Find()
	
	if len(q.sort) > 0 {
		opts.SetSort(q.sort)
	}
	if q.limitValue != nil {
		opts.SetLimit(*q.limitValue)
	}
	if q.skipValue != nil {
		opts.SetSkip(*q.skipValue)
	}

	// Если есть pipeline, используем aggregation
	if len(q.pipeline) > 0 {
		cursor, err := q.collection.Aggregate(ctx, q.pipeline, options.Aggregate())
		if err != nil {
			return nil, fmt.Errorf("failed to execute aggregation: %w", err)
		}
		defer cursor.Close(ctx)

		var entities []T
		if err := cursor.All(ctx, &entities); err != nil {
			return nil, fmt.Errorf("failed to decode results: %w", err)
		}
		return entities, nil
	}

	// Иначе используем обычный find
	cursor, err := q.collection.Find(ctx, q.filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer cursor.Close(ctx)

	var entities []T
	if err := cursor.All(ctx, &entities); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	return entities, nil
}

// Count возвращает количество записей
func (q *MongoQueryBuilder[T]) Count(ctx context.Context) (int64, error) {
	if len(q.pipeline) > 0 {
		// Для aggregation pipeline используем $count stage
		countPipeline := append(q.pipeline, bson.D{{Key: "$count", Value: "count"}})
		cursor, err := q.collection.Aggregate(ctx, countPipeline, options.Aggregate())
		if err != nil {
			return 0, fmt.Errorf("failed to count: %w", err)
		}
		defer cursor.Close(ctx)

		var result struct {
			Count int64 `bson:"count"`
		}
		if cursor.Next(ctx) {
			if err := cursor.Decode(&result); err != nil {
				return 0, fmt.Errorf("failed to decode count: %w", err)
			}
			return result.Count, nil
		}
		return 0, nil
	}

	count, err := q.collection.CountDocuments(ctx, q.filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count: %w", err)
	}
	return count, nil
}

// First возвращает первую запись
func (q *MongoQueryBuilder[T]) First(ctx context.Context) (T, error) {
	var zero T
	q.Limit(1)
	results, err := q.Execute(ctx)
	if err != nil {
		return zero, err
	}
	if len(results) == 0 {
		return zero, fmt.Errorf("no results found")
	}
	return results[0], nil
}

// Exists проверяет существование записей
func (q *MongoQueryBuilder[T]) Exists(ctx context.Context) (bool, error) {
	count, err := q.Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

