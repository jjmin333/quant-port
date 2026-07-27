package marketdata

import (
	"testing"
	"time"
)

func TestParseNewsPubDate_mkRSS(t *testing.T) {
	raw := "Sat, 25 Jul 2026 10:18:06 +09:00"
	got := ParseNewsPubDate(raw)
	if got.IsZero() {
		t.Fatalf("expected parsed time, got zero")
	}
	want := time.Date(2026, 7, 25, 10, 18, 6, 0, time.FixedZone("KST", 9*3600))
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseNewsPubDate_unknownReturnsZero(t *testing.T) {
	if !ParseNewsPubDate("not-a-date").IsZero() {
		t.Fatal("expected zero time for unknown format")
	}
	if !ParseNewsPubDate("").IsZero() {
		t.Fatal("expected zero time for empty input")
	}
}
