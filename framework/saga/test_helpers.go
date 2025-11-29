package saga

import (
	"context"

	"github.com/akriventsev/potter/framework/events"
	"github.com/akriventsev/potter/framework/transport"
)

// mockEventBus mock реализация EventBus для тестов
type mockEventBus struct {
	events []events.Event
}

func (b *mockEventBus) Publish(ctx context.Context, event events.Event) error {
	b.events = append(b.events, event)
	return nil
}

func (b *mockEventBus) Subscribe(eventType string, handler events.EventHandler) error {
	return nil
}

func (b *mockEventBus) Unsubscribe(eventType string, handler events.EventHandler) error {
	return nil
}

// mockCommandBus mock реализация CommandBus для тестов
type mockCommandBus struct {
	commands map[string]bool
}

func (b *mockCommandBus) Send(ctx context.Context, cmd transport.Command) error {
	if b.commands == nil {
		b.commands = make(map[string]bool)
	}
	b.commands[cmd.CommandName()] = true
	return nil
}

func (b *mockCommandBus) Register(handler transport.CommandHandler) error {
	return nil
}

