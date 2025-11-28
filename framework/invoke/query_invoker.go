// Package invoke предоставляет QueryInvoker для type-safe работы с запросами CQRS.
package invoke

import (
	"context"
	"fmt"
	"time"

	"potter/framework/transport"
)

// QueryInvoker generic type-safe обертка над QueryBus
type QueryInvoker[TQuery transport.Query, TResult any] struct {
	queryBus  transport.QueryBus
	cache     transport.QueryCache
	timeout   time.Duration
	validator func(TResult) error
}

// NewQueryInvoker создает новый QueryInvoker
func NewQueryInvoker[TQuery transport.Query, TResult any](bus transport.QueryBus) *QueryInvoker[TQuery, TResult] {
	return &QueryInvoker[TQuery, TResult]{
		queryBus: bus,
		timeout:  10 * time.Second,
	}
}

// NewQueryInvokerWithOptions создает QueryInvoker с использованием InvokeOptions
// Рекомендуется для сложных сценариев с кастомными настройками таймаутов и метаданных
func NewQueryInvokerWithOptions[TQuery transport.Query, TResult any](
	bus transport.QueryBus,
	options ...InvokeOption,
) *QueryInvoker[TQuery, TResult] {
	opts := ApplyOptions(options...)

	invoker := &QueryInvoker[TQuery, TResult]{
		queryBus: bus,
		timeout:  opts.Timeout,
	}

	// Устанавливаем таймаут, если указан
	if opts.Timeout > 0 {
		invoker.timeout = opts.Timeout
	}

	return invoker
}

// WithCache устанавливает кэш для запросов
func (i *QueryInvoker[TQuery, TResult]) WithCache(cache transport.QueryCache) *QueryInvoker[TQuery, TResult] {
	i.cache = cache
	return i
}

// WithTimeout устанавливает таймаут выполнения запроса
func (i *QueryInvoker[TQuery, TResult]) WithTimeout(timeout time.Duration) *QueryInvoker[TQuery, TResult] {
	i.timeout = timeout
	return i
}

// WithValidator устанавливает валидатор результата
func (i *QueryInvoker[TQuery, TResult]) WithValidator(validator func(TResult) error) *QueryInvoker[TQuery, TResult] {
	i.validator = validator
	return i
}

// Invoke выполняет запрос и возвращает типизированный результат
func (i *QueryInvoker[TQuery, TResult]) Invoke(ctx context.Context, query TQuery) (TResult, error) {
	var zero TResult

	// Создаем контекст с timeout
	queryCtx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()

	// Выполняем запрос через QueryBus
	result, err := i.queryBus.Ask(queryCtx, query)
	if err != nil {
		return zero, err
	}

	// Type assertion для типизированного результата
	typedResult, ok := result.(TResult)
	if !ok {
		return zero, NewInvalidResultTypeError(
			fmt.Sprintf("%T", zero),
			fmt.Sprintf("%T", result),
		)
	}

	// Валидация результата
	if i.validator != nil {
		if err := i.validator(typedResult); err != nil {
			return zero, NewValidationFailedError(err)
		}
	}

	return typedResult, nil
}

// InvokeWithMetadata выполняет запрос с дополнительными метаданными
func (i *QueryInvoker[TQuery, TResult]) InvokeWithMetadata(ctx context.Context, query TQuery, metadata map[string]interface{}) (TResult, error) {
	// Добавляем метаданные в контекст
	if metadata != nil {
		for k, v := range metadata {
			ctx = context.WithValue(ctx, k, v)
		}
	}

	return i.Invoke(ctx, query)
}

// InvokeBatch выполняет пакет запросов
func (i *QueryInvoker[TQuery, TResult]) InvokeBatch(ctx context.Context, queries []TQuery) ([]TResult, error) {
	results := make([]TResult, 0, len(queries))
	errors := make([]error, 0)

	for _, query := range queries {
		result, err := i.Invoke(ctx, query)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		results = append(results, result)
	}

	if len(errors) > 0 {
		return results, fmt.Errorf("failed to execute some queries: %v", errors)
	}

	return results, nil
}

