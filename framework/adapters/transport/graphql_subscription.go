// Package transport предоставляет менеджер GraphQL subscriptions с интеграцией EventBus.
package transport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/akriventsev/potter/framework/events"
)

// SubscriptionManager менеджер подписок на события
type SubscriptionManager struct {
	eventBus     events.EventBus
	subscriptions map[string]*Subscription
	mu           sync.RWMutex
	filters      map[string]EventFilter
}

// Subscription представляет активную подписку
type Subscription struct {
	ID        string
	EventType string
	Channel   chan events.Event
	Filter    EventFilter
	Context   context.Context
	Cancel    context.CancelFunc
	CreatedAt time.Time
	handler   events.EventHandler // Сохраняем handler для правильной отписки
}

// EventFilter интерфейс для фильтрации событий
type EventFilter interface {
	Match(event events.Event) bool
}

// NewSubscriptionManager создает новый менеджер подписок
func NewSubscriptionManager(eventBus events.EventBus) *SubscriptionManager {
	return &SubscriptionManager{
		eventBus:      eventBus,
		subscriptions: make(map[string]*Subscription),
		filters:       make(map[string]EventFilter),
	}
}

// Subscribe создает подписку на события
func (sm *SubscriptionManager) Subscribe(ctx context.Context, eventType string, filter EventFilter) (<-chan events.Event, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Создаем контекст с отменой
	subCtx, cancel := context.WithCancel(ctx)
	
	// Создаем канал для событий
	channel := make(chan events.Event, 100) // Buffered channel для backpressure handling
	
	// Генерируем ID подписки
	subscriptionID := fmt.Sprintf("sub-%d", time.Now().UnixNano())
	
	// Создаем подписку
	subscription := &Subscription{
		ID:        subscriptionID,
		EventType: eventType,
		Channel:   channel,
		Filter:    filter,
		Context:   subCtx,
		Cancel:    cancel,
		CreatedAt: time.Now(),
	}
	
	// Регистрируем подписку
	sm.subscriptions[subscriptionID] = subscription
	
	// Подписываемся на EventBus
	handler := &subscriptionEventHandler{
		subscription: subscription,
		manager:     sm,
	}
	
	// Сохраняем handler в subscription для последующей отписки
	subscription.handler = handler
	
	if err := sm.eventBus.Subscribe(eventType, handler); err != nil {
		cancel()
		close(channel)
		delete(sm.subscriptions, subscriptionID)
		return nil, fmt.Errorf("failed to subscribe to event bus: %w", err)
	}
	
	// Запускаем goroutine для отслеживания отмены контекста
	go func() {
		<-subCtx.Done()
		sm.Unsubscribe(subscriptionID)
	}()
	
	return channel, nil
}

// subscriptionEventHandler обработчик событий для подписки
type subscriptionEventHandler struct {
	subscription *Subscription
	manager      *SubscriptionManager
}

// Handle обрабатывает событие
func (h *subscriptionEventHandler) Handle(ctx context.Context, event events.Event) error {
	// Проверяем фильтр
	if h.subscription.Filter != nil && !h.subscription.Filter.Match(event) {
		return nil
	}
	
	// Отправляем событие в канал (неблокирующая отправка)
	select {
	case h.subscription.Channel <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-h.subscription.Context.Done():
		return h.subscription.Context.Err()
	default:
		// Канал заполнен, пропускаем событие (backpressure)
		return nil
	}
}

// EventType возвращает тип события
func (h *subscriptionEventHandler) EventType() string {
	return h.subscription.EventType
}

// Unsubscribe отменяет подписку
func (sm *SubscriptionManager) Unsubscribe(subscriptionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	subscription, ok := sm.subscriptions[subscriptionID]
	if !ok {
		return fmt.Errorf("subscription %s not found", subscriptionID)
	}
	
	// Отменяем контекст
	subscription.Cancel()
	
	// Закрываем канал
	close(subscription.Channel)
	
	// Отписываемся от EventBus используя сохраненный handler
	if subscription.handler != nil {
		_ = sm.eventBus.Unsubscribe(subscription.EventType, subscription.handler)
	}
	
	// Удаляем из реестра
	delete(sm.subscriptions, subscriptionID)
	
	return nil
}

// Broadcast отправляет событие всем подходящим подписчикам
func (sm *SubscriptionManager) Broadcast(event events.Event) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// EventBus автоматически доставит событие всем подписчикам
	// Этот метод может быть использован для ручной рассылки
	return nil
}

// Close закрывает все подписки
func (sm *SubscriptionManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	for subscriptionID := range sm.subscriptions {
		subscription := sm.subscriptions[subscriptionID]
		subscription.Cancel()
		close(subscription.Channel)
		delete(sm.subscriptions, subscriptionID)
	}
	
	return nil
}

// CorrelationIDFilter фильтр по correlation ID
type CorrelationIDFilter struct {
	CorrelationID string
}

// Match проверяет соответствие события фильтру
func (f *CorrelationIDFilter) Match(event events.Event) bool {
	metadata := event.Metadata()
	if metadata == nil {
		return false
	}
	return metadata.CorrelationID() == f.CorrelationID
}

// AggregateIDFilter фильтр по aggregate ID
type AggregateIDFilter struct {
	AggregateID string
}

// Match проверяет соответствие события фильтру
func (f *AggregateIDFilter) Match(event events.Event) bool {
	return event.AggregateID() == f.AggregateID
}

// CompositeFilter комбинация фильтров
type CompositeFilter struct {
	Filters []EventFilter
	Op      string // "AND" или "OR"
}

// Match проверяет соответствие события фильтру
func (f *CompositeFilter) Match(event events.Event) bool {
	if len(f.Filters) == 0 {
		return true
	}
	
	if f.Op == "OR" {
		for _, filter := range f.Filters {
			if filter.Match(event) {
				return true
			}
		}
		return false
	}
	
	// AND (по умолчанию)
	for _, filter := range f.Filters {
		if !filter.Match(event) {
			return false
		}
	}
	return true
}

