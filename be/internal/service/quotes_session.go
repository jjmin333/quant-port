package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// shouldLivePollSymbol reports whether Toss live endpoints should be called for this symbol.
// Always polls while Toss is enabled — session only affects change baseline, not data refresh.
func (s *QuotesService) shouldLivePollSymbol(ctx context.Context, symbol string) bool {
	_ = ctx
	_ = symbol
	return s.TossEnabled()
}

func (s *QuotesService) loadClosedMarketQuoteDetail(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if detail, ok := s.loadQuoteDetailPersisted(ctx, sym); ok {
		return detail, true
	}
	if detail, ok := s.buildQuoteDetailFromDB(ctx, sym); ok {
		s.finalizeQuoteDetail(ctx, &detail)
		s.persistQuoteDetail(ctx, sym, detail, "db")
		return detail, true
	}
	s.backfillClosedMarketFromToss(ctx, sym)
	if detail, ok := s.loadQuoteDetailPersisted(ctx, sym); ok {
		return detail, true
	}
	if detail, ok := s.buildQuoteDetailFromDB(ctx, sym); ok {
		s.finalizeQuoteDetail(ctx, &detail)
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

// filterLiveSessionSymbols keeps only symbols whose market is currently in a
// session where trades and the order book still move.
//
// 폴러의 fast 틱이 쓴다. shouldLivePollSymbol을 바꾸지 않고 여기에 따로 둔 이유는,
// 그 함수가 요청 경로 11곳에서 "캐시를 믿어도 되는가"의 의미로 쓰이고 있어
// 전역으로 바꾸면 영향 범위가 폴러 밖으로 번지기 때문이다. 여기서 막으려는 것은
// 측정된 문제 하나 — 마감된 종목의 체결·호가를 1초마다 다시 받는 것 — 이다.
func (s *QuotesService) filterLiveSessionSymbols(ctx context.Context, symbols []string) []string {
	if len(symbols) == 0 {
		return symbols
	}
	out := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		if quoteSessionHasLiveMicrostructure(s.marketSessionFor(ctx, sym)) {
			out = append(out, sym)
		}
	}
	return out
}
