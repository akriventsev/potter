package application

import (
	"potter/examples/eventsourcing-mongodb/domain"
	"potter/framework/eventsourcing"
)

// NewInventoryRepository создает репозиторий для Inventory агрегата
func NewInventoryRepository(
	eventStore eventsourcing.EventStore,
	snapshotStore eventsourcing.SnapshotStore,
) *eventsourcing.EventSourcedRepository[*domain.Inventory] {
	config := eventsourcing.DefaultRepositoryConfig()
	config.UseSnapshots = true
	config.SnapshotFrequency = 50 // Снапшот каждые 50 событий

	return eventsourcing.NewEventSourcedRepository[*domain.Inventory](
		eventStore,
		snapshotStore,
		config,
		func(id string) *domain.Inventory {
			// Для восстановления создаем пустой инвентарь
			inventory := &domain.Inventory{
				EventSourcedAggregate: eventsourcing.NewEventSourcedAggregate(id),
			}
			inventory.SetApplier(inventory)
			return inventory
		},
	)
}

