package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

const (
	quoteStatsCachePrefix = "quotes:stats:v2:"
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

func (s *QuotesService) usdKrwRate(ctx context.Context) float64 {
	if s.fx != nil {
		mid, _, ok := s.fx.MidBuy(ctx)
		if ok {
			return mid
		}
	}
	return 0
}

func moneyToKRW(amount float64, currency model.QuoteCurrency, fx float64) float64 {
	if currency == model.CurrencyUSD && fx > 0 {
		return amount * fx
	}
	return amount
}

func (s *QuotesService) normalizeQuoteDetailKRW(d *model.QuoteDetail, fx float64) {
	_ = d
	_ = fx
}

func (s *QuotesService) normalizeQuoteSummaryKRW(q *model.QuoteSummary, fx float64) {
	_ = q
	_ = fx
}

func (s *QuotesService) normalizeOrderBookKRW(book *model.OrderBook, fx float64) {
	_ = book
	_ = fx
}

func (s *QuotesService) normalizeTradesKRW(trades []model.TradeTick, fx float64) {
	_ = trades
	_ = fx
}

func (s *QuotesService) buildQuoteDetailFromToss(ctx context.Context, symbol string) (model.QuoteDetail, error) {
	stock, err := s.toss.GetStock(ctx, symbol)
	if err != nil || stock == nil {
		return model.QuoteDetail{}, fmt.Errorf("stock %s: %w", symbol, err)
	}

	currentPrice := 0.0
	if s.marketStore != nil {
		if v, ok := s.marketStore.EnsurePrice(ctx, symbol); ok {
			currentPrice = v
		}
	} else {
		price, err := s.toss.GetPrice(ctx, symbol)
		if err != nil || price == nil {
			return model.QuoteDetail{}, fmt.Errorf("price %s: %w", symbol, err)
		}
		currentPrice = parseDecimal(price.LastPrice)
	}
	if currentPrice <= 0 {
		return model.QuoteDetail{}, fmt.Errorf("price %s: unavailable", symbol)
	}

	stats := s.loadQuoteStats(ctx, symbol, stock, currentPrice)

	prevClose := stats.PrevClose
	change := 0.0
	changeRate := 0.0
	if prevClose > 0 {
		change = currentPrice - prevClose
		changeRate = (change / prevClose) * 100
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
			Change:       change,
			ChangeRate:   changeRate,
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

	fx := s.usdKrwRate(ctx)
	s.normalizeQuoteDetailKRW(&detail, fx)
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

	prevClose, err := fetchPrevClose(ctx, s.toss, symbol)
	if err == nil && prevClose > 0 {
		stats.PrevClose = prevClose
	}

	open, high, low, volume, tradingValue := fetchIntradayStats(ctx, s.toss, symbol, stock.Currency)
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

	weekHigh, weekLow, err := fetchWeek52Range(ctx, s.toss, symbol)
	if err == nil {
		stats.Week52High = weekHigh
		stats.Week52Low = weekLow
	}

	if stock.SharesOutstanding != "" {
		shares := parseDecimal(stock.SharesOutstanding)
		if shares > 0 && currentPrice > 0 {
			stats.MarketCap = shares * currentPrice
		}
	}

	return stats
}

func fetchIntradayStats(ctx context.Context, client *toss.Client, symbol string, currency toss.Currency) (
	open, high, low float64, volume int64, tradingValue float64,
) {
	loc := kst
	if currency == toss.CurrencyUSD {
		loc = usEastern
	}
	today := startOfDay(time.Now().In(loc))

	page, err := client.GetCandles(ctx, toss.GetCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1m,
		Count:    480,
	})
	if err != nil || page == nil || len(page.Candles) == 0 {
		return fallbackDailySessionStats(ctx, client, symbol, today)
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
		return fallbackDailySessionStats(ctx, client, symbol, today)
	}
	return open, high, low, volume, tradingValue
}

func fallbackDailySessionStats(ctx context.Context, client *toss.Client, symbol string, today time.Time) (
	open, high, low float64, volume int64, tradingValue float64,
) {
	adjusted := true
	page, err := client.GetCandles(ctx, toss.GetCandlesParams{
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

func fetchWeek52Range(ctx context.Context, client *toss.Client, symbol string) (high, low float64, err error) {
	adjusted := true
	var collected []toss.Candle
	cursor := ""
	for pages := 0; pages < 3 && len(collected) < 280; pages++ {
		params := toss.GetCandlesParams{
			Symbol:   symbol,
			Interval: toss.CandleInterval1d,
			Count:    200,
			Adjusted: &adjusted,
		}
		if cursor != "" {
			params.Before = cursor
		}
		page, fetchErr := client.GetCandles(ctx, params)
		if fetchErr != nil {
			err = fetchErr
			break
		}
		if page == nil || len(page.Candles) == 0 {
			break
		}
		collected = append(collected, page.Candles...)
		if page.NextBefore == nil {
			break
		}
		cursor = *page.NextBefore
	}
	if len(collected) == 0 {
		return 0, 0, fmt.Errorf("no daily candles for %s", symbol)
	}
	for _, c := range collected {
		h := parseDecimal(c.HighPrice)
		l := parseDecimal(c.LowPrice)
		if h > high {
			high = h
		}
		if low == 0 || l < low {
			low = l
		}
	}
	return high, low, nil
}

func (s *QuotesService) statsFromRawCache(ctx context.Context, symbol string, stock *toss.StockInfo, currentPrice float64) (quoteStatsCache, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var raw1m []model.Candle
	if !s.cacheGetJSON(ctx, fmt.Sprintf(mdRaw1mKeyFmt, sym), &raw1m) || len(raw1m) == 0 {
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

	var raw1d []model.Candle
	if s.cacheGetJSON(ctx, fmt.Sprintf(mdRaw1dKeyFmt, sym), &raw1d) && len(raw1d) >= 2 {
		stats.PrevClose = raw1d[1].Close
	} else if len(raw1d) == 1 {
		stats.PrevClose = raw1d[0].Close
	}

	if stats.PrevClose <= 0 && len(raw1d) > 0 {
		stats.PrevClose = raw1d[0].Close
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

func (s *QuotesService) enrichQuoteSummaries(ctx context.Context, items []model.QuoteSummary) {
	if !s.TossEnabled() || len(items) == 0 {
		return
	}
	fxByUSD := s.usdKrwRate(ctx)

	for i := range items {
		stock, err := s.toss.GetStock(ctx, items[i].Symbol)
		if err != nil || stock == nil {
			continue
		}
		stats := s.loadQuoteStats(ctx, items[i].Symbol, stock, items[i].CurrentPrice)
		if stats.PrevClose > 0 && items[i].CurrentPrice > 0 {
			items[i].Change = items[i].CurrentPrice - stats.PrevClose
			items[i].ChangeRate = (items[i].Change / stats.PrevClose) * 100
		}
		items[i].Volume = stats.Volume
		items[i].TradingValue = stats.TradingValue
		items[i].Name = stock.Name
		items[i].Market = stock.Market
		items[i].AssetType = assetTypeFromMarket(stock.Market, stock.SecurityType)
		items[i].Currency = modelCurrency(stock.Currency)

		fx := fxByUSD
		if items[i].Currency != model.CurrencyUSD {
			fx = 0
		}
		s.normalizeQuoteSummaryKRW(&items[i], fx)
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

func (s *QuotesService) normalizeCandlesKRW(ctx context.Context, symbol string, candles []model.Candle) []model.Candle {
	_ = ctx
	_ = symbol
	return candles
}
