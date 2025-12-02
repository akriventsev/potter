package codegen

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ProtoParser парсер protobuf файлов с извлечением Potter custom options
type ProtoParser struct{}

// NewProtoParser создает новый парсер
func NewProtoParser() *ProtoParser {
	return &ProtoParser{}
}

// ParsedSpec структурированное представление proto файла с Potter аннотациями
type ParsedSpec struct {
	Services   []ServiceSpec
	Aggregates []AggregateSpec
	Events     []EventSpec
	Commands   []CommandSpec
	Queries    []QuerySpec
	ModuleName string
	Transports []string
}

// ServiceSpec спецификация сервиса
type ServiceSpec struct {
	Name       string
	ModuleName string
	Transports []string
	Methods    []MethodSpec
}

// MethodSpec спецификация метода RPC
type MethodSpec struct {
	Name           string
	RequestType    string
	ResponseType   string
	CommandOptions *CommandOptions
	QueryOptions   *QueryOptions
}

// CommandSpec спецификация команды
type CommandSpec struct {
	Name           string
	Aggregate      string
	RequestType    string
	ResponseType   string
	RequestFields  []FieldSpec // Поля из Request сообщения
	ResponseFields []FieldSpec // Поля из Response сообщения
	Async          bool
	Idempotent     bool
	TimeoutSeconds int32
}

// QuerySpec спецификация запроса
type QuerySpec struct {
	Name            string
	RequestType     string
	ResponseType    string
	RequestFields   []FieldSpec // Поля из Request сообщения
	ResponseFields  []FieldSpec // Поля из Response сообщения
	Cacheable       bool
	CacheTTLSeconds int32
	ReadModel       string
}

// EventSpec спецификация события
type EventSpec struct {
	Name      string
	EventType string
	Aggregate string
	Version   int32
	Fields    []FieldSpec
	IsError   bool
	ErrorCode string
	Retryable bool
}

// AggregateSpec спецификация агрегата
type AggregateSpec struct {
	Name       string
	Repository string
	Fields     []FieldSpec
}

// FieldSpec спецификация поля
type FieldSpec struct {
	Name     string
	Type     string
	Number   int32
	Repeated bool
	Optional bool
}

// MessageSpec спецификация сообщения
type MessageSpec struct {
	Name   string
	Fields []FieldSpec
}

// CommandOptions опции команды
type CommandOptions struct {
	Aggregate      string
	Async          bool
	Idempotent     bool
	TimeoutSeconds int32
}

// QueryOptions опции запроса
type QueryOptions struct {
	Cacheable       bool
	CacheTTLSeconds int32
	ReadModel       string
}

// EventOptions опции события
type EventOptions struct {
	EventType string
	Aggregate string
	Version   int32
}

// AggregateOptions опции агрегата
type AggregateOptions struct {
	Name       string
	Repository string
}

// ErrorEventOptions опции события об ошибке
type ErrorEventOptions struct {
	ErrorCode string
	Retryable bool
}

// ParseProtogenFile парсит protogen.File и извлекает Potter аннотации
func (p *ProtoParser) ParseProtogenFile(file *protogen.File) (*ParsedSpec, error) {
	// Используем file.Proto, который уже является *descriptorpb.FileDescriptorProto
	return p.ParseProtoFile(file.Proto)
}

// ParseProtoFile парсит proto файл и извлекает Potter аннотации
func (p *ProtoParser) ParseProtoFile(file *descriptorpb.FileDescriptorProto) (*ParsedSpec, error) {
	spec := &ParsedSpec{
		Services:   []ServiceSpec{},
		Aggregates: []AggregateSpec{},
		Events:     []EventSpec{},
		Commands:   []CommandSpec{},
		Queries:    []QuerySpec{},
	}

	// Парсинг сообщений для поиска агрегатов и событий
	messageMap := make(map[string]*MessageSpec)
	for _, msg := range file.MessageType {
		msgSpec := p.parseMessage(msg)
		messageMap[msgSpec.Name] = msgSpec

		// Проверка на агрегат
		if aggOpts := p.extractAggregateOptions(msg); aggOpts != nil {
			spec.Aggregates = append(spec.Aggregates, AggregateSpec{
				Name:       aggOpts.Name,
				Repository: aggOpts.Repository,
				Fields:     msgSpec.Fields,
			})
		}

		// Проверка на событие
		if eventOpts := p.extractEventOptions(msg); eventOpts != nil {
			spec.Events = append(spec.Events, EventSpec{
				Name:      msgSpec.Name,
				EventType: eventOpts.EventType,
				Aggregate: eventOpts.Aggregate,
				Version:   eventOpts.Version,
				Fields:    msgSpec.Fields,
				IsError:   false,
			})
		}

		// Проверка на error event
		if errorOpts := p.extractErrorEventOptions(msg); errorOpts != nil {
			spec.Events = append(spec.Events, EventSpec{
				Name:      msgSpec.Name,
				EventType: fmt.Sprintf("%s.failed", p.toSnakeCase(msgSpec.Name)),
				Aggregate: "",
				Version:   1,
				Fields:    msgSpec.Fields,
				IsError:   true,
				ErrorCode: errorOpts.ErrorCode,
				Retryable: errorOpts.Retryable,
			})
		}
	}

	// Парсинг сервисов
	for _, svc := range file.Service {
		serviceSpec := ServiceSpec{
			Name:    *svc.Name,
			Methods: []MethodSpec{},
		}

		// Извлечение опций сервиса
		if svcOpts := p.extractServiceOptions(svc); svcOpts != nil {
			serviceSpec.ModuleName = svcOpts.ModuleName
			serviceSpec.Transports = svcOpts.Transports
			spec.ModuleName = svcOpts.ModuleName
			spec.Transports = svcOpts.Transports
		}

		// Парсинг методов
		for _, method := range svc.Method {
			methodSpec := MethodSpec{
				Name:         *method.Name,
				RequestType:  p.resolveTypeName(*method.InputType),
				ResponseType: p.resolveTypeName(*method.OutputType),
			}

			// Извлечение опций команды
			if cmdOpts := p.extractCommandOptions(method); cmdOpts != nil {
				methodSpec.CommandOptions = cmdOpts
				// Получаем поля из Request и Response сообщений
				requestFields := []FieldSpec{}
				responseFields := []FieldSpec{}
				if reqMsg, ok := messageMap[methodSpec.RequestType]; ok {
					requestFields = reqMsg.Fields
				}
				if respMsg, ok := messageMap[methodSpec.ResponseType]; ok {
					responseFields = respMsg.Fields
				}
				spec.Commands = append(spec.Commands, CommandSpec{
					Name:           *method.Name,
					Aggregate:      cmdOpts.Aggregate,
					RequestType:    methodSpec.RequestType,
					ResponseType:   methodSpec.ResponseType,
					RequestFields:  requestFields,
					ResponseFields: responseFields,
					Async:          cmdOpts.Async,
					Idempotent:     cmdOpts.Idempotent,
					TimeoutSeconds: cmdOpts.TimeoutSeconds,
				})
			}

			// Извлечение опций запроса
			if queryOpts := p.extractQueryOptions(method); queryOpts != nil {
				methodSpec.QueryOptions = queryOpts
				// Получаем поля из Request и Response сообщений
				requestFields := []FieldSpec{}
				responseFields := []FieldSpec{}
				if reqMsg, ok := messageMap[methodSpec.RequestType]; ok {
					requestFields = reqMsg.Fields
				}
				if respMsg, ok := messageMap[methodSpec.ResponseType]; ok {
					responseFields = respMsg.Fields
				}
				spec.Queries = append(spec.Queries, QuerySpec{
					Name:            *method.Name,
					RequestType:     methodSpec.RequestType,
					ResponseType:    methodSpec.ResponseType,
					RequestFields:   requestFields,
					ResponseFields:  responseFields,
					Cacheable:       queryOpts.Cacheable,
					CacheTTLSeconds: queryOpts.CacheTTLSeconds,
					ReadModel:       queryOpts.ReadModel,
				})
			}

			serviceSpec.Methods = append(serviceSpec.Methods, methodSpec)
		}

		spec.Services = append(spec.Services, serviceSpec)
	}

	return spec, nil
}

// parseMessage парсит message в MessageSpec
func (p *ProtoParser) parseMessage(msg *descriptorpb.DescriptorProto) *MessageSpec {
	spec := &MessageSpec{
		Name:   *msg.Name,
		Fields: []FieldSpec{},
	}

	for _, field := range msg.Field {
		spec.Fields = append(spec.Fields, FieldSpec{
			Name:     *field.Name,
			Type:     p.resolveFieldType(field),
			Number:   *field.Number,
			Repeated: field.Label != nil && *field.Label == descriptorpb.FieldDescriptorProto_LABEL_REPEATED,
			Optional: field.Label != nil && *field.Label == descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL,
		})
	}

	return spec
}

// resolveFieldType резолвит тип поля
func (p *ProtoParser) resolveFieldType(field *descriptorpb.FieldDescriptorProto) string {
	if field.Type == nil {
		return "string"
	}

	switch *field.Type {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return "string"
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		return "int32"
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		return "int64"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool"
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "float64"
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "float32"
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "[]byte"
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return p.resolveTypeName(*field.TypeName)
	default:
		return "string"
	}
}

// resolveTypeName резолвит имя типа из полного пути
func (p *ProtoParser) resolveTypeName(typeName string) string {
	// Убираем префикс "." и возвращаем только имя типа
	if len(typeName) > 0 && typeName[0] == '.' {
		typeName = typeName[1:]
	}
	// Берем последнюю часть после точки
	for i := len(typeName) - 1; i >= 0; i-- {
		if typeName[i] == '.' {
			return typeName[i+1:]
		}
	}
	return typeName
}

// extractCommandOptions извлекает potter.command опции (extension номер 50001)
func (p *ProtoParser) extractCommandOptions(method *descriptorpb.MethodDescriptorProto) *CommandOptions {
	if method.Options == nil {
		return nil
	}

	optsReflect := method.Options.ProtoReflect()
	unknownFields := optsReflect.GetUnknown()

	// Парсим UnknownFields для поиска extension с номером 50001
	extData := p.findExtensionInUnknownFields(unknownFields, 50001)
	if extData == nil {
		return nil
	}

	// Парсим extension message (CommandOptions)
	return p.parseCommandOptions(extData)
}

// findExtensionInUnknownFields ищет extension в UnknownFields по номеру поля
func (p *ProtoParser) findExtensionInUnknownFields(unknownFields []byte, fieldNum int) []byte {
	for len(unknownFields) > 0 {
		tag, wireType, n := protowire.ConsumeTag(unknownFields)
		if n < 0 {
			break
		}
		unknownFields = unknownFields[n:]

		if int(tag) == fieldNum && wireType == protowire.BytesType {
			data, m := protowire.ConsumeBytes(unknownFields)
			if m < 0 {
				break
			}
			return data
		}

		// Пропускаем поле
		m := protowire.ConsumeFieldValue(tag, wireType, unknownFields)
		if m < 0 {
			break
		}
		unknownFields = unknownFields[m:]
	}
	return nil
}

// parseCommandOptions парсит CommandOptions из байтов
func (p *ProtoParser) parseCommandOptions(data []byte) *CommandOptions {
	opts := &CommandOptions{}

	// Парсим поля CommandOptions
	for len(data) > 0 {
		tag, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]

		switch int(tag) {
		case 1: // aggregate (string)
			if wireType == protowire.BytesType {
				val, m := protowire.ConsumeBytes(data)
				if m >= 0 {
					opts.Aggregate = string(val)
					data = data[m:]
				}
			}
		case 2: // async (bool)
			if wireType == protowire.VarintType {
				val, m := protowire.ConsumeVarint(data)
				if m >= 0 {
					opts.Async = val != 0
					data = data[m:]
				}
			}
		case 3: // idempotent (bool)
			if wireType == protowire.VarintType {
				val, m := protowire.ConsumeVarint(data)
				if m >= 0 {
					opts.Idempotent = val != 0
					data = data[m:]
				}
			}
		case 4: // timeout_seconds (int32)
			if wireType == protowire.VarintType {
				val, m := protowire.ConsumeVarint(data)
				if m >= 0 {
					opts.TimeoutSeconds = int32(val)
					data = data[m:]
				}
			}
		default:
			// Пропускаем неизвестное поле
			m := protowire.ConsumeFieldValue(tag, wireType, data)
			if m < 0 {
				return opts
			}
			data = data[m:]
		}
	}

	return opts
}

// extractQueryOptions извлекает potter.query опции (extension номер 50002)
func (p *ProtoParser) extractQueryOptions(method *descriptorpb.MethodDescriptorProto) *QueryOptions {
	if method.Options == nil {
		return nil
	}

	optsReflect := method.Options.ProtoReflect()
	unknownFields := optsReflect.GetUnknown()

	extData := p.findExtensionInUnknownFields(unknownFields, 50002)
	if extData == nil {
		return nil
	}

	return p.parseQueryOptions(extData)
}

// parseQueryOptions парсит QueryOptions из байтов
func (p *ProtoParser) parseQueryOptions(data []byte) *QueryOptions {
	opts := &QueryOptions{}

	for len(data) > 0 {
		tag, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]

		switch int(tag) {
		case 1: // cacheable (bool)
			if wireType == protowire.VarintType {
				val, m := protowire.ConsumeVarint(data)
				if m >= 0 {
					opts.Cacheable = val != 0
					data = data[m:]
				}
			}
		case 2: // cache_ttl_seconds (int32)
			if wireType == protowire.VarintType {
				val, m := protowire.ConsumeVarint(data)
				if m >= 0 {
					opts.CacheTTLSeconds = int32(val)
					data = data[m:]
				}
			}
		case 3: // read_model (string)
			if wireType == protowire.BytesType {
				val, m := protowire.ConsumeBytes(data)
				if m >= 0 {
					opts.ReadModel = string(val)
					data = data[m:]
				}
			}
		default:
			m := protowire.ConsumeFieldValue(tag, wireType, data)
			if m < 0 {
				return opts
			}
			data = data[m:]
		}
	}

	return opts
}

// extractEventOptions извлекает potter.event опции (extension номер 50001 для MessageOptions)
func (p *ProtoParser) extractEventOptions(msg *descriptorpb.DescriptorProto) *EventOptions {
	if msg.Options == nil {
		return nil
	}

	optsReflect := msg.Options.ProtoReflect()
	unknownFields := optsReflect.GetUnknown()

	extData := p.findExtensionInUnknownFields(unknownFields, 50001)
	if extData == nil {
		return nil
	}

	return p.parseEventOptions(extData)
}

// parseEventOptions парсит EventOptions из байтов
func (p *ProtoParser) parseEventOptions(data []byte) *EventOptions {
	opts := &EventOptions{}

	for len(data) > 0 {
		tag, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]

		switch int(tag) {
		case 1: // event_type (string)
			if wireType == protowire.BytesType {
				val, m := protowire.ConsumeBytes(data)
				if m >= 0 {
					opts.EventType = string(val)
					data = data[m:]
				}
			}
		case 2: // aggregate (string)
			if wireType == protowire.BytesType {
				val, m := protowire.ConsumeBytes(data)
				if m >= 0 {
					opts.Aggregate = string(val)
					data = data[m:]
				}
			}
		case 3: // version (int32)
			if wireType == protowire.VarintType {
				val, m := protowire.ConsumeVarint(data)
				if m >= 0 {
					opts.Version = int32(val)
					data = data[m:]
				}
			}
		default:
			m := protowire.ConsumeFieldValue(tag, wireType, data)
			if m < 0 {
				return opts
			}
			data = data[m:]
		}
	}

	return opts
}

// extractAggregateOptions извлекает potter.aggregate опции (extension номер 50002 для MessageOptions)
func (p *ProtoParser) extractAggregateOptions(msg *descriptorpb.DescriptorProto) *AggregateOptions {
	if msg.Options == nil {
		return nil
	}

	optsReflect := msg.Options.ProtoReflect()
	unknownFields := optsReflect.GetUnknown()

	extData := p.findExtensionInUnknownFields(unknownFields, 50002)
	if extData == nil {
		return nil
	}

	return p.parseAggregateOptions(extData)
}

// parseAggregateOptions парсит AggregateOptions из байтов
func (p *ProtoParser) parseAggregateOptions(data []byte) *AggregateOptions {
	opts := &AggregateOptions{}

	for len(data) > 0 {
		tag, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]

		switch int(tag) {
		case 1: // name (string)
			if wireType == protowire.BytesType {
				val, m := protowire.ConsumeBytes(data)
				if m >= 0 {
					opts.Name = string(val)
					data = data[m:]
				}
			}
		case 2: // repository (string)
			if wireType == protowire.BytesType {
				val, m := protowire.ConsumeBytes(data)
				if m >= 0 {
					opts.Repository = string(val)
					data = data[m:]
				}
			}
		default:
			m := protowire.ConsumeFieldValue(tag, wireType, data)
			if m < 0 {
				return opts
			}
			data = data[m:]
		}
	}

	return opts
}

// extractErrorEventOptions извлекает potter.error_event опции (extension номер 50004 для MessageOptions)
func (p *ProtoParser) extractErrorEventOptions(msg *descriptorpb.DescriptorProto) *ErrorEventOptions {
	if msg.Options == nil {
		return nil
	}

	optsReflect := msg.Options.ProtoReflect()
	unknownFields := optsReflect.GetUnknown()

	extData := p.findExtensionInUnknownFields(unknownFields, 50004)
	if extData == nil {
		return nil
	}

	return p.parseErrorEventOptions(extData)
}

// parseErrorEventOptions парсит ErrorEventOptions из байтов
func (p *ProtoParser) parseErrorEventOptions(data []byte) *ErrorEventOptions {
	opts := &ErrorEventOptions{}

	for len(data) > 0 {
		tag, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]

		switch int(tag) {
		case 1: // error_code (string)
			if wireType == protowire.BytesType {
				val, m := protowire.ConsumeBytes(data)
				if m >= 0 {
					opts.ErrorCode = string(val)
					data = data[m:]
				}
			}
		case 2: // retryable (bool)
			if wireType == protowire.VarintType {
				val, m := protowire.ConsumeVarint(data)
				if m >= 0 {
					opts.Retryable = val != 0
					data = data[m:]
				}
			}
		default:
			m := protowire.ConsumeFieldValue(tag, wireType, data)
			if m < 0 {
				return opts
			}
			data = data[m:]
		}
	}

	return opts
}

// extractServiceOptions извлекает potter.service опции (extension номер 50001 для ServiceOptions)
func (p *ProtoParser) extractServiceOptions(svc *descriptorpb.ServiceDescriptorProto) *ServiceOptions {
	if svc.Options == nil {
		return nil
	}

	optsReflect := svc.Options.ProtoReflect()
	unknownFields := optsReflect.GetUnknown()

	extData := p.findExtensionInUnknownFields(unknownFields, 50001)
	if extData == nil {
		return nil
	}

	return p.parseServiceOptions(extData)
}

// parseServiceOptions парсит ServiceOptions из байтов
func (p *ProtoParser) parseServiceOptions(data []byte) *ServiceOptions {
	opts := &ServiceOptions{}

	for len(data) > 0 {
		tag, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]

		switch int(tag) {
		case 1: // module_name (string)
			if wireType == protowire.BytesType {
				val, m := protowire.ConsumeBytes(data)
				if m >= 0 {
					opts.ModuleName = string(val)
					data = data[m:]
				}
			}
		case 2: // transport (repeated string)
			if wireType == protowire.BytesType {
				val, m := protowire.ConsumeBytes(data)
				if m >= 0 {
					opts.Transports = append(opts.Transports, string(val))
					data = data[m:]
				}
			}
		default:
			m := protowire.ConsumeFieldValue(tag, wireType, data)
			if m < 0 {
				return opts
			}
			data = data[m:]
		}
	}

	return opts
}

// ServiceOptions опции сервиса
type ServiceOptions struct {
	ModuleName string
	Transports []string
}

// toSnakeCase конвертирует CamelCase в snake_case
func (p *ProtoParser) toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return string(result)
}
