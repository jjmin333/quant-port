package service

import (
	"context"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestEntriesForBatchCoversCatalog(t *testing.T) {
	t.Parallel()

	for _, batch := range indexBatches {
		entries := entriesForBatch(batch)
		if len(entries) != len(batch.indexIDs) {
			t.Fatalf("batch %s: got %d entries, want %d ids", batch.id, len(entries), len(batch.indexIDs))
		}
		for i, entry := range entries {
			if entry.id != batch.indexIDs[i] {
				t.Fatalf("batch %s: entry[%d].id = %q, want %q", batch.id, i, entry.id, batch.indexIDs[i])
			}
		}
	}
}

func TestBatchMinIntervalForFREDIndices(t *testing.T) {
	t.Parallel()

	want := 3 * time.Minute
	for _, id := range []string{"nasdaq", "nasdaq100", "sp500", "dow", "vix"} {
		if got := batchMinIntervalForIndex(id); got != want {
			t.Fatalf("batchMinIntervalForIndex(%q) = %s, want %s", id, got, want)
		}
	}
}

func TestDataKindForIndex(t *testing.T) {
	t.Parallel()

	cases := map[string]model.IndexDataKind{
		"kospi":     model.IndexDataKindEOD,
		"kosdaq":    model.IndexDataKindEOD,
		"usd-krw":   model.IndexDataKindEOD,
		"sp500":     model.IndexDataKindEOD,
		"nq100":     model.IndexDataKindLive,
		"sox":       model.IndexDataKindLive,
		"btc":       model.IndexDataKindLive,
		"gold":      model.IndexDataKindLive,
	}
	for id, want := range cases {
		if got := dataKindForIndex(id); got != want {
			t.Fatalf("dataKindForIndex(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestShouldRefreshBatchRespectsInterval(t *testing.T) {
	ctx := context.Background()
	st := &MarketIndexStore{
		lastRefresh:  make(map[string]time.Time),
		groupRefresh: make(map[string]time.Time),
	}
	batch := indexBatch{id: batchFREDUS, minInterval: 3 * time.Minute}

	if !st.shouldRefreshBatch(ctx, batch) {
		t.Fatal("expected refresh when no prior run")
	}

	at := time.Now().Add(-2 * time.Minute)
	st.noteGroupRefresh(string(batch.id), at, "fred")

	if st.shouldRefreshBatch(ctx, batch) {
		t.Fatal("expected skip before min interval")
	}

	at = time.Now().Add(-4 * time.Minute)
	st.noteGroupRefresh(string(batch.id), at, "fred")

	if !st.shouldRefreshBatch(ctx, batch) {
		t.Fatal("expected refresh after min interval")
	}
}
