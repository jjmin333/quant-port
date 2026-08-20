package marketdata

import "testing"

func TestNormalizeRSSNewsTitle_financialJuice(t *testing.T) {
	raw := "FinancialJuice: Japan's finance ministry: monetary authorities have wide array of tools"
	got := normalizeRSSNewsTitle(raw)
	want := "Japan's finance ministry: monetary authorities have wide array of tools"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeRSSNewsDescription_keyPoints(t *testing.T) {
	raw := "Key PointsChipotle and Starbucks saw better-than-expected same-store sales last quarter."
	got := normalizeRSSNewsDescription("", raw)
	want := "Key points: Chipotle and Starbucks saw better-than-expected same-store sales last quarter."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestIsThinNewsSummary(t *testing.T) {
	if !IsThinNewsSummary("Key points: Chipotle and Starbucks saw better-than-expected same-store sales last quarter.") {
		t.Fatal("expected short key-points line to be thin")
	}
}

func TestNormalizeRSSNewsDescription_emptyUsesTitle(t *testing.T) {
	title := "Fed's Barkin: It's a close call whether rates are high enough"
	got := normalizeRSSNewsDescription(title, "")
	if got != title {
		t.Fatalf("got %q want %q", got, title)
	}
}

func TestNormalizeRSSNewsDescription_keepsBody(t *testing.T) {
	title := "MOC Imbalance"
	body := "S&P 500: +465 mln"
	got := normalizeRSSNewsDescription(title, body)
	if got != body {
		t.Fatalf("got %q want %q", got, body)
	}
}
