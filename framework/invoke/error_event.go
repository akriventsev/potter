// Package invoke предоставляет интерфейс и базовую реализацию для событий об ошибках.
package invoke

import (
	"fmt"

	"github.com/akriventsev/potter/framework/events"
)

// ErrorEvent интерфейс для событий об ошибках, расширяющий events.Event
type ErrorEvent interface {
	events.Event
	// Error возвращает ошибку, связанную с событием
	Error() error
	// ErrorCode возвращает код ошибки
	ErrorCode() string
	// ErrorMessage возвращает сообщение об ошибке
	ErrorMessage() string
	// IsRetryable указывает, можно ли повторить операцию
	IsRetryable() bool
	// OriginalCommand возвращает исходную команду, вызвавшую ошибку
	OriginalCommand() interface{}
}

// BaseErrorEvent базовая реализация ErrorEvent
type BaseErrorEvent struct {
	*events.BaseEvent
	err            error
	errorCode      string
	errorMessage   string
	retryable      bool
	originalCommand interface{}
}

// NewBaseErrorEvent создает новое базовое событие об ошибке
func NewBaseErrorEvent(
	eventType, aggregateID, errorCode, errorMessage string,
	err error,
	retryable bool,
) *BaseErrorEvent {
	return &BaseErrorEvent{
		BaseEvent:      events.NewBaseEvent(eventType, aggregateID),
		err:            err,
		errorCode:      errorCode,
		errorMessage:   errorMessage,
		retryable:      retryable,
		originalCommand: nil,
	}
}

// Error возвращает ошибку, связанную с событием
func (e *BaseErrorEvent) Error() error {
	return e.err
}

// ErrorCode возвращает код ошибки
func (e *BaseErrorEvent) ErrorCode() string {
	return e.errorCode
}

// ErrorMessage возвращает сообщение об ошибке
func (e *BaseErrorEvent) ErrorMessage() string {
	return e.errorMessage
}

// IsRetryable указывает, можно ли повторить операцию
func (e *BaseErrorEvent) IsRetryable() bool {
	return e.retryable
}

// OriginalCommand возвращает исходную команду, вызвавшую ошибку
func (e *BaseErrorEvent) OriginalCommand() interface{} {
	return e.originalCommand
}

// WithOriginalCommand устанавливает исходную команду
func (e *BaseErrorEvent) WithOriginalCommand(cmd interface{}) *BaseErrorEvent {
	e.originalCommand = cmd
	return e
}

// WithRetryable устанавливает флаг возможности повтора
func (e *BaseErrorEvent) WithRetryable(retryable bool) *BaseErrorEvent {
	e.retryable = retryable
	return e
}

// WithError устанавливает ошибку
func (e *BaseErrorEvent) WithError(err error) *BaseErrorEvent {
	e.err = err
	if err != nil && e.errorMessage == "" {
		e.errorMessage = err.Error()
	}
	return e
}

// WithErrorCode устанавливает код ошибки
func (e *BaseErrorEvent) WithErrorCode(code string) *BaseErrorEvent {
	e.errorCode = code
	return e
}

// WithErrorMessage устанавливает сообщение об ошибке
func (e *BaseErrorEvent) WithErrorMessage(message string) *BaseErrorEvent {
	e.errorMessage = message
	return e
}

// String возвращает строковое представление события об ошибке
func (e *BaseErrorEvent) String() string {
	if e.err != nil {
		return fmt.Sprintf("ErrorEvent[%s]: %s (%s)", e.errorCode, e.errorMessage, e.err.Error())
	}
	return fmt.Sprintf("ErrorEvent[%s]: %s", e.errorCode, e.errorMessage)
}

