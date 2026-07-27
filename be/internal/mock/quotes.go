package mock

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

var MarketIndices = []model.MarketIndex{
	{ID: "kospi", Name: "KOSPI", Value: 2687.42, Change: 18.35, ChangeRate: 0.69, Currency: model.CurrencyKRW},
	{ID: "kosdaq", Name: "KOSDAQ", Value: 842.18, Change: -4.22, ChangeRate: -0.5, Currency: model.CurrencyKRW},
	{ID: "sp500", Name: "S&P 500", Value: 5487.03, Change: 32.14, ChangeRate: 0.59, Currency: model.CurrencyUSD},
	{ID: "nasdaq", Name: "NASDAQ", Value: 17832.96, Change: 98.42, ChangeRate: 0.55, Currency: model.CurrencyUSD},
	{ID: "usd-krw", Name: "USD/KRW", Value: 1382.5, Change: 3.2, ChangeRate: 0.23, Currency: model.CurrencyKRW},
}

var quoteSummaries = []model.QuoteSummary{
	{Symbol: "005930", Name: "삼성전자", Market: "KRX", Currency: model.CurrencyKRW, AssetType: model.AssetDomestic, CurrentPrice: 62100, Change: 1400, ChangeRate: 2.31, Volume: 12450000, TradingValue: 768000000000, IsWatchlisted: true, IsHeld: true},
	{Symbol: "000660", Name: "SK하이닉스", Market: "KRX", Currency: model.CurrencyKRW, AssetType: model.AssetDomestic, CurrentPrice: 195200, Change: 3500, ChangeRate: 1.85, Volume: 3820000, TradingValue: 742000000000, IsWatchlisted: true, IsHeld: true},
	{Symbol: "035420", Name: "NAVER", Market: "KRX", Currency: model.CurrencyKRW, AssetType: model.AssetDomestic, CurrentPrice: 218500, Change: -2800, ChangeRate: -1.27, Volume: 890000, TradingValue: 194000000000, IsWatchlisted: false, IsHeld: false},
	{Symbol: "NVDA", Name: "NVIDIA", Market: "NASDAQ", Currency: model.CurrencyUSD, AssetType: model.AssetUS, CurrentPrice: 128.4, Change: 1.82, ChangeRate: 1.44, Volume: 48200000, TradingValue: 6180000000, IsWatchlisted: true, IsHeld: false},
	{Symbol: "TSLA", Name: "Tesla Inc.", Market: "NASDAQ", Currency: model.CurrencyUSD, AssetType: model.AssetUS, CurrentPrice: 261.5, Change: 7.92, ChangeRate: 3.12, Volume: 82400000, TradingValue: 21500000000, IsWatchlisted: true, IsHeld: true},
}

type quoteExtra struct {
	Open, High, Low, PrevClose, Week52High, Week52Low, MarketCap float64
	PER, PBR                                                   *float64
	UpdatedAt                                                  string
}

var quoteExtras = map[string]quoteExtra{
	"005930": {Open: 60800, High: 62400, Low: 60500, PrevClose: 60700, Week52High: 89500, Week52Low: 48200, MarketCap: 371000000000000, PER: f64(12.3), PBR: f64(1.18), UpdatedAt: "2026-07-14T18:42:15+09:00"},
	"000660": {Open: 192000, High: 196800, Low: 191500, PrevClose: 191700, Week52High: 218000, Week52Low: 118500, MarketCap: 142000000000000, PER: f64(14.8), PBR: f64(2.42), UpdatedAt: "2026-07-14T18:42:12+09:00"},
	"035420": {Open: 221000, High: 222500, Low: 217800, PrevClose: 221300, Week52High: 285000, Week52Low: 178000, MarketCap: 35800000000000, PER: f64(22.1), PBR: f64(1.05), UpdatedAt: "2026-07-14T18:41:58+09:00"},
	"NVDA":   {Open: 126.8, High: 129.2, Low: 126.1, PrevClose: 126.58, Week52High: 140.76, Week52Low: 47.32, MarketCap: 3150000000000, PER: f64(65.2), PBR: f64(48.5), UpdatedAt: "2026-07-14T05:42:08-04:00"},
	"TSLA":   {Open: 254.2, High: 263.8, Low: 252.4, PrevClose: 253.58, Week52High: 278.98, Week52Low: 138.8, MarketCap: 832000000000, PER: f64(58.4), PBR: f64(9.2), UpdatedAt: "2026-07-14T05:42:05-04:00"},
}

func f64(v float64) *float64 { return &v }

func FilterQuotes(filter string) []model.QuoteSummary {
	switch filter {
	case "domestic":
		var out []model.QuoteSummary
		for _, q := range quoteSummaries {
			if q.AssetType == model.AssetDomestic {
				out = append(out, q)
			}
		}
		return out
	case "us":
		var out []model.QuoteSummary
		for _, q := range quoteSummaries {
			if q.AssetType == model.AssetUS || q.AssetType == model.AssetETF {
				out = append(out, q)
			}
		}
		return out
	case "watchlist":
		var out []model.QuoteSummary
		for _, q := range quoteSummaries {
			if q.IsWatchlisted {
				out = append(out, q)
			}
		}
		return out
	case "held":
		var out []model.QuoteSummary
		for _, q := range quoteSummaries {
			if q.IsHeld {
				out = append(out, q)
			}
		}
		return out
	default:
		return quoteSummaries
	}
}

func GetQuoteDetail(symbol string) (model.QuoteDetail, bool) {
	for _, summary := range quoteSummaries {
		if summary.Symbol != symbol {
			continue
		}
		extra, ok := quoteExtras[symbol]
		if !ok {
			return model.QuoteDetail{}, false
		}
		return model.QuoteDetail{
			QuoteSummary: summary,
			Open:         extra.Open,
			High:         extra.High,
			Low:          extra.Low,
			PrevClose:    extra.PrevClose,
			Week52High:   extra.Week52High,
			Week52Low:    extra.Week52Low,
			MarketCap:    extra.MarketCap,
			PER:          extra.PER,
			PBR:          extra.PBR,
			UpdatedAt:    extra.UpdatedAt,
		}, true
	}
	return model.QuoteDetail{}, false
}

func GetOrderBook(symbol string) (model.OrderBook, bool) {
	detail, ok := GetQuoteDetail(symbol)
	if !ok {
		return model.OrderBook{}, false
	}

	step := 50.0
	if detail.Currency == model.CurrencyUSD {
		step = 0.05
	}

	book := model.OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	for i := 1; i <= 5; i++ {
		book.Asks = append(book.Asks, model.OrderBookLevel{
			Price:      detail.CurrentPrice + step*float64(6-i),
			Quantity:   int64(1000 + rand.Intn(5000)),
			OrderCount: 50 + rand.Intn(200),
		})
		book.Bids = append(book.Bids, model.OrderBookLevel{
			Price:      detail.CurrentPrice - step*float64(i),
			Quantity:   int64(1000 + rand.Intn(5000)),
			OrderCount: 50 + rand.Intn(200),
		})
	}

	return book, true
}

func GetRecentTrades(symbol string) []model.TradeTick {
	detail, ok := GetQuoteDetail(symbol)
	if !ok {
		return nil
	}

	step := 50.0
	if detail.Currency == model.CurrencyUSD {
		step = 0.05
	}

	sides := []string{"buy", "sell", "neutral"}
	var trades []model.TradeTick
	for i := 0; i < 8; i++ {
		price := detail.CurrentPrice
		if i%2 == 1 {
			price -= step
		}
		trades = append(trades, model.TradeTick{
			ID:       fmt.Sprintf("%d", i+1),
			Symbol:   symbol,
			Time:     time.Now().Add(-time.Duration(i) * time.Second).Format("15:04:05"),
			Price:    price,
			Quantity: int64(50 + rand.Intn(400)),
			Side:     sides[i%3],
		})
	}
	return trades
}

func GetRawCandles(symbol string, base model.CandleInterval, before string, count int) ([]model.Candle, *string) {
	detail, ok := GetQuoteDetail(symbol)
	if !ok || count <= 0 {
		return nil, nil
	}

	volatility := detail.CurrentPrice * 0.003
	if detail.Currency == model.CurrencyUSD {
		volatility = detail.CurrentPrice * 0.008
	}

	step := time.Minute
	if base == model.Interval1d {
		step = 24 * time.Hour
	}

	end := time.Now().In(time.Local)
	if base == model.Interval1d {
		end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.Local)
	} else {
		end = end.Truncate(time.Minute)
	}

	if before != "" {
		cursor := parseMockCursor(before)
		if !cursor.IsZero() {
			end = cursor.Add(-step)
			if base == model.Interval1d {
				end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.Local)
			}
		}
	}

	price := detail.Open
	if price == 0 {
		price = detail.CurrentPrice
	}

	candles := make([]model.Candle, 0, count)
	for i := count - 1; i >= 0; i-- {
		t := end.Add(-time.Duration(i) * step)
		open := price
		delta := (rand.Float64() - 0.48) * volatility
		close := open + delta
		high := max(open, close) + rand.Float64()*volatility*0.4
		low := min(open, close) - rand.Float64()*volatility*0.4

		var timeStr string
		if base == model.Interval1d {
			timeStr = t.Format("2006-01-02")
		} else {
			timeStr = t.Format(time.RFC3339)
		}

		candles = append(candles, model.Candle{
			Time:   timeStr,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: int64(100000 + rand.Intn(800000)),
		})
		price = close
	}

	if len(candles) == 0 {
		return candles, nil
	}
	next := candles[0].Time
	return candles, &next
}

func parseMockCursor(before string) time.Time {
	if before == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, before); err == nil {
		return t.In(time.Local)
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", before, time.Local); err == nil {
		return t
	}
	if t, err := time.ParseInLocation("2006-01-02", before, time.Local); err == nil {
		return t
	}
	return time.Time{}
}

// GetCandles keeps the legacy mock entrypoint used in tests.
func GetCandles(symbol string, interval model.CandleInterval) []model.Candle {
	base := model.Interval1m
	if interval == model.Interval1d || interval == model.Interval1w || interval == model.Interval1M {
		base = model.Interval1d
	}
	raw, _ := GetRawCandles(symbol, base, "", 120)
	return raw
}
