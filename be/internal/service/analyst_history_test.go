package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/repository"
)

func fp(v float64) *float64 { return &v }

// 국내 이력은 값이 바뀐 날만 남긴다(099) — 같은 값을 매일 쌓지 않는다.
func TestAnalystValueChanged(t *testing.T) {
	prev := &repository.AnalystHistoryRow{Rating: fp(4.0), TargetMean: fp(495227)}
	cases := []struct {
		name           string
		prev           *repository.AnalystHistoryRow
		rating, target *float64
		want           bool
	}{
		{"이력 없음", nil, fp(4), fp(1), true},
		{"그대로", prev, fp(4.0), fp(495227), false},
		{"목표가 0.05% — 반올림 수준", prev, fp(4.0), fp(495227 * 1.0005), false},
		{"목표가 1% 상향", prev, fp(4.0), fp(495227 * 1.01), true},
		{"의견 4.00 → 4.05", prev, fp(4.05), fp(495227), true},
		{"목표가가 사라짐", prev, fp(4.0), nil, true},
	}
	for _, c := range cases {
		if got := analystValueChanged(c.prev, c.rating, c.target); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestFinnhubHistoryRow(t *testing.T) {
	row := finnhubHistoryRow(marketdata.FinnhubRecommendation{Period: "2026-10-01", StrongBuy: 10, Buy: 20, Hold: 10})
	if row.AsOf != "2026-10-01" || row.Counts[1] != 20 || row.Rating == nil || *row.Rating != 4.0 {
		t.Fatalf("unexpected row %+v rating %v", row, row.Rating)
	}
}
