package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// persist* writes MySQL (source of truth) then Redis (hot cache).
// load* reads Redis first, then MySQL, warming Redis on DB hit.

func (s *QuotesService) persistQuoteDetail(ctx context.Context, symbol string, detail model.QuoteDetail, source string) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.cacheQuoteDetailLast(ctx, sym, detail)
	if s.quoteTerminal != nil {
		_ = s.quoteTerminal.UpsertDetail(ctx, sym, detail, source)
		if detail.CurrentPrice > 0 {
			_ = s.quoteTerminal.UpsertLastPrice(ctx, sym, detail.CurrentPrice, detail.Currency)
		}
	}
}

func (s *QuotesService) loadQuoteDetailPersisted(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if detail, ok := s.loadQuoteDetailLast(ctx, sym); ok {
		return detail, true
	}
	if s.quoteTerminal != nil {
		if detail, ok := s.quoteTerminal.GetDetail(ctx, sym); ok {
			s.cacheQuoteDetailLast(ctx, sym, detail)
			return detail, true
		}
	}
	return model.QuoteDetail{}, false
}

func (s *QuotesService) persistOrderBook(ctx context.Context, symbol string, book model.OrderBook) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.cacheOrderBookLast(ctx, sym, book)
	if s.quoteTerminal != nil {
		_ = s.quoteTerminal.UpsertOrderBook(ctx, sym, book)
	}
}

func (s *QuotesService) loadOrderBookPersisted(ctx context.Context, symbol string) (model.OrderBook, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if book, ok := s.loadOrderBookLast(ctx, sym); ok {
		return book, true
	}
	if s.quoteTerminal != nil {
		if book, ok := s.quoteTerminal.GetOrderBook(ctx, sym); ok {
			s.cacheOrderBookLast(ctx, sym, book)
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
	s.cacheTradesLast(ctx, sym, trades)
	if s.quoteTerminal != nil {
		_ = s.quoteTerminal.ReplaceTrades(ctx, sym, trades)
	}
}

func (s *QuotesService) loadTradesPersisted(ctx context.Context, symbol string) ([]model.TradeTick, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if trades, ok := s.loadTradesLast(ctx, sym); ok {
		return trades, true
	}
	if s.quoteTerminal != nil {
		trades, err := s.quoteTerminal.ListTrades(ctx, sym, 0)
		if err == nil && len(trades) > 0 {
			s.cacheTradesLast(ctx, sym, trades)
			return trades, true
		}
	}
	return nil, false
}
