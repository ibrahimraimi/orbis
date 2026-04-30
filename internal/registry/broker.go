package registry

import (
	"orbis/internal/models"
	"sync"
)

type EventType string

const (
	EventServiceUpserted EventType = "upsert"
	EventServiceDeleted  EventType = "delete"
)

type ServiceEvent struct {
	Type    EventType       `json:"type"`
	Service *models.Service `json:"service"`
	ID      string          `json:"id"`
}

type EventBroker struct {
	mu          sync.RWMutex
	subscribers map[chan ServiceEvent]struct{}
}

func NewEventBroker() *EventBroker {
	return &EventBroker{
		subscribers: make(map[chan ServiceEvent]struct{}),
	}
}

func (b *EventBroker) Subscribe() chan ServiceEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan ServiceEvent, 100) // Buffer to prevent blocking the registry
	b.subscribers[ch] = struct{}{}
	return ch
}

func (b *EventBroker) Unsubscribe(ch chan ServiceEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
}

func (b *EventBroker) Publish(event ServiceEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// If a subscriber's buffer is full, we drop the event to avoid
			// blocking the registry or other subscribers.
			// In a robust system, this might trigger a disconnect.
		}
	}
}
