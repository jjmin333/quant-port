package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/redis/go-redis/v9"
)

const (
	quotesTerminalKeyFmt = "quotes:terminal:%s"
	quotesBookKeyFmt     = "quotes:book:%s"
	quotesTradesKeyFmt   = "quotes:trades:%s"
	quotesListKeyFmt     = "quotes:list:v2:%s:%s:%d:%d" // catalog lists only; watchlist/held bypass cache
	mdSnapKeyFmt         = "md:snap:%s:%s"
	mdFormKeyFmt         = "md:form:%s:%s"
	mdRevKeyFmt          = "md:rev:%s"
	mdRaw1mKeyFmt        = "md:raw:1m:%s"
	mdRaw1dKeyFmt        = "md:raw:1d:%s"
)

func (s *QuotesService) cacheGetJSON(ctx context.Context, key string, dest any) bool {
	if s.redis == nil {
		return false
	}
	raw, err := s.redis.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dest) == nil
}

// cacheKeyExists asks **whether** a key is there without dragging its value
// across the wire. `md:raw:1m:*` is ~291KB (실측 000660) and two call sites only
// wanted to know that it had been warmed.
func (s *QuotesService) cacheKeyExists(ctx context.Context, key string) bool {
	if s.redis == nil {
		return false
	}
	n, err := s.redis.Exists(ctx, key).Result()
	return err == nil && n > 0
}

func (s *QuotesService) cacheSetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, key, payload, ttl).Err()
}

func (s *QuotesService) bumpRev(ctx context.Context, symbol string) int64 {
	if s.redis == nil {
		return time.Now().UnixNano()
	}
	n, err := s.redis.Incr(ctx, fmt.Sprintf(mdRevKeyFmt, symbol)).Result()
	if err != nil {
		return time.Now().UnixNano()
	}
	return n
}

func (s *QuotesService) getRev(ctx context.Context, symbol string) int64 {
	if s.redis == nil {
		return 0
	}
	n, err := s.redis.Get(ctx, fmt.Sprintf(mdRevKeyFmt, symbol)).Int64()
	if err != nil {
		return 0
	}
	return n
}

/*
형성봉은 **스냅샷과 다른 키에 산다** (`md:form:종목:간격`).

왜: 형성봉은 2초마다 바뀌는데 스냅샷은 봉이 닫힐 때만 바뀐다. 둘이 한 칸에 있으면
`refreshFormingBars`가 봉 하나를 고치자고 **수백 봉짜리 JSON을 통째로 읽고 통째로 쓴다**.
실측(2026-09-03, MONITOR 3초): 종목 하나를 열어 둔 것만으로 `md:snap` GET 109회·SET 108회
— 초당 36회다. Redis 전체가 5.5MB/s in · 6.0MB/s out이었고 그 대부분이 이 경로였다.

나눠 두면 같은 갱신이 **수백 바이트짜리 키 하나**를 만진다. 스냅샷 쪽은 봉이 닫힐 때만
쓰이므로, 초당 트래픽에서 큰 payload가 통째로 빠진다.

읽는 쪽은 `loadCandleSnapshot`이 둘을 MGET 한 번으로 받아 합친다 — 왕복 수는 그대로다.
형성봉만 필요한 자리(현재가 해석·형성봉 갱신)는 작은 키만 읽는다.
*/
func (s *QuotesService) cacheCandleSnapshot(ctx context.Context, snap model.CandleSnapshot) {
	ttl := candleSnapshotTTL(snap.Source)
	forming := snap.Forming
	// `snap`은 값 복사본이라 호출자의 구조체는 건드리지 않는다.
	snap.Forming = nil
	s.cacheSetJSON(ctx, fmt.Sprintf(mdSnapKeyFmt, snap.Symbol, snap.Interval), snap, ttl)
	// 형성봉이 없으면 `null`을 적는다 — 지우지 않고 덮어써야, 장이 닫힌 뒤에도
	// 예전 형성봉이 스냅샷에 다시 붙는 일이 없다.
	s.cacheSetJSON(ctx, fmt.Sprintf(mdFormKeyFmt, snap.Symbol, snap.Interval), forming, ttl)
}

func candleSnapshotTTL(source string) time.Duration {
	if source == "db" {
		return time.Duration(model.MarketDataCandleHistCacheSec) * time.Second
	}
	return time.Duration(model.MarketDataCandleTailCacheSec) * time.Second
}

// loadFormingCandle reads only the forming bar. Returns false when the key is
// absent — a stored `null` (market closed) reads as (nil, true).
func (s *QuotesService) loadFormingCandle(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
) (*model.Candle, bool) {
	if s.redis == nil {
		return nil, false
	}
	raw, err := s.redis.Get(ctx, fmt.Sprintf(mdFormKeyFmt, symbol, interval)).Bytes()
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	var forming *model.Candle
	if json.Unmarshal(raw, &forming) != nil {
		return nil, false
	}
	return forming, true
}

/*
storeFormingCandle는 **이미 있는** 형성봉 칸만 덮어쓴다 (`XX` + `KEEPTTL`).

`XX`가 없으면 스냅샷이 만료된 뒤에도 형성봉 키가 되살아나 혼자 남는다 —
`KEEPTTL`은 없는 키에 쓰면 만료 없는 키를 만들기 때문에 그대로 새는 자리가 된다.
TTL을 유지하는 이유는 그 값이 스냅샷 쪽 TTL(일봉 1시간·인트라데이 90초)에서 오기
때문이다. 여기서 다시 정하면 일봉의 형성봉만 90초 만에 사라진다.
*/
func (s *QuotesService) storeFormingCandle(
	ctx context.Context,
	symbol string,
	interval model.CandleInterval,
	forming *model.Candle,
) {
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(forming)
	if err != nil {
		return
	}
	_ = s.redis.SetArgs(ctx, fmt.Sprintf(mdFormKeyFmt, symbol, interval), payload, redis.SetArgs{
		Mode:    "XX",
		KeepTTL: true,
	}).Err()
}

/*
loadQuoteDetailFromTerminalCache — 상세 시세 사다리의 한 칸. **1초 안쪽만** 받는다.

키의 Redis TTL은 60초지만(2026-09-05, `terminalSnapshotTTL`), 그건 「낡은 값을 주고
뒤에서 새로 만들기」 위한 수명이지 **여기서 쓸 값의 수명이 아니다.** 이 함수는 상세
헤더의 현재가를 정하는 사다리 안에 있어서, 60초 된 값을 「지금 값」으로 올리면
v1.11.0에서 고친 「목록과 상세가 다른 가격」이 되살아난다.

그래서 신선함을 여기서 다시 본다. 낡았으면 없는 셈 치고 사다리의 다음 칸으로 간다.
*/
func (s *QuotesService) loadQuoteDetailFromTerminalCache(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var snap model.QuoteTerminalSnapshot
	if !s.cacheGetJSON(ctx, fmt.Sprintf(quotesTerminalKeyFmt, sym), &snap) {
		return model.QuoteDetail{}, false
	}
	if !terminalSnapshotFresh(snap.CachedAt, time.Now()) {
		return model.QuoteDetail{}, false
	}
	if strings.TrimSpace(snap.Detail.Symbol) == "" {
		return model.QuoteDetail{}, false
	}
	return snap.Detail, true
}

func (s *QuotesService) cacheOrderBook(ctx context.Context, symbol string, book model.OrderBook, source string) {
	if len(book.Asks) == 0 && len(book.Bids) == 0 {
		return
	}
	s.cacheSetEnvelope(ctx, fmt.Sprintf(quotesBookKeyFmt, symbol), book, source, s.terminalCacheTTL())
}

func (s *QuotesService) loadOrderBookFromCache(ctx context.Context, symbol string, requireFresh bool) (model.OrderBook, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var book model.OrderBook
	fetchedAt, _, ok := s.cacheGetEnvelope(ctx, fmt.Sprintf(quotesBookKeyFmt, sym), &book)
	if !ok {
		return model.OrderBook{}, false
	}
	if requireFresh && s.terminalDataNeedsRefresh(ctx, sym, fetchedAt, nil) {
		return model.OrderBook{}, false
	}
	return book, true
}

func (s *QuotesService) cacheTrades(ctx context.Context, symbol string, trades []model.TradeTick, source string) {
	if len(trades) == 0 {
		return
	}
	s.cacheSetEnvelope(ctx, fmt.Sprintf(quotesTradesKeyFmt, symbol), trades, source, s.terminalCacheTTL())
}

func (s *QuotesService) loadTradesFromCache(ctx context.Context, symbol string, requireFresh bool) ([]model.TradeTick, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var trades []model.TradeTick
	fetchedAt, _, ok := s.cacheGetEnvelope(ctx, fmt.Sprintf(quotesTradesKeyFmt, sym), &trades)
	if !ok || len(trades) == 0 {
		return nil, false
	}
	if requireFresh && s.terminalDataNeedsRefresh(ctx, sym, fetchedAt, trades) {
		return nil, false
	}
	return trades, true
}

/*
loadCandleSnapshot은 나뉘어 저장된 두 칸을 **MGET 한 번**으로 받아 합친다
(`cacheCandleSnapshot` 주석 참고). 왕복 수는 나누기 전과 같고, 큰 쪽은 봉이 닫힐
때만 바뀌므로 초당 트래픽에서는 작은 쪽만 오간다.

형성봉 칸이 비어 있어도(예전 형식으로 저장된 캐시) 스냅샷은 그대로 낸다 — 그 경우
스냅샷 자체에 형성봉이 박혀 있다.
*/
func (s *QuotesService) loadCandleSnapshot(ctx context.Context, symbol string, interval model.CandleInterval) (model.CandleSnapshot, bool) {
	if s.redis == nil {
		return model.CandleSnapshot{}, false
	}
	vals, err := s.redis.MGet(ctx,
		fmt.Sprintf(mdSnapKeyFmt, symbol, interval),
		fmt.Sprintf(mdFormKeyFmt, symbol, interval),
	).Result()
	if err != nil || len(vals) < 2 {
		return model.CandleSnapshot{}, false
	}
	rawSnap, _ := vals[0].(string)
	if rawSnap == "" {
		return model.CandleSnapshot{}, false
	}
	var snap model.CandleSnapshot
	if json.Unmarshal([]byte(rawSnap), &snap) != nil {
		return model.CandleSnapshot{}, false
	}
	if rawForming, _ := vals[1].(string); rawForming != "" {
		var forming *model.Candle
		if json.Unmarshal([]byte(rawForming), &forming) == nil {
			snap.Forming = forming
		}
	}
	return snap, true
}

func quotesListCacheKey(filter, query string, limit, offset int) string {
	return fmt.Sprintf(quotesListKeyFmt, filter, query, limit, offset)
}

func (s *QuotesService) InvalidateQuotesListCache(ctx context.Context, filter string) {
	if s.redis == nil {
		return
	}
	for _, limit := range []int{100, 200} {
		key := quotesListCacheKey(filter, "", limit, 0)
		_ = s.redis.Del(ctx, key).Err()
	}
}
