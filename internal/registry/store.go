package registry

import (
	"orbis/internal/models"
)

type Store interface {
	Save(service *models.Service) error
	Delete(id string) error
	LoadAll() ([]*models.Service, error)

	SaveConsumer(consumer *models.Consumer) error
	DeleteConsumer(id string) error
	LoadAllConsumers() ([]*models.Consumer, error)

	Close() error
}
