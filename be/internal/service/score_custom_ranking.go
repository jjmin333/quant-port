package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
CustomRankingService — AI 랭킹의 「내 기준」 보기 (2026-09-23 사용자 결정).

**점수를 다시 계산하지 않는다.** 저장된 축 점수를 사용자 가중치로 다시 합산만 한다 —
관리자 「가중치 바꿔 보기」가 쓰는 `compositeReconstruct` 그대로다. 그래서 기본 가중치를
넣으면 저장된 종합 점수가 그대로 나온다(시험이 지킨다). 커버리지는 저장값을 쓴다 —
가중치가 바뀌면 커버리지도 조금 달라지지만, 관리자 도구와 같은 근사를 택해 두 화면이
같은 숫자를 말하게 한다.

# 비용

시장당 한 쿼리(축 점수만, `inputs` 제외)와 Go 안의 합산이다. 같은 (점수일 · 시장)의 재료는
10분 동안 메모리에 둔다 — 슬라이더를 움직일 때마다 DB에 가지 않는다.
*/
type CustomRankingService struct {
	repo     *repository.UserScoreWeightsRepo
	settings *ScoreSettingsStore

	mu    sync.Mutex
	cache map[string]customRankingCache
}

type customRankingCache struct {
	at        time.Time
	tradeDate string
	rows      []repository.CustomRankingRow
}

func NewCustomRankingService(repo *repository.UserScoreWeightsRepo, settings *ScoreSettingsStore) *CustomRankingService {
	return &CustomRankingService{repo: repo, settings: settings, cache: map[string]customRankingCache{}}
}

const customRankingCacheTTL = 10 * time.Minute

// CustomRankingItem is one row of the re-weighted list.
type CustomRankingItem struct {
	Rank   int    `json:"rank"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	// Score is the user's composite; Default is the stored one. Delta = Score − Default.
	Score   int `json:"score"`
	Default int `json:"default"`
	Delta   int `json:"delta"`
}

// CustomRanking re-sums the market's rankable rows with `w` and returns the top `limit`.
func (s *CustomRankingService) CustomRanking(ctx context.Context, market string, w model.ScoreWeights, limit int) (map[string]any, error) {
	assetTypes, ok := scoreRankingMarkets[market]
	if !ok {
		return nil, fmt.Errorf("unknown market %q", market)
	}
	w = w.Clamped()
	total := 0
	for _, axis := range model.ScoreAxes {
		total += w.Get(axis)
	}
	if total <= 0 {
		return nil, fmt.Errorf("weights must not all be zero")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	tradeDate, rows, err := s.rows(ctx, market, assetTypes)
	if err != nil {
		return nil, err
	}
	settings := model.DefaultScoreSettings()
	if s.settings != nil {
		settings = s.settings.Get(ctx)
	}
	items := make([]CustomRankingItem, 0, len(rows))
	for _, r := range rows {
		if !r.Score.Valid {
			continue
		}
		p := compositeReconstruct(r.Breakdown, w, r.Coverage, settings.ShrinkFull)
		if !p.available || r.Coverage < settings.CoverageFloor {
			continue
		}
		score := int(clamp01to100(p.final) + 0.5)
		def := int(r.Score.Int64)
		items = append(items, CustomRankingItem{Symbol: r.Symbol, Name: r.Name, Score: score, Default: def, Delta: score - def})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return items[i].Symbol < items[j].Symbol
	})
	if len(items) > limit {
		items = items[:limit]
	}
	for i := range items {
		items[i].Rank = i + 1
	}
	return map[string]any{
		"tradeDate": tradeDate,
		"market":    market,
		"weights":   w,
		"defaults":  settings.Weights,
		"items":     items,
		// 커버리지는 저장값 근사다 — 화면이 한 줄로 밝힌다.
		"note": fmt.Sprintf("저장된 축 점수를 이 가중치로 다시 합산했습니다(%s 기준). 커버리지 보정은 기본 가중치 때의 값을 씁니다.", tradeDate),
	}, nil
}

func (s *CustomRankingService) rows(ctx context.Context, market string, assetTypes []string) (string, []repository.CustomRankingRow, error) {
	s.mu.Lock()
	if c, ok := s.cache[market]; ok && time.Since(c.at) < customRankingCacheTTL {
		s.mu.Unlock()
		return c.tradeDate, c.rows, nil
	}
	s.mu.Unlock()
	tradeDate, rows, err := s.repo.CustomRankingRows(ctx, assetTypes)
	if err != nil {
		return "", nil, err
	}
	s.mu.Lock()
	s.cache[market] = customRankingCache{at: time.Now(), tradeDate: tradeDate, rows: rows}
	s.mu.Unlock()
	return tradeDate, rows, nil
}

// reproducesStored reports whether default weights give back the stored score
// (used by the test; kept here so the arithmetic under test is the service's own).
func reproducesStored(r repository.CustomRankingRow, settings model.ScoreSettings) bool {
	p := compositeReconstruct(r.Breakdown, settings.Weights, r.Coverage, settings.ShrinkFull)
	return int(math.Round(clamp01to100(p.final))) == int(r.Score.Int64)
}
