package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// persist* writes MySQL (source of truth). book/trades also write Redis envelope cache.
// load* reads Redis envelope (book/trades) or MySQL (detail), warming Redis on DB hit where applicable.

func (s *QuotesService) persistQuoteDetail(ctx context.Context, symbol string, detail model.QuoteDetail, source string) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if s.quoteTerminal != nil {
		_ = s.quoteTerminal.UpsertDetail(ctx, sym, detail, source)
		if detail.CurrentPrice > 0 && !s.shouldSkipLastPricePersist(ctx, sym, detail.CurrentPrice) {
			_ = s.quoteTerminal.UpsertLastPrice(ctx, sym, detail.CurrentPrice, detail.Currency)
		}
	}
}

func (s *QuotesService) shouldSkipLastPricePersist(ctx context.Context, symbol string, price float64) bool {
	session := s.marketSessionFor(ctx, symbol)
	if !quoteSessionHasLiveMicrostructure(session) {
		return false
	}
	return s.priceLooksLikePrevClose(ctx, symbol, price)
}

func (s *QuotesService) loadQuoteDetailPersisted(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if s.quoteTerminal == nil {
		return model.QuoteDetail{}, false
	}
	detail, ok := s.quoteTerminal.GetDetail(ctx, sym)
	if !ok {
		return model.QuoteDetail{}, false
	}
	s.finalizeQuoteDetail(ctx, &detail)
	return detail, true
}

func (s *QuotesService) persistOrderBook(ctx context.Context, symbol string, book model.OrderBook) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.cacheOrderBook(ctx, sym, book, "toss")
	if s.quoteTerminal != nil {
		_ = s.quoteTerminal.UpsertOrderBook(ctx, sym, book)
	}
}

func (s *QuotesService) loadOrderBookPersisted(ctx context.Context, symbol string) (model.OrderBook, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if book, ok := s.loadOrderBookFromCache(ctx, sym, false); ok {
		return book, true
	}
	if s.quoteTerminal != nil {
		if book, ok := s.quoteTerminal.GetOrderBook(ctx, sym); ok {
			s.cacheOrderBook(ctx, sym, book, "db")
			return book, true
		}
	}
	return model.OrderBook{}, false
}

func (s *QuotesService) persistTrades(ctx context.Context, symbol string, trades []model.TradeTick) {
	if len(trades) == 0 {
		return
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.cacheTrades(ctx, sym, trades, "toss")
	if s.quoteTerminal != nil {
		_ = s.quoteTerminal.ReplaceTrades(ctx, sym, trades)
	}
}

func (s *QuotesService) loadTradesPersisted(ctx context.Context, symbol string) ([]model.TradeTick, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if trades, ok := s.loadTradesFromCache(ctx, sym, false); ok {
		return trades, true
	}
	if s.quoteTerminal != nil {
		trades, err := s.quoteTerminal.ListTrades(ctx, sym, 0)
		if err == nil && len(trades) > 0 {
			s.cacheTrades(ctx, sym, trades, "db")
			return trades, true
		}
	}
	return nil, false
}
