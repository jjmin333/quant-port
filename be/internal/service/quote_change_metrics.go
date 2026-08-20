package service

import (
	"sort"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func changeRateFromBaseline(current, baseline float64) (float64, bool) {
	if current <= 0 || baseline <= 0 {
		return 0, false
	}
	return (current - baseline) / baseline * 100, true
}

func sortCandlesAsc(bars []model.Candle) []model.Candle {
	out := append([]model.Candle(nil), bars...)
	sort.Slice(out, func(i, j int) bool {
		return parseQuoteCandleTime(out[i].Time).Before(parseQuoteCandleTime(out[j].Time))
	})
	return out
}

func closeAtOrBefore(bars []model.Candle, cutoff time.Time, loc *time.Location) (float64, bool) {
	if len(bars) == 0 {
		return 0, false
	}
	if loc == nil {
		loc = time.UTC
	}
	cutoff = cutoff.In(loc)
	sorted := sortCandlesAsc(bars)
	var last float64
	var ok bool
	for _, bar := range sorted {
		ts := parseQuoteCandleTime(bar.Time).In(loc)
		if ts.After(cutoff) {
			break
		}
		if bar.Close > 0 {
			last = bar.Close
			ok = true
		}
	}
	return last, ok
}

func barCalendarDay(ts time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	ts = ts.In(loc)
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, loc)
}

// calendarMonthLastClose returns the last daily close belonging to a calendar month.
// For domestic symbols, include the next month's 1st 00:00 bar (KRX month-end stamp).
func calendarMonthLastClose(
	bars []model.Candle,
	year int,
	month time.Month,
	loc *time.Location,
	includeNextDayMidnight bool,
	beforeDay time.Time,
) (float64, bool) {
	closePx, _, ok := calendarMonthLastCloseBar(
		bars, year, month, loc, includeNextDayMidnight, beforeDay, 0,
	)
	return closePx, ok
}

// calendarMonthLastCloseBar returns the last daily close in a calendar month.
// When avoidClose > 0, skips the latest close if it matches avoidClose and walks back.
func calendarMonthLastCloseBar(
	bars []model.Candle,
	year int,
	month time.Month,
	loc *time.Location,
	includeNextDayMidnight bool,
	beforeDay time.Time,
	avoidClose float64,
) (float64, time.Time, bool) {
	if loc == nil {
		loc = time.UTC
	}
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	nextMonthStart := monthStart.AddDate(0, 1, 0)
	beforeDay = barCalendarDay(beforeDay, loc)

	sorted := sortCandlesAsc(bars)
	type monthClose struct {
		close float64
		day   time.Time
	}
	var inMonth []monthClose
	for _, bar := range sorted {
		if bar.Close <= 0 {
			continue
		}
		ts := parseQuoteCandleTime(bar.Time).In(loc)
		day := barCalendarDay(ts, loc)
		if !day.Before(beforeDay) {
			continue
		}
		inCalendarMonth := !day.Before(monthStart) && day.Before(nextMonthStart)
		slop := includeNextDayMidnight &&
			day.Equal(nextMonthStart) &&
			ts.Hour() == 0 && ts.Minute() == 0 && ts.Second() == 0
		if inCalendarMonth || slop {
			inMonth = append(inMonth, monthClose{close: bar.Close, day: day})
		}
	}
	if len(inMonth) == 0 {
		return 0, time.Time{}, false
	}
	for i := len(inMonth) - 1; i >= 0; i-- {
		entry := inMonth[i]
		if avoidClose > 0 && isSamePrice(entry.close, avoidClose) {
			continue
		}
		return entry.close, entry.day, true
	}
	last := inMonth[len(inMonth)-1]
	return last.close, last.day, true
}

func previousWeekClose(bars []model.Candle, now time.Time, loc *time.Location) (float64, bool) {
	if loc == nil {
		loc = time.UTC
	}
	now = now.In(loc)
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	thisMonday := time.Date(now.Year(), now.Month(), now.Day()-(weekday-1), 0, 0, 0, 0, loc)
	return closeAtOrBefore(bars, thisMonday.Add(-time.Nanosecond), loc)
}

func previousMonthClose(bars []model.Candle, now time.Time, loc *time.Location, includeKRXSlop bool, avoidClose float64) (float64, bool) {
	if loc == nil {
		loc = time.UTC
	}
	now = now.In(loc)
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	prevMonth := firstOfThisMonth.AddDate(0, -1, 0)
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	closePx, _, ok := calendarMonthLastCloseBar(
		bars,
		prevMonth.Year(),
		prevMonth.Month(),
		loc,
		includeKRXSlop,
		startOfToday,
		avoidClose,
	)
	return closePx, ok
}

func isSamePrice(a, b float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	// KRW integer quotes and USD cents should match exactly when duplicated.
	if diff < 0.005 {
		return true
	}
	return diff/a < 0.0001
}

func close52WeeksAgo(bars []model.Candle, now time.Time, loc *time.Location) (float64, bool) {
	if loc == nil {
		loc = time.UTC
	}
	cutoff := now.In(loc).AddDate(-1, 0, 0)
	return closeAtOrBefore(bars, cutoff, loc)
}

func closeOnTradingDay(bars []model.Candle, day time.Time, loc *time.Location) (float64, bool) {
	if loc == nil {
		loc = time.UTC
	}
	day = day.In(loc)
	end := time.Date(day.Year(), day.Month(), day.Day(), 23, 59, 59, 0, loc)
	return closeAtOrBefore(bars, end, loc)
}

// barsCoverYearHistory reports whether daily bars reach at least ~1 year back.
func barsCoverYearHistory(bars []model.Candle) bool {
	if len(bars) < 200 {
		return false
	}
	sorted := sortCandlesAsc(bars)
	oldest := parseQuoteCandleTime(sorted[0].Time)
	if oldest.IsZero() {
		return false
	}
	return time.Since(oldest) >= 360*24*time.Hour
}
