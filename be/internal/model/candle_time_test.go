package model

import "testing"

func TestBeforeCursorTime_parsesMixedFormats(t *testing.T) {
	t1 := BeforeCursorTime("2026-03-25T09:04:00+09:00")
	t2 := BeforeCursorTime("2026-03-25 09:05:00")
	if t1.IsZero() || t2.IsZero() {
		t.Fatal("expected parsed times")
	}
	if !t1.Before(t2) {
		t.Fatalf("%v should be before %v", t1, t2)
	}
}

func TestBeforeCursorRFC3339_mixedFormats(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2026-03-25T09:04:00+09:00", "2026-03-25T09:04:00+09:00"},
		{"2026-03-25 09:05:00", "2026-03-25T09:05:00+09:00"},
		{"2026-03-25", "2026-03-25T00:00:00+09:00"},
	}
	for _, tc := range tests {
		got := BeforeCursorRFC3339(tc.in)
		if got != tc.want {
			t.Fatalf("BeforeCursorRFC3339(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeBarTime_intradayCanonical(t *testing.T) {
	got := NormalizeBarTime("2026-03-25 09:05:00", Interval5m)
	want := "2026-03-25T09:05:00+09:00"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCandleTime_sortable(t *testing.T) {
	a := FormatCandleTime(ParseCandleTime("2026-03-25T09:04:00+09:00"), Interval5m)
	b := FormatCandleTime(ParseCandleTime("2026-03-25T09:05:00+09:00"), Interval5m)
	if a >= b {
		t.Fatalf("expected lexicographic order a < b, got %q vs %q", a, b)
	}
}
