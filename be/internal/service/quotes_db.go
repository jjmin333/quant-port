package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/stocklogo"
)

func (s *QuotesService) buildQuoteDetailFromDB(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	if s.stocks == nil || s.candles == nil || !s.candles.HasTable(ctx) {
		return model.QuoteDetail{}, false
	}

	sym := strings.ToUpper(strings.TrimSpace(symbol))
	stock, err := s.stocks.GetBySymbol(ctx, sym)
	if err != nil || stock == nil {
		return model.QuoteDetail{}, false
	}

	stats, currentPrice, ok := s.statsFromDBCandles(ctx, sym, stock)
	if !ok || currentPrice <= 0 {
		return model.QuoteDetail{}, false
	}

	prevClose := stats.PrevClose
	if ref, ok := s.resolvePrevClose(ctx, sym); ok {
		prevClose = ref
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if len(stats.LastBarTime) > 0 {
		if t := parseQuoteCandleTime(stats.LastBarTime); !t.IsZero() {
			updatedAt = t.UTC().Format(time.RFC3339)
		}
	}

	detail := model.QuoteDetail{
		QuoteSummary: model.QuoteSummary{
			Symbol:       stock.Symbol,
			Name:         stock.Name,
			Market:       stock.Market,
			Currency:     stock.Currency,
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
		MarketCap:  stats.MarketCap,
		UpdatedAt:  updatedAt,
	}
	s.applyQuoteChange(ctx, sym, currentPrice, prevClose, &detail)

	watchSet := s.watchSymbolSet(ctx)
	heldSet := s.heldSymbolSet(ctx)
	detail.IsWatchlisted = watchSet[sym]
	detail.IsHeld = heldSet[sym]

	stocklogo.ApplyToQuoteSummary(&detail.QuoteSummary)
	return detail, true
}

type dbQuoteStats struct {
	quoteStatsCache
	LastBarTime string
}

func (s *QuotesService) statsFromDBCandles(ctx context.Context, symbol string, stock *model.Stock) (dbQuoteStats, float64, bool) {
	daily, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1d, "", 280)
	if err != nil || len(daily) == 0 {
		return dbQuoteStats{}, 0, false
	}

	lastDaily := daily[len(daily)-1]
	stats := dbQuoteStats{
		quoteStatsCache: quoteStatsCache{
			Open:         lastDaily.Open,
			High:         lastDaily.High,
			Low:          lastDaily.Low,
			Volume:       lastDaily.Volume,
			TradingValue: lastDaily.Close * float64(lastDaily.Volume),
		},
		LastBarTime: lastDaily.Time,
	}
	if len(daily) >= 2 {
		// PrevClose filled via resolvePrevClose in buildQuoteDetailFromDB.
	} else {
		stats.PrevClose = lastDaily.Close
	}

	currentPrice := lastDaily.Close
	for _, c := range daily {
		if c.High > stats.Week52High {
			stats.Week52High = c.High
		}
		if stats.Week52Low == 0 || (c.Low > 0 && c.Low < stats.Week52Low) {
			stats.Week52Low = c.Low
		}
	}

	intraday, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1m, "", 480)
	if err == nil && len(intraday) > 0 {
		loc := kst
		if stock.Currency == model.CurrencyUSD {
			loc = usEastern
		}
		lastBar := intraday[len(intraday)-1]
		sessionDay := startOfDay(parseQuoteCandleTime(lastBar.Time).In(loc))

		var sessionFound bool
		for _, c := range intraday {
			t := parseQuoteCandleTime(c.Time).In(loc)
			if t.Before(sessionDay) {
				continue
			}
			if !sessionFound {
				stats.Open = c.Open
				stats.High = c.High
				stats.Low = c.Low
				stats.Volume = 0
				stats.TradingValue = 0
				sessionFound = true
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
		if sessionFound {
			currentPrice = lastBar.Close
			stats.LastBarTime = lastBar.Time
		}
	}

	if stats.Open <= 0 && stats.PrevClose > 0 {
		stats.Open = stats.PrevClose
	}

	if stock.SharesOutstanding != nil && *stock.SharesOutstanding > 0 && currentPrice > 0 {
		stats.MarketCap = float64(*stock.SharesOutstanding) * currentPrice
	}

	return stats, currentPrice, currentPrice > 0
}

func (s *QuotesService) warmRawCacheFromDB(ctx context.Context, symbol string) {
	if s.candles == nil || !s.candles.HasTable(ctx) {
		return
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var raw1m []model.Candle
	if s.cacheGetJSON(ctx, fmt.Sprintf(mdRaw1mKeyFmt, sym), &raw1m) && len(raw1m) > 0 {
		// already warm
	} else if stored, _, err := s.candles.ListBefore(ctx, sym, model.Interval1m, "", tossCandlePageSize); err == nil && len(stored) > 0 {
		s.cacheSetJSON(ctx, fmt.Sprintf(mdRaw1mKeyFmt, sym), stored,
			time.Duration(model.MarketDataCandleHistCacheSec)*time.Second)
	}
	var raw1d []model.Candle
	if s.cacheGetJSON(ctx, fmt.Sprintf(mdRaw1dKeyFmt, sym), &raw1d) && len(raw1d) > 0 {
		return
	}
	if stored, _, err := s.candles.ListBefore(ctx, sym, model.Interval1d, "", tossCandlePageSize); err == nil && len(stored) > 0 {
		s.cacheSetJSON(ctx, fmt.Sprintf(mdRaw1dKeyFmt, sym), stored,
			time.Duration(model.MarketDataCandleHistCacheSec)*time.Second)
	}
}
