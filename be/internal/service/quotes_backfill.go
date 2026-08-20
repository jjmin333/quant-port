package service

import (
	"context"
	"strings"
)

// backfillClosedMarketFromToss fetches Toss once when Toss is disabled or live poll is off.
// With continuous Toss polling, this is only used as a cold-start fallback from loadClosedMarket*.
func (s *QuotesService) backfillClosedMarketFromToss(ctx context.Context, symbol string) {
	if s.TossEnabled() {
		return
	}

	s.initQuoteRefresh()
	sym := strings.ToUpper(strings.TrimSpace(symbol))

	s.quoteRefresh.doSync(ctx, "closed-backfill:"+sym, func(refreshCtx context.Context) {
		if detail, ok := s.getQuoteUncached(refreshCtx, sym); ok {
			s.persistQuoteDetail(refreshCtx, sym, detail, "toss")
		}

		if book, ok := s.getOrderBookUncached(refreshCtx, sym); ok {
			s.persistOrderBook(refreshCtx, sym, book)
		}

		if trades, ok := s.getTradesUncached(refreshCtx, sym); ok {
			s.persistTrades(refreshCtx, sym, trades)
		}

		_ = s.refreshRawCandles(refreshCtx, sym)
	})
}
