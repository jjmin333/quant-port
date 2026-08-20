package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestPrevCloseFromDailyCandles_ascendingDBOrder(t *testing.T) {
	bars := []model.Candle{
		{Time: "2026-07-01", Close: 186_000},
		{Time: "2026-07-27", Close: 255_000},
		{Time: "2026-07-28", Close: 230_500},
	}
	prev, ok := prevCloseFromDailyCandles(bars)
	if !ok {
		t.Fatal("expected ok")
	}
	if prev != 255_000 {
		t.Fatalf("prevClose = %.0f want 255000 (yesterday close, not bars[1])", prev)
	}
}

func TestPrevCloseFromDailyCandles_descendingTossOrder(t *testing.T) {
	bars := []model.Candle{
		{Time: "2026-07-28", Close: 230_500},
		{Time: "2026-07-27", Close: 255_000},
	}
	prev, ok := prevCloseFromDailyCandles(bars)
	if !ok {
		t.Fatal("expected ok")
	}
	if prev != 255_000 {
		t.Fatalf("prevClose = %.0f want 255000", prev)
	}
}

func TestPrevCloseFromDailyCandles_singleBar(t *testing.T) {
	bars := []model.Candle{{Time: "2026-07-28", Close: 230_500}}
	prev, ok := prevCloseFromDailyCandles(bars)
	if !ok || prev != 230_500 {
		t.Fatalf("prevClose = %.0f ok=%v want 230500", prev, ok)
	}
}
