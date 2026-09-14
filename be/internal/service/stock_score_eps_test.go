package service

import (
	"database/sql"
	"testing"
)

func nf(v float64) sql.NullFloat64 { return sql.NullFloat64{Float64: v, Valid: true} }

/*
PER의 분모는 **화면과 같은 순서로** 고른다 (2026-09-03 §7).

실측한 문제: SK하이닉스(000660)는 `eps_basic`이 비어 있다. 밸류에이션 탭은
순이익 ÷ 발행주식수로 되짚어 PER 27.6을 보여 주는데, 점수 쪽은 그 갈래가 없어
**PER을 아예 못 본 채** PBR·PSR 둘로만 가치 축을 매겼다 — 각각 동종 백분위 2.6 ·
4.1이라 가치 축이 3.3점으로 바닥에 붙어 있었다.
*/
func TestScoreEPSFallsBackToNetIncomePerShare(t *testing.T) {
	var none sql.NullFloat64

	// 실측값: 순이익 42,947,902,000,000 / 발행주식 730,492,365 ≈ 58,793원
	eps, ok := scoreEPS(none, none, nf(42_947_902_000_000), 730_492_365)
	if !ok {
		t.Fatal("expected a derived EPS when net income and shares are present")
	}
	if eps < 58_000 || eps > 59_500 {
		t.Fatalf("derived EPS %.0f is not net income per share", eps)
	}
	// 그 EPS로 계산한 PER이 화면이 보여 주던 값과 같은 자리에 서야 한다.
	if per := 1_613_000 / eps; per < 26 || per > 29 {
		t.Fatalf("PER from derived EPS = %.1f, expected ~27", per)
	}
}

// 보고된 EPS가 있으면 그것이 이긴다 — 희석 > 기본 > 되짚기.
func TestScoreEPSPrefersReportedValues(t *testing.T) {
	var none sql.NullFloat64
	if eps, _ := scoreEPS(nf(120), nf(100), nf(1_000_000), 1_000); eps != 120 {
		t.Fatalf("diluted EPS should win, got %.1f", eps)
	}
	if eps, _ := scoreEPS(none, nf(100), nf(1_000_000), 1_000); eps != 100 {
		t.Fatalf("basic EPS should win over the derived value, got %.1f", eps)
	}
}

// 되짚을 재료가 없으면 **없다**고 답한다. 0을 돌려주면 부르는 쪽이 그것으로
// 나눠 PER이 무한대가 된다.
func TestScoreEPSAbsentWithoutMaterial(t *testing.T) {
	var none sql.NullFloat64
	if _, ok := scoreEPS(none, none, none, 1_000); ok {
		t.Fatal("expected no EPS without net income")
	}
	if _, ok := scoreEPS(none, none, nf(1_000_000), 0); ok {
		t.Fatal("expected no EPS without a share count")
	}
}

// 적자 기업의 EPS는 음수다. PER은 뜻이 없으므로 부르는 쪽이 `> 0`으로 거르고,
// 이 함수는 값을 그대로 돌려준다 — 「PER 없음」과 「PER 나쁨」은 다른 말이다.
func TestScoreEPSReportsNegativeEarningsAsIs(t *testing.T) {
	var none sql.NullFloat64
	eps, ok := scoreEPS(none, none, nf(-5_000_000), 1_000)
	if !ok || eps >= 0 {
		t.Fatalf("a loss should surface as a negative EPS, got %.1f (ok=%v)", eps, ok)
	}
}
