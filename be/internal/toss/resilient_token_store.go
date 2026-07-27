package toss

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// ResilientTokenStore keeps tokens in memory and mirrors to Redis when available.
type ResilientTokenStore struct {
	redis *redis.Client
	mu    sync.RWMutex
	mem   map[string]cacheEntry
}

func NewResilientTokenStore(client *redis.Client) *ResilientTokenStore {
	return &ResilientTokenStore{
		redis: client,
		mem:   make(map[string]cacheEntry),
	}
}

func (s *ResilientTokenStore) Get(ctx context.Context, key string) ([]byte, error) {
	if s == nil {
		return nil, nil
	}
	if s.redis != nil {
		val, err := s.redis.Get(ctx, key).Bytes()
		if err == nil && len(val) > 0 {
			s.storeMem(key, val, 0)
			return val, nil
		}
		if err != nil && err != redis.Nil {
			// Redis error — fall through to memory
		}
	}
	s.mu.RLock()
	entry, ok := s.mem[key]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		s.Delete(ctx, key)
		return nil, nil
	}
	out := make([]byte, len(entry.value))
	copy(out, entry.value)
	return out, nil
}

func (s *ResilientTokenStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s == nil {
		return nil
	}
	s.storeMem(key, value, ttl)
	if s.redis != nil {
		if err := s.redis.Set(ctx, key, value, ttl).Err(); err != nil {
			return nil
		}
	}
	return nil
}

func (s *ResilientTokenStore) Delete(ctx context.Context, key string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	delete(s.mem, key)
	s.mu.Unlock()
	if s.redis != nil {
		_ = s.redis.Del(ctx, key).Err()
	}
	return nil
}

func (s *ResilientTokenStore) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if s == nil {
		return true, nil
	}
	if s.redis != nil {
		ok, err := s.redis.SetNX(ctx, key, "1", ttl).Result()
		if err == nil {
			return ok, nil
		}
	}
	// Without Redis, allow lock (single-instance dev) — duplicate refresh possible.
	return true, nil
}

func (s *ResilientTokenStore) Unlock(ctx context.Context, key string) error {
	return s.Delete(ctx, key)
}

func (s *ResilientTokenStore) storeMem(key string, value []byte, ttl time.Duration) {
	if key == "" || len(value) == 0 {
		return
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	entry := cacheEntry{value: cp}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	s.mu.Lock()
	s.mem[key] = entry
	s.mu.Unlock()
}
