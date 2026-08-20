package service

import (
	"sort"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func intervalMinutes(interval model.CandleInterval) int {
	switch interval {
	case model.Interval1m:
		return 1
	case model.Interval5m:
		return 5
	case model.Interval10m:
		return 10
	case model.Interval30m:
		return 30
	case model.Interval60m:
		return 60
	default:
		return 0
	}
}

func needsCandleAggregation(interval model.CandleInterval) bool {
	switch interval {
	case model.Interval1m, model.Interval1d:
		return false
	default:
		return true
	}
}

// maxIntradayRawBars caps 1m history fetched for aggregated intraday intervals.
// 7200 ≈ 120×60m bars (~5 US regular sessions of 1m data).
const maxIntradayRawBars = 7200

// intradayDisplayBars is the default tail length returned per intraday interval.
const intradayDisplayBars = 120

func candleDisplayBarCount(interval model.CandleInterval) int {
	if minutes := intervalMinutes(interval); minutes >= 1 {
		return intradayDisplayBars
	}
	switch interval {
	case model.Interval1w:
		return tossCandlePageSize * 2
	case model.Interval1M:
		return tossCandlePageSize
	default:
		return tossCandlePageSize
	}
}

func trimCandlesTail(candles []model.Candle, maxBars int) []model.Candle {
	if maxBars <= 0 || len(candles) <= maxBars {
		return candles
	}
	return candles[len(candles)-maxBars:]
}

// needsIntradayDepthRefresh reports whether an aggregated intraday snapshot is too shallow
// (all intervals sharing the same ~200 1m tail look identical on the chart).
func needsIntradayDepthRefresh(interval model.CandleInterval, snap model.CandleSnapshot) bool {
	minutes := intervalMinutes(interval)
	if minutes <= 1 {
		return false
	}
	target := candleDisplayBarCount(interval)
	if len(snap.Completed) < target {
		return true
	}
	if len(snap.Completed) < 2 {
		return false
	}
	first := parseQuoteCandleTime(snap.Completed[0].Time)
	last := parseQuoteCandleTime(snap.Completed[len(snap.Completed)-1].Time)
	minSpan := time.Duration(minutes*(len(snap.Completed)-1)) * time.Minute
	return last.Sub(first) < minSpan/3
}

func candleFetchCount(interval model.CandleInterval) int {
	if minutes := intervalMinutes(interval); minutes > 1 {
		count := 120 * minutes
		if count > maxIntradayRawBars {
			return maxIntradayRawBars
		}
		return count
	}
	switch interval {
	case model.Interval1w:
		return tossCandlePageSize * 4
	case model.Interval1M:
		return tossCandlePageSize * 2
	default:
		return tossCandlePageSize
	}
}

func aggregateCandles(candles []model.Candle, interval model.CandleInterval) []model.Candle {
	if minutes := intervalMinutes(interval); minutes > 1 {
		return aggregateCandlesMinutes(candles, interval, minutes)
	}
	switch interval {
	case model.Interval1w:
		return aggregateCandlesPeriod(candles, periodWeek)
	case model.Interval1M:
		return aggregateCandlesPeriod(candles, periodMonth)
	default:
		return candles
	}
}

func aggregateCandlesMinutes(candles []model.Candle, interval model.CandleInterval, minutes int) []model.Candle {
	if len(candles) == 0 || minutes <= 1 {
		return candles
	}

	sorted := append([]model.Candle(nil), candles...)
	sort.Slice(sorted, func(i, j int) bool {
		return parseQuoteCandleTime(sorted[i].Time).Before(parseQuoteCandleTime(sorted[j].Time))
	})

	step := time.Duration(minutes) * time.Minute
	_ = step
	var out []model.Candle
	var bucket *model.Candle
	var bucketStart time.Time

	flush := func() {
		if bucket == nil {
			return
		}
		out = append(out, *bucket)
		bucket = nil
	}

	for _, c := range sorted {
		t := parseQuoteCandleTime(c.Time)
		start := truncateCandleTime(t, minutes)
		if bucket == nil || !start.Equal(bucketStart) {
			flush()
			bucketStart = start
			next := c
			next.Time = formatCandleTime(start, interval)
			bucket = &next
			continue
		}
		bucket.High = max(bucket.High, c.High)
		bucket.Low = min(bucket.Low, c.Low)
		bucket.Close = c.Close
		bucket.Volume += c.Volume
	}
	flush()
	return out
}

type periodKind int

const (
	periodWeek periodKind = iota
	periodMonth
)

func aggregateCandlesPeriod(candles []model.Candle, kind periodKind) []model.Candle {
	if len(candles) == 0 {
		return candles
	}

	sorted := append([]model.Candle(nil), candles...)
	sort.Slice(sorted, func(i, j int) bool {
		return parseQuoteCandleTime(sorted[i].Time).Before(parseQuoteCandleTime(sorted[j].Time))
	})

	type key struct {
		y int
		w int
		m time.Month
	}

	var out []model.Candle
	var bucket *model.Candle
	var current key
	var hasKey bool

	periodKey := func(t time.Time) key {
		if kind == periodMonth {
			return key{y: t.Year(), m: t.Month()}
		}
		y, w := t.ISOWeek()
		return key{y: y, w: w}
	}

	flush := func() {
		if bucket == nil {
			return
		}
		out = append(out, *bucket)
		bucket = nil
		hasKey = false
	}

	for _, c := range sorted {
		t := parseQuoteCandleTime(c.Time)
		k := periodKey(t)
		if !hasKey || k != current {
			flush()
			current = k
			hasKey = true
			next := c
			if kind == periodMonth {
				monthStart := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
				next.Time = monthStart.Format("2006-01-02")
			} else {
				next.Time = weekStart(t).Format("2006-01-02")
			}
			bucket = &next
			continue
		}
		bucket.High = max(bucket.High, c.High)
		bucket.Low = min(bucket.Low, c.Low)
		bucket.Close = c.Close
		bucket.Volume += c.Volume
	}
	flush()
	return out
}

func formatCandleTime(t time.Time, interval model.CandleInterval) string {
	return model.FormatCandleTime(t, interval)
}

func parseQuoteCandleTime(raw string) time.Time {
	return model.ParseCandleTime(raw)
}

func normalizeCandleBarTime(raw string, interval model.CandleInterval) string {
	return model.NormalizeBarTime(raw, interval)
}

func truncateCandleTime(t time.Time, minutes int) time.Time {
	t = t.In(model.KST)
	if minutes <= 1 {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, kst)
	}
	total := t.Hour()*60 + t.Minute()
	bucket := (total / minutes) * minutes
	return time.Date(t.Year(), t.Month(), t.Day(), bucket/60, bucket%60, 0, 0, kst)
}

func weekStart(t time.Time) time.Time {
	t = t.In(model.KST)
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, kst).AddDate(0, 0, -(weekday - 1))
}
