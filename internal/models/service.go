package models

import "time"

type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusCritical  HealthStatus = "critical"
)

// Service represents a registered service instance in the Orbis registry.
type Service struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Address   string            `json:"address"`
	Port      int               `json:"port"`
	Tags      []string          `json:"tags"`
	Meta      map[string]string `json:"meta"`
	Health    HealthStatus      `json:"health"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}
