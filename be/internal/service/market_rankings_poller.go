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
	rankingsPollFetchPause = 100 * time.Millisecond
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
			due := s.dueRankingsPollParams(time.Now(), lastFetched)
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

func (s *MarketService) dueRankingsPollParams(now time.Time, lastFetched map[string]time.Time) []toss.GetRankingsParams {
	var due []toss.GetRankingsParams
	for _, p := range s.allRankingsPollParams() {
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
