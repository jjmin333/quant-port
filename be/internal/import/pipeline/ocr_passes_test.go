package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/pipeline"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestParseHoldingsImageSamsungFixture(t *testing.T) {
	path := os.Getenv("HOLDINGS_OCR_FIXTURE")
	if path == "" {
		path = filepath.Join("..", "..", "..", "testdata", "import", "domestic_holdings_samsung.png")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("fixture not found")
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract not available")
	}
	work := t.TempDir()
	lines := pipeline.ParseHoldingsImage(engine, path, path, work).Lines
	wantQty := map[string]float64{
		"삼성SDI":  4,
		"삼성전자":   34,
		"한화솔루션":  55,
		"한화시스템":  23,
		"현대차":    4,
		"LG전자":   7,
		"NAVER":  19,
	}
	if len(lines) != len(wantQty) {
		t.Fatalf("expected %d lines, got %d: %+v", len(wantQty), len(lines), lines)
	}
	for canonical, qty := range wantQty {
		if !lineMatchesWant(lines, canonical, qty) {
			t.Fatalf("missing %s %v주 in %+v", canonical, qty, lines)
		}
	}
	for _, l := range lines {
		upper := strings.ToUpper(l.Name)
		if strings.Contains(upper, "TEADS") || strings.Contains(upper, "ALDEL") || strings.Contains(upper, "HAVER") {
			t.Fatalf("phantom row: %+v", l)
		}
	}
}

func lineMatchesWant(lines []textparse.ParsedLine, canonical string, qty float64) bool {
	for _, l := range lines {
		if l.Quantity != qty {
			continue
		}
		if strings.EqualFold(l.Name, canonical) {
			return true
		}
		if strings.Contains(canonical, l.Name) || strings.Contains(l.Name, canonical) {
			return true
		}
		lr := []rune(l.Name)
		cr := []rune(canonical)
		if len(lr) >= 2 && len(cr) >= 2 && lr[0] == cr[0] && lr[1] == cr[1] {
			return true
		}
		// LG OCR variants often keep "전자" with noisy prefix digits/symbols.
		if strings.Contains(l.Name, "전자") && strings.Contains(canonical, "전자") {
			return true
		}
	}
	return false
}
