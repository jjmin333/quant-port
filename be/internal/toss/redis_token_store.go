package toss

import (
	"github.com/redis/go-redis/v9"
)

// NewTokenStore returns a Redis-first token store with in-memory fallback.
func NewTokenStore(client *redis.Client) TokenStore {
	return NewResilientTokenStore(client)
}
