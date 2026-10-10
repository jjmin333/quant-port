package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

func TestSummarizeWatchTrades(t *testing.T) {
	p := func(v int64) *int64 { return &v }
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	trades := []repository.ReTrade{ // 최신순
		{ComplexKey: "a", DealDate: "2026-09-28", Area: 84.6, Price: p(300)},
		{ComplexKey: "a", DealDate: "2026-09-20", Area: 84.39, Price: p(310), Cancelled: true},
		{ComplexKey: "b", DealDate: "2026-09-10", Area: 59.9, Price: p(200)},
		{ComplexKey: "a", DealDate: "2026-08-01", Area: 84.89, Price: p(290)},
		{ComplexKey: "a", DealDate: "2026-07-01", Area: 59.0, Price: p(400)}, // 다른 평형
		{ComplexKey: "b", DealDate: "2026-06-01", Area: 59.1, Price: p(250)},
		{ComplexKey: "c", DealDate: "2026-09-30", Area: 84, Price: p(999), Direct: true},
		{ComplexKey: "c", DealDate: "2026-01-01", Area: 84, Price: p(500)},
	}
	got := SummarizeWatchTrades(trades, now)
	if a := got["a"]; a.Recent30 != 1 || a.Last == nil || *a.Last.Price != 300 || !a.NewHigh {
		t.Fatalf("a: %+v", a)
	}
	if b := got["b"]; b.NewHigh || b.Recent30 != 1 {
		t.Fatalf("b: %+v", b)
	}
	if c := got["c"]; c.NewHigh {
		t.Fatalf("c direct trade must not be new high: %+v", c)
	}
}
