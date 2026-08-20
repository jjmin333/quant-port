package model

import (
	"strings"
	"time"
)

var KST = time.FixedZone("KST", 9*3600)

// ParseCandleTime parses bar timestamps from Toss, DB, or aggregated buckets.
func ParseCandleTime(raw string) time.Time {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.In(KST)
	}
	layouts := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"15:04:05",
		"15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, KST); err == nil {
			if layout == "15:04" || layout == "15:04:05" {
				now := time.Now().In(KST)
				return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, KST)
			}
			return t
		}
	}
	return time.Time{}
}

// IsDailyOrCoarser reports whether an interval's bars are date-keyed rather
// than timestamped. Those are the ones a SQL DATE(bar_time) is taken of, so
// their stored wall clock must be KST midnight regardless of process timezone.
func IsDailyOrCoarser(interval CandleInterval) bool {
	switch interval {
	case Interval1d, Interval1w, Interval1M:
		return true
	}
	return false
}

// FormatCandleTime returns a canonical stored/display time for the interval.
func FormatCandleTime(t time.Time, interval CandleInterval) string {
	t = t.In(KST)
	switch interval {
	case Interval1d, Interval1w, Interval1M:
		return t.Format("2006-01-02")
	default:
		return t.Format("2006-01-02T15:04:05+09:00")
	}
}

// NormalizeBarTime rewrites legacy formats to canonical bar_time strings.
func NormalizeBarTime(raw string, interval CandleInterval) string {
	t := ParseCandleTime(raw)
	if t.IsZero() {
		return raw
	}
	return FormatCandleTime(t, interval)
}

// BeforeCursorTime parses a pagination cursor for SQL comparison.
func BeforeCursorTime(raw string) time.Time {
	return ParseCandleTime(raw)
}

// BeforeCursorRFC3339 parses a pagination cursor for legacy VARCHAR comparisons.
func BeforeCursorRFC3339(raw string) string {
	t := BeforeCursorTime(raw)
	if t.IsZero() {
		return raw
	}
	return t.In(KST).Format("2006-01-02T15:04:05+09:00")
}

// CanonicalPaginationCursor returns RFC3339 KST cursor for API/Toss pagination.
// Daily/week/month buckets normalize to midnight KST; intraday keeps minute resolution.
func CanonicalPaginationCursor(raw string, base CandleInterval) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	t := BeforeCursorTime(raw)
	if t.IsZero() {
		return BeforeCursorRFC3339(raw)
	}
	switch base {
	case Interval1d, Interval1w, Interval1M:
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, KST)
	default:
		t = t.In(KST)
	}
	return t.Format("2006-01-02T15:04:05+09:00")
}

// CanonicalPaginationCursorPtr normalizes a nullable pagination cursor.
func CanonicalPaginationCursorPtr(raw *string, base CandleInterval) *string {
	if raw == nil {
		return nil
	}
	c := CanonicalPaginationCursor(*raw, base)
	if c == "" {
		return nil
	}
	return &c
}
