package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/redis/go-redis/v9"
)

/*
원본 봉 배열의 짧은 기억 (2026-09-03).

지키려는 것: **291KB짜리 배열을 종목마다 매 틱 다시 받지 않는다.** 형성봉을 떼어낸
뒤 Redis 출력의 대부분이 이 키였다.
*/

func newRawTailService(t *testing.T) (*QuotesService, *miniredis.Miniredis) {
	t.Helper()
	resetRawTailMemo()
	t.Cleanup(resetRawTailMemo)
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return &QuotesService{redis: client}, mr
}

// 두 번째 읽기는 Redis에 가지 않는다 — 키를 지운 뒤에도 값이 나온다.
func TestLoadRawCandlesReusesTheMemo(t *testing.T) {
	s, mr := newRawTailService(t)
	ctx := context.Background()
	key := "md:raw:1m:000660"
	_ = mr.Set(key, `[{"time":"2026-09-03T09:00:00+09:00","open":1,"high":2,"low":1,"close":2,"volume":10}]`)

	got, ok := loadRawCandles(ctx, s.redis, key)
	if !ok || len(got) != 1 || got[0].Close != 2 {
		t.Fatalf("first read = %v/%v", got, ok)
	}

	mr.Del(key)

	got, ok = loadRawCandles(ctx, s.redis, key)
	if !ok || len(got) != 1 {
		t.Fatal("second read went back to redis")
	}
}

// 없다는 사실도 기억한다 — 목록이 종목 수만큼 헛걸음을 반복하지 않게.
func TestLoadRawCandlesRemembersAMiss(t *testing.T) {
	s, mr := newRawTailService(t)
	ctx := context.Background()
	key := "md:raw:1m:000660"

	if _, ok := loadRawCandles(ctx, s.redis, key); ok {
		t.Fatal("empty redis reported a hit")
	}
	_ = mr.Set(key, `[{"time":"t","close":1}]`)
	if _, ok := loadRawCandles(ctx, s.redis, key); ok {
		t.Fatal("the miss was not remembered")
	}

	resetRawTailMemo()
	if _, ok := loadRawCandles(ctx, s.redis, key); !ok {
		t.Fatal("after expiry the value should be read again")
	}
}

/*
쓰는 쪽은 **자기가 쓴 값을 곧바로 본다.**

`storeRawCandles`가 기억을 갈아 끼우지 않으면, 갱신 직후 최대 TTL만큼 예전 배열이
읽힌다 — 오늘의 고저·거래량이 그 시간 동안 뒤로 간다.
*/
func TestStoreRawCandlesRefreshesTheMemo(t *testing.T) {
	s, _ := newRawTailService(t)
	ctx := context.Background()
	key := "md:raw:1m:000660"

	s.storeRawCandles(ctx, key, []model.Candle{{Time: "t1", Close: 100}}, time.Minute)
	if got, _ := loadRawCandles(ctx, s.redis, key); len(got) != 1 || got[0].Close != 100 {
		t.Fatalf("after first write = %v", got)
	}

	s.storeRawCandles(ctx, key, []model.Candle{{Time: "t1", Close: 100}, {Time: "t2", Close: 111}}, time.Minute)
	got, ok := loadRawCandles(ctx, s.redis, key)
	if !ok || len(got) != 2 || got[1].Close != 111 {
		t.Fatalf("writer read back a stale array: %v", got)
	}
}

// redis가 없으면 조용히 실패한다.
func TestLoadRawCandlesToleratesNilRedis(t *testing.T) {
	resetRawTailMemo()
	if _, ok := loadRawCandles(context.Background(), nil, "md:raw:1m:000660"); ok {
		t.Fatal("reported a hit with no redis")
	}
}
