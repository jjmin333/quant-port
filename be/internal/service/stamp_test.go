package service

import (
	"strings"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestNormalizeAnnotation(t *testing.T) {
	conviction, note, tags := normalizeAnnotation(
		9, "  샀다  ", []string{" 실적 ", "실적", "", "a", "b", "c", "d"})

	// Out-of-range conviction falls back instead of rejecting the stamp — a
	// capture must never fail on a cosmetic field.
	if conviction != 3 {
		t.Fatalf("conviction = %d, want 3", conviction)
	}
	if note != "샀다" {
		t.Fatalf("note = %q, want trimmed", note)
	}
	if len(tags) != stampTagsMax {
		t.Fatalf("tags = %v, want %d after dedupe+cap", tags, stampTagsMax)
	}
	if tags[0] != "실적" {
		t.Fatalf("tags[0] = %q, want 실적", tags[0])
	}
}

// Conviction is 1..5 since v2. Clamping to the old 1..3 would file every 4 and
// 5 the form offers as the default.
func TestNormalizeAnnotationKeepsFullConvictionRange(t *testing.T) {
	for _, v := range []int{1, 2, 3, 4, 5} {
		if got, _, _ := normalizeAnnotation(v, "", nil); got != v {
			t.Errorf("conviction %d = %d, want kept", v, got)
		}
	}
	if got, _, tags := normalizeAnnotation(0, "", nil); got != 3 || tags == nil {
		t.Fatalf("conviction 0 = %d tags %v, want 3 and an empty slice", got, tags)
	}
}

func TestNormalizeAnnotationTruncatesLongNote(t *testing.T) {
	long := strings.Repeat("가", stampNoteMaxLen+50)
	_, note, _ := normalizeAnnotation(2, long, nil)
	if got := len([]rune(note)); got != stampNoteMaxLen {
		t.Fatalf("note runes = %d, want %d", got, stampNoteMaxLen)
	}
}

// A mistyped extra digit is a data-entry error, not a 900% call — it would skew
// the average-error stat forever.
func TestValidateTargetPrice(t *testing.T) {
	if err := validateTargetPrice(100, 0); err != ErrStampTargetRequired {
		t.Fatalf("target 0 = %v, want ErrStampTargetRequired", err)
	}
	if err := validateTargetPrice(100, 1100); err != ErrStampTargetOutOfRange {
		t.Fatalf("target 1100 on a 100 price = %v, want out of range", err)
	}
	if err := validateTargetPrice(100, 5); err != ErrStampTargetOutOfRange {
		t.Fatalf("target 5 on a 100 price = %v, want out of range", err)
	}
	if err := validateTargetPrice(100, 130); err != nil {
		t.Fatalf("target 130 on a 100 price = %v, want nil", err)
	}
}

func TestBenchmarkForStock(t *testing.T) {
	cases := []struct {
		assetType model.QuoteAssetType
		market    string
		want      string
	}{
		{model.AssetDomestic, "KOSPI", "kospi"},
		{model.AssetDomestic, "KOSDAQ", "kosdaq"},
		{model.AssetDomestic, "", "kospi"},
		{model.AssetUS, "NASDAQ", "sp500"},
		{model.AssetETF, "NYSE", "sp500"},
	}
	for _, tc := range cases {
		if got := benchmarkForStock(tc.assetType, tc.market); got != tc.want {
			t.Errorf("benchmarkForStock(%q,%q) = %q, want %q", tc.assetType, tc.market, got, tc.want)
		}
	}
}

// A US symbol stamped in the Korean morning belongs to the previous US trading
// day. Using the KST date there would shift every horizon by one day.
func TestStampTradeDateUsesMarketLocalDate(t *testing.T) {
	// 2026-08-10 00:30 UTC = 09:30 KST (08-10) = 20:30 ET on 08-09.
	at := time.Date(2026, 8, 10, 0, 30, 0, 0, time.UTC)

	if got := marketTradeDate(at, model.AssetDomestic); got != "2026-08-10" {
		t.Fatalf("domestic trade date = %q, want 2026-08-10", got)
	}
	if got := marketTradeDate(at, model.AssetUS); got != "2026-08-09" {
		t.Fatalf("US trade date = %q, want 2026-08-09", got)
	}
}
