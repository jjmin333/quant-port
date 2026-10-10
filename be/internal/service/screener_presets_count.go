package service

import (
	"context"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
프리셋 카드의 「지금 몇 종목 · 어제보다 몇 · 새로 들어온 종목」 (2026-10-09 성능 개선 · 2026-10-10 변화 표시).

예전에는 카드마다 Screen()을 통째로 돌렸다 — 집계 셋 + 시세 덮기를 8번(운영 p50 5.7초). 카드에 필요한 것은 통과한
종목이 **누구인가**뿐이라 시세 없이 심볼만 받는다(ScreenSymbols). 앞 점수일도 같은 조건으로 받아 차이를 낸다.
답은 점수일이 바뀔 때까지 같으므로 점수일을 키로 들고 있는다 — TTL 없이, 점수일이 넘어가면 새로 센다.
*/

// presetNewNamesMax — 카드 한 장에 적을 「새로 들어온 종목」 수.
const presetNewNamesMax = 3

type presetStat struct {
	count    int
	prev     *int
	newNames []string
}

func (s *ScreenerService) presetCounts(ctx context.Context, market string, presets []ScreenPreset) bool {
	if s.scores == nil {
		return false
	}
	tradeDate, err := s.scores.LatestScoredDate(ctx)
	if err != nil || tradeDate == "" {
		return false
	}
	_, mkt := screenAssetTypes(market)
	key := tradeDate + "|" + mkt

	s.mu.RLock()
	cached, ok := s.presetStats[key]
	s.mu.RUnlock()
	if !ok {
		prevDate, _ := s.repo.PrevScoredDate(ctx, tradeDate)
		assetTypes, _ := screenAssetTypes(mkt)
		scope := repository.ScreenScope{AssetTypes: assetTypes}
		cached = make([]presetStat, len(presets))
		for i, p := range presets {
			cached[i].count = -1
			norm, _, _, err := normalizeScreenQuery(model.ScreenQuery{Market: mkt, Rules: p.Rules, Limit: 1})
			if err != nil {
				continue
			}
			today, err := s.repo.ScreenSymbols(ctx, tradeDate, scope, norm.Rules)
			if err != nil {
				return false // DB 오류는 기억하지 않는다 — 다음 요청이 다시 센다
			}
			cached[i].count = len(today)
			if prevDate == "" {
				continue
			}
			before, err := s.repo.ScreenSymbols(ctx, prevDate, scope, norm.Rules)
			if err != nil {
				return false
			}
			n := len(before)
			cached[i].prev = &n
			cached[i].newNames = newcomers(today, before, presetNewNamesMax)
		}
		s.mu.Lock()
		if s.presetStats == nil || len(s.presetStats) > 8 {
			s.presetStats = map[string][]presetStat{} // 점수일이 넘어가면 옛 키는 버린다
		}
		s.presetStats[key] = cached
		s.mu.Unlock()
	}
	for i := range presets {
		if i < len(cached) {
			presets[i].Count = cached[i].count
			presets[i].PrevCount = cached[i].prev
			presets[i].NewNames = cached[i].newNames
		}
	}
	return true
}

// newcomers — today에 있고 before에 없는 종목 이름, today 순서(점수 높은 순)로 max개.
func newcomers(today, before []repository.ScreenHit, max int) []string {
	seen := make(map[string]bool, len(before))
	for _, h := range before {
		seen[h.Symbol] = true
	}
	var out []string
	for _, h := range today {
		if len(out) >= max {
			break
		}
		if !seen[h.Symbol] {
			name := h.Name
			if name == "" {
				name = h.Symbol
			}
			out = append(out, name)
		}
	}
	return out
}
