package service

import (
	"encoding/json"
	"time"

	"github.com/enact/quant-port/be/internal/model"
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

/*
RankingAPIItem — 응답에 나가는 랭킹 한 줄.

업스트림이 준 필드(`toss.RankingItem`)에 **카탈로그에서 온 표시용 필드**를 얹는다.
임베드라 JSON은 평평하게 나가므로 기존 필드 이름은 그대로다.

**이름을 여기 넣는 이유.** 예전에는 랭킹이 심볼만 주고 화면이 이름을 따로 받았다.
그 두 번째 요청이 `requestIdleCallback` 뒤에 나가서, 목록이 **종목코드로 먼저 그려졌다가
이름으로 바뀌었다**. 배경 탭에서는 idle 콜백이 아예 안 돌아 코드가 그대로 남는다
(실측 2026-09-01, 100행 전부). 뉴스 종목 칩이 v1.0.2에서 같은 증상을 겪었고 같은
방법으로 고쳤다 — **이름을 아는 쪽이 이름을 준다.**

비용은 DB 조회 한 번(`ListBySymbols`, 심볼 IN)이고 상류 호출은 없다.
*/
type RankingAPIItem struct {
	toss.RankingItem
	Name string `json:"name,omitempty"`
	// AssetType·LogoURL은 화면이 로고를 고르는 데 쓴다 — 이름과 같은 조회에서 온다.
	AssetType model.QuoteAssetType `json:"assetType,omitempty"`
	LogoURL   string               `json:"logoUrl,omitempty"`
}

type RankingsAPIResult struct {
	RankedAt    *string `json:"rankedAt"`
	RefreshedAt *string `json:"refreshedAt"`
	/*
		이 목록이 속한 시장의 세션 시계 — 목록 전체가 한 달력을 공유하므로 행이
		아니라 응답에 한 번 붙는다.

		화면은 이 값으로 두 가지를 한다: ① 마감 중에는 「갱신 시각」 자리에
		**장이 닫힌 시각**을 날짜와 함께 찍는다(그 자리가 5분 주기 스냅샷 시각을
		보여 주는 동안 사용자는 그것을 「지금」으로 읽었다). ② `nextSessionAt`
		전까지 `/rankings/prices` 폴링을 멈춘다 — 가격이 바뀔 수 없는 구간이다.
	*/
	MarketClock
	Rankings []RankingAPIItem `json:"rankings"`
}

// tossItems는 가격 보정(EnrichRankingPrices)이 다루는 업스트림 필드만 떼어 낸다.
func (r *RankingsAPIResult) tossItems() []toss.RankingItem {
	out := make([]toss.RankingItem, len(r.Rankings))
	for i := range r.Rankings {
		out[i] = r.Rankings[i].RankingItem
	}
	return out
}

// applyTossItems는 tossItems로 떼어 내 보정한 값을 제자리에 되돌린다.
func (r *RankingsAPIResult) applyTossItems(items []toss.RankingItem) {
	for i := range r.Rankings {
		if i >= len(items) {
			break
		}
		r.Rankings[i].RankingItem = items[i]
	}
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
		rankings = rankings[:count]
	}
	items := make([]RankingAPIItem, len(rankings))
	for i, row := range rankings {
		items[i] = RankingAPIItem{RankingItem: row}
	}
	out := &RankingsAPIResult{Rankings: items}
	if p.TossRankedAt != nil {
		out.RankedAt = p.TossRankedAt
	}
	if p.RefreshedAt != "" {
		v := p.RefreshedAt
		out.RefreshedAt = &v
	}
	return out
}
