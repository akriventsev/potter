package application

import (
	"context"
	"fmt"
	"time"

	"potter/framework/invoke"
	"potter/framework/saga"
)

// NewConditionalSagaDefinition создает новое определение саги с условными шагами
func NewConditionalSagaDefinition(
	asyncCommandBus *invoke.AsyncCommandBus,
	eventAwaiter *invoke.EventAwaiter,
) saga.SagaDefinition {
	builder := saga.NewSagaBuilder("conditional_order_saga")

	// Базовый шаг - всегда выполняется
	baseStep := saga.NewBaseStep("base_validation")
	baseStep.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		// Базовая валидация
		return nil
	})

	// Условный шаг - верификация для крупных сумм (> 1000)
	verificationStep := saga.NewBaseStep("verification")
	verificationStep.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		// Верификация
		return nil
	})
	conditionalVerification := saga.NewConditionalStep(
		"conditional_verification",
		func(ctx context.Context, sagaCtx saga.SagaContext) bool {
			amount := sagaCtx.GetFloat64("amount")
			return amount > 1000.0
		},
		verificationStep,
	)

	// Условный шаг - VIP обработка
	vipStep := saga.NewBaseStep("vip_processing")
	vipStep.WithExecute(func(ctx context.Context, sagaCtx saga.SagaContext) error {
		// VIP обработка
		return nil
	})
	conditionalVIP := saga.NewConditionalStep(
		"conditional_vip",
		func(ctx context.Context, sagaCtx saga.SagaContext) bool {
			customerType := sagaCtx.GetString("customer_type")
			return customerType == "vip"
		},
		vipStep,
	)

	builder.AddStep(baseStep)
	builder.AddStep(conditionalVerification)
	builder.AddStep(conditionalVIP)

	builder.WithTimeout(5 * 60 * time.Second)

	definition, err := builder.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build conditional saga: %v", err))
	}

	return definition
}

