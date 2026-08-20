package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestDecodeCacheEnvelope_plainAndWrapped(t *testing.T) {
	plain, at, env, ok := decodeCacheEnvelope[[]model.TradeTick]([]byte(`[{"symbol":"005930","price":70000,"time":"2026-08-04T10:00:00+09:00"}]`))
	if !ok || env || !at.IsZero() || len(plain) != 1 || plain[0].Price != 70000 {
		t.Fatalf("plain decode failed: %+v ok=%v env=%v", plain, ok, env)
	}

	wrapped, err := json.Marshal(cacheEnvelope[model.QuoteDetail]{
		Data: model.QuoteDetail{
			QuoteSummary: model.QuoteSummary{Symbol: "000660", CurrentPrice: 1_535_000},
		},
		FetchedAt: time.Date(2026, 8, 4, 2, 30, 0, 0, time.UTC),
		Source:    "toss",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, gotAt, gotEnv, ok := decodeCacheEnvelope[model.QuoteDetail](wrapped)
	if !ok || !gotEnv || got.QuoteSummary.Symbol != "000660" || got.QuoteSummary.CurrentPrice != 1_535_000 {
		t.Fatalf("wrapped decode = %+v ok=%v env=%v", got, ok, gotEnv)
	}
	if gotAt.IsZero() {
		t.Fatal("expected fetchedAt")
	}
}

func TestTradesDataStale(t *testing.T) {
	old := []model.TradeTick{{Time: time.Now().Add(-24 * time.Hour).Format(time.RFC3339), Price: 1_718_000}}
	if !tradesDataStale(old, liveMicrostructureMaxAge) {
		t.Fatal("expected stale trades")
	}
	fresh := []model.TradeTick{{Time: time.Now().Add(-30 * time.Second).Format(time.RFC3339), Price: 1_535_000}}
	if tradesDataStale(fresh, liveMicrostructureMaxAge) {
		t.Fatal("expected fresh trades")
	}
}

func TestEnvelopeFetchedAtStale(t *testing.T) {
	if !envelopeFetchedAtStale(time.Time{}, time.Minute) {
		t.Fatal("zero time should be stale")
	}
	if !envelopeFetchedAtStale(time.Now().Add(-10*time.Minute), 5*time.Minute) {
		t.Fatal("10m old should be stale")
	}
	if envelopeFetchedAtStale(time.Now(), 5*time.Minute) {
		t.Fatal("now should be fresh")
	}
}
