package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphQLSchemaGenerator_Generate(t *testing.T) {
	// Создаем временную директорию
	tmpDir, err := os.MkdirTemp("", "graphql-gen-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	generator := NewGraphQLSchemaGenerator(tmpDir)

	// Создаем тестовый spec
	spec := &ParsedSpec{
		ModuleName: "test",
		Queries: []QuerySpec{
			{
				Name:         "GetProduct",
				RequestType:  "GetProductRequest",
				ResponseType: "Product",
				Cacheable:    true,
				CacheTTLSeconds: 300,
			},
		},
		Commands: []CommandSpec{
			{
				Name:           "CreateProduct",
				RequestType:    "CreateProductRequest",
				ResponseType:   "CreateProductResponse",
				Async:          true,
				Idempotent:     true,
			},
		},
		Events: []EventSpec{
			{
				Name:      "ProductCreatedEvent",
				EventType: "product.created",
				Aggregate: "Product",
				Version:   1,
				IsError:   false,
			},
		},
		Aggregates: []AggregateSpec{
			{
				Name:       "Product",
				Repository: "postgres",
				Fields: []FieldSpec{
					{Name: "id", Type: "string", Number: 1},
					{Name: "name", Type: "string", Number: 2},
					{Name: "price", Type: "float64", Number: 3},
				},
			},
		},
	}

	config := &GeneratorConfig{
		ModulePath: "test",
		OutputDir:  tmpDir,
		PackageName: "test",
		Overwrite: true,
	}

	err = generator.Generate(spec, config)
	require.NoError(t, err)

	// Проверка создания файлов
	schemaPath := filepath.Join(tmpDir, "api/graphql/schema.graphql")
	assert.FileExists(t, schemaPath)

	configPath := filepath.Join(tmpDir, "api/graphql/gqlgen.yml")
	assert.FileExists(t, configPath)

	resolversPath := filepath.Join(tmpDir, "api/graphql/resolvers.go")
	assert.FileExists(t, resolversPath)
}

func TestTypeMapper_MapProtoType(t *testing.T) {
	mapper := NewTypeMapper()

	tests := []struct {
		name     string
		protoType string
		repeated bool
		optional bool
		expected string
	}{
		{"string", "string", false, false, "String!"},
		{"int32", "int32", false, false, "Int!"},
		{"float64", "float64", false, false, "Float!"},
		{"bool", "bool", false, false, "Boolean!"},
		{"repeated string", "string", true, false, "[String!]!"},
		{"optional string", "string", false, true, "String"},
		{"repeated optional", "string", true, true, "[String!]!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.MapProtoType(tt.protoType, tt.repeated, tt.optional)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSchemaBuilder_BuildQueryField(t *testing.T) {
	builder := NewSchemaBuilder()

	query := QuerySpec{
		Name:            "GetProduct",
		RequestType:     "GetProductRequest",
		ResponseType:    "Product",
		Cacheable:       true,
		CacheTTLSeconds: 300,
	}

	field := builder.BuildQueryField(query)
	assert.Contains(t, field, "getProduct")
	assert.Contains(t, field, "Product")
	assert.Contains(t, field, "@cacheControl")
}

func TestSchemaBuilder_BuildMutationField(t *testing.T) {
	builder := NewSchemaBuilder()

	command := CommandSpec{
		Name:         "CreateProduct",
		RequestType:  "CreateProductRequest",
		ResponseType: "CreateProductResponse",
		Async:        true,
		Idempotent:   true,
	}

	field := builder.BuildMutationField(command)
	assert.Contains(t, field, "createProduct")
	assert.Contains(t, field, "@async")
	assert.Contains(t, field, "@idempotent")
}

func TestSchemaBuilder_BuildSubscriptionField(t *testing.T) {
	builder := NewSchemaBuilder()

	event := EventSpec{
		Name:      "ProductCreatedEvent",
		EventType: "product.created",
		Aggregate: "Product",
		Version:   1,
		IsError:   false,
	}

	field := builder.BuildSubscriptionField(event)
	assert.Contains(t, field, "productCreatedEvent")
	assert.Contains(t, field, "ProductCreatedEvent")
}

