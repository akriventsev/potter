// Package events предоставляет реализации EventSubscriber.
package events

import (
	"fmt"
	"sort"
	"sync"
)

// InMemoryEventSubscriber реализация подписчика на события в памяти
type InMemoryEventSubscriber struct {
	handlers      map[string][]EventHandler
	priorities    map[string]int
	consumerGroups map[string]string
	mu            sync.RWMutex
}

// NewInMemoryEventSubscriber создает новый in-memory подписчик
func NewInMemoryEventSubscriber() *InMemoryEventSubscriber {
	return &InMemoryEventSubscriber{
		handlers:       make(map[string][]EventHandler),
		priorities:     make(map[string]int),
		consumerGroups: make(map[string]string),
	}
}

// Subscribe подписывается на тип события
func (s *InMemoryEventSubscriber) Subscribe(eventType string, handler EventHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.handlers == nil {
		s.handlers = make(map[string][]EventHandler)
	}

	// Проверяем, не подписан ли уже этот handler
	for _, h := range s.handlers[eventType] {
		if h == handler {
			return fmt.Errorf("handler already subscribed to event type %s", eventType)
		}
	}

	s.handlers[eventType] = append(s.handlers[eventType], handler)
	return nil
}

// SubscribeWithPriority подписывается с приоритетом
func (s *InMemoryEventSubscriber) SubscribeWithPriority(eventType string, handler EventHandler, priority int) error {
	if err := s.Subscribe(eventType, handler); err != nil {
		return err
	}

	s.mu.Lock()
	key := fmt.Sprintf("%s:%p", eventType, handler)
	s.priorities[key] = priority
	s.mu.Unlock()

	return nil
}

// SubscribeWithGroup подписывается с группой потребителей
func (s *InMemoryEventSubscriber) SubscribeWithGroup(eventType string, handler EventHandler, group string) error {
	if err := s.Subscribe(eventType, handler); err != nil {
		return err
	}

	s.mu.Lock()
	key := fmt.Sprintf("%s:%p", eventType, handler)
	s.consumerGroups[key] = group
	s.mu.Unlock()

	return nil
}

// Unsubscribe отписывается от типа события
func (s *InMemoryEventSubscriber) Unsubscribe(eventType string, handler EventHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	handlers := s.handlers[eventType]
	for i, h := range handlers {
		if h == handler {
			s.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			key := fmt.Sprintf("%s:%p", eventType, handler)
			delete(s.priorities, key)
			delete(s.consumerGroups, key)
			return nil
		}
	}

	return fmt.Errorf("handler not found for event type %s", eventType)
}

// GetHandlers возвращает обработчики для типа события с учетом приоритетов
func (s *InMemoryEventSubscriber) GetHandlers(eventType string) []EventHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	handlers := s.handlers[eventType]
	if len(handlers) == 0 {
		return handlers
	}

	// Сортируем обработчики по приоритету
	type handlerWithPriority struct {
		handler  EventHandler
		priority int
	}

	handlerList := make([]handlerWithPriority, 0, len(handlers))
	for _, h := range handlers {
		key := fmt.Sprintf("%s:%p", eventType, h)
		priority := s.priorities[key]
		handlerList = append(handlerList, handlerWithPriority{
			handler:  h,
			priority: priority,
		})
	}

	// Сортируем по приоритету (меньше = выше приоритет)
	sort.Slice(handlerList, func(i, j int) bool {
		return handlerList[i].priority < handlerList[j].priority
	})

	result := make([]EventHandler, len(handlerList))
	for i, item := range handlerList {
		result[i] = item.handler
	}

	return result
}

// GetHandlersByGroup возвращает обработчики для типа события в указанной группе потребителей
func (s *InMemoryEventSubscriber) GetHandlersByGroup(eventType string, group string) []EventHandler {
	allHandlers := s.GetHandlers(eventType)
	if group == "" {
		return allHandlers
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []EventHandler
	for _, h := range allHandlers {
		key := fmt.Sprintf("%s:%p", eventType, h)
		if handlerGroup, ok := s.consumerGroups[key]; ok && handlerGroup == group {
			result = append(result, h)
		}
	}

	return result
}

// FilterHandlers фильтрует обработчики по метаданным
func (s *InMemoryEventSubscriber) FilterHandlers(eventType string, filter func(EventHandler) bool) []EventHandler {
	handlers := s.GetHandlers(eventType)
	filtered := make([]EventHandler, 0)
	for _, h := range handlers {
		if filter(h) {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

