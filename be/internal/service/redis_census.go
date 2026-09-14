package service

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/enact/quant-port/be/internal/metrics"
)

/*
Redis 키 센서스 — TTL 없는 키가 실질 캐시 용량을 얼마나 잠식하는지 센다.

**왜 이게 지표여야 하나.** `md:rev:*`는 캐시 무효화 카운터라 **의도적으로 TTL이
없다** — 사라지면 낡은 캐시가 새 것으로 보인다. 문제는 상한이 없다는 것이다:
심볼 17,912개가 전부 조회되면 그만큼 영구 키가 쌓인다.

그리고 `maxmemory`에 닿는 순간 `volatile-lru`는 **TTL 있는 키만** 버린다.
즉 영구 키 비중이 곧 **실질 캐시 용량의 잠식률**이고, 그 비율이 커질수록
같은 메모리로 담을 수 있는 캐시가 줄어든다. 실측(2026-08-23) 전체 28,121키 중
3,675개(13%)가 TTL 없음, `used_memory_peak` 592MB / `maxmemory` 1GB.

지금 조치할 필요는 없다 — 약 123KB로 무해하다. **감시만 하면 된다.**

`SCAN`은 커서 기반이라 Redis를 막지 않지만 그래도 전량 순회다. 그래서
주기를 길게(1시간) 두고 한 번에 도는 양도 제한한다.
*/
const (
	redisCensusInterval  = 1 * time.Hour
	redisCensusScanCount = 500
	// redisCensusMaxKeys — 한 번의 센서스가 훑을 키 수 상한. 키가 수십만 개가
	// 되어도 이 루프가 Redis를 오래 붙들지 않게 한다. 표본이지만 비율을 보는
	// 지표라 전량이 아니어도 의미가 유지된다.
	redisCensusMaxKeys = 50000
)

// RunRedisCensus periodically records how many keys lack a TTL.
func RunRedisCensus(ctx context.Context, rdb *redis.Client) {
	if rdb == nil {
		return
	}
	censusOnce(ctx, rdb)

	ticker := time.NewTicker(redisCensusInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			censusOnce(ctx, rdb)
		}
	}
}

func censusOnce(ctx context.Context, rdb *redis.Client) {
	scanCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var (
		cursor    uint64
		total     int64
		persisted int64
	)
	for {
		keys, next, err := rdb.Scan(scanCtx, cursor, "*", redisCensusScanCount).Result()
		if err != nil {
			log.Printf("redis census: scan: %v", err)
			return
		}

		// TTL을 키마다 따로 물으면 왕복이 키 수만큼 난다. 파이프라인으로 묶는다.
		pipe := rdb.Pipeline()
		cmds := make([]*redis.DurationCmd, 0, len(keys))
		for _, k := range keys {
			cmds = append(cmds, pipe.TTL(scanCtx, k))
		}
		if _, err := pipe.Exec(scanCtx); err != nil && err != redis.Nil {
			log.Printf("redis census: ttl pipeline: %v", err)
			return
		}
		for _, cmd := range cmds {
			// go-redis는 TTL을 duration으로 준다: **-1ns = 만료 없음**,
			// -2ns = 그 사이 사라진 키. 후자는 세지 않는다 — 비율이 왜곡된다.
			switch cmd.Val() {
			case -2 * time.Nanosecond:
				continue
			case -1 * time.Nanosecond:
				persisted++
			}
			total++
		}

		cursor = next
		if cursor == 0 || total >= redisCensusMaxKeys {
			break
		}
	}

	metrics.SetRedisKeyCounts(total, persisted)
	if total > 0 {
		log.Printf("redis census: %d keys, %d without TTL (%.1f%%)",
			total, persisted, float64(persisted)*100/float64(total))
	}
}
