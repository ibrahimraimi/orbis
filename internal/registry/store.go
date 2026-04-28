package registry

import (
	"encoding/json"
	"fmt"
	"orbis/internal/models"

	"go.etcd.io/bbolt"
)

var bucketName = []byte("services")

type Store interface {
	Save(service *models.Service) error
	Delete(id string) error
	LoadAll() ([]*models.Service, error)
	Close() error
}

type BoltStore struct {
	db *bbolt.DB
}

func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt db: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	return &BoltStore{db: db}, nil
}

func (s *BoltStore) Save(service *models.Service) error {
	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Put([]byte(service.ID), data)
	})
}

func (s *BoltStore) Delete(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Delete([]byte(id))
	})
}

func (s *BoltStore) LoadAll() ([]*models.Service, error) {
	var services []*models.Service

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.ForEach(func(k, v []byte) error {
			var service models.Service
			if err := json.Unmarshal(v, &service); err != nil {
				return err
			}
			services = append(services, &service)
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load services: %w", err)
	}

	return services, nil
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}
