package registry

import (
	"orbis/internal/models"
)

type Store interface {
	Save(service *models.Service) error
	Delete(id string) error
	LoadAll() ([]*models.Service, error)
	Close() error
}
