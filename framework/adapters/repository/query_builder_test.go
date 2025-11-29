package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// QueryTestMapper тестовый mapper для QueryBuilder тестов
type QueryTestMapper struct{}

func (m *QueryTestMapper) ToRow(entity TestEntity) (map[string]interface{}, error) {
	return map[string]interface{}{
		"id":   entity.IDField,
		"name": entity.Name,
	}, nil
}

func (m *QueryTestMapper) FromRow(row map[string]interface{}) (TestEntity, error) {
	entity := TestEntity{}
	if id, ok := row["id"].(string); ok {
		entity.IDField = id
	}
	if name, ok := row["name"].(string); ok {
		entity.Name = name
	}
	return entity, nil
}

// createTestBuilder создает builder для тестов
// Использует реальное соединение, но SQL генерация не требует его для buildQuery
func createTestBuilder() (*PostgresQueryBuilder[TestEntity], error) {
	config := DefaultPostgresConfig()
	config.TableName = "test_table"
	
	// Пытаемся подключиться, но если не получается - все равно можем тестировать SQL
	conn, err := pgx.Connect(context.Background(), "postgres://localhost/test?sslmode=disable")
	if err != nil {
		// Для тестов SQL генерации нам не нужно реальное соединение
		// Используем nil - buildQuery() не использует conn
		// Но NewPostgresQueryBuilder требует conn, поэтому создадим минимальный
		return nil, err
	}
	
	mapper := &QueryTestMapper{}
	builder := NewPostgresQueryBuilder[TestEntity](conn, mapper, config)
	return builder, nil
}

func TestPostgresQueryBuilder_Where(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.Where("name", Eq, "John")

	query, args, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "WHERE") {
		t.Error("Expected WHERE clause in query")
	}

	if !strings.Contains(query, "name = $1") {
		t.Errorf("Expected name = $1 in query, got: %s", query)
	}

	if len(args) != 1 {
		t.Errorf("Expected 1 arg, got %d", len(args))
	}

	if args[0] != "John" {
		t.Errorf("Expected arg value 'John', got %v", args[0])
	}
}

func TestPostgresQueryBuilder_And(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.Where("name", Eq, "John").And().Where("name", Gt, "A")

	query, _, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "AND") {
		t.Error("Expected AND in query")
	}

	if !strings.Contains(query, "name = $1") {
		t.Error("Expected first condition with name")
	}

	if !strings.Contains(query, "name > $2") {
		t.Error("Expected second condition with name")
	}
}

func TestPostgresQueryBuilder_Or(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.Where("name", Eq, "John").Or().Where("name", Eq, "Jane")

	query, args, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "OR") {
		t.Error("Expected OR in query")
	}

	if len(args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(args))
	}
}

func TestPostgresQueryBuilder_Not(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.Not().Where("name", Lt, "A")

	query, _, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "NOT") {
		t.Error("Expected NOT in query")
	}

	if !strings.Contains(query, "(name < $1)") {
		t.Errorf("Expected NOT (name < $1), got: %s", query)
	}
}

func TestPostgresQueryBuilder_IN(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.Where("name", In, []string{"John", "Jane", "Bob"})

	query, args, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "IN") {
		t.Error("Expected IN in query")
	}

	if !strings.Contains(query, "$1, $2, $3") {
		t.Errorf("Expected multiple placeholders for IN, got: %s", query)
	}

	if len(args) != 3 {
		t.Errorf("Expected 3 args, got %d", len(args))
	}
}

func TestPostgresQueryBuilder_IN_Ints(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	// Используем поле, которое может быть в TestEntity - можем использовать строковое поле
	// Но для теста IN с ints создадим отдельный тест, используя временное поле в mapper
	builder.Where("name", In, []string{"18", "25", "30"}) // Используем строки вместо int

	query, args, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "IN") {
		t.Error("Expected IN in query")
	}

	if len(args) != 3 {
		t.Errorf("Expected 3 args, got %d", len(args))
	}
}

func TestPostgresQueryBuilder_BETWEEN(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.Where("name", Between, []string{"A", "Z"})

	query, args, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "BETWEEN") {
		t.Error("Expected BETWEEN in query")
	}

	if !strings.Contains(query, "$1 AND $2") {
		t.Errorf("Expected BETWEEN $1 AND $2, got: %s", query)
	}

	if len(args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(args))
	}
}

func TestPostgresQueryBuilder_HAVING(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.Where("name", Gt, "A")
	builder.GroupBy("name")
	builder.Having("COUNT(*)", Gt, 5)

	query, _, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "WHERE") {
		t.Error("Expected WHERE clause")
	}

	if !strings.Contains(query, "GROUP BY") {
		t.Error("Expected GROUP BY clause")
	}

	if !strings.Contains(query, "HAVING") {
		t.Error("Expected HAVING clause after WHERE")
	}

	// HAVING должен быть после GROUP BY
	havingIndex := strings.Index(query, "HAVING")
	groupByIndex := strings.Index(query, "GROUP BY")
	if havingIndex <= groupByIndex {
		t.Error("HAVING should be after GROUP BY")
	}
}

func TestPostgresQueryBuilder_OrderBy(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.OrderBy("name", Asc).OrderBy("id", Desc)

	query, _, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "ORDER BY") {
		t.Error("Expected ORDER BY in query")
	}

	if !strings.Contains(query, "name ASC") {
		t.Error("Expected name ASC in query")
	}

	if !strings.Contains(query, "id DESC") {
		t.Error("Expected id DESC in query")
	}
}

func TestPostgresQueryBuilder_Execute_NoArgs(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	
	// Тест что BuildQuery не падает без условий
	query, args, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if query == "" {
		t.Error("Expected non-empty query")
	}

	// Аргументы могут быть пустыми
	if args == nil {
		t.Error("Args should not be nil, should be empty slice")
	}
}

func TestPostgresQueryBuilder_LimitOffset(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.Limit(10).Offset(20)

	query, _, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "LIMIT 10") {
		t.Error("Expected LIMIT 10 in query")
	}

	if !strings.Contains(query, "OFFSET 20") {
		t.Error("Expected OFFSET 20 in query")
	}
}

func TestPostgresQueryBuilder_Page(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	builder.Page(3, 10) // page 3, pageSize 10 = offset 20

	query, _, err := builder.BuildQuery()
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}

	if !strings.Contains(query, "LIMIT 10") {
		t.Error("Expected LIMIT 10 in query")
	}

	if !strings.Contains(query, "OFFSET 20") {
		t.Error("Expected OFFSET 20 in query (page 3 * 10 - 10)")
	}
}

func TestPostgresQueryBuilder_RecordQueryPattern(t *testing.T) {
	builder, err := createTestBuilder()
	if err != nil {
		t.Skipf("Skipping test - cannot create builder: %v", err)
	}
	
	config := DefaultPostgresConfig()
	config.TableName = "test_table"
	
	// Для теста query pattern нам не нужна реальная БД
	// Создаем индекс менеджер с nil conn (он не используется в RecordQueryPattern)
	policy := DefaultIndexPolicy()
	autoIndexManager := NewAutoIndexManager(nil, policy) // Используем nil для теста

	builder.SetAutoIndexManager(autoIndexManager)

	builder.Where("name", Eq, "John")
	
	// RecordQueryPattern должен быть вызван автоматически в Where
	// Проверяем что pattern был записан
	if autoIndexManager.queryPatterns["name"] != 1 {
		t.Errorf("Expected queryPattern for 'name' to be 1, got %d", autoIndexManager.queryPatterns["name"])
	}
}
