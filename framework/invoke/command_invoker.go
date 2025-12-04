// Package invoke предоставляет CommandInvoker для type-safe работы с командами CQRS.
package invoke

import (
	"context"
	"fmt"
	"time"

	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/transport"
)

// CommandInvoker generic invoker для команд с ожиданием событий по correlation ID
// TCommand - тип команды
// TSuccessEvent - тип успешного события
// TErrorEvent - тип ошибочного события (должен реализовывать ErrorEvent)
type CommandInvoker[TCommand transport.Command, TSuccessEvent events.Event, TErrorEvent ErrorEvent] struct {
	commandBus     *AsyncCommandBus
	eventAwaiter   *EventAwaiter
	successEventType string
	errorEventType   string
	timeout        time.Duration
	serializer     transport.MessageSerializer
}

// NewCommandInvoker создает новый CommandInvoker с поддержкой ошибочных событий
func NewCommandInvoker[TCommand transport.Command, TSuccessEvent events.Event, TErrorEvent ErrorEvent](
	bus *AsyncCommandBus,
	awaiter *EventAwaiter,
	successEventType, errorEventType string,
) *CommandInvoker[TCommand, TSuccessEvent, TErrorEvent] {
	return &CommandInvoker[TCommand, TSuccessEvent, TErrorEvent]{
		commandBus:       bus,
		eventAwaiter:     awaiter,
		successEventType: successEventType,
		errorEventType:   errorEventType,
		timeout:          30 * time.Second,
		serializer:       DefaultSerializer(),
	}
}

// NewCommandInvokerWithoutError создает CommandInvoker без поддержки ошибочных событий (для обратной совместимости)
func NewCommandInvokerWithoutError[TCommand transport.Command, TSuccessEvent events.Event](
	bus *AsyncCommandBus,
	awaiter *EventAwaiter,
	successEventType string,
) *CommandInvoker[TCommand, TSuccessEvent, *BaseErrorEvent] {
	return &CommandInvoker[TCommand, TSuccessEvent, *BaseErrorEvent]{
		commandBus:       bus,
		eventAwaiter:     awaiter,
		successEventType: successEventType,
		errorEventType:   "",
		timeout:          30 * time.Second,
		serializer:       DefaultSerializer(),
	}
}

// NewCommandInvokerWithOptions создает CommandInvoker с использованием InvokeOptions
// Рекомендуется для сложных сценариев с кастомными настройками таймаутов, источников событий и типов событий
func NewCommandInvokerWithOptions[TCommand transport.Command, TSuccessEvent events.Event, TErrorEvent ErrorEvent](
	bus *AsyncCommandBus,
	options ...InvokeOption,
) (*CommandInvoker[TCommand, TSuccessEvent, TErrorEvent], error) {
	opts := ApplyOptions(options...)

	// Проверяем наличие EventSource
	if opts.EventSource == nil {
		return nil, NewEventSourceNotConfiguredError()
	}

	// Создаем EventAwaiter из EventSource
	awaiter := NewEventAwaiterFromEventSource(opts.EventSource)

	// Определяем типы событий
	successEventType := opts.SuccessEventType
	if successEventType == "" {
		return nil, fmt.Errorf("success event type is required")
	}

	errorEventType := opts.ErrorEventType

	invoker := &CommandInvoker[TCommand, TSuccessEvent, TErrorEvent]{
		commandBus:       bus,
		eventAwaiter:     awaiter,
		successEventType: successEventType,
		errorEventType:   errorEventType,
		timeout:          opts.Timeout,
		serializer:       DefaultSerializer(),
	}

	// Применяем SubjectResolver к bus, если указан
	if opts.SubjectResolver != nil {
		bus.WithSubjectResolver(opts.SubjectResolver)
	}

	// Устанавливаем таймаут, если указан
	if opts.Timeout > 0 {
		invoker.timeout = opts.Timeout
	}

	return invoker, nil
}

// WithTimeout устанавливает таймаут ожидания события
func (i *CommandInvoker[TCommand, TSuccessEvent, TErrorEvent]) WithTimeout(timeout time.Duration) *CommandInvoker[TCommand, TSuccessEvent, TErrorEvent] {
	i.timeout = timeout
	return i
}

// WithSerializer устанавливает сериализатор
func (i *CommandInvoker[TCommand, TSuccessEvent, TErrorEvent]) WithSerializer(serializer transport.MessageSerializer) *CommandInvoker[TCommand, TSuccessEvent, TErrorEvent] {
	i.serializer = serializer
	return i
}

// Invoke выполняет команду и ожидает событие (fire-and-await)
func (i *CommandInvoker[TCommand, TSuccessEvent, TErrorEvent]) Invoke(ctx context.Context, cmd TCommand) (TSuccessEvent, error) {
	var zero TSuccessEvent

	// Генерируем correlation ID
	correlationID := GenerateCorrelationID()
	commandID := GenerateCommandID()

	// Создаем метаданные
	metadata := transport.NewBaseCommandMetadata(commandID, correlationID, ExtractCausationID(ctx))

	// Распространяем метаданные в контекст
	ctx = PropagateMetadata(ctx, metadata)

	// Если errorEventType не указан, используем старый подход (только успешное событие)
	if i.errorEventType == "" {
		// Создаем дочерний контекст с таймаутом для горутины ожидания
		waitCtx, waitCancel := context.WithTimeout(ctx, i.timeout)
		defer waitCancel() // Гарантируем отмену при любом исходе

		// Регистрируем awaiter для ожидания события
		eventCh := make(chan core.Result[TSuccessEvent], 1)
		go func() {
			defer waitCancel() // Дополнительная гарантия отмены при завершении горутины
			event, err := i.eventAwaiter.Await(waitCtx, correlationID, i.successEventType, i.timeout)
			if err != nil {
				eventCh <- core.Err[TSuccessEvent](err)
				return
			}

			// Type assertion для типизированного события
			typedEvent, ok := event.(TSuccessEvent)
			if !ok {
				eventCh <- core.Err[TSuccessEvent](NewInvalidResultTypeError(
					fmt.Sprintf("%T", zero),
					fmt.Sprintf("%T", event),
				))
				return
			}

			eventCh <- core.Ok(typedEvent)
		}()

		// Публикуем команду (pure produce)
		if err := i.commandBus.SendAsync(ctx, cmd, metadata); err != nil {
			waitCancel()
			i.eventAwaiter.Cancel(correlationID)
			return zero, err
		}

		// Ожидаем событие с timeout
		select {
		case result := <-eventCh:
			waitCancel()
			if result.IsErr() {
				return zero, result.Error
			}
			return result.Value, nil
		case <-ctx.Done():
			waitCancel()
			i.eventAwaiter.Cancel(correlationID)
			return zero, ctx.Err()
		case <-waitCtx.Done():
			waitCancel()
			i.eventAwaiter.Cancel(correlationID)
			if waitCtx.Err() == context.DeadlineExceeded {
				return zero, NewEventTimeoutError(correlationID, i.timeout.String())
			}
			return zero, waitCtx.Err()
		}
	}

	// Поддержка ошибочных событий: ожидаем любое из двух событий
	// Создаем дочерний контекст с таймаутом для горутины ожидания
	waitCtx, waitCancel := context.WithTimeout(ctx, i.timeout)
	defer waitCancel() // Гарантируем отмену при любом исходе

	eventTypes := []string{i.successEventType, i.errorEventType}
	eventCh := make(chan core.Result[TSuccessEvent], 1)
	go func() {
		defer waitCancel() // Дополнительная гарантия отмены при завершении горутины
		event, receivedType, err := i.eventAwaiter.AwaitAny(waitCtx, correlationID, eventTypes, i.timeout)
		if err != nil {
			eventCh <- core.Err[TSuccessEvent](err)
			return
		}

		// Если получено ошибочное событие, возвращаем ошибку
		if receivedType == i.errorEventType {
			if errorEvent, ok := event.(TErrorEvent); ok {
				eventCh <- core.Err[TSuccessEvent](NewErrorEventReceivedError(errorEvent))
				return
			}
			// Если не удалось привести к TErrorEvent, создаем базовую ошибку
			eventCh <- core.Err[TSuccessEvent](fmt.Errorf("error event received: %s", receivedType))
			return
		}

		// Type assertion для успешного события
		typedEvent, ok := event.(TSuccessEvent)
		if !ok {
			eventCh <- core.Err[TSuccessEvent](NewInvalidResultTypeError(
				fmt.Sprintf("%T", zero),
				fmt.Sprintf("%T", event),
			))
			return
		}

		eventCh <- core.Ok(typedEvent)
	}()

	// Публикуем команду (pure produce)
	if err := i.commandBus.SendAsync(ctx, cmd, metadata); err != nil {
		waitCancel()
		i.eventAwaiter.Cancel(correlationID)
		return zero, err
	}

	// Ожидаем событие с timeout
	select {
	case result := <-eventCh:
		waitCancel()
		if result.IsErr() {
			return zero, result.Error
		}
		return result.Value, nil
	case <-ctx.Done():
		waitCancel()
		i.eventAwaiter.Cancel(correlationID)
		return zero, ctx.Err()
	case <-waitCtx.Done():
		waitCancel()
		i.eventAwaiter.Cancel(correlationID)
		if waitCtx.Err() == context.DeadlineExceeded {
			return zero, NewEventTimeoutError(correlationID, i.timeout.String())
		}
		return zero, waitCtx.Err()
	}
}

// InvokeAsync выполняет команду асинхронно и возвращает канал с результатом
func (i *CommandInvoker[TCommand, TSuccessEvent, TErrorEvent]) InvokeAsync(ctx context.Context, cmd TCommand) (<-chan core.Result[TSuccessEvent], error) {
	resultCh := make(chan core.Result[TSuccessEvent], 1)

	// Генерируем correlation ID
	correlationID := GenerateCorrelationID()
	commandID := GenerateCommandID()

	// Создаем метаданные
	metadata := transport.NewBaseCommandMetadata(commandID, correlationID, ExtractCausationID(ctx))

	// Распространяем метаданные в контекст
	ctx = PropagateMetadata(ctx, metadata)

	// Если errorEventType не указан, используем старый подход
	if i.errorEventType == "" {
		// Регистрируем awaiter для ожидания события
		go func() {
			event, err := i.eventAwaiter.Await(ctx, correlationID, i.successEventType, i.timeout)
			if err != nil {
				resultCh <- core.Err[TSuccessEvent](err)
				return
			}

			// Type assertion для типизированного события
			var zero TSuccessEvent
			typedEvent, ok := event.(TSuccessEvent)
			if !ok {
				resultCh <- core.Err[TSuccessEvent](NewInvalidResultTypeError(
					fmt.Sprintf("%T", zero),
					fmt.Sprintf("%T", event),
				))
				return
			}

			resultCh <- core.Ok(typedEvent)
		}()

		// Публикуем команду (pure produce)
		if err := i.commandBus.SendAsync(ctx, cmd, metadata); err != nil {
			i.eventAwaiter.Cancel(correlationID)
			resultCh <- core.Err[TSuccessEvent](err)
			return resultCh, err
		}

		return resultCh, nil
	}

	// Поддержка ошибочных событий
	eventTypes := []string{i.successEventType, i.errorEventType}
	go func() {
		event, receivedType, err := i.eventAwaiter.AwaitAny(ctx, correlationID, eventTypes, i.timeout)
		if err != nil {
			resultCh <- core.Err[TSuccessEvent](err)
			return
		}

		// Если получено ошибочное событие, возвращаем ошибку
		if receivedType == i.errorEventType {
			if errorEvent, ok := event.(TErrorEvent); ok {
				resultCh <- core.Err[TSuccessEvent](NewErrorEventReceivedError(errorEvent))
				return
			}
			resultCh <- core.Err[TSuccessEvent](fmt.Errorf("error event received: %s", receivedType))
			return
		}

		// Type assertion для успешного события
		var zero TSuccessEvent
		typedEvent, ok := event.(TSuccessEvent)
		if !ok {
			resultCh <- core.Err[TSuccessEvent](NewInvalidResultTypeError(
				fmt.Sprintf("%T", zero),
				fmt.Sprintf("%T", event),
			))
			return
		}

		resultCh <- core.Ok(typedEvent)
	}()

	// Публикуем команду (pure produce)
	if err := i.commandBus.SendAsync(ctx, cmd, metadata); err != nil {
		i.eventAwaiter.Cancel(correlationID)
		resultCh <- core.Err[TSuccessEvent](err)
		return resultCh, err
	}

	return resultCh, nil
}

// InvokeWithBothResults выполняет команду и возвращает оба типа событий для детального анализа
func (i *CommandInvoker[TCommand, TSuccessEvent, TErrorEvent]) InvokeWithBothResults(ctx context.Context, cmd TCommand) (TSuccessEvent, TErrorEvent, error) {
	var zeroSuccess TSuccessEvent
	var zeroError TErrorEvent

	if i.errorEventType == "" {
		// Если ошибочные события не поддерживаются, используем обычный Invoke
		success, err := i.Invoke(ctx, cmd)
		return success, zeroError, err
	}

	// Генерируем correlation ID
	correlationID := GenerateCorrelationID()
	commandID := GenerateCommandID()

	// Создаем метаданные
	metadata := transport.NewBaseCommandMetadata(commandID, correlationID, ExtractCausationID(ctx))

	// Распространяем метаданные в контекст
	ctx = PropagateMetadata(ctx, metadata)

	// Ожидаем любое из событий
	eventTypes := []string{i.successEventType, i.errorEventType}
	eventCh := make(chan struct {
		event events.Event
		err   error
	}, 1)

	go func() {
		event, _, err := i.eventAwaiter.AwaitAny(ctx, correlationID, eventTypes, i.timeout)
		eventCh <- struct {
			event events.Event
			err   error
		}{event, err}
	}()

	// Публикуем команду (pure produce)
	if err := i.commandBus.SendAsync(ctx, cmd, metadata); err != nil {
		i.eventAwaiter.Cancel(correlationID)
		return zeroSuccess, zeroError, err
	}

	// Ожидаем событие с timeout
	select {
	case result := <-eventCh:
		if result.err != nil {
			return zeroSuccess, zeroError, result.err
		}

		// Проверяем тип события
		if result.event.EventType() == i.errorEventType {
			if errorEvent, ok := result.event.(TErrorEvent); ok {
				return zeroSuccess, errorEvent, nil
			}
			return zeroSuccess, zeroError, fmt.Errorf("failed to cast error event to %T", zeroError)
		}

		if successEvent, ok := result.event.(TSuccessEvent); ok {
			return successEvent, zeroError, nil
		}
		return zeroSuccess, zeroError, fmt.Errorf("failed to cast success event to %T", zeroSuccess)
	case <-ctx.Done():
		i.eventAwaiter.Cancel(correlationID)
		return zeroSuccess, zeroError, ctx.Err()
	case <-time.After(i.timeout):
		i.eventAwaiter.Cancel(correlationID)
		return zeroSuccess, zeroError, NewEventTimeoutError(correlationID, i.timeout.String())
	}
}

