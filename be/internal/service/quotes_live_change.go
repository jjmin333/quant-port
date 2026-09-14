package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// liveChangeMax bounds one call. Callers are leaderboards (20 rows) and the
// home card (8); the cap only exists so a bad caller cannot turn this into a
// full-universe scan.
const liveChangeMax = 100

// liveChangeParallelism mirrors EnrichRankingPrices — the cost here is Redis
// round-trips, not upstream calls, and 8 bounds the worst case where every
// entry misses the cache.
const liveChangeParallelism = 8

// LiveChangeRequest is one symbol to price. AssetType comes from the caller
// because the caller already knows it; looking it up here would add one
// `stocks` query per row.
type LiveChangeRequest struct {
	Symbol    string
	AssetType string
	/*
		기준가 대안 — 캐시된 기준가가 없을 때 쓴다. 0이면 없는 것으로 친다.

		1순위는 여전히 `resolvePrevClose`(Toss 랭킹 기준가 · Redis 캐시)다. 그게 비면
		**업스트림으로 내려가지 않고** 이 값을 쓴다: 목록에서 종목마다 기준가를 받아
		오면 1분봉을 세션 통째로 페이지네이션하게 되어 캔들 예산이 바닥난다
		(`resolvePrevClose`의 listMode 주석).
	*/
	PrevCloseHint float64
}

// LiveChange is a symbol's price right now and its move against the previous
// session's close — the same baseline the stored daily close uses, so the
// number does not jump when one replaces the other.
type LiveChange struct {
	Price      float64
	ChangeRate float64
	Currency   string
}

/*
LiveChanges — 목록에 실시간 등락률을 얹는다. **장이 열려 있는 동안에만.**

이 함수가 답하는 질문은 「지금 이 종목이 몇 % 움직였나」다. 그래서 세션이
`pre`·`regular`·`after`·`day` 중 하나일 때만 값을 돌려주고, **애프터마켓까지 끝난
`closed`에서는 아무것도 돌려주지 않는다** — 그때는 호출부가 확정된 종가를 그대로
쓴다. 마감 후에 실시간 경로를 계속 쓰면 화면이 공식 종가와 어긋난 값을 든다
(실측 2026-09-01 05:07 KST, KRX 마감 후: 삼성전자 종가 259,500 vs 실시간 258,000).

**기준가는 전일 종가다.** 목록의 등락률 칸은 종가 기준 등락률과 같은 자리에 서므로
기준이 같아야 하고, 그래야 마감 순간에 값이 이어진다. 예전에는 시세 상세만 시간외에
「당일 정규장 종가 대비」를 써서 두 화면이 갈렸는데, 지금은 상세도 전일 종가를 쓴다
(`computeQuoteChange` 주석 참조).

비용: 배치 가격 1회(`EnsurePrices`, 200종목/호출) + 종목마다 캐시·DB 조회.
`withListMode`가 종목별 업스트림 호출을 막는다 — 목록 길이에 비례하는 호출은
Toss 한도에서 초선형으로 나빠진다(quotes_overlay.go의 실측 참조).
*/
func (s *QuotesService) LiveChanges(ctx context.Context, reqs []LiveChangeRequest) map[string]LiveChange {
	if s == nil || len(reqs) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(reqs))
	ordered := make([]LiveChangeRequest, 0, len(reqs))
	for _, r := range reqs {
		sym := strings.ToUpper(strings.TrimSpace(r.Symbol))
		if sym == "" {
			continue
		}
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		ordered = append(ordered, LiveChangeRequest{
			Symbol: sym, AssetType: r.AssetType, PrevCloseHint: r.PrevCloseHint,
		})
		if len(ordered) >= liveChangeMax {
			break
		}
	}
	if len(ordered) == 0 {
		return nil
	}

	ctx = withListMode(withOverlayCache(ctx))

	// 달력은 배치당 한 번만 읽는다 — 종목마다 EnsureCalendars를 부르면 같은 값을
	// 목록 길이만큼 다시 푼다.
	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.EnsureCalendars(ctx)
	}
	now := time.Now()

	open := make([]LiveChangeRequest, 0, len(ordered))
	for _, r := range ordered {
		if detectMarketSession(r.AssetType, cals, now) != "closed" {
			open = append(open, r)
		}
	}
	if len(open) == 0 {
		return nil
	}

	base := make([]model.QuoteSummary, len(open))
	for i, r := range open {
		base[i] = model.QuoteSummary{Symbol: r.Symbol}
	}
	priceMap, _ := s.fetchPriceMap(ctx, base)

	/*
		방금 MGET 한 번으로 받은 가격을 요청 안에서 공유한다.

		이걸 안 넘기면 종목별 가격 해석이 **같은 키를 다시** `GetPrice`로 물어서
		Redis 왕복이 종목 수만큼 더 붙는다(withListPrices 헤더의 실측 참조).
	*/
	listPrices := make(map[string]float64, len(priceMap))
	for sym, p := range priceMap {
		if v := parseDecimal(p.LastPrice); v > 0 {
			listPrices[sym] = v
		}
	}
	ctx = withListPrices(ctx, listPrices)

	slots := make([]LiveChange, len(open))
	sem := make(chan struct{}, liveChangeParallelism)
	var wg sync.WaitGroup

	for i, r := range open {
		wg.Add(1)
		go func(idx int, req LiveChangeRequest) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}

			var fallback float64
			currency := ""
			if p, ok := priceMap[req.Symbol]; ok {
				fallback = parseDecimal(p.LastPrice)
				currency = string(modelCurrency(p.Currency))
			}
			price := s.ResolveLiveQuotePrice(ctx, req.Symbol, fallback)
			if price <= 0 {
				return
			}
			prev, ok := s.resolvePrevClose(ctx, req.Symbol)
			if !ok || prev <= 0 {
				prev = req.PrevCloseHint
			}
			if prev <= 0 {
				return
			}
			slots[idx] = LiveChange{
				Price:      price,
				ChangeRate: (price - prev) / prev * 100,
				Currency:   currency,
			}
		}(i, r)
	}
	wg.Wait()

	out := make(map[string]LiveChange, len(open))
	for i, r := range open {
		if slots[i].Price > 0 {
			out[r.Symbol] = slots[i]
		}
	}
	return out
}
