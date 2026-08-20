package repository

import (
	"testing"
	"time"
)

func TestNormalizeNewsPublishedAt_zeroUsesNow(t *testing.T) {
	got := NormalizeNewsPublishedAt(time.Time{})
	if got.IsZero() || got.Before(newsPublishedAtMin) {
		t.Fatalf("expected valid fallback time, got %v", got)
	}
}

func TestNormalizeNewsPublishedAt_preservesValid(t *testing.T) {
	in := time.Date(2026, 7, 25, 10, 18, 6, 0, time.FixedZone("KST", 9*3600))
	got := NormalizeNewsPublishedAt(in)
	want := in.UTC()
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNormalizeNewsPublishedAt_rejectsLegacyZeroDate(t *testing.T) {
	got := NormalizeNewsPublishedAt(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC))
	if got.Before(newsPublishedAtMin) {
		t.Fatalf("expected fallback for legacy zero date, got %v", got)
	}
}
