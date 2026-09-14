package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/safego"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

type QuotesService struct {
	toss           *toss.Client
	external       *marketdata.Clients
	fx             *ExchangeRateService
	watchlist      *repository.WatchlistRepo
	userHoldings   *UserHoldingsService
	stocks         *repository.StockRepo
	redis          *redis.Client
	candles        *repository.CandleBarRepo
	quoteTerminal  *repository.QuoteTerminalRepo
	rankingsRepo   *repository.MarketRankingsRepo
	marketStore    *MarketDataStore
	indexStore     *MarketIndexStore
	streamHub      *streamHub
	quoteStreamHub *quoteStreamHub
	// listPriceHub — 목록 화면(마켓 랭킹·AI 랭킹·보유·관심)이 공유하는 가격 방송.
	// 발행자는 MarketDataStore 한 곳이고, 이 서비스는 구독만 넘겨준다.
	listPriceHub *listPriceHub
	poller         *marketPoller
	pollerStarted  atomic.Bool
	quoteRefresh   *quoteRefreshCoordinator

	stockInfoBatchMu sync.RWMutex
	stockInfoBatch   map[string]stockInfoBatchEntry

	// assetTypes — 종목별 자산 구분 캐시. 목록이 종목마다 `stocks`를 다시 읽던 것을
	// 없앤다(quotes_catalog_cache.go 헤더의 실측 참조).
	assetTypes *assetTypeCache

	// dailyFetchAt — 심볼별로 Toss 일봉을 **마지막으로 받아 온** 시각.
	// refreshRawCandles가 30초마다 200봉을 통째로 다시 받던 것을 막는다
	// (dailyRefetchInterval 헤더 참조).
	// stream — 웹소켓 구독 서비스. nil이면 폴링만 돈다(기본값).
	stream *QuoteStreamService

	dailyFetchMu sync.Mutex
	dailyFetchAt map[string]time.Time
}

func (s *QuotesService) SetUserHoldings(svc *UserHoldingsService) {
	s.userHoldings = svc
}

func NewQuotesService(
	tossClient *toss.Client,
	external *marketdata.Clients,
	fx *ExchangeRateService,
	watchlist *repository.WatchlistRepo,
	stockRepo *repository.StockRepo,
	redisClient *redis.Client,
	candleRepo *repository.CandleBarRepo,
	quoteTerminalRepo *repository.QuoteTerminalRepo,
) *QuotesService {
	s := &QuotesService{
		toss:           tossClient,
		external:       external,
		fx:             fx,
		watchlist:      watchlist,
		stocks:         stockRepo,
		redis:          redisClient,
		candles:        candleRepo,
		quoteTerminal:  quoteTerminalRepo,
		streamHub:      newStreamHub(),
		quoteStreamHub: newQuoteStreamHub(),
		listPriceHub:   newListPriceHub(),
		stockInfoBatch: make(map[string]stockInfoBatchEntry),
		assetTypes:     newAssetTypeCache(),
	}
	s.poller = newMarketPoller(s)
	return s
}

// StartPolling launches the market poller. It is deliberately separate from the
// constructor: the poller reads collaborators wired by the Set* methods
// (marketStore, indexStore, userHoldings, rankingsRepo), so starting it inside
// NewQuotesService would race with that wiring.
func (s *QuotesService) StartPolling(ctx context.Context) {
	if !s.pollerStarted.CompareAndSwap(false, true) {
		return
	}
	safego.GoCtx(ctx, "market-poller", s.poller.Run)
}

func (s *QuotesService) SetIndexStore(store *MarketIndexStore) {
	s.indexStore = store
}

func (s *QuotesService) SetMarketStore(store *MarketDataStore) {
	s.marketStore = store
	if s.poller != nil {
		s.poller.quotes = s
	}
	// 가격을 쓰는 쪽과 구독하는 쪽을 같은 허브에 맞춘다. 여기서 매는 이유는
	// 스토어가 생성자보다 늦게 오기 때문이다(StartPolling 주석과 같은 사정).
	store.AttachListPriceHub(s.listPriceHub)
}

func (s *QuotesService) SetRankingsRepo(repo *repository.MarketRankingsRepo) {
	s.rankingsRepo = repo
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
	// One watchlist/holdings lookup for the whole list instead of one per item.
	ctx = withOverlayCache(ctx)

	// Per-user filters must never hit Redis (symbols, metrics, live prices).
	if filter == "watchlist" || filter == "held" {
		return s.listQuotesUncached(ctx, filter, query, limit, offset)
	}

	watchSet := s.watchSymbolSet(ctx)
	heldSet := s.heldSymbolSet(ctx)

	key := quotesListCacheKey(filter, query, limit, offset)
	var cached QuoteListResult
	if s.cacheGetJSON(ctx, key, &cached) {
		s.overlayQuoteFlags(cached.Items, watchSet, heldSet)
		return cached
	}

	out := s.listQuotesUncached(ctx, filter, query, limit, offset)
	s.overlayQuoteFlags(out.Items, watchSet, heldSet)
	s.cacheSetJSON(ctx, key, stripQuoteListForCache(out), time.Duration(model.MarketDataListCacheSec)*time.Second)
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
		s.enrichWatchlistQuotes(ctx, items)
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
			return mapOrderbook(symbol, book), true
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
		return mapTrades(symbol, trades), true
	}
	return nil, false
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
	case model.Interval1m, model.Interval5m, model.Interval10m, model.Interval30m:
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
