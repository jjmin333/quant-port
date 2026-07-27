package service

import (
	"context"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// shouldLivePollSymbol reports whether Toss live endpoints should be called for this symbol.
// When false (weekend, holiday, after close), reads must come from Redis/DB last snapshots only.
func (s *QuotesService) shouldLivePollSymbol(ctx context.Context, symbol string) bool {
	if !s.TossEnabled() {
		return false
	}

	assetType := model.AssetDomestic
	if s.stocks != nil {
		if stock, err := s.stocks.GetBySymbol(ctx, symbol); err == nil && stock != nil && stock.AssetType != "" {
			assetType = stock.AssetType
		}
	}

	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.EnsureCalendars(ctx)
	}

	session := detectMarketSession(string(assetType), cals, time.Now())
	switch session {
	case "regular", "after", "pre", "day":
		return true
	default:
		return false
	}
}

func (s *QuotesService) loadClosedMarketQuoteDetail(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if detail, ok := s.loadQuoteDetailPersisted(ctx, sym); ok {
		return detail, true
	}
	if detail, ok := s.buildQuoteDetailFromDB(ctx, sym); ok {
		s.persistQuoteDetail(ctx, sym, detail, "db")
		return detail, true
	}
	s.backfillClosedMarketFromToss(ctx, sym)
	if detail, ok := s.loadQuoteDetailPersisted(ctx, sym); ok {
		return detail, true
	}
	if detail, ok := s.buildQuoteDetailFromDB(ctx, sym); ok {
		s.persistQuoteDetail(ctx, sym, detail, "db")
		return detail, true
	}
	return model.QuoteDetail{}, false
}

func (s *QuotesService) loadClosedMarketTrades(ctx context.Context, symbol string) ([]model.TradeTick, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if trades, ok := s.loadTradesPersisted(ctx, sym); ok {
		return trades, true
	}
	s.backfillClosedMarketFromToss(ctx, sym)
	return s.loadTradesPersisted(ctx, sym)
}

func (s *QuotesService) loadClosedMarketOrderBook(ctx context.Context, symbol string) (model.OrderBook, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if book, ok := s.loadOrderBookPersisted(ctx, sym); ok {
		return book, true
	}
	s.backfillClosedMarketFromToss(ctx, sym)
	return s.loadOrderBookPersisted(ctx, sym)
}
