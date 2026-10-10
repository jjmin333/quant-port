package service

import (
	"strings"
	"testing"
)

// EDGAR 값은 덮지 않고, 빈 칸만 Finnhub 보조로 채운다(2026-09-28).
func TestApplyExtMetricsFillsOnlyGaps(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	in := scoreInput{roe: f(12)} // EDGAR ROE 있음, PER 없음(TSM처럼 통화 가드로 비움)
	in.applyExtMetrics([9]*float64{f(28.3), f(7.27), nil, f(39.9), nil, nil, nil, nil, nil})
	if in.per == nil || *in.per != 28.3 || *in.pbr != 7.27 {
		t.Fatalf("gaps not filled: per=%v pbr=%v", in.per, in.pbr)
	}
	if *in.roe != 12 {
		t.Fatalf("EDGAR ROE overwritten: %v", *in.roe)
	}
	if in.psr != nil || !strings.HasSuffix(in.method(), "|ext") {
		t.Fatalf("psr=%v method=%s", in.psr, in.method())
	}

	none := scoreInput{per: f(20)}
	none.applyExtMetrics([9]*float64{f(30)})
	if *none.per != 20 || none.extUsed {
		t.Fatal("must not touch filled PER or mark ext")
	}
}
