package toss

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisTokenStore struct {
	client *redis.Client
}

func NewRedisTokenStore(client *redis.Client) *RedisTokenStore {
	return &RedisTokenStore{client: client}
}

// NewTokenStore returns a Redis-first token store with in-memory fallback.
func NewTokenStore(client *redis.Client) TokenStore {
	return NewResilientTokenStore(client)
}

func (s *RedisTokenStore) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

func (s *RedisTokenStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisTokenStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *RedisTokenStore) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, "1", ttl).Result()
}

func (s *RedisTokenStore) Unlock(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
