package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"orbis/internal/models"
	"sync"
	"time"
)

// Resolver discovers healthy service instances from Consul.
type Resolver struct {
	consulAddr string
	client     *http.Client
	mu         sync.Mutex
	indices    map[string]int
}

// NewResolver creates a new Resolver.
func NewResolver(consulAddr string) *Resolver {
	return &Resolver{
		consulAddr: consulAddr,
		client:     &http.Client{Timeout: 5 * time.Second},
		indices:    make(map[string]int),
	}
}

// Resolve returns a healthy instance of the requested service name using Round-Robin.
func (r *Resolver) Resolve(serviceName string) (*models.Service, error) {
	instances, err := r.fetchHealthyInstances(serviceName)
	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instances found for service: %s", serviceName)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := r.indices[serviceName]
	instance := instances[idx%len(instances)]
	r.indices[serviceName] = (idx + 1) % len(instances)

	return instance, nil
}

func (r *Resolver) fetchHealthyInstances(name string) ([]*models.Service, error) {
	url := fmt.Sprintf("%s/v1/services/%s", r.consulAddr, name)
	resp, err := r.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to contact consul: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("consul returned status: %d", resp.StatusCode)
	}

	var instances []*models.Service
	if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
		return nil, fmt.Errorf("failed to decode consul response: %w", err)
	}

	return instances, nil
}
