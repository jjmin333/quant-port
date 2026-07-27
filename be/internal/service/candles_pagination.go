package service

import (
	"github.com/enact/quant-port/be/internal/model"
)

// paginateAggregatedFromRaw rolls up stored base-interval bars into the target
// interval page. nextBefore uses the oldest raw bar only when more history exists.
func paginateAggregatedFromRaw(
	stored []model.Candle,
	rawNextBefore *string,
	interval model.CandleInterval,
	base model.CandleInterval,
	pageSize int,
) CandlesPage {
	if len(stored) == 0 {
		return CandlesPage{}
	}

	agg := aggregateCandles(stored, interval)
	if len(agg) == 0 {
		return CandlesPage{}
	}
	if len(agg) > pageSize {
		agg = agg[len(agg)-pageSize:]
	}

	var nextBefore *string
	if rawNextBefore != nil {
		nb := normalizeCandleBarTime(stored[0].Time, base)
		nextBefore = &nb
	}

	return CandlesPage{Candles: agg, NextBefore: nextBefore}
}
