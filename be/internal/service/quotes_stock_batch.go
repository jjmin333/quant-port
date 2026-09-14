package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

// stockInfoBatchTTL bounds how long a poller-prefetched entry stays usable.
// Short on purpose: this only exists to let one fast tick's worth of
// concurrent symbol refreshes share a single GetStocks call instead of each
// making its own — it is not a general-purpose cache for /api/v1/stocks.
const stockInfoBatchTTL = 3 * time.Second

type stockInfoBatchEntry struct {
	info      toss.StockInfo
	expiresAt time.Time
}

// prefetchStockInfo fetches reference data for symbols in one batched call
// (GetStocks supports up to 200 symbols per request) and stores the results
// for buildQuoteDetailFromToss to read instead of each symbol calling
// GetStock individually. /api/v1/stocks and GetStock/GetStocks share Toss's
// STOCK rate-limit bucket, which the per-symbol call pattern was exhausting
// on every fast tick.
func (s *QuotesService) prefetchStockInfo(ctx context.Context, symbols []string) {
	if !s.TossEnabled() || len(symbols) == 0 {
		return
	}

	seen := map[string]struct{}{}
	batch := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		up := strings.ToUpper(strings.TrimSpace(sym))
		if up == "" {
			continue
		}
		if _, ok := seen[up]; ok {
			continue
		}
		seen[up] = struct{}{}
		batch = append(batch, up)
	}
	if len(batch) == 0 {
		return
	}

	expiresAt := time.Now().Add(stockInfoBatchTTL)
	for i := 0; i < len(batch); i += 200 {
		end := min(i+200, len(batch))
		stocks, err := s.toss.GetStocks(ctx, batch[i:end])
		if err != nil {
			// Rate-limited or transient — callers fall back to an individual
			// GetStock for any symbol this chunk would have covered.
			continue
		}

		s.stockInfoBatchMu.Lock()
		for _, st := range stocks {
			sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
			if sym == "" {
				continue
			}
			s.stockInfoBatch[sym] = stockInfoBatchEntry{info: st, expiresAt: expiresAt}
		}
		s.stockInfoBatchMu.Unlock()

		// 배치가 받은 것을 Redis에도 넣는다 — 그래야 요청 경로(stockInfo)와
		// 다른 프로세스가 같은 호출을 재사용한다. 배치 맵은 이 프로세스의
		// 3초짜리라 요청 경로가 기대기에는 너무 짧고 좁다.
		for _, st := range stocks {
			sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
			if sym == "" {
				continue
			}
			s.cacheSetJSON(ctx, fmt.Sprintf(stockInfoCacheKeyFmt, sym), st, stockInfoCacheTTL)
		}
	}
}

func (s *QuotesService) lookupPrefetchedStock(symbol string) (*toss.StockInfo, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))

	s.stockInfoBatchMu.RLock()
	entry, ok := s.stockInfoBatch[sym]
	s.stockInfoBatchMu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	info := entry.info
	return &info, true
}

/*
stockInfoCacheTTL — `/api/v1/stocks` 응답을 Redis에 두는 시간.

이 응답에 든 것은 **장중에 바뀌지 않는 참조 데이터**다 — 종목명·시장·통화·
증권유형·상장주식수·거래정지 플래그. 시세는 하나도 없다(가격은 `/prices`,
호가는 `/orderbook`).

그런데 상세 화면이 2초마다 스냅샷을 다시 조립하면서 매번 이것을 새로 받아 왔다.
실측(2026-08-25, 시청자 1명·30초): `/api/v1/stocks` **19회**. `STOCK` 버킷은
**5 req/s**라 이 비율이면 **동시 시청자 8명에서 버킷이 소진된다.**

**구독이 이 낭비를 오히려 키웠다.** 폴러의 `prefetchStockInfo`가 배치로 한 번에
받아 두는데, 구독 중인 종목은 `filterStreamedSymbols`가 폴링에서 빼므로 배치
대상이 아니다 — 그래서 요청 경로가 종목마다 개별 `GetStock`을 부른다.

30초는 「거래정지 플래그가 늦게 반영돼도 되는 한계」로 잡았다. 그 플래그는
장중에 바뀌면 화면의 경고 배지에 반영되는데, 30초 지연은 사용자가 알아채기
전에 따라잡는다. 캐시가 없으면 예전 동작 그대로다(Redis 미설정 시 그대로 REST).
*/
const stockInfoCacheTTL = 30 * time.Second

const stockInfoCacheKeyFmt = "toss:stockinfo:%s"

/*
stockInfo — 종목 참조 데이터를 **3단 계층**으로 읽는다.

 1. 폴러 배치 맵(3초)  — 한 틱 안의 동시 갱신이 호출 하나를 나눠 쓰게 한다
 2. Redis(30초)       — 프로세스·요청을 가로질러 공유한다
 3. Toss REST         — 위 둘이 다 비었을 때만

2단이 없어서 요청마다 3단으로 갔다. 여기서 막아야 `STOCK` 버킷이 시청자 수에
비례해 소진되는 것을 끊는다.
*/
func (s *QuotesService) stockInfo(ctx context.Context, symbol string) (*toss.StockInfo, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil, fmt.Errorf("stock: empty symbol")
	}

	if info, ok := s.lookupPrefetchedStock(sym); ok {
		return info, nil
	}

	key := fmt.Sprintf(stockInfoCacheKeyFmt, sym)
	var cached toss.StockInfo
	if s.cacheGetJSON(ctx, key, &cached) && cached.Symbol != "" {
		return &cached, nil
	}

	info, err := s.toss.GetStock(ctx, sym)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	s.cacheSetJSON(ctx, key, info, stockInfoCacheTTL)
	return info, nil
}
