package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

/*
랭킹 가격 표 회귀 시험 (2026-09-03).

지키려는 것: 종목 하나를 찾자고 18KB짜리 랭킹 payload를 **다시 받아 오지 않는다**.
같은 슬롯을 여러 종목이 물어보면 첫 번째만 Redis에 간다.
*/

func rankingPayloadJSON(t *testing.T, items ...toss.RankingItem) string {
	t.Helper()
	raw, err := json.Marshal(rankingsStorePayload{
		RefreshedAt: "2026-09-03T05:00:00Z",
		Rankings:    items,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func rankingItem(rank int, symbol, last, base string) toss.RankingItem {
	return toss.RankingItem{
		Rank:   rank,
		Symbol: symbol,
		Price:  toss.RankingPrice{LastPrice: last, BasePrice: base},
	}
}

func newRankingIndexRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	resetRankingPriceIndex()
	t.Cleanup(resetRankingPriceIndex)
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

func TestBuildRankingPriceIndexKeepsFirstRowAndDropsEmptyPrices(t *testing.T) {
	idx := buildRankingPriceIndex(rankingsStorePayload{Rankings: []toss.RankingItem{
		rankingItem(1, "000660", "1619000", "1612000"),
		rankingItem(2, "005930", "", ""), // 값이 없는 행은 표에 넣을 이유가 없다
		rankingItem(3, "000660", "999", "888"), // 중복이면 순위가 높은 앞줄이 남는다
		rankingItem(4, " spy ", "761.68", "760.10"),
	}})

	if len(idx) != 2 {
		t.Fatalf("index size = %d, want 2 (%v)", len(idx), idx)
	}
	if got := idx["000660"]; got.last != 1619000 || got.base != 1612000 {
		t.Fatalf("000660 = %+v, want last 1619000 / base 1612000", got)
	}
	// 심볼은 공백을 털고 대문자로 — 조회 쪽이 그렇게 찾는다.
	if got := idx["SPY"]; got.last != 761.68 {
		t.Fatalf("SPY = %+v, want last 761.68", got)
	}
}

/*
같은 슬롯을 두 번째로 물어보면 **Redis에 가지 않는다**.

키를 지운 뒤에도 값이 나오는 것으로 확인한다 — 다시 갔다면 빈손으로 돌아왔을 것이다.
*/
func TestRankingPriceLookupReusesTheSlotWithoutRefetching(t *testing.T) {
	rdb, mr := newRankingIndexRedis(t)
	ctx := context.Background()
	key := rankingsCacheKey(rankingBasePriceSlots[0])
	_ = mr.Set(key, rankingPayloadJSON(t, rankingItem(1, "000660", "1619000", "1612000")))

	if v, ok := lookupRankingBasePrice(ctx, rdb, nil, "000660"); !ok || v != 1612000 {
		t.Fatalf("first lookup = %v/%v, want 1612000/true", v, ok)
	}

	mr.Del(key)

	if v, ok := lookupRankingBasePrice(ctx, rdb, nil, "000660"); !ok || v != 1612000 {
		t.Fatalf("second lookup went back to redis: %v/%v", v, ok)
	}
	if v, ok := lookupRankingLastPrice(ctx, rdb, nil, "000660"); !ok || v != 1619000 {
		t.Fatalf("lastPrice lookup = %v/%v, want 1619000/true", v, ok)
	}
}

// 표에 없는 종목은 없다고 답한다 — 캐시가 있다고 없는 값을 지어내면 안 된다.
func TestRankingPriceLookupMissIsAMiss(t *testing.T) {
	rdb, mr := newRankingIndexRedis(t)
	ctx := context.Background()
	_ = mr.Set(rankingsCacheKey(rankingBasePriceSlots[0]),
		rankingPayloadJSON(t, rankingItem(1, "000660", "1619000", "1612000")))

	if v, ok := lookupRankingBasePrice(ctx, rdb, nil, "005930"); ok {
		t.Fatalf("unlisted symbol resolved to %v", v)
	}
}

/*
슬롯이 **비어 있어도** 그 사실을 기억한다.

장 시작 전이나 폴러가 아직 한 바퀴 못 돈 때가 이 경우인데, 기억해 두지 않으면
목록의 종목마다 같은 헛걸음을 네 번씩 반복한다.
*/
func TestRankingPriceLookupRemembersAnEmptySlot(t *testing.T) {
	rdb, mr := newRankingIndexRedis(t)
	ctx := context.Background()
	key := rankingsCacheKey(rankingBasePriceSlots[0])

	if _, ok := lookupRankingBasePrice(ctx, rdb, nil, "000660"); ok {
		t.Fatal("empty redis resolved a price")
	}

	// 이제 값을 넣어도, 유예 안에서는 아까 본 「없음」이 그대로다.
	_ = mr.Set(key, rankingPayloadJSON(t, rankingItem(1, "000660", "1619000", "1612000")))
	if _, ok := lookupRankingBasePrice(ctx, rdb, nil, "000660"); ok {
		t.Fatal("empty-slot result was not cached")
	}

	// 유예가 지나면 새로 읽는다.
	resetRankingPriceIndex()
	if v, ok := lookupRankingBasePrice(ctx, rdb, nil, "000660"); !ok || v != 1612000 {
		t.Fatalf("after expiry = %v/%v, want 1612000/true", v, ok)
	}
}
