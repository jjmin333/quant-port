package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

type QuotesService struct {
	toss        *toss.Client
	external    *marketdata.Clients
	fx          *ExchangeRateService
	watchlist   *repository.WatchlistRepo
	portfolio   *PortfolioService
	stocks      *repository.StockRepo
	redis       *redis.Client
	candles       *repository.CandleBarRepo
	quoteTerminal *repository.QuoteTerminalRepo
	marketStore   *MarketDataStore
	indexStore  *MarketIndexStore
	streamHub   *streamHub
	poller      *marketPoller
	quoteRefresh *quoteRefreshCoordinator
}

func NewQuotesService(
	tossClient *toss.Client,
	external *marketdata.Clients,
	fx *ExchangeRateService,
	watchlist *repository.WatchlistRepo,
	portfolio *PortfolioService,
	stockRepo *repository.StockRepo,
	redisClient *redis.Client,
	candleRepo *repository.CandleBarRepo,
	quoteTerminalRepo *repository.QuoteTerminalRepo,
) *QuotesService {
	s := &QuotesService{
		toss:          tossClient,
		external:      external,
		fx:            fx,
		watchlist:     watchlist,
		portfolio:     portfolio,
		stocks:        stockRepo,
		redis:         redisClient,
		candles:       candleRepo,
		quoteTerminal: quoteTerminalRepo,
		streamHub:     newStreamHub(),
	}
	s.poller = newMarketPoller(s)
	go s.poller.Run(context.Background())
	return s
}

func (s *QuotesService) SetIndexStore(store *MarketIndexStore) {
	s.indexStore = store
}

func (s *QuotesService) SetMarketStore(store *MarketDataStore) {
	s.marketStore = store
	if s.poller != nil {
		s.poller.quotes = s
	}
}

func (s *QuotesService) TossEnabled() bool {
	return s.toss != nil && s.toss.Enabled()
}

func emptyMarketIndex(id, name string, currency model.QuoteCurrency) model.MarketIndex {
	return model.MarketIndex{
		ID: id, Name: name, Value: 0, Change: 0, ChangeRate: 0, Currency: currency,
	}
}

type QuoteListResult struct {
	Items []model.QuoteSummary `json:"items"`
	Total int                  `json:"total"`
}

func (s *QuotesService) ListQuotes(ctx context.Context, filter, query string, limit, offset int) QuoteListResult {
	key := quotesListCacheKey(filter, query, limit, offset)
	var cached QuoteListResult
	if s.cacheGetJSON(ctx, key, &cached) {
		return cached
	}

	out := s.listQuotesUncached(ctx, filter, query, limit, offset)
	s.cacheSetJSON(ctx, key, out, time.Duration(model.MarketDataListCacheSec)*time.Second)
	return out
}

func (s *QuotesService) listQuotesUncached(ctx context.Context, filter, query string, limit, offset int) QuoteListResult {
	watchSet := s.watchSymbolSet(ctx)
	heldSet := s.heldSymbolSet(ctx)

	switch filter {
	case "watchlist":
		base := s.quotesForSymbols(ctx, s.watchlistSymbols(ctx), watchSet, heldSet)
		filtered := filterQuotesByQuery(base, query)
		items := s.applyLivePrices(ctx, filtered, watchSet, heldSet)
		return QuoteListResult{Items: items, Total: len(items)}
	case "held":
		base := filterQuotesByHeld(ctx, s, nil, heldSet)
		filtered := filterQuotesByQuery(base, query)
		items := s.applyLivePrices(ctx, filtered, watchSet, heldSet)
		return QuoteListResult{Items: items, Total: len(items)}
	default:
		items, total := s.listQuotesFromCatalog(ctx, filter, query, limit, offset, watchSet, heldSet)
		return QuoteListResult{Items: items, Total: total}
	}
}

func filterQuotesByQuery(items []model.QuoteSummary, query string) []model.QuoteSummary {
	if len(items) == 0 {
		return []model.QuoteSummary{}
	}
	if strings.TrimSpace(query) == "" {
		return items
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var out []model.QuoteSummary
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Symbol), q) ||
			strings.Contains(strings.ToLower(item.Name), q) {
			out = append(out, item)
		}
	}
	return out
}

func (s *QuotesService) GetQuote(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	return s.EnsureQuoteDetail(ctx, symbol)
}

func (s *QuotesService) getQuoteUncached(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	if !s.TossEnabled() {
		return model.QuoteDetail{}, false
	}

	detail, err := s.buildQuoteDetailFromToss(ctx, symbol)
	if err != nil {
		return model.QuoteDetail{}, false
	}

	watchSet := s.watchSymbolSet(ctx)
	heldSet := s.heldSymbolSet(ctx)
	sym := strings.ToUpper(detail.Symbol)
	detail.IsWatchlisted = watchSet[sym]
	detail.IsHeld = heldSet[sym]
	return detail, true
}

func (s *QuotesService) GetOrderBook(ctx context.Context, symbol string) (model.OrderBook, bool) {
	return s.EnsureOrderBook(ctx, symbol)
}

func (s *QuotesService) getOrderBookUncached(ctx context.Context, symbol string) (model.OrderBook, bool) {
	if s.TossEnabled() {
		book, err := s.toss.GetOrderbook(ctx, symbol)
		if err == nil && book != nil {
			out := mapOrderbook(symbol, book)
			if fx := s.fxForSymbol(ctx, symbol); fx > 0 {
				s.normalizeOrderBookKRW(&out, fx)
			}
			return out, true
		}
		return model.OrderBook{}, false
	}
	return model.OrderBook{}, false
}

func (s *QuotesService) GetTrades(ctx context.Context, symbol string) ([]model.TradeTick, bool) {
	return s.EnsureTrades(ctx, symbol)
}

func (s *QuotesService) getTradesUncached(ctx context.Context, symbol string) ([]model.TradeTick, bool) {
	if !s.TossEnabled() {
		return nil, false
	}
	trades, err := s.toss.GetTrades(ctx, symbol, 50)
	if err == nil && len(trades) > 0 {
		out := mapTrades(symbol, trades)
		if fx := s.fxForSymbol(ctx, symbol); fx > 0 {
			s.normalizeTradesKRW(out, fx)
		}
		return out, true
	}
	return nil, false
}

func (s *QuotesService) fxForSymbol(ctx context.Context, symbol string) float64 {
	if !s.TossEnabled() {
		return 0
	}
	stock, err := s.toss.GetStock(ctx, symbol)
	if err != nil || stock == nil || stock.Currency != toss.CurrencyUSD {
		return 0
	}
	return s.usdKrwRate(ctx)
}

func (s *QuotesService) GetCandles(ctx context.Context, symbol string, interval model.CandleInterval) ([]model.Candle, bool) {
	page, ok := s.GetCandlesPage(ctx, symbol, interval, "")
	if !ok {
		return nil, false
	}
	return page.Candles, true
}

func modelCurrency(c toss.Currency) model.QuoteCurrency {
	if c == toss.CurrencyUSD {
		return model.CurrencyUSD
	}
	return model.CurrencyKRW
}

func mapCandleInterval(interval model.CandleInterval) (toss.CandleInterval, bool) {
	switch interval {
	case model.Interval1m, model.Interval5m, model.Interval10m, model.Interval30m, model.Interval60m:
		return toss.CandleInterval1m, true
	case model.Interval1d, model.Interval1w, model.Interval1M:
		return toss.CandleInterval1d, true
	default:
		return "", false
	}
}

func mapOrderbook(symbol string, book *toss.OrderbookResponse) model.OrderBook {
	out := model.OrderBook{Symbol: symbol}
	if book.Timestamp != nil {
		out.Timestamp = *book.Timestamp
	}
	for _, ask := range book.Asks {
		out.Asks = append(out.Asks, model.OrderBookLevel{
			Price:    parseDecimal(ask.Price),
			Quantity: parseDecimalInt(ask.Volume),
		})
	}
	for _, bid := range book.Bids {
		out.Bids = append(out.Bids, model.OrderBookLevel{
			Price:    parseDecimal(bid.Price),
			Quantity: parseDecimalInt(bid.Volume),
		})
	}
	return out
}

func mapTrades(symbol string, trades []toss.TradeResponse) []model.TradeTick {
	out := make([]model.TradeTick, len(trades))
	for i, t := range trades {
		out[i] = model.TradeTick{
			ID:       fmt.Sprintf("%s-%s-%d-%d", t.Timestamp, t.Price, parseDecimalInt(t.Volume), i),
			Symbol:   symbol,
			Time:     t.Timestamp,
			Price:    parseDecimal(t.Price),
			Quantity: parseDecimalInt(t.Volume),
			Side:     "unknown",
		}
	}
	return out
}

func mapCandles(candles []toss.Candle, base model.CandleInterval) []model.Candle {
	out := make([]model.Candle, len(candles))
	for i, c := range candles {
		out[i] = model.Candle{
			Time:   model.NormalizeBarTime(c.Timestamp, base),
			Open:   parseDecimal(c.OpenPrice),
			High:   parseDecimal(c.HighPrice),
			Low:    parseDecimal(c.LowPrice),
			Close:  parseDecimal(c.ClosePrice),
			Volume: parseDecimalInt(c.Volume),
		}
	}
	return out
}
