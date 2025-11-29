package application

import (
	"github.com/akriventsev/potter/examples/eventsourcing-basic/domain"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/eventsourcing"
)

// BankAccountAggregate обертка над доменным агрегатом для использования в репозитории
// Реализует интерфейс AggregateInterface через делегирование к domain.BankAccount
type BankAccountAggregate struct {
	*domain.BankAccount
}

// NewBankAccountAggregate создает новый агрегат банковского счета
// Используется для загрузки существующих счетов из репозитория
func NewBankAccountAggregate(id string) *BankAccountAggregate {
	// Создаем базовый агрегат без события открытия
	// События будут применены при загрузке из репозитория
	baseAggregate := eventsourcing.NewEventSourcedAggregate(id)
	bankAccount := &domain.BankAccount{
		EventSourcedAggregate: baseAggregate,
	}
	bankAccount.SetApplier(bankAccount)

	return &BankAccountAggregate{
		BankAccount: bankAccount,
	}
}

// OpenAccount открывает новый счет
func (a *BankAccountAggregate) OpenAccount(accountNumber, ownerName string) {
	event := &domain.AccountOpenedEvent{
		BaseEvent:     events.NewBaseEvent("account.opened", accountNumber),
		AccountNumber: accountNumber,
		OwnerName:     ownerName,
	}
	a.RaiseEvent(event)
}

