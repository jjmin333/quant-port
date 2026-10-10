package service

import (
	"database/sql"
	"fmt"
	"math"
	"testing"
)

func ttmQ(year, idx int, rev, op, ni, eq float64) ttmQuarter {
	v := func(x float64) sql.NullFloat64 { return sql.NullFloat64{Float64: x, Valid: true} }
	return ttmQuarter{seq: year*4 + idx, label: fmt.Sprintf("%dQ%d", year, idx), revenue: v(rev), opIncome: v(op), netIncome: v(ni), equity: v(eq)}
}

func TestComputeTTM_overlaysAnnualWithLastFourQuarters(t *testing.T) {
	// 최신순 8분기. 최근 4분기 순이익 합 = 40, 그 전 4분기 = 20.
	quarters := []ttmQuarter{
		ttmQ(2026, 2, 100, 30, 16, 500), ttmQ(2026, 1, 80, 20, 10, 480),
		ttmQ(2025, 4, 70, 15, 8, 460), ttmQ(2025, 3, 50, 10, 6, 440),
		ttmQ(2025, 2, 50, 10, 5, 420), ttmQ(2025, 1, 50, 10, 5, 400),
		ttmQ(2024, 4, 50, 10, 5, 380), ttmQ(2024, 3, 50, 10, 5, 360),
	}
	ttm, ok := computeTTM(quarters)
	if !ok {
		t.Fatal("8 연속 분기인데 TTM이 없다")
	}
	in := &scoreInput{assetType: "domestic", close: 100, shares: 4, marketCap: 400}
	old := 999.0
	in.per = &old // 연간 PER — 덮여야 한다
	in.applyTTM(ttm)

	// EPS = 40/4 = 10 → PER 10. 연간 값(999)이 남으면 이 고침이 안 된 것이다.
	if in.per == nil || math.Abs(*in.per-10) > 1e-9 {
		t.Fatalf("PER = %v, want 10", in.per)
	}
	if in.pbr == nil || math.Abs(*in.pbr-0.8) > 1e-9 { // 400 / 최근 분기말 자본 500
		t.Fatalf("PBR = %v, want 0.8", in.pbr)
	}
	if in.revGrowth == nil || math.Abs(*in.revGrowth-50) > 1e-9 { // 300 vs 200
		t.Fatalf("매출 성장 = %v, want 50", in.revGrowth)
	}
	if in.epsGrowth == nil || math.Abs(*in.epsGrowth-100) > 1e-9 { // 40 vs 20
		t.Fatalf("순이익 성장 = %v, want 100", in.epsGrowth)
	}
	if in.ttmPeriod == "" || in.method() == scoreMethodVersion {
		t.Fatalf("판에 |ttm이 안 붙었다: %q", in.method())
	}
}

func TestComputeTTM_gapFallsBackToAnnual(t *testing.T) {
	// 2025Q4가 빠졌다 — 세 분기를 1년으로 읽으면 안 된다.
	quarters := []ttmQuarter{
		ttmQ(2026, 2, 1, 1, 1, 1), ttmQ(2026, 1, 1, 1, 1, 1), ttmQ(2025, 3, 1, 1, 1, 1), ttmQ(2025, 2, 1, 1, 1, 1),
	}
	if _, ok := computeTTM(quarters); ok {
		t.Fatal("빈틈 있는 분기로 TTM을 만들었다")
	}
}

func TestApplyTTM_lossClearsPER(t *testing.T) {
	quarters := []ttmQuarter{
		ttmQ(2026, 2, 10, -5, -3, 50), ttmQ(2026, 1, 10, 1, 1, 50), ttmQ(2025, 4, 10, 1, 1, 50), ttmQ(2025, 3, 10, 1, 0, 50),
	}
	ttm, _ := computeTTM(quarters)
	in := &scoreInput{close: 10, shares: 1, marketCap: 10}
	old := 5.0
	in.per = &old
	in.applyTTM(ttm)
	if in.per != nil {
		t.Fatalf("4분기 합이 적자인데 PER = %v", *in.per)
	}
}
