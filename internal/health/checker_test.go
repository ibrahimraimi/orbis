package health

import (
	"net/http"
	"net/http/httptest"
	"orbis/internal/models"
	"orbis/internal/registry"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestChecker_PingHTTP(t *testing.T) {
	logger := zap.NewNop()
	reg, _ := registry.NewRegistry(nil)
	checker := NewChecker(reg, logger, 1*time.Second, 500*time.Millisecond)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Extract address and port from server URL
	url := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(url, ":")
	addr := parts[0]
	port, _ := strconv.Atoi(parts[1])

	svc := &models.Service{
		ID:      "test-http",
		Name:    "test-service",
		Address: addr,
		Port:    port,
	}
	_ = reg.Register(svc)

	err := checker.ping(svc)
	assert.NoError(t, err)

	// Test failing health check
	svcFail := &models.Service{
		ID:      "test-fail",
		Name:    "fail-service",
		Address: addr,
		Port:    port,
		Meta:    map[string]string{"health_check_path": "/missing"},
	}
	err = checker.ping(svcFail)
	assert.Error(t, err)
}

func TestChecker_CheckAll(t *testing.T) {
	logger := zap.NewNop()
	reg, _ := registry.NewRegistry(nil)
	checker := NewChecker(reg, logger, 100*time.Millisecond, 50*time.Millisecond)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	url := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(url, ":")
	addr := parts[0]
	port, _ := strconv.Atoi(parts[1])

	svc := &models.Service{
		ID:      "s1",
		Address: addr,
		Port:    port,
	}
	_ = reg.Register(svc)

	// Initially healthy after register
	s, _ := reg.GetService("s1")
	assert.Equal(t, models.StatusHealthy, s.Health)

	// Run checkAll
	checker.checkAll()

	// Wait for async update
	time.Sleep(200 * time.Millisecond)

	s, _ = reg.GetService("s1")
	assert.Equal(t, models.StatusHealthy, s.Health)
}
