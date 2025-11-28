// Package core предоставляет систему ошибок фреймворка.
package core

import (
	"fmt"
	"runtime"
	"strings"
)

// Коды ошибок фреймворка
const (
	ErrNotFound            = "NOT_FOUND"
	ErrAlreadyExists       = "ALREADY_EXISTS"
	ErrInvalidConfig       = "INVALID_CONFIG"
	ErrInitializationFailed = "INITIALIZATION_FAILED"
	ErrDependencyNotFound  = "DEPENDENCY_NOT_FOUND"
)

// FrameworkError базовый тип ошибки фреймворка
type FrameworkError struct {
	Code       string
	Message    string
	Cause      error
	StackTrace string
}

// Error реализует интерфейс error
func (e *FrameworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap возвращает причину ошибки
func (e *FrameworkError) Unwrap() error {
	return e.Cause
}

// Is проверяет, соответствует ли ошибка коду
func (e *FrameworkError) Is(target error) bool {
	if t, ok := target.(*FrameworkError); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext добавляет контекст к ошибке
func (e *FrameworkError) WithContext(context string) *FrameworkError {
	return &FrameworkError{
		Code:       e.Code,
		Message:    fmt.Sprintf("%s: %s", context, e.Message),
		Cause:      e.Cause,
		StackTrace: e.StackTrace,
	}
}

// NewError создает новую ошибку фреймворка
func NewError(code, message string) *FrameworkError {
	return &FrameworkError{
		Code:       code,
		Message:    message,
		StackTrace: captureStackTrace(),
	}
}

// Wrap оборачивает существующую ошибку
func Wrap(err error, code, message string) *FrameworkError {
	if err == nil {
		return nil
	}
	return &FrameworkError{
		Code:       code,
		Message:    message,
		Cause:      err,
		StackTrace: captureStackTrace(),
	}
}

// WrapWithCode оборачивает ошибку с кодом
func WrapWithCode(err error, code string) *FrameworkError {
	if err == nil {
		return nil
	}
	return &FrameworkError{
		Code:       code,
		Message:    err.Error(),
		Cause:      err,
		StackTrace: captureStackTrace(),
	}
}

// captureStackTrace захватывает stack trace
func captureStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])
	
	// Убираем первые несколько строк (сама функция captureStackTrace)
	lines := strings.Split(stack, "\n")
	if len(lines) > 4 {
		lines = lines[4:]
	}
	return strings.Join(lines, "\n")
}

