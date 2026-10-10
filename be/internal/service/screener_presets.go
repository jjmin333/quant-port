package service

import (
	"context"

	"github.com/enact/quant-port/be/internal/model"
)

/*
스크리너 프리셋 (2026-10-09 사용자 결정 A — docs/mockups/requests-1009-v1.html ②).

「무엇부터 눌러야 할지 모른다」가 스크리너를 안 쓰는 이유였다. 목적으로 이름 붙인 조건 묶음을 카드로
먼저 보여 주고, 카드마다 **지금 몇 종목이 걸리는지**를 같이 적는다(토스증권 「주식 골라보기」의 장점 +
근거·값이 안 보이는 단점은 칩·파란 값으로 보완).

결과 수는 깔때기 집계만으로 세고 점수일마다 한 번만 센다(screener_presets_count.go — 운영 p50 5.7초였다).
*/

// ScreenPreset is one purpose card.
type ScreenPreset struct {
	ID    string             `json:"id"`
	Label string             `json:"label"`
	Icon  string             `json:"icon"`
	Desc  string             `json:"desc"`
	Rules []model.ScreenRule `json:"rules"`
	Count int                `json:"count"`
	// PrevCount — 앞 점수일의 통과 수(없으면 생략). NewNames — 그때 없다가 새로 걸린 종목, 점수 높은 순 최대 3(2026-10-10).
	PrevCount *int     `json:"prevCount,omitempty"`
	NewNames  []string `json:"newNames,omitempty"`
}

func presetRule(field string, min, max *float64) model.ScreenRule {
	return model.ScreenRule{Field: field, Min: min, Max: max}
}

func screenPresetDefs() []ScreenPreset {
	f := floatPtr
	return []ScreenPreset{
		{ID: "value", Label: "저평가 우량주", Icon: "💎", Desc: "ROE 10%↑ · PER 12배↓ · 부채비율 100%↓",
			Rules: []model.ScreenRule{presetRule("roe", f(10), nil), presetRule("per", nil, f(12)), presetRule("debtRatio", nil, f(100))}},
		{ID: "growth", Label: "실적 성장주", Icon: "🚀", Desc: "매출·EPS 1년 20%↑ 성장",
			Rules: []model.ScreenRule{presetRule("revenueGrowthYoY", f(20), nil), presetRule("epsGrowthYoY", f(20), nil)}},
		{ID: "upside", Label: "목표가 여력", Icon: "🎯", Desc: "목표가 대비 30%↑ · 종합 60점↑",
			Rules: []model.ScreenRule{presetRule("targetUpside", f(30), nil), presetRule("score", f(60), nil)}},
		{ID: "earnings", Label: "실적 발표 임박", Icon: "📅", Desc: "7일 안에 실적 발표(추정 포함)",
			Rules: []model.ScreenRule{presetRule("daysToEarnings", f(0), f(7))}},
		{ID: "flow", Label: "외국인·기관 순매수", Icon: "🏦", Desc: "순매수 0.5%↑ · 20일 수익 플러스",
			Rules: []model.ScreenRule{presetRule("flowNetPct", f(0.5), nil), presetRule("return20d", f(0), nil)}},
		{ID: "oversold", Label: "과매도 반등 후보", Icon: "🌱", Desc: "RSI 30↓ · 종합 55점↑",
			Rules: []model.ScreenRule{presetRule("rsi14", nil, f(30)), presetRule("score", f(55), nil)}},
		{ID: "high", Label: "52주 신고가 근접", Icon: "🆕", Desc: "52주 고점 대비 -3% 이내",
			Rules: []model.ScreenRule{presetRule("pctFromHigh52Week", f(-3), nil)}},
		{ID: "jump", Label: "AI 점수 급등", Icon: "⚡", Desc: "전 점수일보다 8점↑",
			Rules: []model.ScreenRule{presetRule("scoreChange", f(8), nil)}},
	}
}

// Presets returns the purpose cards with today's pass counts for a market.
// 한 프리셋이 실패해도(필드가 그 시장에 없음 등) 나머지는 낸다 — 그 카드의 수만 -1(화면이 「—」).
func (s *ScreenerService) Presets(ctx context.Context, market string) []ScreenPreset {
	out := screenPresetDefs()
	if !s.presetCounts(ctx, market, out) {
		for i := range out {
			out[i].Count = -1 // 셀 수 없음 — 카드는 수 없이 그린다
		}
	}
	return out
}
