package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestMergePaginationCursor_fallsBackToOldestBar(t *testing.T) {
	stored := []model.Candle{
		{Time: "2025-10-02"},
		{Time: "2026-07-31"},
	}
	got := mergePaginationCursor(stored, model.Interval1d, nil)
	if got == nil {
		t.Fatal("expected fallback cursor")
	}
	if *got != "2025-10-02T00:00:00+09:00" {
		t.Fatalf("cursor=%q want 2025-10-02T00:00:00+09:00", *got)
	}
}

func TestMergePaginationCursor_prefersDBContinuation(t *testing.T) {
	dbNext := "2025-01-01T00:00:00+09:00"
	stored := []model.Candle{{Time: "2025-10-02"}}
	got := mergePaginationCursor(stored, model.Interval1d, &dbNext)
	if got == nil || *got != dbNext {
		t.Fatalf("cursor=%v want %q", got, dbNext)
	}
}

func TestEnsureSnapshotPaginationCursor(t *testing.T) {
	snap := ensureSnapshotPaginationCursor(model.CandleSnapshot{
		Interval: model.Interval1d,
		Completed: []model.Candle{
			{Time: "2025-11-01"},
			{Time: "2025-10-01"},
		},
	})
	if snap.NextBefore == nil || *snap.NextBefore != "2025-10-01T00:00:00+09:00" {
		t.Fatalf("nextBefore=%v", snap.NextBefore)
	}
}
