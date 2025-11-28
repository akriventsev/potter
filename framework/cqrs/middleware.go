// Package cqrs предоставляет middleware для обработчиков команд и запросов.
package cqrs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"potter/framework/transport"
)

// LoggingCommandMiddleware логирует выполнение команд
func LoggingCommandMiddleware(logger interface{ Log(string, ...interface{}) }) CommandMiddleware {
	return func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error {
		start := time.Now()
		logger.Log("Executing command: %s", cmd.CommandName())

		err := next(ctx, cmd)

		duration := time.Since(start)
		if err != nil {
			logger.Log("Command %s failed after %v: %v", cmd.CommandName(), duration, err)
		} else {
			logger.Log("Command %s completed in %v", cmd.CommandName(), duration)
		}

		return err
	}
}

// DefaultLoggingCommandMiddleware использует стандартный log
func DefaultLoggingCommandMiddleware() CommandMiddleware {
	logger := &defaultLogger{}
	return LoggingCommandMiddleware(logger)
}

// LoggingQueryMiddleware логирует выполнение запросов
func LoggingQueryMiddleware(logger interface{ Log(string, ...interface{}) }) QueryMiddleware {
	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		start := time.Now()
		logger.Log("Executing query: %s", q.QueryName())

		result, err := next(ctx, q)

		duration := time.Since(start)
		if err != nil {
			logger.Log("Query %s failed after %v: %v", q.QueryName(), duration, err)
		} else {
			logger.Log("Query %s completed in %v", q.QueryName(), duration)
		}

		return result, err
	}
}

// DefaultLoggingQueryMiddleware использует стандартный log
func DefaultLoggingQueryMiddleware() QueryMiddleware {
	logger := &defaultLogger{}
	return LoggingQueryMiddleware(logger)
}

// defaultLogger простая реализация logger
type defaultLogger struct{}

func (l *defaultLogger) Log(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// ValidationCommandMiddleware валидирует команду перед выполнением
func ValidationCommandMiddleware(validator func(ctx context.Context, cmd transport.Command) error) CommandMiddleware {
	return func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error {
		if err := validator(ctx, cmd); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		return next(ctx, cmd)
	}
}

// ValidationQueryMiddleware валидирует запрос перед выполнением
func ValidationQueryMiddleware(validator func(ctx context.Context, q transport.Query) error) QueryMiddleware {
	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		if err := validator(ctx, q); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		return next(ctx, q)
	}
}

// RecoveryCommandMiddleware восстанавливает панику в обработчиках команд
func RecoveryCommandMiddleware() CommandMiddleware {
	return func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic recovered: %v", r)
			}
		}()
		return next(ctx, cmd)
	}
}

// RecoveryQueryMiddleware восстанавливает панику в обработчиках запросов
func RecoveryQueryMiddleware() QueryMiddleware {
	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (result interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic recovered: %v", r)
			}
		}()
		return next(ctx, q)
	}
}

// TimeoutCommandMiddleware добавляет timeout к выполнению команды
func TimeoutCommandMiddleware(timeout time.Duration) CommandMiddleware {
	return func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return next(ctx, cmd)
	}
}

// TimeoutQueryMiddleware добавляет timeout к выполнению запроса
func TimeoutQueryMiddleware(timeout time.Duration) QueryMiddleware {
	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return next(ctx, q)
	}
}

// RetryCommandMiddleware добавляет повторы с exponential backoff
func RetryCommandMiddleware(maxAttempts int, initialDelay, maxDelay time.Duration) CommandMiddleware {
	return func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error {
		var lastErr error
		delay := initialDelay

		for attempt := 0; attempt < maxAttempts; attempt++ {
			if err := next(ctx, cmd); err == nil {
				return nil
			} else {
				lastErr = err
				if attempt < maxAttempts-1 {
					time.Sleep(delay)
					delay = time.Duration(float64(delay) * 1.5)
					if delay > maxDelay {
						delay = maxDelay
					}
				}
			}
		}
		return lastErr
	}
}

// RetryQueryMiddleware добавляет повторы с exponential backoff
func RetryQueryMiddleware(maxAttempts int, initialDelay, maxDelay time.Duration) QueryMiddleware {
	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		var lastErr error
		delay := initialDelay

		for attempt := 0; attempt < maxAttempts; attempt++ {
			if result, err := next(ctx, q); err == nil {
				return result, nil
			} else {
				lastErr = err
				if attempt < maxAttempts-1 {
					time.Sleep(delay)
					delay = time.Duration(float64(delay) * 1.5)
					if delay > maxDelay {
						delay = maxDelay
					}
				}
			}
		}
		return nil, lastErr
	}
}

// CircuitBreakerCommandMiddleware добавляет circuit breaker для защиты от сбоев
func CircuitBreakerCommandMiddleware(failureThreshold int, timeout time.Duration) CommandMiddleware {
	failures := 0
	var lastFailure time.Time
	var mu sync.Mutex

	return func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error {
		mu.Lock()
		if failures >= failureThreshold {
			if time.Since(lastFailure) < timeout {
				mu.Unlock()
				return fmt.Errorf("circuit breaker is open")
			}
			failures = 0
		}
		mu.Unlock()

		err := next(ctx, cmd)
		if err != nil {
			mu.Lock()
			failures++
			lastFailure = time.Now()
			mu.Unlock()
		} else {
			mu.Lock()
			failures = 0
			mu.Unlock()
		}
		return err
	}
}

// CircuitBreakerQueryMiddleware добавляет circuit breaker для защиты от сбоев
func CircuitBreakerQueryMiddleware(failureThreshold int, timeout time.Duration) QueryMiddleware {
	failures := 0
	var lastFailure time.Time
	var mu sync.Mutex

	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		mu.Lock()
		if failures >= failureThreshold {
			if time.Since(lastFailure) < timeout {
				mu.Unlock()
				return nil, fmt.Errorf("circuit breaker is open")
			}
			failures = 0
		}
		mu.Unlock()

		result, err := next(ctx, q)
		if err != nil {
			mu.Lock()
			failures++
			lastFailure = time.Now()
			mu.Unlock()
		} else {
			mu.Lock()
			failures = 0
			mu.Unlock()
		}
		return result, err
	}
}

// RateLimitCommandMiddleware добавляет ограничение нагрузки
func RateLimitCommandMiddleware(maxConcurrent int) CommandMiddleware {
	semaphore := make(chan struct{}, maxConcurrent)

	return func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error {
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
			return next(ctx, cmd)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// RateLimitQueryMiddleware добавляет ограничение нагрузки
func RateLimitQueryMiddleware(maxConcurrent int) QueryMiddleware {
	semaphore := make(chan struct{}, maxConcurrent)

	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
			return next(ctx, q)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// TracingCommandMiddleware добавляет distributed tracing
func TracingCommandMiddleware(tracer interface {
	StartSpan(string) interface{ End() }
}) CommandMiddleware {
	return func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error {
		span := tracer.StartSpan("command." + cmd.CommandName())
		defer span.End()
		return next(ctx, cmd)
	}
}

// TracingQueryMiddleware добавляет distributed tracing
func TracingQueryMiddleware(tracer interface {
	StartSpan(string) interface{ End() }
}) QueryMiddleware {
	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		span := tracer.StartSpan("query." + q.QueryName())
		defer span.End()
		return next(ctx, q)
	}
}

// AuthorizationCommandMiddleware добавляет проверку прав доступа
func AuthorizationCommandMiddleware(authorizer func(ctx context.Context, cmd transport.Command) error) CommandMiddleware {
	return func(ctx context.Context, cmd transport.Command, next func(ctx context.Context, cmd transport.Command) error) error {
		if err := authorizer(ctx, cmd); err != nil {
			return fmt.Errorf("authorization failed: %w", err)
		}
		return next(ctx, cmd)
	}
}

// AuthorizationQueryMiddleware добавляет проверку прав доступа
func AuthorizationQueryMiddleware(authorizer func(ctx context.Context, q transport.Query) error) QueryMiddleware {
	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		if err := authorizer(ctx, q); err != nil {
			return nil, fmt.Errorf("authorization failed: %w", err)
		}
		return next(ctx, q)
	}
}

// CachingQueryMiddleware добавляет кэширование результатов
func CachingQueryMiddleware(cache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}) QueryMiddleware {
	return func(ctx context.Context, q transport.Query, next func(ctx context.Context, q transport.Query) (interface{}, error)) (interface{}, error) {
		key := "query:" + q.QueryName()
		if result, ok := cache.Get(ctx, key); ok {
			return result, nil
		}

		result, err := next(ctx, q)
		if err == nil {
			_ = cache.Set(ctx, key, result, 5*time.Minute)
		}
		return result, err
	}
}
