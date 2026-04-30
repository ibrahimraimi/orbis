package registry

import (
	"orbis/internal/models"
	"sync"
)

type EventType string

const (
	EventServiceUpserted  EventType = "service_upsert"
	EventServiceDeleted   EventType = "service_delete"
	EventConsumerUpserted EventType = "consumer_upsert"
	EventConsumerDeleted  EventType = "consumer_delete"
)

type Event struct {
	Type     EventType        `json:"type"`
	Service  *models.Service  `json:"service,omitempty"`
	Consumer *models.Consumer `json:"consumer,omitempty"`
	ID       string           `json:"id"`
}

type EventBroker struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

func NewEventBroker() *EventBroker {
	return &EventBroker{
		subscribers: make(map[chan Event]struct{}),
	}
}

func (b *EventBroker) Subscribe() chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, 100) // Buffer to prevent blocking the registry
	b.subscribers[ch] = struct{}{}
	return ch
}

func (b *EventBroker) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
}

func (b *EventBroker) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
