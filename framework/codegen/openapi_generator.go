// Copyright 2024 Potter Framework Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codegen

import (
	"fmt"
	"strings"
)

// OpenAPIGenerator генератор OpenAPI 3.0 спецификаций из proto файлов
type OpenAPIGenerator struct {
	*BaseGenerator
	typeMapper    *OpenAPITypeMapper
	schemaBuilder *OpenAPISchemaBuilder
}

// NewOpenAPIGenerator создает новый генератор OpenAPI спецификаций
func NewOpenAPIGenerator(outputDir string) *OpenAPIGenerator {
	base := NewBaseGenerator("openapi", outputDir)
	return &OpenAPIGenerator{
		BaseGenerator: base,
		typeMapper:    NewOpenAPITypeMapper(),
		schemaBuilder: NewOpenAPISchemaBuilder(),
	}
}

// Generate генерирует OpenAPI спецификацию из ParsedSpec
func (g *OpenAPIGenerator) Generate(spec *ParsedSpec, config *GeneratorConfig) error {
	// Генерация OpenAPI спецификации
	if err := g.generateOpenAPISpec(spec, config); err != nil {
		return fmt.Errorf("failed to generate OpenAPI spec: %w", err)
	}

	// Swagger UI конфигурация генерируется в PresentationGenerator.generateSwaggerUIAdapter()
	// Этот метод больше не используется

	return nil
}

// generateOpenAPISpec генерирует OpenAPI 3.0 спецификацию
func (g *OpenAPIGenerator) generateOpenAPISpec(spec *ParsedSpec, config *GeneratorConfig) error {
	var content strings.Builder

	// OpenAPI заголовок и версия
	// TODO: извлечь openapi_version из ServiceOptions.OpenAPIInfo
	openAPIVersion := "3.0.3"
	content.WriteString(fmt.Sprintf("openapi: %s\n", openAPIVersion))

	// Info блок
	content.WriteString("info:\n")
	title := fmt.Sprintf("%s API", spec.ModuleName)
	version := "1.0.0"
	description := fmt.Sprintf("API for %s service", spec.ModuleName)

	// Извлечение OpenAPIInfo из ServiceOptions (если доступно)
	// TODO: расширить ServiceOptions для поддержки OpenAPIInfo
	if len(spec.Services) > 0 && spec.Services[0].ModuleName != "" {
		// Можно использовать module_name как базовое значение
	}

	content.WriteString(fmt.Sprintf("  title: %s\n", title))
	content.WriteString(fmt.Sprintf("  version: %s\n", version))
	content.WriteString(fmt.Sprintf("  description: %s\n", description))
	content.WriteString("\n")

	// Servers
	content.WriteString("servers:\n")
	content.WriteString("  - url: http://localhost:8080/api/v1\n")
	content.WriteString("    description: Local development server\n")
	content.WriteString("\n")

	// Paths
	content.WriteString("paths:\n")

	// Генерация paths для команд
	for _, cmd := range spec.Commands {
		pathDef := g.schemaBuilder.BuildPathFromCommand(cmd)
		content.WriteString(pathDef)
		content.WriteString("\n")
	}

	// Генерация paths для запросов
	for _, query := range spec.Queries {
		pathDef := g.schemaBuilder.BuildPathFromQuery(query)
		content.WriteString(pathDef)
		content.WriteString("\n")
	}

	// Components
	content.WriteString("components:\n")
	content.WriteString("  schemas:\n")

	// Собираем все уникальные типы сообщений для генерации schemas
	messageTypes := make(map[string]bool)

	// Добавляем типы из команд
	for _, cmd := range spec.Commands {
		if cmd.RequestType != "" {
			messageTypes[cmd.RequestType] = true
		}
		if cmd.ResponseType != "" {
			messageTypes[cmd.ResponseType] = true
		}
	}

	// Добавляем типы из запросов
	for _, query := range spec.Queries {
		if query.RequestType != "" {
			messageTypes[query.RequestType] = true
		}
		if query.ResponseType != "" {
			messageTypes[query.ResponseType] = true
		}
	}

	// Генерация schemas из агрегатов
	for _, agg := range spec.Aggregates {
		schemaDef := g.schemaBuilder.BuildSchemaFromAggregate(agg)
		content.WriteString(schemaDef)
		content.WriteString("\n")
		// Помечаем, что schema для агрегата уже сгенерирована
		messageTypes[agg.Name] = false
	}

	// Генерация schemas для request/response messages из команд
	for _, cmd := range spec.Commands {
		if cmd.RequestType != "" && messageTypes[cmd.RequestType] {
			schemaDef := g.schemaBuilder.BuildSchemaFromFields(cmd.RequestType, cmd.RequestFields)
			content.WriteString(schemaDef)
			content.WriteString("\n")
			messageTypes[cmd.RequestType] = false
		}
		if cmd.ResponseType != "" && messageTypes[cmd.ResponseType] {
			schemaDef := g.schemaBuilder.BuildSchemaFromFields(cmd.ResponseType, cmd.ResponseFields)
			content.WriteString(schemaDef)
			content.WriteString("\n")
			messageTypes[cmd.ResponseType] = false
		}
	}

	// Генерация schemas для request/response messages из запросов
	for _, query := range spec.Queries {
		if query.RequestType != "" && messageTypes[query.RequestType] {
			schemaDef := g.schemaBuilder.BuildSchemaFromFields(query.RequestType, query.RequestFields)
			content.WriteString(schemaDef)
			content.WriteString("\n")
			messageTypes[query.RequestType] = false
		}
		if query.ResponseType != "" && messageTypes[query.ResponseType] {
			schemaDef := g.schemaBuilder.BuildSchemaFromFields(query.ResponseType, query.ResponseFields)
			content.WriteString(schemaDef)
			content.WriteString("\n")
			messageTypes[query.ResponseType] = false
		}
	}

	path := "api/openapi/openapi.yaml"
	return g.writer.WriteFile(path, content.String())
}

// OpenAPITypeMapper маппинг proto типов → OpenAPI типы
type OpenAPITypeMapper struct{}

// NewOpenAPITypeMapper создает новый OpenAPI TypeMapper
func NewOpenAPITypeMapper() *OpenAPITypeMapper {
	return &OpenAPITypeMapper{}
}

// MapProtoType конвертирует proto тип в OpenAPI тип
func (tm *OpenAPITypeMapper) MapProtoType(protoType string, repeated bool, optional bool) map[string]interface{} {
	var schema map[string]interface{}

	switch protoType {
	case "string":
		schema = map[string]interface{}{
			"type": "string",
		}
	case "int32", "int64":
		schema = map[string]interface{}{
			"type": "integer",
			"format": func() string {
				if protoType == "int64" {
					return "int64"
				}
				return "int32"
			}(),
		}
	case "float32", "float64", "double":
		schema = map[string]interface{}{
			"type": "number",
			"format": func() string {
				if protoType == "float64" || protoType == "double" {
					return "double"
				}
				return "float"
			}(),
		}
	case "bool":
		schema = map[string]interface{}{
			"type": "boolean",
		}
	case "bytes":
		schema = map[string]interface{}{
			"type":   "string",
			"format": "byte",
		}
	default:
		// Custom message type - используем $ref
		schema = map[string]interface{}{
			"$ref": fmt.Sprintf("#/components/schemas/%s", protoType),
		}
	}

	// Обработка repeated (массивы)
	if repeated {
		schema = map[string]interface{}{
			"type":  "array",
			"items": schema,
		}
	}

	// Обработка optional (nullable)
	if optional {
		schema["nullable"] = true
	}

	return schema
}

// OpenAPISchemaBuilder построение OpenAPI paths и schemas
type OpenAPISchemaBuilder struct {
	typeMapper *OpenAPITypeMapper
}

// NewOpenAPISchemaBuilder создает новый OpenAPI SchemaBuilder
func NewOpenAPISchemaBuilder() *OpenAPISchemaBuilder {
	return &OpenAPISchemaBuilder{
		typeMapper: NewOpenAPITypeMapper(),
	}
}

// BuildPathFromCommand генерирует POST/PUT/DELETE path для команды
func (sb *OpenAPISchemaBuilder) BuildPathFromCommand(cmd CommandSpec) string {
	var builder strings.Builder

	cmdName := strings.ToLower(cmd.Name)
	resourceName := sb.toSnakeCase(cmd.Aggregate)
	if !strings.HasSuffix(resourceName, "s") {
		resourceName = resourceName + "s"
	}

	var httpMethod string
	var path string

	if strings.HasPrefix(cmdName, "create") {
		httpMethod = "post"
		path = fmt.Sprintf("/%s", resourceName)
	} else if strings.HasPrefix(cmdName, "update") {
		httpMethod = "put"
		path = fmt.Sprintf("/%s/{id}", resourceName)
	} else if strings.HasPrefix(cmdName, "delete") {
		httpMethod = "delete"
		path = fmt.Sprintf("/%s/{id}", resourceName)
	} else {
		httpMethod = "post"
		path = fmt.Sprintf("/%s/%s", resourceName, sb.toSnakeCase(cmd.Name))
	}

	builder.WriteString(fmt.Sprintf("  %s:\n", path))
	builder.WriteString(fmt.Sprintf("    %s:\n", httpMethod))

	// Summary
	if cmd.Summary != "" {
		builder.WriteString(fmt.Sprintf("      summary: %s\n", cmd.Summary))
	} else {
		builder.WriteString("      summary: " + cmd.Name + "\n")
	}

	// Description
	if cmd.Description != "" {
		builder.WriteString(fmt.Sprintf("      description: %s\n", cmd.Description))
	} else {
		builder.WriteString("      description: " + cmd.Name + " command\n")
	}

	builder.WriteString("      operationId: " + cmd.Name + "\n")

	// Tags
	if len(cmd.Tags) > 0 {
		builder.WriteString("      tags:\n")
		for _, tag := range cmd.Tags {
			builder.WriteString(fmt.Sprintf("        - %s\n", tag))
		}
	}

	// Deprecated
	if cmd.Deprecated {
		builder.WriteString("      deprecated: true\n")
	}

	// Request body
	if httpMethod != "delete" {
		builder.WriteString("      requestBody:\n")
		builder.WriteString("        required: true\n")
		builder.WriteString("        content:\n")
		builder.WriteString("          application/json:\n")
		builder.WriteString("            schema:\n")
		builder.WriteString(fmt.Sprintf("              $ref: '#/components/schemas/%s'\n", cmd.RequestType))
	}

	// Responses
	builder.WriteString("      responses:\n")
	builder.WriteString("        '200':\n")
	builder.WriteString("          description: Success\n")
	builder.WriteString("          content:\n")
	builder.WriteString("            application/json:\n")
	builder.WriteString("              schema:\n")
	builder.WriteString(fmt.Sprintf("                $ref: '#/components/schemas/%s'\n", cmd.ResponseType))
	builder.WriteString("        '400':\n")
	builder.WriteString("          description: Bad Request\n")
	builder.WriteString("        '500':\n")
	builder.WriteString("          description: Internal Server Error\n")

	// Extensions для Potter annotations
	if cmd.Async {
		builder.WriteString("      x-async: true\n")
	}
	if cmd.Idempotent {
		builder.WriteString("      x-idempotent: true\n")
	}
	if cmd.TimeoutSeconds > 0 {
		builder.WriteString(fmt.Sprintf("      x-timeout-seconds: %d\n", cmd.TimeoutSeconds))
	}

	return builder.String()
}

// BuildPathFromQuery генерирует GET path для запроса
func (sb *OpenAPISchemaBuilder) BuildPathFromQuery(query QuerySpec) string {
	var builder strings.Builder

	queryName := strings.ToLower(query.Name)
	resourceName := sb.inferResourceFromQuery(query.Name)
	if !strings.HasSuffix(resourceName, "s") {
		resourceName = resourceName + "s"
	}

	var path string
	if strings.HasPrefix(queryName, "get") {
		path = fmt.Sprintf("/%s/{id}", resourceName)
	} else if strings.HasPrefix(queryName, "list") {
		path = fmt.Sprintf("/%s", resourceName)
	} else {
		path = fmt.Sprintf("/%s/%s", resourceName, sb.toSnakeCase(query.Name))
	}

	builder.WriteString(fmt.Sprintf("  %s:\n", path))
	builder.WriteString("    get:\n")

	// Summary
	if query.Summary != "" {
		builder.WriteString(fmt.Sprintf("      summary: %s\n", query.Summary))
	} else {
		builder.WriteString("      summary: " + query.Name + "\n")
	}

	// Description
	if query.Description != "" {
		builder.WriteString(fmt.Sprintf("      description: %s\n", query.Description))
	} else {
		builder.WriteString("      description: " + query.Name + " query\n")
	}

	builder.WriteString("      operationId: " + query.Name + "\n")

	// Tags
	if len(query.Tags) > 0 {
		builder.WriteString("      tags:\n")
		for _, tag := range query.Tags {
			builder.WriteString(fmt.Sprintf("        - %s\n", tag))
		}
	}

	// Deprecated
	if query.Deprecated {
		builder.WriteString("      deprecated: true\n")
	}

	// Parameters для GET запросов
	if strings.HasPrefix(queryName, "get") {
		builder.WriteString("      parameters:\n")
		builder.WriteString("        - name: id\n")
		builder.WriteString("          in: path\n")
		builder.WriteString("          required: true\n")
		builder.WriteString("          schema:\n")
		builder.WriteString("            type: string\n")
	} else {
		// Query parameters для list запросов
		builder.WriteString("      parameters:\n")
		for _, field := range query.RequestFields {
			builder.WriteString(fmt.Sprintf("        - name: %s\n", sb.toSnakeCase(field.Name)))
			builder.WriteString("          in: query\n")
			builder.WriteString("          required: false\n")
			schema := sb.typeMapper.MapProtoType(field.Type, field.Repeated, field.Optional)
			builder.WriteString("          schema:\n")
			if schemaType, ok := schema["type"].(string); ok {
				builder.WriteString(fmt.Sprintf("            type: %s\n", schemaType))
			}
		}
	}

	// Responses
	builder.WriteString("      responses:\n")
	builder.WriteString("        '200':\n")
	builder.WriteString("          description: Success\n")
	builder.WriteString("          content:\n")
	builder.WriteString("            application/json:\n")
	builder.WriteString("              schema:\n")
	builder.WriteString(fmt.Sprintf("                $ref: '#/components/schemas/%s'\n", query.ResponseType))
	builder.WriteString("        '400':\n")
	builder.WriteString("          description: Bad Request\n")
	builder.WriteString("        '500':\n")
	builder.WriteString("          description: Internal Server Error\n")

	// Extensions для Potter annotations
	if query.Cacheable {
		builder.WriteString("      x-cacheable: true\n")
		if query.CacheTTLSeconds > 0 {
			builder.WriteString(fmt.Sprintf("      x-cache-ttl-seconds: %d\n", query.CacheTTLSeconds))
		}
	}

	return builder.String()
}

// BuildSchemaFromAggregate генерирует schema из агрегата
func (sb *OpenAPISchemaBuilder) BuildSchemaFromAggregate(agg AggregateSpec) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("    %s:\n", agg.Name))
	builder.WriteString("      type: object\n")
	builder.WriteString("      properties:\n")

	for _, field := range agg.Fields {
		builder.WriteString(fmt.Sprintf("        %s:\n", sb.toSnakeCase(field.Name)))
		schema := sb.typeMapper.MapProtoType(field.Type, field.Repeated, field.Optional)
		if schemaType, ok := schema["type"].(string); ok {
			builder.WriteString(fmt.Sprintf("          type: %s\n", schemaType))
		}
		if format, ok := schema["format"].(string); ok {
			builder.WriteString(fmt.Sprintf("          format: %s\n", format))
		}
		if items, ok := schema["items"].(map[string]interface{}); ok {
			builder.WriteString("          items:\n")
			if itemType, ok := items["type"].(string); ok {
				builder.WriteString(fmt.Sprintf("            type: %s\n", itemType))
			}
		}
		if nullable, ok := schema["nullable"].(bool); ok && nullable {
			builder.WriteString("          nullable: true\n")
		}
	}

	builder.WriteString("      required:\n")
	for _, field := range agg.Fields {
		if !field.Optional && !field.Repeated {
			builder.WriteString(fmt.Sprintf("        - %s\n", sb.toSnakeCase(field.Name)))
		}
	}

	return builder.String()
}

// BuildSchemaFromFields генерирует schema из полей сообщения
func (sb *OpenAPISchemaBuilder) BuildSchemaFromFields(messageName string, fields []FieldSpec) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("    %s:\n", messageName))
	builder.WriteString("      type: object\n")

	if len(fields) > 0 {
		builder.WriteString("      properties:\n")

		for _, field := range fields {
			builder.WriteString(fmt.Sprintf("        %s:\n", sb.toSnakeCase(field.Name)))
			schema := sb.typeMapper.MapProtoType(field.Type, field.Repeated, field.Optional)
			if schemaType, ok := schema["type"].(string); ok {
				builder.WriteString(fmt.Sprintf("          type: %s\n", schemaType))
			}
			if format, ok := schema["format"].(string); ok {
				builder.WriteString(fmt.Sprintf("          format: %s\n", format))
			}
			if items, ok := schema["items"].(map[string]interface{}); ok {
				builder.WriteString("          items:\n")
				if itemType, ok := items["type"].(string); ok {
					builder.WriteString(fmt.Sprintf("            type: %s\n", itemType))
				}
			}
			if nullable, ok := schema["nullable"].(bool); ok && nullable {
				builder.WriteString("          nullable: true\n")
			}
		}

		// Required fields
		requiredFields := []string{}
		for _, field := range fields {
			if !field.Optional && !field.Repeated {
				requiredFields = append(requiredFields, sb.toSnakeCase(field.Name))
			}
		}
		if len(requiredFields) > 0 {
			builder.WriteString("      required:\n")
			for _, fieldName := range requiredFields {
				builder.WriteString(fmt.Sprintf("        - %s\n", fieldName))
			}
		}
	}

	return builder.String()
}

// BuildRequestBodySchema генерирует requestBody schema для команды
func (sb *OpenAPISchemaBuilder) BuildRequestBodySchema(cmd CommandSpec) map[string]interface{} {
	return map[string]interface{}{
		"$ref": fmt.Sprintf("#/components/schemas/%s", cmd.RequestType),
	}
}

// BuildResponseSchema генерирует response schema
func (sb *OpenAPISchemaBuilder) BuildResponseSchema(responseType string) map[string]interface{} {
	return map[string]interface{}{
		"$ref": fmt.Sprintf("#/components/schemas/%s", responseType),
	}
}

// toSnakeCase конвертирует CamelCase в snake_case
func (sb *OpenAPISchemaBuilder) toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return strings.ToLower(string(result))
}

// inferResourceFromQuery определяет имя ресурса из имени запроса
func (sb *OpenAPISchemaBuilder) inferResourceFromQuery(queryName string) string {
	queryNameLower := strings.ToLower(queryName)

	prefixes := []string{"get", "list", "find", "search", "fetch", "retrieve", "query"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(queryNameLower, prefix) {
			resource := strings.TrimPrefix(queryNameLower, prefix)
			if len(resource) <= 1 {
				resource = queryNameLower
			}
			resourceSnake := sb.toSnakeCase(resource)
			if strings.HasSuffix(resourceSnake, "s") && len(resourceSnake) > 1 {
				return strings.TrimSuffix(resourceSnake, "s")
			}
			return resourceSnake
		}
	}

	resourceSnake := sb.toSnakeCase(queryName)
	if strings.HasSuffix(resourceSnake, "s") && len(resourceSnake) > 1 {
		return strings.TrimSuffix(resourceSnake, "s")
	}
	return resourceSnake
}
