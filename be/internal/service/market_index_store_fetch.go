package service

import (
	"context"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func (st *MarketIndexStore) fetchIndex(ctx context.Context, entry indexCatalogEntry) model.MarketIndex {
	switch entry.source {
	case indexSourceToss:
		return st.fetchTossIndex(ctx, entry)
	default:
		return st.fetchExternalIndex(ctx, entry)
	}
}

func (st *MarketIndexStore) fetchTossIndex(ctx context.Context, entry indexCatalogEntry) model.MarketIndex {
	fresh := emptyMarketIndex(entry.id, entry.name, entry.currency)
	if st.toss == nil || !st.toss.Enabled() {
		return fresh
	}

	switch entry.id {
	case "kospi", "kosdaq":
		prices, err := st.toss.GetMarketIndicatorPrices(ctx, []string{entry.symbol})
		if err != nil {
			return fresh
		}
		for _, p := range prices {
			if p.Symbol != entry.symbol {
				continue
			}
			fresh.Value = parseDecimal(p.LastPrice)
			if ch, cr, ok := st.indexChangeFromCandles(ctx, entry.symbol); ok {
				fresh.Change, fresh.ChangeRate = ch, cr
			}
			fresh.Sparkline = st.tossIndexSparkline(ctx, entry.symbol)
			break
		}
	case "usd-krw":
		if val := st.usdKrwRate(ctx); val > 0 {
			fresh.Value = val
		}
	}
	return fresh
}

func (st *MarketIndexStore) tossIndexSparkline(ctx context.Context, symbol string) []float64 {
	page, err := st.toss.GetMarketIndicatorCandles(ctx, toss.GetMarketIndicatorCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1d,
		Count:    indexSparklinePoints,
	})
	if err != nil || page == nil || len(page.Candles) == 0 {
		return nil
	}
	closes := make([]float64, 0, len(page.Candles))
	for i := len(page.Candles) - 1; i >= 0; i-- {
		close := parseDecimal(page.Candles[i].ClosePrice)
		if close > 0 {
			closes = append(closes, close)
		}
	}
	if len(closes) < 2 {
		return nil
	}
	return closes
}

func (st *MarketIndexStore) usdKrwRate(ctx context.Context) float64 {
	if st.fx != nil {
		mid, _, ok := st.fx.MidBuy(ctx)
		if ok {
			return mid
		}
	}
	return 0
}

func (st *MarketIndexStore) indexChangeFromCandles(ctx context.Context, indicatorSymbol string) (change, changeRate float64, ok bool) {
	page, err := st.toss.GetMarketIndicatorCandles(ctx, toss.GetMarketIndicatorCandlesParams{
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

func (st *MarketIndexStore) fetchExternalIndex(ctx context.Context, entry indexCatalogEntry) model.MarketIndex {
	fresh := emptyMarketIndex(entry.id, entry.name, entry.currency)
	if st.external == nil {
		return fresh
	}

	var q marketdata.Quote
	var err error
	switch entry.source {
	case indexSourceFinnhub:
		q, err = st.external.Finnhub.Quote(ctx, entry.symbol)
	case indexSourceFRED:
		q, err = st.external.FRED.QuoteWithSparkline(ctx, entry.symbol, indexSparklinePoints)
	case indexSourceUpbit:
		q, err = st.external.Upbit.Ticker(ctx, entry.symbol)
		if err == nil {
			if closes, cerr := st.external.Upbit.DailyCloses(ctx, entry.symbol, indexSparklinePoints); cerr == nil {
				q.Sparkline = closes
			}
		}
	case indexSourceGoldPrice:
		currency := "USD"
		if entry.currency == model.CurrencyKRW {
			currency = "KRW"
		}
		q, err = st.external.GoldPrice.Spot(ctx, entry.symbol, currency)
		if err == nil {
			q = enrichGoldQuote(ctx, st.redis, q)
		}
	default:
		return fresh
	}
	if err != nil {
		return fresh
	}
	return indexFromQuote(entry, q, time.Now().UTC())
}
