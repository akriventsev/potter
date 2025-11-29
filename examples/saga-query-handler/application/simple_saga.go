package application

import (
	"context"
	"fmt"
	"time"

	"potter/framework/saga"
)

// SimpleSagaDefinition создает простое определение саги для демонстрации
func NewSimpleSagaDefinition() saga.SagaDefinition {
	builder := saga.NewSagaBuilder("simple_saga")

	// Добавляем простые шаги
	builder.AddStep(NewSimpleStep1())
	builder.AddStep(NewSimpleStep2())
	builder.AddStep(NewSimpleStep3())

	builder.WithTimeout(5 * 60 * time.Second)

	definition, err := builder.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build simple saga: %v", err))
	}

	return definition
}

// SimpleStep1 первый шаг саги
type SimpleStep1 struct {
	*saga.BaseStep
}

func NewSimpleStep1() *SimpleStep1 {
	step := &SimpleStep1{
		BaseStep: saga.NewBaseStep("step1"),
	}
	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		// Имитируем работу
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	step.WithCompensate(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		return nil
	})
	return step
}

// SimpleStep2 второй шаг саги
type SimpleStep2 struct {
	*saga.BaseStep
}

func NewSimpleStep2() *SimpleStep2 {
	step := &SimpleStep2{
		BaseStep: saga.NewBaseStep("step2"),
	}
	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	step.WithCompensate(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		return nil
	})
	return step
}

// SimpleStep3 третий шаг саги
type SimpleStep3 struct {
	*saga.BaseStep
}

func NewSimpleStep3() *SimpleStep3 {
	step := &SimpleStep3{
		BaseStep: saga.NewBaseStep("step3"),
	}
	step.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	step.WithCompensate(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		return nil
	})
	return step
}
