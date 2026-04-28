package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"orbis/internal/models"
	"orbis/internal/registry"
	"time"

	"go.uber.org/zap"
)

// Checker handles periodic health checks for registered services.
type Checker struct {
	registry *registry.Registry
	logger   *zap.Logger
	interval time.Duration
	timeout  time.Duration
}

func NewChecker(reg *registry.Registry, logger *zap.Logger, interval, timeout time.Duration) *Checker {
	return &Checker{
		registry: reg,
		logger:   logger,
		interval: interval,
		timeout:  timeout,
	}
}

func (c *Checker) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.logger.Info("Starting health checker", zap.Duration("interval", c.interval))

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping health checker")
			return
		case <-ticker.C:
			c.checkAll()
		}
	}
}

func (c *Checker) checkAll() {
	services := c.registry.ListServices()
	for _, s := range services {
		go c.checkService(s)
	}
}

func (c *Checker) checkService(s *models.Service) {
	// Simple logic: if no health check meta is provided, assume HTTP on /health
	// In a real system, this would be more configurable.
	
	status := models.StatusHealthy
	err := c.ping(s)
	if err != nil {
		c.logger.Warn("Health check failed", zap.String("id", s.ID), zap.Error(err))
		status = models.StatusUnhealthy
	}

	if err := c.registry.UpdateHealth(s.ID, status); err != nil {
		c.logger.Error("Failed to update health status", zap.String("id", s.ID), zap.Error(err))
	}
}

func (c *Checker) ping(s *models.Service) error {
	address := fmt.Sprintf("%s:%d", s.Address, s.Port)
	
	// Check if it's TCP or HTTP based on tags or meta
	isTCP := false
	for _, tag := range s.Tags {
		if tag == "protocol:tcp" {
			isTCP = true
			break
		}
	}

	if isTCP {
		return c.pingTCP(address)
	}
	
	return c.pingHTTP(s, address)
}

func (c *Checker) pingTCP(address string) error {
	conn, err := net.DialTimeout("tcp", address, c.timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

func (c *Checker) pingHTTP(s *models.Service, address string) error {
	path := s.Meta["health_check_path"]
	if path == "" {
		path = "/health"
	}
	
	url := fmt.Sprintf("http://%s%s", address, path)
	
	client := &http.Client{
		Timeout: c.timeout,
	}
	
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}
