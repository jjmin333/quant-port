package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func TestNormalizeTossCandlesParams_dailyDateOnly(t *testing.T) {
	params := normalizeTossCandlesParams(toss.GetCandlesParams{
		Symbol:   "000660",
		Interval: toss.CandleInterval1d,
		Before:   "2024-01-26",
		Count:    200,
	})
	want := "2024-01-26T00:00:00+09:00"
	if params.Before != want {
		t.Fatalf("before=%q want %q", params.Before, want)
	}
}

func TestNormalizeCandlesPageCursor_daily(t *testing.T) {
	raw := "2024-01-26"
	page := normalizeCandlesPageCursor(CandlesPage{NextBefore: &raw}, model.Interval1d)
	if page.NextBefore == nil || *page.NextBefore != "2024-01-26T00:00:00+09:00" {
		t.Fatalf("nextBefore=%v", page.NextBefore)
	}
}

func TestGetCandlesPage_inboundBeforeCanonical(t *testing.T) {
	base, ok := paginationBaseInterval(model.Interval1M)
	if !ok || base != model.Interval1d {
		t.Fatalf("base=%q ok=%v", base, ok)
	}
	got := model.CanonicalPaginationCursor("2024-01-26", base)
	if got != "2024-01-26T00:00:00+09:00" {
		t.Fatalf("got %q", got)
	}
}
