package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// backfillClosedMarketFromToss fetches the last available Toss snapshot once when
// local caches/DB are cold after market close. Results are written to MySQL + Redis.
func (s *QuotesService) backfillClosedMarketFromToss(ctx context.Context, symbol string) {
	if !s.TossEnabled() || s.shouldLivePollSymbol(ctx, symbol) {
		return
	}

	s.initQuoteRefresh()
	sym := strings.ToUpper(strings.TrimSpace(symbol))

	s.quoteRefresh.doSync(ctx, "closed-backfill:"+sym, func(refreshCtx context.Context) {
		if detail, ok := s.getQuoteUncached(refreshCtx, sym); ok {
			s.persistQuoteDetail(refreshCtx, sym, detail, "toss")
			s.cacheSetJSON(refreshCtx, fmt.Sprintf(quotesDetailKeyFmt, sym), detail,
				time.Duration(model.MarketDataLastGoodCacheSec)*time.Second)
		}

		if book, ok := s.getOrderBookUncached(refreshCtx, sym); ok {
			s.persistOrderBook(refreshCtx, sym, book)
			s.cacheSetJSON(refreshCtx, fmt.Sprintf(quotesBookKeyFmt, sym), book,
				time.Duration(model.MarketDataLastGoodCacheSec)*time.Second)
		}

		if trades, ok := s.getTradesUncached(refreshCtx, sym); ok {
			s.persistTrades(refreshCtx, sym, trades)
			s.cacheSetJSON(refreshCtx, fmt.Sprintf(quotesTradesKeyFmt, sym), trades,
				time.Duration(model.MarketDataLastGoodCacheSec)*time.Second)
		}

		_ = s.refreshRawCandles(refreshCtx, sym)
	})
}
