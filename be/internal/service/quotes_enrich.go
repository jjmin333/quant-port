package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/stocklogo"
	"github.com/enact/quant-port/be/internal/toss"
)

const (
	quoteStatsCachePrefix = "quotes:stats:v3:"
)

var usEastern *time.Location

func init() {
	usEastern, _ = time.LoadLocation("America/New_York")
	if usEastern == nil {
		usEastern = time.FixedZone("EST", -5*3600)
	}
}

type quoteStatsCache struct {
	Open         float64 `json:"open"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	PrevClose    float64 `json:"prevClose"`
	Week52High   float64 `json:"week52High"`
	Week52Low    float64 `json:"week52Low"`
	MarketCap    float64 `json:"marketCap"`
	TradingValue float64 `json:"tradingValue"`
	Volume       int64   `json:"volume"`
}

func (s *QuotesService) buildQuoteDetailFromToss(ctx context.Context, symbol string) (model.QuoteDetail, error) {
	stock, err := s.stockInfo(ctx, symbol)
	if err != nil || stock == nil {
		return model.QuoteDetail{}, fmt.Errorf("stock %s: %w", symbol, err)
	}

	currentPrice := s.resolveBestLivePrice(ctx, symbol, 0, nil, nil)
	if currentPrice <= 0 && s.toss != nil {
		price, err := s.toss.GetPrice(ctx, symbol)
		if err == nil && price != nil {
			currentPrice = parseDecimal(price.LastPrice)
		}
	}
	if currentPrice <= 0 {
		return model.QuoteDetail{}, fmt.Errorf("price %s: unavailable", symbol)
	}

	stats := s.loadQuoteStats(ctx, symbol, stock, currentPrice)

	prevClose, ok := s.resolvePrevClose(ctx, symbol)
	if !ok || prevClose <= 0 {
		return model.QuoteDetail{}, fmt.Errorf("reference price %s: unavailable", symbol)
	}

	marketCap := stats.MarketCap
	if marketCap <= 0 && stock.SharesOutstanding != "" {
		shares := parseDecimal(stock.SharesOutstanding)
		if shares > 0 && currentPrice > 0 {
			marketCap = shares * currentPrice
		}
	}

	currency := modelCurrency(stock.Currency)
	updatedAt := time.Now().UTC().Format(time.RFC3339)

	detail := model.QuoteDetail{
		QuoteSummary: model.QuoteSummary{
			Symbol:       stock.Symbol,
			Name:         stock.Name,
			Market:       stock.Market,
			Currency:     currency,
			AssetType:    assetTypeFromMarket(stock.Market, stock.SecurityType),
			CurrentPrice: currentPrice,
			Volume:       stats.Volume,
			TradingValue: stats.TradingValue,
		},
		Open:       stats.Open,
		High:       stats.High,
		Low:        stats.Low,
		PrevClose:  prevClose,
		Week52High: stats.Week52High,
		Week52Low:  stats.Week52Low,
		MarketCap:  marketCap,
		UpdatedAt:  updatedAt,
	}
	s.applyQuoteChange(ctx, symbol, currentPrice, prevClose, &detail)

	stocklogo.ApplyToQuoteSummary(&detail.QuoteSummary)
	return detail, nil
}

func (s *QuotesService) loadQuoteStats(ctx context.Context, symbol string, stock *toss.StockInfo, currentPrice float64) quoteStatsCache {
	key := quoteStatsCachePrefix + strings.ToUpper(symbol)
	if s.redis != nil {
		if raw, err := s.redis.Get(ctx, key).Bytes(); err == nil && len(raw) > 0 {
			var cached quoteStatsCache
			if json.Unmarshal(raw, &cached) == nil && cached.PrevClose > 0 {
				return cached
			}
		}
	}

	if stats, ok := s.statsFromRawCache(ctx, symbol, stock, currentPrice); ok && stats.PrevClose > 0 {
		s.cacheSetJSON(ctx, key, stats, 60*time.Second)
		return stats
	}

	if s.stocks != nil {
		if dbStock, err := s.stocks.GetBySymbol(ctx, symbol); err == nil && dbStock != nil {
			if dbStats, _, ok := s.statsFromDBCandles(ctx, symbol, dbStock); ok && dbStats.PrevClose > 0 {
				s.cacheSetJSON(ctx, key, dbStats.quoteStatsCache, time.Duration(model.MarketDataLastGoodCacheSec)*time.Second)
				return dbStats.quoteStatsCache
			}
		}
	}

	s.triggerQuoteStatsRefresh(symbol)
	return quoteStatsCache{}
}

func (s *QuotesService) fetchQuoteStats(ctx context.Context, symbol string, stock *toss.StockInfo, currentPrice float64) quoteStatsCache {
	stats := quoteStatsCache{}

	prevClose, ok := s.resolvePrevClose(ctx, symbol)
	if !ok && s.toss != nil {
		assetType := string(s.quoteAssetType(ctx, symbol))
		var cals marketCalendars
		if s.marketStore != nil {
			cals, _ = s.marketStore.EnsureCalendars(ctx)
		}
		var err error
		prevClose, err = fetchPrevClose(ctx, s.toss, symbol, assetType, cals)
		if err == nil && prevClose > 0 {
			cacheReferencePrice(ctx, s.redis, symbol, prevClose)
		}
	}
	if prevClose > 0 {
		stats.PrevClose = prevClose
	}

	open, high, low, volume, tradingValue := s.fetchIntradayStats(ctx, symbol, stock.Currency)
	stats.Open = open
	stats.High = high
	stats.Low = low
	stats.Volume = volume
	stats.TradingValue = tradingValue

	if stats.Open <= 0 && prevClose > 0 {
		stats.Open = prevClose
	}
	if stats.High <= 0 && currentPrice > 0 {
		stats.High = currentPrice
	}
	if stats.Low <= 0 && currentPrice > 0 {
		stats.Low = currentPrice
	}
	if stats.High > 0 && currentPrice > stats.High {
		stats.High = currentPrice
	}
	if stats.Low > 0 && currentPrice > 0 && currentPrice < stats.Low {
		stats.Low = currentPrice
	}

	if bars := s.ensureDailyBarsHistory(ctx, symbol, metricsDailyBarLimit); len(bars) > 0 {
		if weekHigh, weekLow, ok := week52RangeFromBars(bars); ok {
			stats.Week52High = weekHigh
			stats.Week52Low = weekLow
		}
	}

	if stock.SharesOutstanding != "" {
		shares := parseDecimal(stock.SharesOutstanding)
		if shares > 0 && currentPrice > 0 {
			stats.MarketCap = shares * currentPrice
		}
	}

	return stats
}

// fetchIntradayStats computes today's open/high/low/volume/tradingValue.
// Tries DB-stored 1m bars first — refreshRawCandles already keeps these warm
// for every tracked symbol via its own delta fetch, so this can usually
// avoid hitting Toss at all. Only symbols with no tracked history yet (cold
// start) fall through to a live GetCandles(Count:480) call.
func (s *QuotesService) fetchIntradayStats(ctx context.Context, symbol string, currency toss.Currency) (
	open, high, low float64, volume int64, tradingValue float64,
) {
	loc := kst
	if currency == toss.CurrencyUSD {
		loc = usEastern
	}
	today := startOfDay(time.Now().In(loc))

	if bars := s.listIntradayBarsFromDB(ctx, symbol, 480); len(bars) > 0 {
		if o, h, l, v, tv, ok := sumIntradayBarsFrom(bars, today); ok {
			return o, h, l, v, tv
		}
	}

	page, err := s.toss.GetCandles(ctx, toss.GetCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1m,
		Count:    480,
	})
	if err != nil || page == nil || len(page.Candles) == 0 {
		return s.fallbackDailySessionStats(ctx, symbol, today)
	}

	var first bool
	for _, c := range page.Candles {
		t := parseQuoteCandleTime(c.Timestamp)
		if t.Before(today) {
			continue
		}
		o := parseDecimal(c.OpenPrice)
		h := parseDecimal(c.HighPrice)
		l := parseDecimal(c.LowPrice)
		v := parseDecimalInt(c.Volume)
		closePx := parseDecimal(c.ClosePrice)

		if !first {
			open = o
			high = h
			low = l
			first = true
		} else {
			if h > high {
				high = h
			}
			if l < low || low == 0 {
				low = l
			}
		}
		volume += v
		tradingValue += closePx * float64(v)
	}

	if !first {
		return s.fallbackDailySessionStats(ctx, symbol, today)
	}
	return open, high, low, volume, tradingValue
}

// sumIntradayBarsFrom reduces DB-stored 1m bars to today's session stats.
// ok is false when none of the bars fall on/after `today`, signaling the
// caller should fall through to a live Toss fetch instead of reporting an
// all-zero session.
func sumIntradayBarsFrom(bars []model.Candle, today time.Time) (
	open, high, low float64, volume int64, tradingValue float64, ok bool,
) {
	for _, c := range bars {
		if parseQuoteCandleTime(c.Time).Before(today) {
			continue
		}
		if !ok {
			open, high, low = c.Open, c.High, c.Low
			ok = true
		} else {
			if c.High > high {
				high = c.High
			}
			if c.Low < low || low == 0 {
				low = c.Low
			}
		}
		volume += c.Volume
		tradingValue += c.Close * float64(c.Volume)
	}
	return
}

func (s *QuotesService) fallbackDailySessionStats(ctx context.Context, symbol string, today time.Time) (
	open, high, low float64, volume int64, tradingValue float64,
) {
	adjusted := true
	page, err := s.toss.GetCandles(ctx, toss.GetCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1d,
		Count:    2,
		Adjusted: &adjusted,
	})
	if err != nil || page == nil || len(page.Candles) == 0 {
		return 0, 0, 0, 0, 0
	}
	c := page.Candles[0]
	t := parseQuoteCandleTime(c.Timestamp)
	if startOfDay(t).Before(today.AddDate(0, 0, -1)) {
		return 0, 0, 0, 0, 0
	}
	open = parseDecimal(c.OpenPrice)
	high = parseDecimal(c.HighPrice)
	low = parseDecimal(c.LowPrice)
	volume = parseDecimalInt(c.Volume)
	closePx := parseDecimal(c.ClosePrice)
	tradingValue = closePx * float64(volume)
	return open, high, low, volume, tradingValue
}

func (s *QuotesService) statsFromRawCache(ctx context.Context, symbol string, stock *toss.StockInfo, currentPrice float64) (quoteStatsCache, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	// 291KB짜리 배열이라 짧은 기억을 거쳐 읽는다 (`quotes_raw_tail_memo.go`).
	// 아래 반복은 전부 읽기라 공유 슬라이스를 써도 안전하다.
	raw1m, ok := loadRawCandles(ctx, s.redis, fmt.Sprintf(mdRaw1mKeyFmt, sym))
	if !ok {
		return quoteStatsCache{}, false
	}

	stats := quoteStatsCache{}
	loc := kst
	if stock != nil && stock.Currency == toss.CurrencyUSD {
		loc = usEastern
	}
	today := startOfDay(time.Now().In(loc))

	var first bool
	for _, c := range raw1m {
		t := parseQuoteCandleTime(c.Time)
		if t.Before(today) {
			continue
		}
		if !first {
			stats.Open = c.Open
			stats.High = c.High
			stats.Low = c.Low
			first = true
		} else {
			if c.High > stats.High {
				stats.High = c.High
			}
			if c.Low < stats.Low || stats.Low == 0 {
				stats.Low = c.Low
			}
		}
		stats.Volume += c.Volume
		stats.TradingValue += c.Close * float64(c.Volume)
	}

	// 일봉 원본은 아래 52주 고저 계산이 그대로 쓴다 — 존재 확인만으로 대체할 수 없다.
	raw1d, hasRaw1d := loadRawCandles(ctx, s.redis, fmt.Sprintf(mdRaw1dKeyFmt, sym))
	if hasRaw1d {
		if ref, ok := s.resolvePrevClose(ctx, sym); ok {
			stats.PrevClose = ref
		}
	}

	if stats.Open <= 0 && stats.PrevClose > 0 {
		stats.Open = stats.PrevClose
	}
	if stats.High <= 0 && currentPrice > 0 {
		stats.High = currentPrice
	}
	if stats.Low <= 0 && currentPrice > 0 {
		stats.Low = currentPrice
	}
	if stats.High > 0 && currentPrice > stats.High {
		stats.High = currentPrice
	}
	if stats.Low > 0 && currentPrice > 0 && currentPrice < stats.Low {
		stats.Low = currentPrice
	}

	for _, c := range raw1d {
		if c.High > stats.Week52High {
			stats.Week52High = c.High
		}
		if stats.Week52Low == 0 || (c.Low > 0 && c.Low < stats.Week52Low) {
			stats.Week52Low = c.Low
		}
	}

	if stock != nil && stock.SharesOutstanding != "" {
		shares := parseDecimal(stock.SharesOutstanding)
		if shares > 0 && currentPrice > 0 {
			stats.MarketCap = shares * currentPrice
		}
	}

	return stats, stats.PrevClose > 0
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// prevCloseFromDailyCandles returns the prior session close from daily bars.
// Bars may be ascending (DB) or descending (Toss tail) — always sort by time first.
func prevCloseFromDailyCandles(bars []model.Candle) (float64, bool) {
	if len(bars) == 0 {
		return 0, false
	}
	sorted := append([]model.Candle(nil), bars...)
	sort.Slice(sorted, func(i, j int) bool {
		return parseQuoteCandleTime(sorted[i].Time).Before(parseQuoteCandleTime(sorted[j].Time))
	})
	if len(sorted) >= 2 {
		return sorted[len(sorted)-2].Close, true
	}
	return sorted[len(sorted)-1].Close, true
}

func (s *QuotesService) enrichQuoteSummaries(ctx context.Context, items []model.QuoteSummary) {
	if !s.TossEnabled() || len(items) == 0 {
		return
	}
	for i := range items {
		stock, err := s.toss.GetStock(ctx, items[i].Symbol)
		if err != nil || stock == nil {
			continue
		}
		stats := s.loadQuoteStats(ctx, items[i].Symbol, stock, items[i].CurrentPrice)
		prevClose := stats.PrevClose
		if ref, ok := s.resolvePrevClose(ctx, items[i].Symbol); ok {
			prevClose = ref
		}
		if items[i].CurrentPrice > 0 {
			items[i].Change, items[i].ChangeRate = computeQuoteChange(items[i].CurrentPrice, prevClose)
		}
		items[i].Volume = stats.Volume
		items[i].TradingValue = stats.TradingValue
		items[i].Name = stock.Name
		items[i].Market = stock.Market
		items[i].AssetType = assetTypeFromMarket(stock.Market, stock.SecurityType)
		items[i].Currency = modelCurrency(stock.Currency)
	}
}

func (s *QuotesService) indexChangeFromCandles(ctx context.Context, indicatorSymbol string) (change, changeRate float64, ok bool) {
	page, err := s.toss.GetMarketIndicatorCandles(ctx, toss.GetMarketIndicatorCandlesParams{
		Symbol:   indicatorSymbol,
		Interval: toss.CandleInterval1d,
		Count:    2,
	})
	if err != nil || page == nil || len(page.Candles) == 0 {
		return 0, 0, false
	}
	last := parseDecimal(page.Candles[0].ClosePrice)
	if len(page.Candles) >= 2 {
		prev := parseDecimal(page.Candles[1].ClosePrice)
		if prev > 0 {
			change = last - prev
			changeRate = (change / prev) * 100
			return change, changeRate, true
		}
	}
	return 0, 0, false
}

func roundIndexValue(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}
