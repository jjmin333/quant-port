package catalog

import (
	"context"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

type stubETPSource struct {
	domestic []model.Stock
	us       []model.Stock
}

func (s stubETPSource) DomesticETP(context.Context) ([]model.Stock, error) {
	return s.domestic, nil
}

func (s stubETPSource) US_ETP(context.Context) ([]model.Stock, error) {
	return s.us, nil
}

func TestMergeStocksDedupes(t *testing.T) {
	merged := mergeStocks([]model.Stock{
		{Symbol: "qqq", Name: "A"},
		{Symbol: "QQQ", Name: "B"},
		{Symbol: "0193T0", Name: "C"},
	})
	if len(merged) != 2 {
		t.Fatalf("want 2 unique symbols, got %d", len(merged))
	}
	if merged[0].Symbol != "QQQ" {
		t.Fatalf("symbol normalized to uppercase")
	}
}
