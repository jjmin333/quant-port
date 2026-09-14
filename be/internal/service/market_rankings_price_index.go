package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
랭킹 스냅샷에서 뽑은 **종목 → 기준가·현재가** 표. 프로세스 안에 잠깐 둔다.

# 왜 필요한가

`lookupRankingBasePrice`·`lookupRankingLastPrice`는 종목 하나의 값을 찾자고 랭킹 슬롯
넷을 차례로 Redis에서 **통째로** 받아 왔다. 슬롯 하나가 100종목·18KB고(실측
2026-09-03 `--bigkeys`), 이 둘은 현재가·전일 종가를 해석하는 경로라 **목록의 종목마다**
불린다. 스무 줄짜리 목록 한 번이 최악의 경우 80번의 18KB GET — 1.4MB다.

# 왜 캐시가 정당한가

이 값의 출처는 랭킹 폴러이고, 폴러는 **5분에 한 번** 갈아 끼운다(`defaultRankingsPollIntervalFor`).
Redis 쪽 TTL도 실시간 슬롯이 2분이다. 즉 이 표는 애초에 초 단위로 움직이는 값이
아니었다 — 초 단위로 움직이는 현재가는 배치 가격 캐시(`marketStore.GetPrice`)가
가지고 있고, 이 경로는 그것이 비었을 때의 **아랫단**이다.

그래서 슬롯마다 payload 전체가 아니라 **필요한 두 숫자만** 뽑아 두고, 그 표를
`rankingPriceIndexTTL` 동안 재사용한다. 늘어나는 낡음은 최대 그 시간이고, 아끼는 것은
같은 슬롯을 종목 수만큼 다시 받아 오는 왕복이다.
*/

// rankingPriceIndexTTL — 폴러 주기(5분)와 Redis TTL(2분)보다 한참 짧게 둔다.
// 여기서 더 늘려도 아끼는 왕복은 거의 그대로이고, 낡음만 커진다.
const rankingPriceIndexTTL = 20 * time.Second

type rankingPricePair struct {
	base float64
	last float64
}

type rankingPriceIndexEntry struct {
	prices   map[string]rankingPricePair
	fetchedA time.Time
}

var (
	rankingPriceIndexMu sync.RWMutex
	rankingPriceIndex   = map[string]rankingPriceIndexEntry{}
)

// resetRankingPriceIndex clears the memo. Tests use it; nothing else should.
func resetRankingPriceIndex() {
	rankingPriceIndexMu.Lock()
	rankingPriceIndex = map[string]rankingPriceIndexEntry{}
	rankingPriceIndexMu.Unlock()
}

func rankingPriceIndexGet(key string, now time.Time) (map[string]rankingPricePair, bool) {
	rankingPriceIndexMu.RLock()
	entry, ok := rankingPriceIndex[key]
	rankingPriceIndexMu.RUnlock()
	if !ok || now.Sub(entry.fetchedA) > rankingPriceIndexTTL {
		return nil, false
	}
	return entry.prices, true
}

func rankingPriceIndexPut(key string, prices map[string]rankingPricePair, now time.Time) {
	rankingPriceIndexMu.Lock()
	rankingPriceIndex[key] = rankingPriceIndexEntry{prices: prices, fetchedA: now}
	rankingPriceIndexMu.Unlock()
}

// buildRankingPriceIndex reduces a ranking payload to the two numbers the
// lookups need. Rows with neither price are dropped — keeping them would only
// grow the map with entries every caller has to skip anyway.
func buildRankingPriceIndex(payload rankingsStorePayload) map[string]rankingPricePair {
	out := make(map[string]rankingPricePair, len(payload.Rankings))
	for _, item := range payload.Rankings {
		sym := strings.ToUpper(strings.TrimSpace(item.Symbol))
		if sym == "" {
			continue
		}
		pair := rankingPricePair{
			base: parseDecimal(item.Price.BasePrice),
			last: parseDecimal(item.Price.LastPrice),
		}
		if pair.base <= 0 && pair.last <= 0 {
			continue
		}
		// 같은 심볼이 두 번 나오면 먼저 온 쪽(=순위가 높은 쪽)을 남긴다.
		if _, dup := out[sym]; dup {
			continue
		}
		out[sym] = pair
	}
	return out
}

/*
rankingPricesForSlot은 슬롯 하나의 가격 표를 돌려준다 — 캐시에 있으면 Redis를 건드리지
않는다.

Redis에서 못 읽으면 **빈 표를 캐시한다**. 그래야 슬롯이 비어 있는 동안(장 시작 전,
폴러가 아직 한 바퀴 못 돈 때) 종목마다 같은 헛걸음을 반복하지 않는다 — 헛걸음도
왕복이다.
*/
func rankingPricesForSlot(ctx context.Context, rdb *redis.Client, key string) map[string]rankingPricePair {
	now := time.Now()
	if prices, ok := rankingPriceIndexGet(key, now); ok {
		return prices
	}
	prices := map[string]rankingPricePair{}
	if rdb != nil {
		if raw, err := rdb.Get(ctx, key).Bytes(); err == nil && len(raw) > 0 {
			if payload, ok := decodeRankingsStore(raw); ok {
				prices = buildRankingPriceIndex(payload)
			}
		}
	}
	rankingPriceIndexPut(key, prices, now)
	return prices
}
