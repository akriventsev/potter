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

// GraphQLSchemaGenerator генератор GraphQL схем из proto файлов
type GraphQLSchemaGenerator struct {
	*BaseGenerator
	typeMapper    *TypeMapper
	schemaBuilder *SchemaBuilder
}

// NewGraphQLSchemaGenerator создает новый генератор GraphQL схем
func NewGraphQLSchemaGenerator(outputDir string) *GraphQLSchemaGenerator {
	base := NewBaseGenerator("graphql", outputDir)
	return &GraphQLSchemaGenerator{
		BaseGenerator: base,
		typeMapper:    NewTypeMapper(),
		schemaBuilder: NewSchemaBuilder(),
	}
}

// Generate генерирует GraphQL схему из ParsedSpec
func (g *GraphQLSchemaGenerator) Generate(spec *ParsedSpec, config *GeneratorConfig) error {
	// Генерация GraphQL schema
	if err := g.generateSchema(spec, config); err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	// Генерация gqlgen config
	if err := g.generateGqlgenConfig(spec, config); err != nil {
		return fmt.Errorf("failed to generate gqlgen config: %w", err)
	}

	// Генерация resolver stubs
	if err := g.generateResolverStubs(spec, config); err != nil {
		return fmt.Errorf("failed to generate resolver stubs: %w", err)
	}

	// Генерация Potter dispatch резолверов (документация и примеры)
	if err := g.GeneratePotterResolvers(spec, config); err != nil {
		return fmt.Errorf("failed to generate potter resolvers: %w", err)
	}

	return nil
}

// generateSchema генерирует GraphQL schema файл
func (g *GraphQLSchemaGenerator) generateSchema(spec *ParsedSpec, config *GeneratorConfig) error {
	var content strings.Builder

	// Заголовок схемы
	content.WriteString("# GraphQL Schema generated from proto files\n")
	content.WriteString("# DO NOT EDIT MANUALLY\n\n")

	// Генерация Query type
	content.WriteString("type Query {\n")
	for _, query := range spec.Queries {
		fieldDef := g.schemaBuilder.BuildQueryField(query)
		content.WriteString(fmt.Sprintf("  %s\n", fieldDef))
	}
	content.WriteString("}\n\n")

	// Генерация Mutation type
	content.WriteString("type Mutation {\n")
	for _, command := range spec.Commands {
		fieldDef := g.schemaBuilder.BuildMutationField(command)
		content.WriteString(fmt.Sprintf("  %s\n", fieldDef))
	}
	content.WriteString("}\n\n")

	// Генерация Subscription type
	content.WriteString("type Subscription {\n")
	for _, event := range spec.Events {
		if !event.IsError {
			fieldDef := g.schemaBuilder.BuildSubscriptionField(event)
			content.WriteString(fmt.Sprintf("  %s\n", fieldDef))
		}
	}
	content.WriteString("}\n\n")

	// Генерация типов из агрегатов
	for _, agg := range spec.Aggregates {
		typeDef := g.schemaBuilder.BuildTypeFromAggregate(agg)
		content.WriteString(typeDef)
		content.WriteString("\n\n")
	}

	// Генерация Input типов для команд
	for _, command := range spec.Commands {
		inputDef := g.schemaBuilder.BuildInputType(command)
		content.WriteString(inputDef)
		content.WriteString("\n\n")
	}

	path := "api/graphql/schema.graphql"
	return g.writer.WriteFile(path, content.String())
}

// generateGqlgenConfig генерирует gqlgen.yml конфигурацию
func (g *GraphQLSchemaGenerator) generateGqlgenConfig(spec *ParsedSpec, config *GeneratorConfig) error {
	var content strings.Builder

	content.WriteString("# gqlgen configuration\n")
	content.WriteString("schema:\n")
	content.WriteString("  - api/graphql/schema.graphql\n\n")
	content.WriteString("exec:\n")
	content.WriteString("  filename: api/graphql/generated.go\n")
	content.WriteString("  package: graphql\n\n")
	content.WriteString("model:\n")
	content.WriteString("  filename: api/graphql/models_gen.go\n")
	content.WriteString("  package: graphql\n\n")
	content.WriteString("resolver:\n")
	content.WriteString("  layout: follow-schema\n")
	content.WriteString("  dir: api/graphql\n")
	content.WriteString("  package: graphql\n")
	content.WriteString("  filename_template: {name}.resolvers.go\n\n")

	path := "api/graphql/gqlgen.yml"
	return g.writer.WriteFile(path, content.String())
}

// generateResolverStubs генерирует resolver stubs
func (g *GraphQLSchemaGenerator) generateResolverStubs(spec *ParsedSpec, config *GeneratorConfig) error {
	// Проверяем, существует ли файл resolvers.go
	if g.writer.FileExists("api/graphql/resolvers.go") {
		// Не перезаписываем существующий файл
		return nil
	}

	var content strings.Builder

	content.WriteString("package graphql\n\n")
	content.WriteString("// This file will be overwritten by gqlgen\n")
	content.WriteString("// Run 'gqlgen generate' to regenerate\n\n")

	path := "api/graphql/resolvers.go"
	return g.writer.WriteFile(path, content.String())
}

// GeneratePotterResolvers генерирует dispatch резолверы для интеграции с Potter CQRS
// Этот метод генерирует файл с резолверами, которые автоматически интегрируются через potterExecutableSchema
func (g *GraphQLSchemaGenerator) GeneratePotterResolvers(spec *ParsedSpec, config *GeneratorConfig) error {
	var content strings.Builder

	content.WriteString("// Code generated by potter-gen. DO NOT EDIT.\n\n")
	content.WriteString("package graphql\n\n")
	content.WriteString("import (\n")
	content.WriteString("\t\"context\"\n")
	content.WriteString("\t\"github.com/akriventsev/potter/framework/adapters/transport\"\n")
	content.WriteString(")\n\n")
	content.WriteString("// Этот файл содержит примеры dispatch резолверов для интеграции с Potter CQRS.\n")
	content.WriteString("// В реальности, резолверы автоматически регистрируются через potterExecutableSchema.AutoRegisterResolvers()\n")
	content.WriteString("// при использовании NewGraphQLAdapterWithCQRS.\n\n")
	content.WriteString("// Для кастомных резолверов используйте adapter.RegisterResolver():\n")
	content.WriteString("// adapter.RegisterResolver(\"Query\", \"customField\", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {\n")
	content.WriteString("//     // Кастомная логика\n")
	content.WriteString("//     return result, nil\n")
	content.WriteString("// })\n\n")

	// Генерация примеров для Query резолверов
	if len(spec.Queries) > 0 {
		content.WriteString("// Примеры Query резолверов (автоматически регистрируются через potterExecutableSchema):\n")
		for _, query := range spec.Queries {
			fieldName := g.toCamelCase(query.Name)
			content.WriteString(fmt.Sprintf("// Query.%s автоматически маппится на QueryResolver.Resolve(\"%s\", args)\n", fieldName, query.Name))
		}
		content.WriteString("\n")
	}

	// Генерация примеров для Mutation резолверов
	if len(spec.Commands) > 0 {
		content.WriteString("// Примеры Mutation резолверов (автоматически регистрируются через potterExecutableSchema):\n")
		for _, command := range spec.Commands {
			fieldName := g.toCamelCase(command.Name)
			content.WriteString(fmt.Sprintf("// Mutation.%s автоматически маппится на CommandResolver.Resolve(\"%s\", args)\n", fieldName, command.Name))
		}
		content.WriteString("\n")
	}

	// Генерация примеров для Subscription резолверов
	if len(spec.Events) > 0 {
		content.WriteString("// Примеры Subscription резолверов (автоматически регистрируются через potterExecutableSchema):\n")
		for _, event := range spec.Events {
			if !event.IsError {
				fieldName := g.toCamelCase(event.Name)
				content.WriteString(fmt.Sprintf("// Subscription.%s автоматически маппится на SubscriptionResolver.Subscribe(ctx, \"%s\")\n", fieldName, event.EventType))
			}
		}
		content.WriteString("\n")
	}

	content.WriteString("// Для ручной регистрации резолверов используйте:\n")
	content.WriteString("// func registerCustomResolvers(adapter *transport.GraphQLAdapter) {\n")
	content.WriteString("//     // Кастомные резолверы будут переопределять автоматические\n")
	content.WriteString("//     adapter.RegisterResolver(\"Query\", \"customField\", customResolver)\n")
	content.WriteString("// }\n")

	path := "api/graphql/potter_resolvers.go"
	return g.writer.WriteFile(path, content.String())
}

// toCamelCase конвертирует имя в camelCase (для использования в GraphQL полях)
func (g *GraphQLSchemaGenerator) toCamelCase(s string) string {
	if len(s) == 0 {
		return s
	}
	// Простая конвертация: первая буква в нижний регистр
	return strings.ToLower(s[:1]) + s[1:]
}

// TypeMapper маппинг proto типов → GraphQL типы
type TypeMapper struct{}

// NewTypeMapper создает новый TypeMapper
func NewTypeMapper() *TypeMapper {
	return &TypeMapper{}
}

// MapProtoType конвертирует proto тип в GraphQL тип
func (tm *TypeMapper) MapProtoType(protoType string, repeated bool, optional bool) string {
	var gqlType string

	switch protoType {
	case "string":
		gqlType = "String"
	case "int32", "int64":
		gqlType = "Int"
	case "float32", "float64", "double":
		gqlType = "Float"
	case "bool":
		gqlType = "Boolean"
	case "bytes":
		gqlType = "String" // Base64 encoded
	default:
		// Custom message type
		gqlType = protoType
	}

	// Обработка repeated (массивы)
	if repeated {
		gqlType = fmt.Sprintf("[%s!]!", gqlType)
	} else if !optional {
		gqlType = gqlType + "!"
	}

	return gqlType
}

// MapProtoMessage генерирует GraphQL type из proto message
func (tm *TypeMapper) MapProtoMessage(messageName string, fields []FieldSpec) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("type %s {\n", messageName))
	for _, field := range fields {
		gqlType := tm.MapProtoType(field.Type, field.Repeated, field.Optional)
		fieldName := tm.toCamelCase(field.Name)
		builder.WriteString(fmt.Sprintf("  %s: %s\n", fieldName, gqlType))
	}
	builder.WriteString("}\n")

	return builder.String()
}

// toCamelCase конвертирует snake_case в camelCase
func (tm *TypeMapper) toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	var result strings.Builder
	for i, part := range parts {
		if i == 0 {
			result.WriteString(part)
		} else {
			if len(part) > 0 {
				result.WriteString(strings.ToUpper(part[:1]) + part[1:])
			}
		}
	}
	return result.String()
}

// SchemaBuilder построение GraphQL schema
type SchemaBuilder struct {
	typeMapper *TypeMapper
}

// NewSchemaBuilder создает новый SchemaBuilder
func NewSchemaBuilder() *SchemaBuilder {
	return &SchemaBuilder{
		typeMapper: NewTypeMapper(),
	}
}

// BuildQueryField генерирует поле Query type
func (sb *SchemaBuilder) BuildQueryField(query QuerySpec) string {
	// Маппинг имени запроса
	fieldName := sb.toCamelCase(query.Name)

	// Генерация аргументов (упрощенная версия)
	args := "id: ID!"

	// Генерация возвращаемого типа
	returnType := query.ResponseType

	// Применение директив
	directives := ""
	if query.Cacheable {
		directives = fmt.Sprintf(" @cacheControl(maxAge: %d)", query.CacheTTLSeconds)
	}

	return fmt.Sprintf("%s(%s): %s%s", fieldName, args, returnType, directives)
}

// BuildMutationField генерирует поле Mutation type
func (sb *SchemaBuilder) BuildMutationField(command CommandSpec) string {
	// Маппинг имени команды
	fieldName := sb.toCamelCase(command.Name)

	// Генерация аргументов
	inputType := command.RequestType + "Input"
	args := fmt.Sprintf("input: %s!", inputType)

	// Генерация возвращаемого типа
	returnType := command.ResponseType

	// Применение директив
	directives := ""
	if command.Async {
		directives += " @async"
	}
	if command.Idempotent {
		directives += " @idempotent"
	}

	return fmt.Sprintf("%s(%s): %s%s", fieldName, args, returnType, directives)
}

// BuildSubscriptionField генерирует поле Subscription type
func (sb *SchemaBuilder) BuildSubscriptionField(event EventSpec) string {
	// Маппинг имени события
	fieldName := sb.toCamelCase(event.Name)

	// Генерация возвращаемого типа
	returnType := event.Name

	return fmt.Sprintf("%s: %s!", fieldName, returnType)
}

// BuildTypeFromAggregate генерирует GraphQL type из агрегата
func (sb *SchemaBuilder) BuildTypeFromAggregate(agg AggregateSpec) string {
	return sb.typeMapper.MapProtoMessage(agg.Name, agg.Fields)
}

// BuildInputType генерирует Input type для команды
func (sb *SchemaBuilder) BuildInputType(command CommandSpec) string {
	// В реальной реализации нужно парсить RequestType и извлекать поля
	// Здесь упрощенная версия
	inputName := command.RequestType + "Input"

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("input %s {\n", inputName))
	// Поля будут добавлены при парсинге RequestType message
	builder.WriteString("  # Fields will be generated from proto message\n")
	builder.WriteString("}\n")

	return builder.String()
}

// toCamelCase конвертирует имя в camelCase
func (sb *SchemaBuilder) toCamelCase(s string) string {
	if len(s) == 0 {
		return s
	}
	// Простая конвертация: первая буква в нижний регистр
	return strings.ToLower(s[:1]) + s[1:]
}
