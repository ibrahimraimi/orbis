package discovery

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"orbis/internal/models"
	"sync"
	"time"

	"go.uber.org/zap"
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

	mu        sync.RWMutex
	cache     map[string]map[string]*models.Service // name -> id -> service
	consumers map[string]*models.Consumer           // hash -> consumer
}

func NewResolver(consulAddr string) *Resolver {
	return &Resolver{
		consulAddr: consulAddr,
		client:     &http.Client{Timeout: 5 * time.Second},
		balancer:   NewRoundRobinBalancer(),
		cache:      make(map[string]map[string]*models.Service),
		consumers:  make(map[string]*models.Consumer),
	}
}

func (r *Resolver) SetBalancer(lb LoadBalancer) {
	r.balancer = lb
}

// Watch starts listening to the registry's SSE stream and updates the cache in real-time.
func (r *Resolver) Watch(ctx context.Context, logger *zap.Logger) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := r.fullSync(); err != nil {
				logger.Warn("resolver full sync failed", zap.Error(err))
				time.Sleep(2 * time.Second)
				continue
			}
			
			if err := r.fullSyncConsumers(); err != nil {
				logger.Warn("resolver consumer sync failed", zap.Error(err))
				time.Sleep(2 * time.Second)
				continue
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.consulAddr+"/v1/watch", nil)
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			req.Header.Set("Accept", "text/event-stream")

			sseClient := &http.Client{} // No timeout for SSE
			resp, err := sseClient.Do(req)
			if err != nil {
				logger.Warn("resolver SSE connect failed", zap.Error(err))
				time.Sleep(2 * time.Second)
				continue
			}

			logger.Info("Connected to registry SSE stream")

			reader := bufio.NewReader(resp.Body)
			for {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					logger.Warn("resolver SSE disconnected", zap.Error(err))
					resp.Body.Close()
					break
				}

				if len(line) == 0 || line[0] == '\n' {
					continue
				}

				if bytes.HasPrefix(line, []byte("data: ")) {
					data := bytes.TrimPrefix(line, []byte("data: "))
					var event struct {
						Type     string           `json:"type"`
						Service  *models.Service  `json:"service"`
						Consumer *models.Consumer `json:"consumer"`
						ID       string           `json:"id"`
					}
					if err := json.Unmarshal(data, &event); err != nil {
						logger.Error("failed to decode SSE event", zap.Error(err))
						continue
					}

					r.mu.Lock()
					switch event.Type {
					case "service_upsert":
						if event.Service != nil {
							if r.cache[event.Service.Name] == nil {
								r.cache[event.Service.Name] = make(map[string]*models.Service)
							}
							r.cache[event.Service.Name][event.Service.ID] = event.Service
						}
					case "service_delete":
						for name, instances := range r.cache {
							if _, ok := instances[event.ID]; ok {
								delete(instances, event.ID)
								if len(instances) == 0 {
									delete(r.cache, name)
								}
								break
							}
						}
					case "consumer_upsert":
						if event.Consumer != nil {
							r.consumers[event.Consumer.APIKeyHash] = event.Consumer
						}
					case "consumer_delete":
						for hash, c := range r.consumers {
							if c.ID == event.ID {
								delete(r.consumers, hash)
								break
							}
						}
					}
					r.mu.Unlock()
				}
			}
			time.Sleep(2 * time.Second)
		}
	}()
}

func (r *Resolver) fullSync() error {
	url := fmt.Sprintf("%s/v1/services", r.consulAddr)
	resp, err := r.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	var services []*models.Service
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return err
	}

	r.mu.Lock()
	r.cache = make(map[string]map[string]*models.Service)
	for _, s := range services {
		if r.cache[s.Name] == nil {
			r.cache[s.Name] = make(map[string]*models.Service)
		}
		r.cache[s.Name][s.ID] = s
	}
	r.mu.Unlock()

	return nil
}

func (r *Resolver) fullSyncConsumers() error {
	url := fmt.Sprintf("%s/v1/consumers", r.consulAddr)
	resp, err := r.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	var consumers []*models.Consumer
	if err := json.NewDecoder(resp.Body).Decode(&consumers); err != nil {
		return err
	}

	r.mu.Lock()
	r.consumers = make(map[string]*models.Consumer)
	for _, c := range consumers {
		r.consumers[c.APIKeyHash] = c
	}
	r.mu.Unlock()

	return nil
}

func (r *Resolver) GetConsumerByKeyHash(hash string) (*models.Consumer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.consumers[hash]
	return c, ok
}

// Resolve returns a healthy instance of the requested service name from the local cache.
func (r *Resolver) Resolve(serviceName, version string) (*models.Service, error) {
	r.mu.RLock()
	instancesMap, ok := r.cache[serviceName]
	r.mu.RUnlock()

	if !ok || len(instancesMap) == 0 {
		return nil, fmt.Errorf("no healthy instances found for service: %s (version: %s)", serviceName, version)
	}

	var instances []*models.Service
	for _, inst := range instancesMap {
		if inst.Health == models.StatusHealthy {
			instances = append(instances, inst)
		}
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
