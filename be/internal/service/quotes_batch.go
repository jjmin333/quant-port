package service

import (
	"context"
	"strings"
	"sync"

	"github.com/enact/quant-port/be/internal/model"
)

// BatchQuoteSummariesMax bounds how many distinct symbols one batch call
// resolves. Exported so the HTTP layer can publish the same number instead of
// silently handing back a shorter list than the caller asked for.
const BatchQuoteSummariesMax = 100

const batchQuoteSummariesMax = BatchQuoteSummariesMax

// batchQuoteConcurrency bounds simultaneous upstream lookups for one batch call.
const batchQuoteConcurrency = 8

// BatchQuoteSummaries returns live quote summaries using the same path as GET /quotes/{symbol}.
func (s *QuotesService) BatchQuoteSummaries(ctx context.Context, symbols []string) []model.QuoteSummary {
	if len(symbols) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	var ordered []string
	for _, raw := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(raw))
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		ordered = append(ordered, sym)
		if len(ordered) >= batchQuoteSummariesMax {
			break
		}
	}
	if len(ordered) == 0 {
		return nil
	}

	// Resolve the caller's watchlist/holdings once for the whole batch.
	ctx = withOverlayCache(ctx)

	// Each goroutine owns one slot, so results keep the requested order and no
	// mutex is needed. Slots for symbols that failed to resolve stay empty and
	// are dropped below.
	slots := make([]model.QuoteSummary, len(ordered))
	filled := make([]bool, len(ordered))
	sem := make(chan struct{}, batchQuoteConcurrency)
	var wg sync.WaitGroup

	for i, sym := range ordered {
		wg.Add(1)
		go func(idx int, symbol string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}
			s.poller.Touch(symbol)
			detail, ok := s.EnsureQuoteDetail(ctx, symbol)
			if !ok {
				return
			}
			slots[idx] = detail.QuoteSummary
			filled[idx] = true
		}(i, sym)
	}
	wg.Wait()

	out := make([]model.QuoteSummary, 0, len(ordered))
	for i := range slots {
		if filled[i] {
			out = append(out, slots[i])
		}
	}
	return out
}
