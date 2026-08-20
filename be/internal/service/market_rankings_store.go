package service

import (
	"encoding/json"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

// rankingsStorePayload wraps Toss rankings with BE refresh metadata.
type rankingsStorePayload struct {
	RefreshedAt  string             `json:"refreshedAt"`
	TossRankedAt *string            `json:"tossRankedAt,omitempty"`
	RankedAt     *string            `json:"rankedAt,omitempty"` // legacy plain Toss payload
	Rankings     []toss.RankingItem `json:"rankings"`
}

func (p *rankingsStorePayload) normalize() {
	if p.TossRankedAt == nil && p.RankedAt != nil {
		p.TossRankedAt = p.RankedAt
	}
}

type RankingsAPIResult struct {
	RankedAt    *string            `json:"rankedAt"`
	RefreshedAt *string            `json:"refreshedAt"`
	Rankings    []toss.RankingItem `json:"rankings"`
}

func wrapRankingsStore(resp *toss.RankingResponse, refreshedAt time.Time) rankingsStorePayload {
	out := rankingsStorePayload{
		RefreshedAt: refreshedAt.UTC().Format(time.RFC3339),
		Rankings:    []toss.RankingItem{},
	}
	if resp == nil {
		return out
	}
	if resp.RankedAt != nil {
		v := *resp.RankedAt
		out.TossRankedAt = &v
	}
	if resp.Rankings != nil {
		out.Rankings = resp.Rankings
	}
	return out
}

func decodeRankingsStore(raw []byte) (rankingsStorePayload, bool) {
	if len(raw) == 0 {
		return rankingsStorePayload{}, false
	}

	var wrapped rankingsStorePayload
	if json.Unmarshal(raw, &wrapped) == nil {
		wrapped.normalize()
		if len(wrapped.Rankings) > 0 || wrapped.RefreshedAt != "" {
			return wrapped, true
		}
	}

	// Legacy: plain Toss RankingResponse JSON.
	var legacy toss.RankingResponse
	if json.Unmarshal(raw, &legacy) != nil {
		return rankingsStorePayload{}, false
	}
	wrapped = rankingsStorePayload{Rankings: legacy.Rankings}
	if legacy.RankedAt != nil {
		v := *legacy.RankedAt
		wrapped.TossRankedAt = &v
	}
	return wrapped, len(wrapped.Rankings) > 0
}

func (p rankingsStorePayload) toAPIResult(count int) *RankingsAPIResult {
	rankings := p.Rankings
	if count > 0 && len(rankings) > count {
		rankings = append([]toss.RankingItem(nil), rankings[:count]...)
	}
	out := &RankingsAPIResult{Rankings: rankings}
	if p.TossRankedAt != nil {
		out.RankedAt = p.TossRankedAt
	}
	if p.RefreshedAt != "" {
		v := p.RefreshedAt
		out.RefreshedAt = &v
	}
	return out
}
