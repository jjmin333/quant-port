package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

const (
	rankingsPollFetchCount = 100
	rankingsPollTick       = time.Minute
	rankingsPruneInterval  = 12 * time.Hour
	/*
		랭킹 배치의 호출 간격 — **100ms에서 늘렸다** (2026-09-05 사용자 지적 6).

		Toss가 응답 헤더로 알려 주는 한도는 **자격증명 하나당 1초에 5회**다. 100ms는
		초당 10회이고, 클라이언트가 primary·secondary를 번갈아 쓰므로 키마다 정확히
		5회 — **한도에 딱 붙는다**. 실측 로그(2026-09-05 07:01, 기동 직후 6줄):

			toss api rate-limit low: GET /api/v1/rankings key=primary limit=5 remaining=1 resetSec=1

		`remaining=1`은 그 1초 창에 **사용자 요청이 하나 들어올 자리밖에 없다**는 뜻이다.
		둘이 들어오면 뒤엣것은 창이 리셋될 때까지 기다린다 — 사용자에게는 「가끔 한 번씩
		엄청 느리다」로 보이고, 사용자 수와 무관한 것도 이걸로 설명된다(배치가 혼자
		한도를 다 쓴다).

		200ms는 키마다 초당 2.5회로, **창의 절반을 사용자 요청에 남긴다.** 대신 기동 시
		첫 적재가 8슬롯 기준 0.8초에서 1.6초로 늘어나는데, 그 값은 아무도 안 본다 —
		화면은 DB·Redis만 읽고 이 배치는 뒤에서 돈다.
	*/
	rankingsPollFetchPause = 200 * time.Millisecond
)

var (
	rankingsPollMarkets = []toss.MarketCountry{toss.MarketKR, toss.MarketUS}
	rankingsPollTypes   = []toss.RankingType{
		toss.RankingMarketTradingAmount,
		toss.RankingMarketTradingVolume,
	}
	rankingsPollDurations = []toss.RankingDuration{
		toss.RankingRealtime,
		toss.Ranking1d,
		toss.Ranking1w,
		toss.Ranking1mo,
		toss.Ranking3mo,
		toss.Ranking6mo,
		toss.Ranking1y,
	}
)

func defaultRankingsPollIntervalFor(d toss.RankingDuration) time.Duration {
	switch d {
	case toss.RankingRealtime:
		// Not truly real-time: the ranking list doesn't need per-second
		// freshness the way a single quote's header does, and every symbol
		// on it draws from the same RANKING rate-limit bucket regardless of
		// user count. Aligned to clean wall-clock marks (00:00, 00:05, ...)
		// by alignedRankingBucket below, not just "5 minutes since last poll".
		return 5 * time.Minute
	case toss.Ranking1d, toss.Ranking1w:
		return 15 * time.Minute
	case toss.Ranking1mo, toss.Ranking3mo, toss.Ranking6mo, toss.Ranking1y:
		return time.Hour
	default:
		return time.Hour
	}
}

// alignedRankingBucket floors t to the most recent wall-clock multiple of
// interval (e.g. 14:07:23 with a 5m interval -> 14:05:00). time.Truncate
// rounds relative to the zero Time value, which for round divisors of an
// hour (5m/15m/1h) lands exactly on clean clock marks in UTC — and KST's
// +9:00 offset is itself a whole multiple of 5 minutes, so the same buckets
// line up in KST too.
func alignedRankingBucket(t time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		return t
	}
	return t.UTC().Truncate(interval)
}

func isSupportedRankingPollParams(p toss.GetRankingsParams) bool {
	if !isSupportedRankingDuration(p.Duration) {
		return false
	}
	switch p.Type {
	case toss.RankingMarketTradingAmount, toss.RankingMarketTradingVolume:
	default:
		return false
	}
	switch p.MarketCountry {
	case toss.MarketKR, toss.MarketUS:
		return true
	default:
		return false
	}
}

func isSupportedRankingDuration(d toss.RankingDuration) bool {
	switch d {
	case toss.RankingRealtime, toss.Ranking1d, toss.Ranking1w,
		toss.Ranking1mo, toss.Ranking3mo, toss.Ranking6mo, toss.Ranking1y:
		return true
	default:
		return false
	}
}

func rankingsPollSlotKey(p toss.GetRankingsParams) string {
	return fmt.Sprintf("%s:%s:%s", p.MarketCountry, p.Type, p.Duration)
}

func (s *MarketService) allRankingsPollParams() []toss.GetRankingsParams {
	out := make([]toss.GetRankingsParams, 0, len(rankingsPollMarkets)*len(rankingsPollTypes)*len(rankingsPollDurations))
	for _, market := range rankingsPollMarkets {
		for _, typ := range rankingsPollTypes {
			for _, duration := range rankingsPollDurations {
				out = append(out, toss.GetRankingsParams{
					Type:          typ,
					MarketCountry: market,
					Duration:      duration,
					Count:         rankingsPollFetchCount,
				})
			}
		}
	}
	return out
}

// RunRankingsPoller fetches all ranking slots once immediately on startup
// (refreshedAt = actual start time), then on a 1-minute tick refreshes slots
// whose duration-specific bucket has rolled over — landing on clean
// wall-clock marks (00:00, 00:05, 00:10, ... for the 5m realtime bucket)
// rather than "N minutes after whenever the process happened to start".
// User-facing GET /rankings reads DB/Redis only.
func (s *MarketService) RunRankingsPoller(ctx context.Context) {
	lastFetched := make(map[string]time.Time)
	all := s.allRankingsPollParams()

	log.Printf("market rankings poller: startup fetch %d slots", len(all))
	s.pollRankingsBatch(ctx, all, lastFetched, false)

	ticker := time.NewTicker(rankingsPollTick)
	defer ticker.Stop()

	// ranking_snapshot_entries only ever grew; prune it on a slow cadence so the
	// sparkline window stays cheap to query.
	prune := time.NewTicker(rankingsPruneInterval)
	defer prune.Stop()
	go s.pruneRankingHistory(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			due := s.dueRankingsPollParams(ctx, time.Now(), lastFetched)
			if len(due) == 0 {
				continue
			}
			s.pollRankingsBatch(ctx, due, lastFetched, true)
		case <-prune.C:
			s.pruneRankingHistory(ctx)
		}
	}
}

func (s *MarketService) pruneRankingHistory(ctx context.Context) {
	if s.rankingSnapshots == nil {
		return
	}
	n, err := s.rankingSnapshots.PruneOlderThan(ctx, 0)
	if err != nil {
		log.Printf("market rankings poller: prune history: %v", err)
		return
	}
	if n > 0 {
		log.Printf("market rankings poller: pruned %d history rows", n)
	}
}

/*
rankingsClosedGrace — 그 시장이 닫힌 뒤에도 계속 받아 보는 시간.

닫히자마자 끊으면 **마지막 순위를 놓친다.** KRX는 15:30에 닫히는데 15:30 버킷의
호출은 그 경계와 초 단위로 겹치므로, 그때 받은 값이 「장 마감 시점의 순위」가 아닐
수 있다. 30분을 더 받아 보면 정산이 끝난 최종 상태가 들어온다.

그 뒤로는 값이 바뀔 수 없다 — 거래가 없으므로 거래대금·거래량 순위는 다음 개장까지
그대로다.
*/
const rankingsClosedGrace = 30 * time.Minute

/*
skipClosedMarketRanking — 「이 시장은 지금 받아 볼 이유가 있나」.

# 왜 필요했나 (2026-09-11 사용자 요청 7)

이 폴러는 **장이 닫혀 있어도 같은 박자로 돌았다.** 실측하면 슬롯이 28개
(2시장 × 2종류 × 7기간)라 하루 약 2,300호출인데, 국내는 하루 6시간 반만 열리고
미국은 6시간 반이다. 즉 그 호출의 **절반 이상이 바뀔 수 없는 값을 다시 받는 데** 쓰였고,
그 값은 `RANKING` 버킷을 태우면서 사용자 요청과 같은 창을 나눠 썼다.

화면은 이미 이 사실을 알고 있었다 — 마감 중에는 `sessionClosedAt`을 찍는다
(`lib/market-radar-clock`). 서버만 모르고 계속 받고 있었던 것이다.

# 세션을 모르면 받는다

달력을 못 읽었을 때(`MarketClock`이 비었을 때) 건너뛰면, 달력 한 번 실패가 순위를
하루 동안 얼려 버린다. 모를 때는 예전처럼 받는 쪽이 안전하다 — 이 판단의 비용은
호출 몇 개이고, 반대 방향의 비용은 낡은 화면이다.
*/
func (s *MarketService) skipClosedMarketRanking(ctx context.Context, country toss.MarketCountry, now time.Time) bool {
	// **캐시만 읽는다.** 매 틱 도는 자리라 상류 호출을 유발하면 안 된다
	// (MarketClockCachedFor 머리글 — 실측으로 휴장일에 분당 7회가 나갔다).
	clock := s.rankingMarketClockCached(ctx, country)
	if !clock.Closed() || clock.ClosedAt == "" {
		return false // 열려 있거나 모른다 — 받는다
	}
	closedAt, err := time.Parse(time.RFC3339, clock.ClosedAt)
	if err != nil {
		return false
	}
	return now.Sub(closedAt) > rankingsClosedGrace
}

func (s *MarketService) dueRankingsPollParams(ctx context.Context, now time.Time, lastFetched map[string]time.Time) []toss.GetRankingsParams {
	// 시장별 판정은 한 회차에 한 번만 한다 — 슬롯이 28개라 파라미터마다 물으면
	// 같은 달력을 열네 번씩 읽는다.
	skip := make(map[toss.MarketCountry]bool, len(rankingsPollMarkets))
	for _, m := range rankingsPollMarkets {
		skip[m] = s.skipClosedMarketRanking(ctx, m, now)
	}

	var due []toss.GetRankingsParams
	for _, p := range s.allRankingsPollParams() {
		if skip[p.MarketCountry] {
			continue
		}
		key := rankingsPollSlotKey(p)
		interval := s.rankingsPollIntervalFor(p.Duration)
		bucket := alignedRankingBucket(now, interval)
		if last, ok := lastFetched[key]; ok && !bucket.After(last) {
			continue
		}
		due = append(due, p)
	}
	return due
}

func (s *MarketService) rankingsPollIntervalFor(d toss.RankingDuration) time.Duration {
	if s.dataSettings != nil {
		return s.dataSettings.RankingsPollInterval(d)
	}
	return defaultRankingsPollIntervalFor(d)
}

// pollRankingsBatch fetches each param and records refreshedAt.
// aligned=false (startup only) stamps refreshedAt with the actual fetch time;
// aligned=true (the recurring ticker path) stamps it with the wall-clock
// bucket boundary (e.g. 14:05:00) so the "갱신" timestamp shown to users
// lands on clean marks. Either way, lastFetched always records the bucket —
// due-checking must stay bucket-based regardless of how refreshedAt is
// displayed, or the startup fetch's un-floored timestamp would make the
// first scheduled fetch fire early.
func (s *MarketService) pollRankingsBatch(
	ctx context.Context,
	params []toss.GetRankingsParams,
	lastFetched map[string]time.Time,
	aligned bool,
) {
	for i, p := range params {
		if i > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(rankingsPollFetchPause):
			}
		}

		key := rankingsPollSlotKey(p)
		interval := s.rankingsPollIntervalFor(p.Duration)
		now := time.Now()
		bucket := alignedRankingBucket(now, interval)
		refreshedAt := now
		if aligned {
			refreshedAt = bucket
		}

		if err := s.refreshRankingsFromToss(ctx, p, refreshedAt); err != nil {
			log.Printf("market rankings poller: %s %s %s: %v", p.MarketCountry, p.Type, p.Duration, err)
			continue
		}
		lastFetched[key] = bucket
		log.Printf("market rankings poller: refreshed %s %s %s refreshedAt=%s",
			p.MarketCountry, p.Type, p.Duration, refreshedAt.Format(time.RFC3339))
	}
}

func (s *MarketService) refreshRankingsFromToss(ctx context.Context, p toss.GetRankingsParams, refreshedAt time.Time) error {
	resp, err := s.fetchRankingsFromToss(ctx, p)
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}

	if err := s.persistRankings(ctx, p, resp, refreshedAt); err != nil {
		log.Printf("market rankings persist: %v", err)
	}
	// 상장 첫날 순위에 오른 종목이 이름 없이 코드로 뜨지 않게(market_rankings_catalog.go).
	s.registerUnknownRankingSymbols(ctx, resp.Rankings)

	return nil
}

func (s *MarketService) persistRankings(ctx context.Context, p toss.GetRankingsParams, resp *toss.RankingResponse, refreshedAt time.Time) error {
	wrapped := wrapRankingsStore(resp, refreshedAt)
	cacheKey := rankingsCacheKey(p)
	s.storeRankingsPayload(ctx, cacheKey, wrapped, rankingsCacheTTLFor(p.Duration))

	if resp != nil && len(resp.Rankings) > 0 {
		cacheReferencePrices(ctx, s.redis, resp.Rankings)
	}

	if s.rankingsRepo == nil {
		return nil
	}

	payload, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	var rankedAt *time.Time
	if resp.RankedAt != nil && stringsTrim(*resp.RankedAt) != "" {
		raw := stringsTrim(*resp.RankedAt)
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			utc := t.UTC()
			rankedAt = &utc
		} else if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			utc := t.UTC()
			rankedAt = &utc
		}
	}

	return s.rankingsRepo.Upsert(
		ctx,
		string(p.MarketCountry),
		string(p.Type),
		string(p.Duration),
		rankedAt,
		payload,
	)
}

func (s *MarketService) loadRankingsFromStore(ctx context.Context, p toss.GetRankingsParams) (rankingsStorePayload, bool) {
	cacheKey := rankingsCacheKey(p)
	if cached, ok := s.loadRankingsPayload(ctx, cacheKey); ok {
		return cached, true
	}

	if s.rankingsRepo == nil {
		return rankingsStorePayload{}, false
	}

	row, err := s.rankingsRepo.Get(ctx, string(p.MarketCountry), string(p.Type), string(p.Duration))
	if err != nil || row == nil || len(row.Payload) == 0 {
		return rankingsStorePayload{}, false
	}

	wrapped, ok := decodeRankingsStore(row.Payload)
	if !ok {
		return rankingsStorePayload{}, false
	}

	s.storeRankingsPayload(ctx, cacheKey, wrapped, rankingsCacheTTLFor(p.Duration))
	return wrapped, true
}

func (s *MarketService) WarmRankingsCache(ctx context.Context) int {
	total := len(rankingsPollMarkets) * len(rankingsPollTypes) * len(rankingsPollDurations)
	loaded := 0
	for _, p := range s.allRankingsPollParams() {
		if stored, ok := s.loadRankingsFromStore(ctx, p); ok && len(stored.Rankings) > 0 {
			loaded++
		}
	}
	if loaded > 0 {
		log.Printf("market rankings: warmed %d/%d slots from store", loaded, total)
	}
	return loaded
}

func stringsTrim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
