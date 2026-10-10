package repository

import "testing"

func TestFlowStreak(t *testing.T) {
	cases := []struct {
		nets []int64
		want int
	}{
		{[]int64{10, 5, 3, -1, 8}, 3},
		{[]int64{-4, -2, 7}, -2},
		{[]int64{0, 5}, 0},
		{nil, 0},
		{[]int64{1, 1, 1}, 3},
	}
	for _, c := range cases {
		if got := FlowStreak(c.nets); got != c.want {
			t.Errorf("FlowStreak(%v)=%d want %d", c.nets, got, c.want)
		}
	}
}

func TestMedianPositiveDropsLossMakers(t *testing.T) {
	m, n := MedianPositive([]float64{-5, 0, 10, 20, 30, 40})
	if n != 4 || m != 25 {
		t.Fatalf("적자(음수)·0은 빼고 중앙값: m=%v n=%d", m, n)
	}
	if _, n := MedianPositive([]float64{-1, 0}); n != 0 {
		t.Fatal("양수가 없으면 0개")
	}
}

func TestIsRiskDisclosure(t *testing.T) {
	for _, title := range []string{"주요사항보고서(유상증자결정)", "전환사채권발행결정", "소송등의제기", "감사의견 거절"} {
		if !IsRiskDisclosure(title) {
			t.Errorf("%q는 위험 공시", title)
		}
	}
	for _, title := range []string{"현금ㆍ현물배당결정", "분기보고서 (2026.06)", "자기주식취득결정"} {
		if IsRiskDisclosure(title) {
			t.Errorf("%q는 위험 공시가 아니다", title)
		}
	}
}

func TestPriceContext(t *testing.T) {
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = 100 - float64(i) // 최신이 앞: 100, 99, …
	}
	ret, pos, ok := PriceContext(closes)
	if !ok || ret != 25 || pos != 100 {
		t.Fatalf("20일 수익률 +25%%·52주 고점: ret=%v pos=%v ok=%v", ret, pos, ok)
	}
	if _, _, ok := PriceContext(closes[:10]); ok {
		t.Fatal("21봉 미만은 계산하지 않는다")
	}
}

func TestFiscalLabel(t *testing.T) {
	if got := (EarningsPreviewTarget{FiscalYear: 2026, FiscalPeriod: "Q3"}).FiscalLabel(); got != "2026 3Q" {
		t.Fatalf("%q", got)
	}
}
