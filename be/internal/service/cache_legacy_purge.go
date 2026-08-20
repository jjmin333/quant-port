package service

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

var legacyTerminalCachePatterns = []string{
	"quotes:detail:last:*",
	"quotes:book:last:*",
	"quotes:trades:last:*",
	"market:price:last:v1:*",
	"quotes:detail:*",
}

// PurgeLegacyTerminalCacheKeys removes deprecated Redis keys after cache migrations.
func PurgeLegacyTerminalCacheKeys(ctx context.Context, rdb *redis.Client) int {
	if rdb == nil {
		return 0
	}
	deleted := 0
	for _, pattern := range legacyTerminalCachePatterns {
		deleted += purgeRedisScanKeys(ctx, rdb, pattern)
	}
	if deleted > 0 {
		log.Printf("cache: purged %d legacy terminal cache keys", deleted)
	}
	return deleted
}

func purgeRedisScanKeys(ctx context.Context, rdb *redis.Client, pattern string) int {
	var cursor uint64
	deleted := 0
	for {
		keys, next, err := rdb.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			log.Printf("cache: purge scan %q: %v", pattern, err)
			return deleted
		}
		if len(keys) > 0 {
			n, err := rdb.Del(ctx, keys...).Result()
			if err != nil {
				log.Printf("cache: purge del %q: %v", pattern, err)
				return deleted
			}
			deleted += int(n)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return deleted
}
