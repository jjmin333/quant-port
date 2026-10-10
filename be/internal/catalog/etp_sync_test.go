package catalog

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

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
