package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"orbis/internal/models"

	"github.com/redis/go-redis/v9"
)

const redisHashKey = "orbis:services"

type RedisStore struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisStore(addr string) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx := context.Background()
	// Ping to verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis at %s: %w", addr, err)
	}

	return &RedisStore{
		client: client,
		ctx:    ctx,
	}, nil
}

func (s *RedisStore) Save(service *models.Service) error {
	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service: %w", err)
	}

	err = s.client.HSet(s.ctx, redisHashKey, service.ID, data).Err()
	if err != nil {
		return fmt.Errorf("failed to save service to redis: %w", err)
	}
	return nil
}

func (s *RedisStore) Delete(id string) error {
	err := s.client.HDel(s.ctx, redisHashKey, id).Err()
	if err != nil {
		return fmt.Errorf("failed to delete service from redis: %w", err)
	}
	return nil
}

func (s *RedisStore) LoadAll() ([]*models.Service, error) {
	res, err := s.client.HGetAll(s.ctx, redisHashKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to load services from redis: %w", err)
	}

	var services []*models.Service
	for _, v := range res {
		var service models.Service
		if err := json.Unmarshal([]byte(v), &service); err != nil {
			return nil, fmt.Errorf("failed to unmarshal service from redis: %w", err)
		}
		services = append(services, &service)
	}

	return services, nil
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}
