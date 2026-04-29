package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"orbis/internal/models"
	"sync"
	"time"
)

type LoadBalancer interface {
	Next(instances []*models.Service) (*models.Service, error)
}

type RoundRobinBalancer struct {
	mu      sync.Mutex
	indices map[string]int
}

func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		indices: make(map[string]int),
	}
}

func (r *RoundRobinBalancer) Next(instances []*models.Service) (*models.Service, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instances available")
	}

	serviceName := instances[0].Name

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := r.indices[serviceName]
	instance := instances[idx%len(instances)]
	r.indices[serviceName] = (idx + 1) % len(instances)

	return instance, nil
}

type Resolver struct {
	consulAddr string
	client     *http.Client
	balancer   LoadBalancer
}

func NewResolver(consulAddr string) *Resolver {
	return &Resolver{
		consulAddr: consulAddr,
		client:     &http.Client{Timeout: 5 * time.Second},
		balancer:   NewRoundRobinBalancer(),
	}
}

func (r *Resolver) SetBalancer(lb LoadBalancer) {
	r.balancer = lb
}

// Resolve returns a healthy instance of the requested service name using the configured LoadBalancer.
func (r *Resolver) Resolve(serviceName, version string) (*models.Service, error) {
	instances, err := r.fetchHealthyInstances(serviceName)
	if err != nil {
		return nil, err
	}

	if version != "" {
		instances = filterByVersion(instances, version)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instances found for service: %s (version: %s)", serviceName, version)
	}

	return r.balancer.Next(instances)
}

func filterByVersion(instances []*models.Service, version string) []*models.Service {
	var filtered []*models.Service
	tag := "version:" + version
	for _, inst := range instances {
		for _, t := range inst.Tags {
			if t == tag {
				filtered = append(filtered, inst)
				break
			}
		}
	}
	return filtered
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
