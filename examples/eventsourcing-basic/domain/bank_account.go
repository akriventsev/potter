package domain

import (
	"errors"
	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/eventsourcing"
)

// BankAccount Event Sourced агрегат для банковского счета
type BankAccount struct {
	*eventsourcing.EventSourcedAggregate
	accountNumber string
	ownerName     string
	balance       int64
	isActive      bool
}

// NewBankAccount создает новый банковский счет
func NewBankAccount(accountNumber, ownerName string) *BankAccount {
	account := &BankAccount{
		EventSourcedAggregate: eventsourcing.NewEventSourcedAggregate(accountNumber),
	}
	// Устанавливаем applier для применения событий
	account.SetApplier(account)
	event := &AccountOpenedEvent{
		BaseEvent: events.NewBaseEvent("account.opened", accountNumber),
		AccountNumber: accountNumber,
		OwnerName:     ownerName,
	}
	account.RaiseEvent(event)
	return account
}

// Deposit пополняет счет
func (a *BankAccount) Deposit(amount int64) error {
	if !a.isActive {
		return errors.New("account is closed")
	}
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	event := &MoneyDepositedEvent{
		BaseEvent: events.NewBaseEvent("account.deposited", a.ID()),
		Amount: amount,
	}
	a.RaiseEvent(event)
	return nil
}

// Withdraw снимает деньги со счета
func (a *BankAccount) Withdraw(amount int64) error {
	if !a.isActive {
		return errors.New("account is closed")
	}
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	if a.balance < amount {
		return errors.New("insufficient funds")
	}
	event := &MoneyWithdrawnEvent{
		BaseEvent: events.NewBaseEvent("account.withdrawn", a.ID()),
		Amount: amount,
	}
	a.RaiseEvent(event)
	return nil
}

// Close закрывает счет
func (a *BankAccount) Close() error {
	if !a.isActive {
		return errors.New("account is already closed")
	}
	event := &AccountClosedEvent{
		BaseEvent: events.NewBaseEvent("account.closed", a.ID()),
	}
	a.RaiseEvent(event)
	return nil
}

// Apply применяет события для восстановления состояния
func (a *BankAccount) Apply(event events.Event) error {
	switch e := event.(type) {
	case *AccountOpenedEvent:
		a.accountNumber = e.AccountNumber
		a.ownerName = e.OwnerName
		a.balance = 0
		a.isActive = true
	case *MoneyDepositedEvent:
		a.balance += e.Amount
	case *MoneyWithdrawnEvent:
		a.balance -= e.Amount
	case *AccountClosedEvent:
		a.isActive = false
	}
	return nil
}

// GetAccountNumber возвращает номер счета
func (a *BankAccount) GetAccountNumber() string {
	return a.accountNumber
}

// GetOwnerName возвращает имя владельца
func (a *BankAccount) GetOwnerName() string {
	return a.ownerName
}

// GetBalance возвращает баланс
func (a *BankAccount) GetBalance() int64 {
	return a.balance
}

// IsActive возвращает статус счета
func (a *BankAccount) IsActive() bool {
	return a.isActive
}

