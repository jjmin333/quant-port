package service

import (
	"context"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

// stockInfoBatchTTL bounds how long a poller-prefetched entry stays usable.
// Short on purpose: this only exists to let one fast tick's worth of
// concurrent symbol refreshes share a single GetStocks call instead of each
// making its own — it is not a general-purpose cache for /api/v1/stocks.
const stockInfoBatchTTL = 3 * time.Second

type stockInfoBatchEntry struct {
	info      toss.StockInfo
	expiresAt time.Time
}

// prefetchStockInfo fetches reference data for symbols in one batched call
// (GetStocks supports up to 200 symbols per request) and stores the results
// for buildQuoteDetailFromToss to read instead of each symbol calling
// GetStock individually. /api/v1/stocks and GetStock/GetStocks share Toss's
// STOCK rate-limit bucket, which the per-symbol call pattern was exhausting
// on every fast tick.
func (s *QuotesService) prefetchStockInfo(ctx context.Context, symbols []string) {
	if !s.TossEnabled() || len(symbols) == 0 {
		return
	}

	seen := map[string]struct{}{}
	batch := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		up := strings.ToUpper(strings.TrimSpace(sym))
		if up == "" {
			continue
		}
		if _, ok := seen[up]; ok {
			continue
		}
		seen[up] = struct{}{}
		batch = append(batch, up)
	}
	if len(batch) == 0 {
		return
	}

	expiresAt := time.Now().Add(stockInfoBatchTTL)
	for i := 0; i < len(batch); i += 200 {
		end := min(i+200, len(batch))
		stocks, err := s.toss.GetStocks(ctx, batch[i:end])
		if err != nil {
			// Rate-limited or transient — callers fall back to an individual
			// GetStock for any symbol this chunk would have covered.
			continue
		}

		s.stockInfoBatchMu.Lock()
		for _, st := range stocks {
			sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
			if sym == "" {
				continue
			}
			s.stockInfoBatch[sym] = stockInfoBatchEntry{info: st, expiresAt: expiresAt}
		}
		s.stockInfoBatchMu.Unlock()
	}
}

func (s *QuotesService) lookupPrefetchedStock(symbol string) (*toss.StockInfo, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))

	s.stockInfoBatchMu.RLock()
	entry, ok := s.stockInfoBatch[sym]
	s.stockInfoBatchMu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	info := entry.info
	return &info, true
}
