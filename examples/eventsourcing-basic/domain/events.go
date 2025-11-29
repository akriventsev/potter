package domain

import "github.com/akriventsev/potter/framework/events"

// AccountOpenedEvent событие открытия счета
type AccountOpenedEvent struct {
	*events.BaseEvent
	AccountNumber string
	OwnerName     string
}

// MoneyDepositedEvent событие пополнения счета
type MoneyDepositedEvent struct {
	*events.BaseEvent
	Amount int64
}

// MoneyWithdrawnEvent событие снятия денег
type MoneyWithdrawnEvent struct {
	*events.BaseEvent
	Amount int64
}

// AccountClosedEvent событие закрытия счета
type AccountClosedEvent struct {
	*events.BaseEvent
}

