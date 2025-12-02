package codegen

import (
	"fmt"
	"strings"
)

// DomainGenerator генератор доменного слоя
type DomainGenerator struct {
	*BaseGenerator
}

// NewDomainGenerator создает новый генератор доменного слоя
func NewDomainGenerator(outputDir string) *DomainGenerator {
	return &DomainGenerator{
		BaseGenerator: NewBaseGenerator("domain", outputDir),
	}
}

// Generate генерирует доменный слой
func (g *DomainGenerator) Generate(spec *ParsedSpec, config *GeneratorConfig) error {
	if err := g.generateAggregates(spec, config); err != nil {
		return fmt.Errorf("failed to generate aggregates: %w", err)
	}

	if err := g.generateEvents(spec, config); err != nil {
		return fmt.Errorf("failed to generate events: %w", err)
	}

	if err := g.generateRepositories(spec, config); err != nil {
		return fmt.Errorf("failed to generate repositories: %w", err)
	}

	return nil
}

// generateAggregates генерирует агрегаты
func (g *DomainGenerator) generateAggregates(spec *ParsedSpec, _ *GeneratorConfig) error {
	var content strings.Builder

	// Заголовок файла
	content.WriteString(g.addFileHeader("domain"))
	content.WriteString("import (\n")
	content.WriteString("\t\"time\"\n")
	content.WriteString("\n")
	content.WriteString("\t\"github.com/google/uuid\"\n")
	content.WriteString(")\n\n")

	// Генерация BaseAggregate
	content.WriteString(g.generateBaseAggregate())
	content.WriteString("\n")

	// Генерация каждого агрегата
	for _, agg := range spec.Aggregates {
		content.WriteString(g.generateAggregate(agg))
		content.WriteString("\n")
	}

	path := "domain/aggregates.go"
	return g.writer.WriteFile(path, content.String())
}

// generateBaseAggregate генерирует базовый агрегат
func (g *DomainGenerator) generateBaseAggregate() string {
	var builder strings.Builder

	builder.WriteString("// BaseAggregate базовая реализация агрегата\n")
	builder.WriteString("type BaseAggregate struct {\n")
	builder.WriteString("\tid      string\n")
	builder.WriteString("\tversion int\n")
	builder.WriteString("\tevents  []Event\n")
	builder.WriteString("}\n\n")

	builder.WriteString("// NewBaseAggregate создает новый базовый агрегат с UUID\n")
	builder.WriteString("func NewBaseAggregate() *BaseAggregate {\n")
	builder.WriteString("\treturn &BaseAggregate{\n")
	builder.WriteString("\t\tid:      uuid.New().String(),\n")
	builder.WriteString("\t\tversion: 0,\n")
	builder.WriteString("\t\tevents:  make([]Event, 0),\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n\n")

	builder.WriteString("// NewBaseAggregateWithID создает базовый агрегат с указанным ID\n")
	builder.WriteString("func NewBaseAggregateWithID(id string) *BaseAggregate {\n")
	builder.WriteString("\treturn &BaseAggregate{\n")
	builder.WriteString("\t\tid:      id,\n")
	builder.WriteString("\t\tversion: 0,\n")
	builder.WriteString("\t\tevents:  make([]Event, 0),\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n\n")

	builder.WriteString("// ID возвращает идентификатор агрегата\n")
	builder.WriteString("func (a *BaseAggregate) ID() string {\n")
	builder.WriteString("\treturn a.id\n")
	builder.WriteString("}\n\n")

	builder.WriteString("// Version возвращает версию агрегата\n")
	builder.WriteString("func (a *BaseAggregate) Version() int {\n")
	builder.WriteString("\treturn a.version\n")
	builder.WriteString("}\n\n")

	builder.WriteString("// Events возвращает список событий агрегата\n")
	builder.WriteString("func (a *BaseAggregate) Events() []Event {\n")
	builder.WriteString("\tif a.events == nil {\n")
	builder.WriteString("\t\treturn make([]Event, 0)\n")
	builder.WriteString("\t}\n")
	builder.WriteString("\treturn a.events\n")
	builder.WriteString("}\n\n")

	builder.WriteString("// AddEvent добавляет событие к агрегату и увеличивает версию\n")
	builder.WriteString("func (a *BaseAggregate) AddEvent(e Event) {\n")
	builder.WriteString("\tif a.events == nil {\n")
	builder.WriteString("\t\ta.events = make([]Event, 0)\n")
	builder.WriteString("\t}\n")
	builder.WriteString("\ta.events = append(a.events, e)\n")
	builder.WriteString("\ta.version++\n")
	builder.WriteString("}\n\n")

	builder.WriteString("// ClearEvents очищает список событий агрегата\n")
	builder.WriteString("func (a *BaseAggregate) ClearEvents() {\n")
	builder.WriteString("\ta.events = make([]Event, 0)\n")
	builder.WriteString("}\n")

	return builder.String()
}

// generateAggregate генерирует код для одного агрегата
func (g *DomainGenerator) generateAggregate(agg AggregateSpec) string {
	var builder strings.Builder

	// Struct агрегата
	builder.WriteString(fmt.Sprintf("// %s доменная сущность\n", agg.Name))
	builder.WriteString(fmt.Sprintf("type %s struct {\n", agg.Name))
	builder.WriteString("\t*BaseAggregate\n")

	// Приватные поля из proto
	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue // ID уже есть в BaseAggregate
		}
		goType := g.protoToGoType(field.Type)
		builder.WriteString(fmt.Sprintf("\t%s %s\n", g.toPrivateField(field.Name), goType))
	}

	builder.WriteString("\tcreatedAt time.Time\n")
	builder.WriteString("\tupdatedAt time.Time\n")
	builder.WriteString("}\n\n")

	// Constructor
	builder.WriteString(fmt.Sprintf("// New%s создает новый %s\n", agg.Name, strings.ToLower(agg.Name)))
	builder.WriteString(fmt.Sprintf("func New%s(", agg.Name))

	// Параметры конструктора
	var params []string
	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue
		}
		goType := g.protoToGoType(field.Type)
		params = append(params, fmt.Sprintf("%s %s", g.toPrivateField(field.Name), goType))
	}
	builder.WriteString(strings.Join(params, ", "))
	builder.WriteString(fmt.Sprintf(") *%s {\n", agg.Name))

	builder.WriteString(fmt.Sprintf("\t%s := &%s{\n", strings.ToLower(agg.Name), agg.Name))
	builder.WriteString("\t\tBaseAggregate: NewBaseAggregate(),\n")

	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue
		}
		builder.WriteString(fmt.Sprintf("\t\t%s: %s,\n", g.toPrivateField(field.Name), g.toPrivateField(field.Name)))
	}

	builder.WriteString("\t\tcreatedAt: time.Now(),\n")
	builder.WriteString("\t\tupdatedAt: time.Now(),\n")
	builder.WriteString("\t}\n\n")

	// Генерация события создания
	eventName := fmt.Sprintf("%sCreatedEvent", agg.Name)
	builder.WriteString(fmt.Sprintf("\t%s.AddEvent(%s{\n", strings.ToLower(agg.Name), eventName))
	builder.WriteString(fmt.Sprintf("\t\tBaseEvent: NewBaseEvent(\"%s.created\", %s.ID()),\n",
		g.converter.ToSnakeCase(agg.Name), strings.ToLower(agg.Name)))

	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue
		}
		builder.WriteString(fmt.Sprintf("\t\t%s: %s.%s(),\n",
			g.toPublicField(field.Name), strings.ToLower(agg.Name), g.toPublicField(field.Name)))
	}

	builder.WriteString("\t})\n\n")
	builder.WriteString(fmt.Sprintf("\treturn %s\n", strings.ToLower(agg.Name)))
	builder.WriteString("}\n\n")

	// NewXXXWithID
	builder.WriteString(fmt.Sprintf("// New%sWithID создает %s с указанным ID\n", agg.Name, strings.ToLower(agg.Name)))
	builder.WriteString(fmt.Sprintf("func New%sWithID(id string", agg.Name))
	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue
		}
		goType := g.protoToGoType(field.Type)
		builder.WriteString(fmt.Sprintf(", %s %s", g.toPrivateField(field.Name), goType))
	}
	builder.WriteString(fmt.Sprintf(") *%s {\n", agg.Name))
	builder.WriteString(fmt.Sprintf("\treturn &%s{\n", agg.Name))
	builder.WriteString("\t\tBaseAggregate: NewBaseAggregateWithID(id),\n")
	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue
		}
		builder.WriteString(fmt.Sprintf("\t\t%s: %s,\n", g.toPrivateField(field.Name), g.toPrivateField(field.Name)))
	}
	builder.WriteString("\t\tcreatedAt: time.Now(),\n")
	builder.WriteString("\t\tupdatedAt: time.Now(),\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n\n")

	// NewXXXWithState
	builder.WriteString(fmt.Sprintf("// New%sWithState создает %s с указанным ID и полным состоянием (для восстановления из БД)\n",
		agg.Name, strings.ToLower(agg.Name)))
	builder.WriteString(fmt.Sprintf("func New%sWithState(id string", agg.Name))
	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue
		}
		goType := g.protoToGoType(field.Type)
		builder.WriteString(fmt.Sprintf(", %s %s", g.toPrivateField(field.Name), goType))
	}
	builder.WriteString(", createdAt, updatedAt time.Time) *")
	builder.WriteString(fmt.Sprintf("%s {\n", agg.Name))
	builder.WriteString(fmt.Sprintf("\treturn &%s{\n", agg.Name))
	builder.WriteString("\t\tBaseAggregate: NewBaseAggregateWithID(id),\n")
	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue
		}
		builder.WriteString(fmt.Sprintf("\t\t%s: %s,\n", g.toPrivateField(field.Name), g.toPrivateField(field.Name)))
	}
	builder.WriteString("\t\tcreatedAt: createdAt,\n")
	builder.WriteString("\t\tupdatedAt: updatedAt,\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n\n")

	// Getters
	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue
		}
		builder.WriteString(fmt.Sprintf("func (%s *%s) %s() %s {\n",
			strings.ToLower(string(agg.Name[0])), agg.Name, g.toPublicField(field.Name), g.protoToGoType(field.Type)))
		builder.WriteString(fmt.Sprintf("\treturn %s.%s\n", strings.ToLower(string(agg.Name[0])), g.toPrivateField(field.Name)))
		builder.WriteString("}\n\n")
	}

	builder.WriteString(fmt.Sprintf("func (%s *%s) CreatedAt() time.Time {\n",
		strings.ToLower(string(agg.Name[0])), agg.Name))
	builder.WriteString(fmt.Sprintf("\treturn %s.createdAt\n", strings.ToLower(string(agg.Name[0]))))
	builder.WriteString("}\n\n")

	builder.WriteString(fmt.Sprintf("func (%s *%s) UpdatedAt() time.Time {\n",
		strings.ToLower(string(agg.Name[0])), agg.Name))
	builder.WriteString(fmt.Sprintf("\treturn %s.updatedAt\n", strings.ToLower(string(agg.Name[0]))))
	builder.WriteString("}\n\n")

	// Методы обновления (заглушки с маркерами)
	builder.WriteString(fmt.Sprintf("// Update%s обновляет %s\n", agg.Name, strings.ToLower(agg.Name)))
	builder.WriteString(fmt.Sprintf("func (%s *%s) Update%s(",
		strings.ToLower(string(agg.Name[0])), agg.Name, agg.Name))

	var updateParams []string
	for _, field := range agg.Fields {
		if field.Name == "id" {
			continue
		}
		goType := g.protoToGoType(field.Type)
		updateParams = append(updateParams, fmt.Sprintf("%s %s", g.toPrivateField(field.Name), goType))
	}
	builder.WriteString(strings.Join(updateParams, ", "))
	builder.WriteString(") {\n")

	builder.WriteString("// USER CODE BEGIN: Update")
	builder.WriteString(fmt.Sprintf("%s\n", agg.Name))
	builder.WriteString("// Add your update logic here\n")
	builder.WriteString("// USER CODE END: Update")
	builder.WriteString(fmt.Sprintf("%s\n", agg.Name))

	builder.WriteString(fmt.Sprintf("\t%s.updatedAt = time.Now()\n", strings.ToLower(string(agg.Name[0]))))
	builder.WriteString("}\n\n")

	return builder.String()
}

// generateEvents генерирует события
func (g *DomainGenerator) generateEvents(spec *ParsedSpec, config *GeneratorConfig) error {
	var content strings.Builder

	content.WriteString(g.addFileHeader("domain"))
	content.WriteString("import (\n")
	content.WriteString("\t\"time\"\n")
	content.WriteString("\n")
	content.WriteString("\t\"github.com/google/uuid\"\n")
	potterPath := ""
	if config != nil {
		potterPath = config.PotterImportPath
	}
	if potterPath == "" {
		potterPath = "github.com/akriventsev/potter"
	}
	// Удаляем @main или другие суффиксы версии для import-путей
	baseImportPath := strings.Split(potterPath, "@")[0]
	content.WriteString(fmt.Sprintf("\t\"%s/framework/events\"\n", baseImportPath))
	content.WriteString(fmt.Sprintf("\t\"%s/framework/invoke\"\n", baseImportPath))
	content.WriteString(")\n\n")

	// Базовые типы
	content.WriteString("// Event представляет доменное событие\n")
	content.WriteString("type Event interface {\n")
	content.WriteString("\tevents.Event\n")
	content.WriteString("}\n\n")

	content.WriteString("// BaseEvent базовая реализация события\n")
	content.WriteString("type BaseEvent struct {\n")
	content.WriteString("\teventID     string\n")
	content.WriteString("\teventType   string\n")
	content.WriteString("\toccurredAt  time.Time\n")
	content.WriteString("\taggregateID string\n")
	content.WriteString("\tmetadata    events.EventMetadata\n")
	content.WriteString("}\n\n")

	content.WriteString("func NewBaseEvent(eventType, aggregateID string) BaseEvent {\n")
	content.WriteString("\treturn BaseEvent{\n")
	content.WriteString("\t\teventID:     uuid.New().String(),\n")
	content.WriteString("\t\teventType:   eventType,\n")
	content.WriteString("\t\toccurredAt:  time.Now(),\n")
	content.WriteString("\t\taggregateID: aggregateID,\n")
	content.WriteString("\t\tmetadata:    make(events.EventMetadata),\n")
	content.WriteString("\t}\n")
	content.WriteString("}\n\n")

	content.WriteString("func (e BaseEvent) EventID() string {\n")
	content.WriteString("\treturn e.eventID\n")
	content.WriteString("}\n\n")

	content.WriteString("func (e BaseEvent) EventType() string {\n")
	content.WriteString("\treturn e.eventType\n")
	content.WriteString("}\n\n")

	content.WriteString("func (e BaseEvent) OccurredAt() time.Time {\n")
	content.WriteString("\treturn e.occurredAt\n")
	content.WriteString("}\n\n")

	content.WriteString("func (e BaseEvent) AggregateID() string {\n")
	content.WriteString("\treturn e.aggregateID\n")
	content.WriteString("}\n\n")

	content.WriteString("func (e BaseEvent) Metadata() events.EventMetadata {\n")
	content.WriteString("\tif e.metadata == nil {\n")
	content.WriteString("\t\treturn make(events.EventMetadata)\n")
	content.WriteString("\t}\n")
	content.WriteString("\treturn e.metadata\n")
	content.WriteString("}\n\n")

	// Генерация событий
	for _, event := range spec.Events {
		content.WriteString(g.generateEvent(event))
		content.WriteString("\n")
	}

	path := "domain/events.go"
	return g.writer.WriteFile(path, content.String())
}

// generateEvent генерирует код для одного события
func (g *DomainGenerator) generateEvent(event EventSpec) string {
	var builder strings.Builder

	if event.IsError {
		// Error event
		builder.WriteString(fmt.Sprintf("// %s событие об ошибке\n", event.Name))
		builder.WriteString(fmt.Sprintf("type %s struct {\n", event.Name))
		builder.WriteString("\t*invoke.BaseErrorEvent\n")

		for _, field := range event.Fields {
			if field.Name == "error_code" || field.Name == "retryable" {
				continue
			}
			goType := g.protoToGoType(field.Type)
			builder.WriteString(fmt.Sprintf("\t%s %s\n", g.toPublicField(field.Name), goType))
		}

		builder.WriteString("}\n\n")

		builder.WriteString(fmt.Sprintf("// New%s создает новое событие об ошибке\n", event.Name))
		builder.WriteString(fmt.Sprintf("func New%s(", event.Name))

		var params []string
		for _, field := range event.Fields {
			if field.Name == "error_code" || field.Name == "retryable" {
				continue
			}
			goType := g.protoToGoType(field.Type)
			params = append(params, fmt.Sprintf("%s %s", g.toPrivateField(field.Name), goType))
		}
		params = append(params, "err error")
		builder.WriteString(strings.Join(params, ", "))
		builder.WriteString(fmt.Sprintf(") *%s {\n", event.Name))

		builder.WriteString(fmt.Sprintf("\treturn &%s{\n", event.Name))
		builder.WriteString("\t\tBaseErrorEvent: invoke.NewBaseErrorEvent(\n")
		builder.WriteString(fmt.Sprintf("\t\t\t%q,\n", event.EventType))
		builder.WriteString("\t\t\t\"\",\n")
		builder.WriteString(fmt.Sprintf("\t\t\t%q,\n", event.ErrorCode))
		builder.WriteString("\t\t\t\"\",\n")
		builder.WriteString("\t\t\terr,\n")
		builder.WriteString(fmt.Sprintf("\t\t\t%v,\n", event.Retryable))
		builder.WriteString("\t\t),\n")

		for _, field := range event.Fields {
			if field.Name == "error_code" || field.Name == "retryable" {
				continue
			}
			builder.WriteString(fmt.Sprintf("\t\t%s: %s,\n",
				g.toPublicField(field.Name), g.toPrivateField(field.Name)))
		}

		builder.WriteString("\t}\n")
		builder.WriteString("}\n\n")
	} else {
		// Обычное событие
		builder.WriteString(fmt.Sprintf("// %s событие\n", event.Name))
		builder.WriteString(fmt.Sprintf("type %s struct {\n", event.Name))
		builder.WriteString("\tBaseEvent\n")

		for _, field := range event.Fields {
			goType := g.protoToGoType(field.Type)
			builder.WriteString(fmt.Sprintf("\t%s %s\n", g.toPublicField(field.Name), goType))
		}

		builder.WriteString("}\n\n")
	}

	return builder.String()
}

// generateRepositories генерирует интерфейсы репозиториев
func (g *DomainGenerator) generateRepositories(spec *ParsedSpec, _ *GeneratorConfig) error {
	var content strings.Builder

	content.WriteString(g.addFileHeader("domain"))
	content.WriteString("import \"context\"\n\n")

	// Генерация интерфейса для каждого агрегата
	for _, agg := range spec.Aggregates {
		content.WriteString(g.generateRepository(agg))
		content.WriteString("\n")
	}

	path := "domain/repository.go"
	return g.writer.WriteFile(path, content.String())
}

// generateRepository генерирует интерфейс репозитория
func (g *DomainGenerator) generateRepository(agg AggregateSpec) string {
	var builder strings.Builder

	repoName := fmt.Sprintf("%sRepository", agg.Name)
	builder.WriteString(fmt.Sprintf("// %s интерфейс репозитория %s\n", repoName, strings.ToLower(agg.Name)))
	builder.WriteString(fmt.Sprintf("type %s interface {\n", repoName))
	builder.WriteString(fmt.Sprintf("\tSave(ctx context.Context, %s *%s) error\n",
		strings.ToLower(agg.Name), agg.Name))
	builder.WriteString(fmt.Sprintf("\tFindByID(ctx context.Context, id string) (*%s, error)\n", agg.Name))
	builder.WriteString("\tDelete(ctx context.Context, id string) error\n")
	builder.WriteString("}\n")

	return builder.String()
}

// protoToGoType конвертирует proto тип в Go тип
func (g *DomainGenerator) protoToGoType(protoType string) string {
	switch protoType {
	case "string":
		return "string"
	case "int32":
		return "int32"
	case "int64":
		return "int64"
	case "bool":
		return "bool"
	case "float64":
		return "float64"
	case "float32":
		return "float32"
	default:
		return protoType
	}
}

// toPrivateField конвертирует имя поля в приватное
func (g *DomainGenerator) toPrivateField(name string) string {
	if len(name) == 0 {
		return name
	}
	return strings.ToLower(name[:1]) + name[1:]
}

// toPublicField конвертирует имя поля в публичное
func (g *DomainGenerator) toPublicField(name string) string {
	if len(name) == 0 {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
