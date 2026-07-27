package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/toss"
)

func TestOrderStockInfosPreservesRequestOrder(t *testing.T) {
	t.Parallel()

	order := []string{"ARM", "CCJ", "AMAT", "SOXL"}
	missing := make([]string, 0, len(order))
	dbHits := map[string]toss.StockInfo{
		"ARM":  {Symbol: "ARM"},
		"CCJ":  {Symbol: "CCJ"},
		"AMAT": {Symbol: "AMAT"},
	}

	out := make([]toss.StockInfo, 0, len(order))
	for _, sym := range order {
		if info, ok := dbHits[sym]; ok {
			out = append(out, info)
			continue
		}
		missing = append(missing, sym)
	}

	if order[0] != "ARM" {
		t.Fatalf("order corrupted after missing append: %v", order)
	}

	out = append(out, toss.StockInfo{Symbol: "SOXL"})
	got := orderStockInfos(out, order)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	for i, sym := range order {
		if got[i].Symbol != sym {
			t.Fatalf("got[%d].Symbol = %q, want %q", i, got[i].Symbol, sym)
		}
	}
}
