package registry

import (
	"orbis/internal/models"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistry_Register(t *testing.T) {
	reg, _ := NewRegistry(nil)
	svc := &models.Service{
		ID:      "test-1",
		Name:    "user-service",
		Address: "127.0.0.1",
		Port:    8080,
	}

	err := reg.Register(svc)
	assert.NoError(t, err)

	saved, ok := reg.GetService("test-1")
	assert.True(t, ok)
	assert.Equal(t, "user-service", saved.Name)
	assert.Equal(t, models.StatusHealthy, saved.Health)
	assert.NotZero(t, saved.CreatedAt)
	assert.NotZero(t, saved.UpdatedAt)
}

func TestRegistry_Deregister(t *testing.T) {
	reg, _ := NewRegistry(nil)
	svc := &models.Service{ID: "test-1"}
	_ = reg.Register(svc)

	err := reg.Deregister("test-1")
	assert.NoError(t, err)

	_, ok := reg.GetService("test-1")
	assert.False(t, ok)
}

func TestRegistry_GetHealthyServicesByName(t *testing.T) {
	reg, _ := NewRegistry(nil)
	_ = reg.Register(&models.Service{ID: "s1", Name: "api", Health: models.StatusHealthy})
	_ = reg.Register(&models.Service{ID: "s2", Name: "api", Health: models.StatusHealthy})
	_ = reg.Register(&models.Service{ID: "s3", Name: "other", Health: models.StatusHealthy})

	// Manually set s2 to unhealthy to test filtering
	_ = reg.UpdateHealth("s2", models.StatusUnhealthy)

	healthy := reg.GetHealthyServicesByName("api")
	assert.Len(t, healthy, 1)
	assert.Equal(t, "s1", healthy[0].ID)
}

func TestRegistry_Persistence(t *testing.T) {
	dbPath := "test.db"
	defer os.Remove(dbPath)

	store, err := NewBoltStore(dbPath)
	assert.NoError(t, err)
	defer store.Close()

	reg, err := NewRegistry(store)
	assert.NoError(t, err)

	svc := &models.Service{ID: "p1", Name: "persisted"}
	err = reg.Register(svc)
	assert.NoError(t, err)

	// Create a new registry instance with the same store to verify persistence
	reg2, err := NewRegistry(store)
	assert.NoError(t, err)

	saved, ok := reg2.GetService("p1")
	assert.True(t, ok)
	assert.Equal(t, "persisted", saved.Name)
}
