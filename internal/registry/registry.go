package registry

import (
	"fmt"
	"orbis/internal/models"
	"sync"
	"time"
)

// Registry manages the service instances and consumers in memory and persists them.
type Registry struct {
	mu        sync.RWMutex
	services  map[string]*models.Service
	consumers map[string]*models.Consumer
	store     Store
	Broker    *EventBroker
}

func NewRegistry(store Store) (*Registry, error) {
	services := make(map[string]*models.Service)
	consumers := make(map[string]*models.Consumer)

	if store != nil {
		persisted, err := store.LoadAll()
		if err != nil {
			return nil, fmt.Errorf("failed to load persisted services: %w", err)
		}
		for _, s := range persisted {
			services[s.ID] = s
		}

		persistedCons, err := store.LoadAllConsumers()
		if err != nil {
			return nil, fmt.Errorf("failed to load persisted consumers: %w", err)
		}
		for _, c := range persistedCons {
			consumers[c.ID] = c
		}
	}

	return &Registry{
		services:  services,
		consumers: consumers,
		store:     store,
		Broker:    NewEventBroker(),
	}, nil
}

func (r *Registry) Register(s *models.Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if _, ok := r.services[s.ID]; !ok {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	s.Health = models.StatusHealthy

	r.services[s.ID] = s

	if r.store != nil {
		if err := r.store.Save(s); err != nil {
			return fmt.Errorf("failed to persist service: %w", err)
		}
	}

	r.Broker.Publish(Event{
		Type:    EventServiceUpserted,
		Service: s,
		ID:      s.ID,
	})

	return nil
}

func (r *Registry) Deregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.services, id)

	if r.store != nil {
		if err := r.store.Delete(id); err != nil {
			return fmt.Errorf("failed to delete service from store: %w", err)
		}
	}

	r.Broker.Publish(Event{
		Type: EventServiceDeleted,
		ID:   id,
	})

	return nil
}

func (r *Registry) GetService(id string) (*models.Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.services[id]
	return s, ok
}

func (r *Registry) ListServices() []*models.Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*models.Service, 0, len(r.services))
	for _, s := range r.services {
		list = append(list, s)
	}
	return list
}

func (r *Registry) GetHealthyServicesByName(name string) []*models.Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var healthy []*models.Service
	for _, s := range r.services {
		if s.Name == name && s.Health == models.StatusHealthy {
			healthy = append(healthy, s)
		}
	}
	return healthy
}

func (r *Registry) UpdateHealth(id string, status models.HealthStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.services[id]
	if !ok {
		return fmt.Errorf("service not found: %s", id)
	}

	s.Health = status
	s.UpdatedAt = time.Now()

	if r.store != nil {
		if err := r.store.Save(s); err != nil {
			return fmt.Errorf("failed to persist health update: %w", err)
		}
	}

	r.Broker.Publish(Event{
		Type:    EventServiceUpserted,
		Service: s,
		ID:      s.ID,
	})

	return nil
}

func (r *Registry) Heartbeat(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.services[id]
	if !ok {
		return fmt.Errorf("service not found: %s", id)
	}

	s.UpdatedAt = time.Now()

	if r.store != nil {
		if err := r.store.Save(s); err != nil {
			return fmt.Errorf("failed to persist heartbeat: %w", err)
		}
	}

	r.Broker.Publish(Event{
		Type:    EventServiceUpserted,
		Service: s,
		ID:      s.ID,
	})

	return nil
}

// Consumer Methods
func (r *Registry) CreateConsumer(c *models.Consumer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c.CreatedAt = time.Now()
	r.consumers[c.ID] = c

	if r.store != nil {
		if err := r.store.SaveConsumer(c); err != nil {
			return fmt.Errorf("failed to persist consumer: %w", err)
		}
	}

	r.Broker.Publish(Event{
		Type:     EventConsumerUpserted,
		Consumer: c,
		ID:       c.ID,
	})

	return nil
}

func (r *Registry) DeleteConsumer(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.consumers, id)

	if r.store != nil {
		if err := r.store.DeleteConsumer(id); err != nil {
			return fmt.Errorf("failed to delete consumer from store: %w", err)
		}
	}

	r.Broker.Publish(Event{
		Type: EventConsumerDeleted,
		ID:   id,
	})

	return nil
}

func (r *Registry) ListConsumers() []*models.Consumer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*models.Consumer, 0, len(r.consumers))
	for _, c := range r.consumers {
		list = append(list, c)
	}
	return list
}
