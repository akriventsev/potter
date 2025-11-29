package application

import (
	"context"
	"time"

	"github.com/akriventsev/potter/framework/adapters/messagebus"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/invoke"
	"github.com/akriventsev/potter/framework/saga"
)

// NewWarehouseSagaDefinition создает новое определение саги для интеграции с warehouse через 2PC
func NewWarehouseSagaDefinition(
	asyncCommandBus *invoke.AsyncCommandBus,
	eventAwaiter *invoke.EventAwaiter,
	eventBus events.EventBus,
	natsAdapter *messagebus.NATSAdapter,
	// twopcCoord *twopc.NATSCoordinator, // Пакет twopc не реализован
) saga.SagaDefinition {
	builder := saga.NewSagaBuilder("warehouse_saga")

	// Добавляем шаги
	builder.AddStep(NewValidateOrderStep())
	
	// Добавляем шаг с 2PC для резервирования на нескольких складах
	// Используем простой координатор для примера (в реальности нужен twopc.NATSCoordinator)
	coordinator := NewSimpleTwoPhaseCommitCoordinator()
	twoPCStep := saga.NewTwoPhaseCommitStep(
		"reserve_multi_warehouse",
		coordinator,
		func(ctx context.Context, sagaCtx saga.SagaContext) []saga.TwoPhaseCommitParticipant {
			// Получаем список складов из контекста
			warehouseIDsVal := sagaCtx.Get("warehouse_ids")
			if warehouseIDsVal == nil {
				return []saga.TwoPhaseCommitParticipant{}
			}

			warehouseIDs, ok := warehouseIDsVal.([]string)
			if !ok {
				// Пытаемся преобразовать из []interface{}
				if idsInterface, ok := warehouseIDsVal.([]interface{}); ok {
					warehouseIDs = make([]string, len(idsInterface))
					for i, id := range idsInterface {
						if idStr, ok := id.(string); ok {
							warehouseIDs[i] = idStr
						}
					}
				} else {
					return []saga.TwoPhaseCommitParticipant{}
				}
			}

			orderID := sagaCtx.GetString("order_id")
			items := sagaCtx.Get("items")

			// Создаем participants для каждого склада
			participants := make([]saga.TwoPhaseCommitParticipant, 0, len(warehouseIDs))
			for _, warehouseID := range warehouseIDs {
				participant := NewWarehouseStockParticipant(warehouseID, orderID, items)
				participants = append(participants, participant)
			}

			return participants
		},
	)
	builder.AddStep(twoPCStep)

	builder.AddStep(NewProcessPaymentStep(asyncCommandBus, eventAwaiter))

	// Устанавливаем общий timeout (5 минут)
	builder.WithTimeout(5 * 60 * time.Second)

	definition, err := builder.Build()
	if err != nil {
		panic("failed to build warehouse saga: " + err.Error())
	}

	return definition
}

// SimpleTwoPhaseCommitCoordinator простая реализация координатора 2PC для примера
type SimpleTwoPhaseCommitCoordinator struct{}

// NewSimpleTwoPhaseCommitCoordinator создает простой координатор 2PC
func NewSimpleTwoPhaseCommitCoordinator() saga.TwoPhaseCommitCoordinator {
	return &SimpleTwoPhaseCommitCoordinator{}
}

// Execute выполняет 2PC транзакцию
func (c *SimpleTwoPhaseCommitCoordinator) Execute(ctx context.Context, transactionID string, participants []saga.TwoPhaseCommitParticipant) error {
	// Фаза 1: Prepare
	for _, p := range participants {
		if err := p.Prepare(ctx, transactionID); err != nil {
			// Откатываем все подготовленные участники
			for _, rollbackP := range participants {
				_ = rollbackP.Abort(ctx, transactionID)
			}
			return err
		}
	}

	// Фаза 2: Commit
	for _, p := range participants {
		if err := p.Commit(ctx, transactionID); err != nil {
			// В случае ошибки commit пытаемся откатить
			// В реальности нужна более сложная логика
			_ = p.Abort(ctx, transactionID)
			return err
		}
	}

	return nil
}
