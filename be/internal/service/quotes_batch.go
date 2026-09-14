package service

import (
	"context"
	"strings"
	"sync"

	"github.com/enact/quant-port/be/internal/model"
)

// BatchQuoteSummariesMax bounds how many distinct symbols one batch call
// resolves. Exported so the HTTP layer can publish the same number instead of
// silently handing back a shorter list than the caller asked for.
const BatchQuoteSummariesMax = 100

const batchQuoteSummariesMax = BatchQuoteSummariesMax

// batchQuoteConcurrency bounds simultaneous lookups for one batch call.
const batchQuoteConcurrency = 8

/*
BatchQuoteResult — 배치가 **무엇을 풀었고 무엇을 못 풀었는지**.

예전에는 `[]QuoteSummary` 하나만 돌려줬다. 그래서 63개를 요청해 27개가 오면
호출부는 「나머지 36개는 없는 종목이다」와 「서버가 시간 안에 못 풀었다」를
구분할 수 없었다 — 뉴스 목록이 종목명 대신 종목코드를 그리던 이유가 이것이다
(FE는 빈 자리를 「못 찾음」으로 읽고 코드로 폴백했다).

이제 못 푼 심볼과 그 이유를 함께 돌려준다. 응답 meta에 실려 나간다.
*/
type BatchQuoteResult struct {
	Summaries []model.QuoteSummary
	// Missing lists requested symbols that resolved to nothing, in request order.
	Missing []string
	// Truncated counts distinct symbols dropped by BatchQuoteSummariesMax
	// before any lookup happened.
	Truncated int
	// Deadline reports that the request context ended mid-batch, so at least
	// part of Missing is "we ran out of time", not "no such symbol".
	Deadline bool
}

// BatchQuoteSummaries returns live quote summaries using the same path as GET /quotes/{symbol}.
//
// 이유까지 필요한 호출부는 BatchQuoteSummariesResult를 쓴다.
func (s *QuotesService) BatchQuoteSummaries(ctx context.Context, symbols []string) []model.QuoteSummary {
	return s.BatchQuoteSummariesResult(ctx, symbols).Summaries
}

/*
BatchQuoteSummariesResult — 목록용 시세 배치.

# 왜 목록 모드인가

이 함수는 **목록**이다. 뉴스 화면이 기사에 걸린 종목 60여 개를 한 번에 묻고,
검색 자동완성·분석·리포트도 같은 경로를 쓴다. 그런데 종목마다
`EnsureQuoteDetail`을 부르면서 컨텍스트에 `withListMode`를 붙이지 않아,
캐시에 없는 종목이 **종목당 업스트림**으로 내려갔다:

  - `resolvePrevClose` → `fetchPrevClose`는 전 거래일 1분봉을 **페이지네이션으로
    전부** 받는다(종목당 `/candles` 여러 번).
  - 마감 후에는 `backfillClosedMarketFromToss`가 체결·호가까지 받아 왔다.

실측(2026-09-01 장 마감 후, 42심볼 요청): 한 요청이 **`/candles` 142회**를 냈고
Toss 캔들 한도(20/s)를 35번 초과해 failover·재시도가 붙었다. 그 사이
`REQUEST_TIMEOUT_SECONDS`(기본 10초)가 컨텍스트를 끊어, 아직 못 푼 goroutine이
전부 빈손으로 돌아왔다 — **42개 중 29개만, 13.9초**. 같은 심볼을 잘게 나눠
부르면 전부 정상이라 심볼 문제로 보였지만, 실제로는 배치 길이에 비례해
업스트림을 부르다 마감에 걸린 것이다.

그래서 `LiveChanges`와 같은 모양으로 고쳤다: **배치 가격 1회**(`fetchPriceMap`,
200종목/호출) + 종목마다 캐시·DB만. 상류 호출이 목록 길이에 비례하지 않는다.

# 트레이드오프

아무도 보고 있지 않은 종목은 「최신 체결가」가 아니라 「배치 마지막가」로 보인다
(급변동 중이면 한두 틱 차이). 상세 화면·스트림은 목록 모드가 아니므로 그대로다.
목록에 그 정밀도를 주려면 종목당 호출이 필요한데, 그것이 방금 없앤 비용이다.
*/
func (s *QuotesService) BatchQuoteSummariesResult(ctx context.Context, symbols []string) BatchQuoteResult {
	if len(symbols) == 0 {
		return BatchQuoteResult{}
	}

	seen := map[string]struct{}{}
	var ordered []string
	truncated := 0
	for _, raw := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(raw))
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		if len(ordered) >= batchQuoteSummariesMax {
			// 상한을 넘긴 것은 조회하지 않는다. 몇 개를 버렸는지는 세어서
			// 응답에 싣는다 — 예전에는 목록이 조용히 짧아졌다.
			truncated++
			continue
		}
		ordered = append(ordered, sym)
	}
	if len(ordered) == 0 {
		return BatchQuoteResult{Truncated: truncated}
	}

	// Resolve the caller's watchlist/holdings once for the whole batch, and
	// forbid per-symbol upstream calls for the rest of this call.
	ctx = withListMode(withOverlayCache(ctx))

	// 목록 전체의 가격을 **한 번에** 받아 둔다. 아래 팬아웃은 이 값을 fallback으로
	// 쓰고 업스트림에 다시 내려가지 않는다.
	base := make([]model.QuoteSummary, len(ordered))
	for i, sym := range ordered {
		base[i] = model.QuoteSummary{Symbol: sym}
	}
	priceMap, _ := s.fetchPriceMap(ctx, base)

	// Each goroutine owns one slot, so results keep the requested order and no
	// mutex is needed. Slots that failed to resolve are reported in Missing.
	slots := make([]model.QuoteSummary, len(ordered))
	filled := make([]bool, len(ordered))
	sem := make(chan struct{}, batchQuoteConcurrency)
	var wg sync.WaitGroup

	for i, sym := range ordered {
		wg.Add(1)
		go func(idx int, symbol string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}
			s.poller.Touch(symbol)
			detail, ok := s.EnsureQuoteDetail(ctx, symbol)
			if !ok {
				return
			}
			summary := detail.QuoteSummary
			if p, found := priceMap[symbol]; found {
				// 배치 가격이 가장 최근에 관측된 체결가다 — 마감 세션에서는 그것이
				// 「시간외까지 끝난 뒤의 최종가」이고, 저장된 상세는 그보다 낡을 수
				// 있다. 비어 있을 때만 채우면 낡은 값이 그대로 남는다.
				if v := parseDecimal(p.LastPrice); v > 0 {
					summary.CurrentPrice = v
				}
				if p.Currency != "" {
					summary.Currency = modelCurrency(p.Currency)
				}
			}
			s.enrichSummaryLivePrice(ctx, &summary)
			slots[idx] = summary
			filled[idx] = true
		}(i, sym)
	}
	wg.Wait()

	out := make([]model.QuoteSummary, 0, len(ordered))
	var missing []string
	for i := range slots {
		if filled[i] {
			out = append(out, slots[i])
			continue
		}
		missing = append(missing, ordered[i])
	}
	return BatchQuoteResult{
		Summaries: out,
		Missing:   missing,
		Truncated: truncated,
		Deadline:  ctx.Err() != nil,
	}
}
