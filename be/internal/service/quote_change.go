package service

import (
	"context"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// quoteChangeUsesRegularClose reports whether displayed change should be vs today's regular close.
func quoteChangeUsesRegularClose(session string) bool {
	switch session {
	case "after", "closed":
		return true
	default:
		return false
	}
}

func computeQuoteChange(currentPrice, prevClose, regularClose float64, session string) (change, changeRate float64) {
	baseline := prevClose
	if quoteChangeUsesRegularClose(session) && regularClose > 0 {
		baseline = regularClose
	}
	if baseline <= 0 || currentPrice <= 0 {
		return 0, 0
	}
	change = currentPrice - baseline
	changeRate = (change / baseline) * 100
	return change, changeRate
}

func (s *QuotesService) quoteAssetType(ctx context.Context, symbol string) model.QuoteAssetType {
	if s.stocks != nil {
		if stock, err := s.stocks.GetBySymbol(ctx, symbol); err == nil && stock != nil {
			if at := assetTypeFromMarket(stock.Market, stock.SecurityType); at == model.AssetDomestic {
				return model.AssetDomestic
			}
			if stock.Currency == model.CurrencyKRW || isDomesticKRSymbol(stock.Symbol) {
				return model.AssetDomestic
			}
			if stock.AssetType != "" {
				return stock.AssetType
			}
		}
	}
	return model.AssetDomestic
}

func (s *QuotesService) marketSessionFor(ctx context.Context, symbol string) string {
	assetType := s.quoteAssetType(ctx, symbol)
	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.EnsureCalendars(ctx)
	}
	return detectMarketSession(string(assetType), cals, time.Now())
}

func (s *QuotesService) resolveRegularClose(ctx context.Context, symbol string, assetType model.QuoteAssetType, session string) float64 {
	if !quoteChangeUsesRegularClose(session) {
		return 0
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	now := time.Now()
	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.EnsureCalendars(ctx)
	}

	if s.redis != nil {
		if rc, ok := regularCloseFromRawCandles(ctx, s.redis, sym, string(assetType), cals, now); ok {
			return rc
		}
	}
	if s.candles != nil && s.candles.HasTable(ctx) {
		if rc, ok := s.regularCloseFromDBCandles(ctx, sym, string(assetType), cals, now); ok {
			return rc
		}
	}
	if s.toss != nil && s.toss.Enabled() {
		if rc, ok := fetchRegularSessionClose(ctx, s.toss, sym, string(assetType), cals, now); ok {
			return rc
		}
	}
	return 0
}

func (s *QuotesService) regularCloseFromDBCandles(
	ctx context.Context,
	symbol string,
	assetType string,
	cals marketCalendars,
	now time.Time,
) (float64, bool) {
	regularStart, regularEnd, ok := regularSessionBounds(assetType, cals)
	if !ok || now.Before(regularEnd) {
		return 0, false
	}
	intraday, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1m, "", 480)
	if err != nil || len(intraday) == 0 {
		return 0, false
	}
	var lastClose float64
	var lastTs time.Time
	for _, c := range intraday {
		ts, tsOk := parseCandleTime(c.Time)
		if !tsOk || ts.Before(regularStart) || ts.After(regularEnd) {
			continue
		}
		if c.Close > 0 && (lastTs.IsZero() || !ts.Before(lastTs)) {
			lastClose = c.Close
			lastTs = ts
		}
	}
	return lastClose, lastClose > 0
}

func (s *QuotesService) applyQuoteChange(
	ctx context.Context,
	symbol string,
	currentPrice float64,
	prevClose float64,
	detail *model.QuoteDetail,
) {
	session := s.marketSessionFor(ctx, symbol)
	assetType := s.quoteAssetType(ctx, symbol)
	regularClose := s.resolveRegularClose(ctx, symbol, assetType, session)
	if regularClose <= 0 && detail.RegularClose > 0 {
		regularClose = detail.RegularClose
	}
	change, changeRate := computeQuoteChange(currentPrice, prevClose, regularClose, session)
	detail.MarketSession = session
	detail.RegularClose = regularClose
	detail.PrevClose = prevClose
	detail.CurrentPrice = currentPrice
	detail.Change = change
	detail.ChangeRate = changeRate
	detail.AssetType = assetType
}

// rehydrateQuoteDetail recomputes live price + reference baseline on every read (guards stale persisted fields).
func (s *QuotesService) rehydrateQuoteDetail(ctx context.Context, detail *model.QuoteDetail) {
	s.rehydrateQuoteDetailWithSources(ctx, detail, nil, nil)
}

// finalizeQuoteDetail applies live price recompute and per-request user flags after any cache hit.
func (s *QuotesService) finalizeQuoteDetail(ctx context.Context, detail *model.QuoteDetail) {
	if detail == nil || strings.TrimSpace(detail.Symbol) == "" {
		return
	}
	detail.AssetType = s.quoteAssetType(ctx, detail.Symbol)
	s.ensurePriceMicrostructure(ctx, detail.Symbol)

	var trades []model.TradeTick
	if cached, ok := s.loadTradesPersisted(ctx, detail.Symbol); ok {
		trades = cached
	}
	var book *model.OrderBook
	if cached, ok := s.loadOrderBookPersisted(ctx, detail.Symbol); ok {
		book = &cached
	}
	s.rehydrateQuoteDetailWithSources(ctx, detail, trades, book)
	s.overlayUserQuoteFlags(ctx, detail)
}

func (s *QuotesService) rehydrateQuoteDetailWithSources(
	ctx context.Context,
	detail *model.QuoteDetail,
	trades []model.TradeTick,
	book *model.OrderBook,
) {
	if detail == nil || strings.TrimSpace(detail.Symbol) == "" {
		return
	}
	detail.AssetType = s.quoteAssetType(ctx, detail.Symbol)
	ref, ok := s.resolvePrevClose(ctx, detail.Symbol)
	if !ok || ref <= 0 {
		return
	}
	fallback := detail.CurrentPrice
	price := s.resolveBestLivePrice(ctx, detail.Symbol, fallback, trades, book)
	if price <= 0 {
		return
	}
	session := s.marketSessionFor(ctx, detail.Symbol)
	if quoteSessionHasLiveMicrostructure(session) && isSamePrice(price, ref) {
		if p, ok := s.directTossPrice(ctx, detail.Symbol); ok && !isSamePrice(p, ref) {
			price = p
		} else if p, ok := lookupRankingLastPrice(ctx, s.redis, s.rankingsRepo, detail.Symbol); ok && !isSamePrice(p, ref) {
			price = p
		} else if fallback > 0 && !isSamePrice(fallback, ref) {
			price = fallback
		}
	}
	if price <= 0 {
		return
	}
	s.applyQuoteChange(ctx, detail.Symbol, price, ref, detail)
	detail.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (s *QuotesService) finalizeTerminalSnapshot(ctx context.Context, snap *model.QuoteTerminalSnapshot) {
	if snap == nil {
		return
	}
	sym := strings.TrimSpace(snap.Detail.Symbol)
	if sym != "" {
		s.ensurePriceMicrostructure(ctx, sym)
		if len(snap.Trades) == 0 {
			if trades, ok := s.loadTradesPersisted(ctx, sym); ok {
				snap.Trades = trades
			}
		}
		if len(snap.OrderBook.Asks) == 0 && len(snap.OrderBook.Bids) == 0 {
			if book, ok := s.loadOrderBookPersisted(ctx, sym); ok {
				snap.OrderBook = book
			}
		}
	}
	s.rehydrateQuoteDetailWithSources(ctx, &snap.Detail, snap.Trades, &snap.OrderBook)
	s.overlayUserQuoteFlags(ctx, &snap.Detail)
}
