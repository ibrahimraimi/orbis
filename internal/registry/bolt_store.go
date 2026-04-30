package registry

import (
	"encoding/json"
	"fmt"
	"orbis/internal/models"

	"go.etcd.io/bbolt"
)

var (
	bucketName          = []byte("services")
	consumersBucketName = []byte("consumers")
)

type BoltStore struct {
	db *bbolt.DB
}

func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt db: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketName); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(consumersBucketName); err != nil {
			return err
		}
		return nil
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

func (s *BoltStore) SaveConsumer(consumer *models.Consumer) error {
	data, err := json.Marshal(consumer)
	if err != nil {
		return fmt.Errorf("failed to marshal consumer: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(consumersBucketName)
		return b.Put([]byte(consumer.ID), data)
	})
}

func (s *BoltStore) DeleteConsumer(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(consumersBucketName)
		return b.Delete([]byte(id))
	})
}

func (s *BoltStore) LoadAllConsumers() ([]*models.Consumer, error) {
	var consumers []*models.Consumer

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(consumersBucketName)
		return b.ForEach(func(k, v []byte) error {
			var consumer models.Consumer
			if err := json.Unmarshal(v, &consumer); err != nil {
				return err
			}
			consumers = append(consumers, &consumer)
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load consumers: %w", err)
	}

	return consumers, nil
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}
