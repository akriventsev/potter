// Package invoke предоставляет систему ошибок для модуля Invoke.
package invoke

import (
	"potter/framework/core"
)

// Коды ошибок модуля Invoke
const (
	ErrEventTimeout            = "EVENT_TIMEOUT"
	ErrInvalidResultType       = "INVALID_RESULT_TYPE"
	ErrCommandPublishFailed     = "COMMAND_PUBLISH_FAILED"
	ErrValidationFailed        = "VALIDATION_FAILED"
	ErrQueryTimeout            = "QUERY_TIMEOUT"
	ErrCorrelationIDNotFound   = "CORRELATION_ID_NOT_FOUND"
	ErrEventAwaiterStopped     = "EVENT_AWAITER_STOPPED"
	ErrInvalidSubjectResolver  = "INVALID_SUBJECT_RESOLVER"
	ErrEventSourceNotConfigured = "EVENT_SOURCE_NOT_CONFIGURED"
	ErrErrorEventReceived      = "ERROR_EVENT_RECEIVED"
)

// NewEventTimeoutError создает ошибку таймаута ожидания события
func NewEventTimeoutError(correlationID string, timeout string) *core.FrameworkError {
	return core.NewError(
		ErrEventTimeout,
		"event timeout: correlation_id="+correlationID+", timeout="+timeout,
	)
}

// NewInvalidResultTypeError создает ошибку неверного типа результата
func NewInvalidResultTypeError(expected, actual string) *core.FrameworkError {
	return core.NewError(
		ErrInvalidResultType,
		"invalid result type: expected="+expected+", actual="+actual,
	)
}

// NewCommandPublishFailedError создает ошибку публикации команды
func NewCommandPublishFailedError(commandName string, cause error) *core.FrameworkError {
	return core.Wrap(
		cause,
		ErrCommandPublishFailed,
		"failed to publish command: "+commandName,
	)
}

// NewValidationFailedError создает ошибку валидации
func NewValidationFailedError(cause error) *core.FrameworkError {
	return core.Wrap(
		cause,
		ErrValidationFailed,
		"validation failed",
	)
}

// NewQueryTimeoutError создает ошибку таймаута запроса
func NewQueryTimeoutError(queryName string, timeout string) *core.FrameworkError {
	return core.NewError(
		ErrQueryTimeout,
		"query timeout: query="+queryName+", timeout="+timeout,
	)
}

// NewCorrelationIDNotFoundError создает ошибку отсутствия correlation ID
func NewCorrelationIDNotFoundError() *core.FrameworkError {
	return core.NewError(
		ErrCorrelationIDNotFound,
		"correlation ID not found in context",
	)
}

// NewEventAwaiterStoppedError создает ошибку остановленного EventAwaiter
func NewEventAwaiterStoppedError() *core.FrameworkError {
	return core.NewError(
		ErrEventAwaiterStopped,
		"event awaiter is stopped",
	)
}

// NewInvalidSubjectResolverError создает ошибку для некорректного SubjectResolver
func NewInvalidSubjectResolverError(reason string) *core.FrameworkError {
	return core.NewError(
		ErrInvalidSubjectResolver,
		"invalid subject resolver: "+reason,
	)
}

// NewEventSourceNotConfiguredError создает ошибку для отсутствующего источника событий
func NewEventSourceNotConfiguredError() *core.FrameworkError {
	return core.NewError(
		ErrEventSourceNotConfigured,
		"event source is not configured",
	)
}

// NewErrorEventReceivedError создает ошибку-обертку для полученного ошибочного события
func NewErrorEventReceivedError(errorEvent ErrorEvent) *core.FrameworkError {
	var cause error
	var errorMessage string
	
	if errorEvent == nil {
		errorMessage = "unknown error event"
	} else {
		if errorEvent.Error() != nil {
			cause = errorEvent.Error()
		}
		errorMessage = errorEvent.ErrorMessage()
	}
	
	if cause == nil {
		return core.NewError(
			ErrErrorEventReceived,
			"error event received: "+errorMessage,
		)
	}
	
	return core.Wrap(
		cause,
		ErrErrorEventReceived,
		"error event received: "+errorMessage,
	)
}

