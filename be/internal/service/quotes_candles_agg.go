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

/*
maxIntradayRawBars — 집계 인트라데이 간격을 만들기 위해 받아 두는 1분봉 상한.

값은 **가장 굵은 인트라데이 간격 × 표시 봉 수**에서 나온다. 60분봉이 있던 동안은
`120 × 60 = 7,200`이었고, 60분봉을 기능에서 뺀 지금은 30분봉이 가장 굵어
`120 × 30 = 3,600`이다.

이 상수는 화면용 창 크기로만 쓰이지 않는다 — **콜드 미스 경로의 페치 크기**이기도
하다(`intraday1mFetchTarget`이 저장된 봉이 하나도 없으면 이 값을 그대로 요청한다).
Toss `/candles`가 200봉/호출이므로 7,200은 **36호출**, 3,600은 **18호출**이다.
일봉 1호출까지 더하면 콜드 미스 1회가 37 → 19호출로 줄어든다.

그래서 이 값을 낮추는 것은 60분봉을 보던 사람만이 아니라 **모든 인트라데이 첫
열람**에 효과가 있다.
*/
const maxIntradayRawBars = 3600

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
			next.SrcBars = candleCoverage(c)
			bucket = &next
			continue
		}
		bucket.High = max(bucket.High, c.High)
		bucket.Low = min(bucket.Low, c.Low)
		bucket.Close = c.Close
		bucket.Volume += c.Volume
		// 이 버킷이 몇 개의 1분봉에서 나왔는지 이어 센다. 계층이 섞인 소스를
		// 접을 때 어느 쪽이 더 완전한지 판단하는 근거가 된다.
		bucket.SrcBars += candleCoverage(c)
	}
	flush()
	return out
}

// candleCoverage — 이 봉이 대표하는 1분봉 수. 저장 경로를 거치지 않은 봉
// (Toss에서 갓 받은 것, 테스트 픽스처)은 SrcBars가 0이라 1로 센다.
func candleCoverage(c model.Candle) int {
	if c.SrcBars <= 0 {
		return 1
	}
	return c.SrcBars
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
