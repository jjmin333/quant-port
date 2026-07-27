package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(url string) (*Redis, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	r := &Redis{client: client}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return r, nil
}

func (r *Redis) Client() *redis.Client {
	if r == nil {
		return nil
	}
	return r.client
}

func (r *Redis) Ping(ctx context.Context) error {
	if r == nil || r.client == nil {
		return redis.ErrClosed
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Status(ctx context.Context) string {
	if r == nil || r.client == nil {
		return "unavailable"
	}
	if err := r.Ping(ctx); err != nil {
		return "degraded: " + err.Error()
	}
	return "ok"
}

func (r *Redis) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}
