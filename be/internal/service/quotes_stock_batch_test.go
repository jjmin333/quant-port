package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

func newTestQuotesServiceWithBatch() *QuotesService {
	return &QuotesService{stockInfoBatch: make(map[string]stockInfoBatchEntry)}
}

func TestLookupPrefetchedStock_hit(t *testing.T) {
	s := newTestQuotesServiceWithBatch()
	s.stockInfoBatch["005930"] = stockInfoBatchEntry{
		info:      toss.StockInfo{Symbol: "005930", Name: "삼성전자"},
		expiresAt: time.Now().Add(stockInfoBatchTTL),
	}

	stock, ok := s.lookupPrefetchedStock("005930")
	if !ok {
		t.Fatal("ok=false, want true")
	}
	if stock.Name != "삼성전자" {
		t.Fatalf("name=%q want 삼성전자", stock.Name)
	}
}

func TestLookupPrefetchedStock_caseInsensitive(t *testing.T) {
	s := newTestQuotesServiceWithBatch()
	s.stockInfoBatch["AAPL"] = stockInfoBatchEntry{
		info:      toss.StockInfo{Symbol: "AAPL"},
		expiresAt: time.Now().Add(stockInfoBatchTTL),
	}

	if _, ok := s.lookupPrefetchedStock("aapl"); !ok {
		t.Fatal("lowercase lookup missed an uppercase-keyed entry")
	}
}

func TestLookupPrefetchedStock_miss(t *testing.T) {
	s := newTestQuotesServiceWithBatch()
	if _, ok := s.lookupPrefetchedStock("000660"); ok {
		t.Fatal("ok=true for a symbol never prefetched, want false")
	}
}

// A symbol from a stale tick's batch must not be served past its TTL — the
// caller falls back to an individual GetStock instead of serving old data.
func TestLookupPrefetchedStock_expired(t *testing.T) {
	s := newTestQuotesServiceWithBatch()
	s.stockInfoBatch["000660"] = stockInfoBatchEntry{
		info:      toss.StockInfo{Symbol: "000660"},
		expiresAt: time.Now().Add(-time.Second),
	}

	if _, ok := s.lookupPrefetchedStock("000660"); ok {
		t.Fatal("ok=true for an expired entry, want false")
	}
}

func TestPrefetchStockInfo_disabledWhenTossNotEnabled(t *testing.T) {
	s := newTestQuotesServiceWithBatch()
	s.prefetchStockInfo(nil, []string{"005930"}) //nolint:staticcheck // TossEnabled() short-circuits before ctx is used

	if _, ok := s.lookupPrefetchedStock("005930"); ok {
		t.Fatal("prefetch populated the batch despite Toss being disabled")
	}
}
