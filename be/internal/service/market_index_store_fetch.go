package service

import (
	"context"

	"github.com/enact/quant-port/be/internal/toss"
)

// tossIndexDailyCloses — 지표 일봉 종가, **최신이 먼저**(오늘 봉의 종가 = 지금 값). 일봉은 지수·국채 모두 지원한다.
func (st *MarketIndexStore) tossIndexDailyCloses(ctx context.Context, symbol string) []float64 {
	page, err := st.toss.GetMarketIndicatorCandles(ctx, toss.GetMarketIndicatorCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1d,
		Count:    indexSparklinePoints,
	})
	if err != nil || page == nil {
		return nil
	}
	closes := make([]float64, 0, len(page.Candles))
	for _, c := range page.Candles {
		if v := parseDecimal(c.ClosePrice); v > 0 {
			closes = append(closes, v)
		}
	}
	return closes
}

// changeFromCloses — 최신 봉 대 직전 봉. 봉이 둘 미만이면 false.
func changeFromCloses(newestFirst []float64) (change, changeRate float64, ok bool) {
	if len(newestFirst) < 2 || newestFirst[1] <= 0 {
		return 0, 0, false
	}
	change = newestFirst[0] - newestFirst[1]
	return change, change / newestFirst[1] * 100, true
}

// sparklineFromCloses — 화면용(오래된 것이 먼저). 점이 둘 미만이면 그리지 않는다.
func sparklineFromCloses(newestFirst []float64) []float64 {
	if len(newestFirst) < 2 {
		return nil
	}
	out := make([]float64, len(newestFirst))
	for i, v := range newestFirst {
		out[len(newestFirst)-1-i] = v
	}
	return out
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

