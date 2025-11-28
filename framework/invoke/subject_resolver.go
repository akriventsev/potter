// Package invoke предоставляет интерфейсы и реализации для разрешения subjects команд и событий.
package invoke

import (
	"fmt"

	"potter/framework/transport"
)

// SubjectResolver интерфейс для динамического определения subjects команд и событий
type SubjectResolver interface {
	// ResolveCommandSubject определяет subject для команды
	ResolveCommandSubject(cmd transport.Command) string
	// ResolveEventSubject определяет subject для события по типу
	ResolveEventSubject(eventType string) string
}

// DefaultSubjectResolver реализация SubjectResolver с префиксами
type DefaultSubjectResolver struct {
	commandPrefix string
	eventPrefix   string
}

// NewDefaultSubjectResolver создает новый DefaultSubjectResolver
func NewDefaultSubjectResolver(commandPrefix, eventPrefix string) *DefaultSubjectResolver {
	return &DefaultSubjectResolver{
		commandPrefix: commandPrefix,
		eventPrefix:   eventPrefix,
	}
}

// ResolveCommandSubject формирует subject как {prefix}.{commandName}
func (r *DefaultSubjectResolver) ResolveCommandSubject(cmd transport.Command) string {
	return fmt.Sprintf("%s.%s", r.commandPrefix, cmd.CommandName())
}

// ResolveEventSubject формирует subject как {prefix}.{eventType}
func (r *DefaultSubjectResolver) ResolveEventSubject(eventType string) string {
	return fmt.Sprintf("%s.%s", r.eventPrefix, eventType)
}

// FunctionSubjectResolver реализация SubjectResolver с кастомными функциями
type FunctionSubjectResolver struct {
	commandFunc func(transport.Command) string
	eventFunc   func(string) string
}

// NewFunctionSubjectResolver создает новый FunctionSubjectResolver
func NewFunctionSubjectResolver(
	commandFunc func(transport.Command) string,
	eventFunc func(string) string,
) *FunctionSubjectResolver {
	return &FunctionSubjectResolver{
		commandFunc: commandFunc,
		eventFunc:   eventFunc,
	}
}

// ResolveCommandSubject вызывает кастомную функцию для команды
func (r *FunctionSubjectResolver) ResolveCommandSubject(cmd transport.Command) string {
	if r.commandFunc == nil {
		return ""
	}
	return r.commandFunc(cmd)
}

// ResolveEventSubject вызывает кастомную функцию для типа события
func (r *FunctionSubjectResolver) ResolveEventSubject(eventType string) string {
	if r.eventFunc == nil {
		return ""
	}
	return r.eventFunc(eventType)
}

// StaticSubjectResolver реализация SubjectResolver со статическим маппингом
type StaticSubjectResolver struct {
	commandSubjects map[string]string
	eventSubjects   map[string]string
	defaultPrefix   string
}

// NewStaticSubjectResolver создает новый StaticSubjectResolver
func NewStaticSubjectResolver(
	commandMap map[string]string,
	eventMap map[string]string,
) *StaticSubjectResolver {
	return &StaticSubjectResolver{
		commandSubjects: commandMap,
		eventSubjects:   eventMap,
	}
}

// ResolveCommandSubject возвращает subject из маппинга или пустую строку
func (r *StaticSubjectResolver) ResolveCommandSubject(cmd transport.Command) string {
	if subject, ok := r.commandSubjects[cmd.CommandName()]; ok {
		return subject
	}
	return ""
}

// ResolveEventSubject возвращает subject из маппинга или пустую строку
func (r *StaticSubjectResolver) ResolveEventSubject(eventType string) string {
	if subject, ok := r.eventSubjects[eventType]; ok {
		return subject
	}
	return ""
}

