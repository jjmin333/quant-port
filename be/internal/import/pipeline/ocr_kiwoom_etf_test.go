package pipeline

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/import/costestimate"
	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
)

func TestParseHoldingsImageKiwoomETFFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "import", "domestic_etf_kiwoom.png")
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "ocr_kiwoom_test")
	_ = os.MkdirAll(work, 0o755)
	scaled := filepath.Join(work, "scaled_x2.png")
	_ = imageutil.UpscaleFile(path, scaled, 2)
	pre := filepath.Join(work, "preprocessed.png")
	if err := imageutil.PreprocessForOCR(scaled, pre); err != nil {
		pre = scaled
	}

	lines := ParseHoldingsImage(engine, path, pre, work).Lines
	want := []struct {
		issuer  string
		qty     float64
		wantAvg float64
	}{
		{"KODEX", 785, 21_475},
		{"TIGER", 100, 52_095},
	}
	if len(lines) != len(want) {
		t.Fatalf("want %d rows got %d: %+v", len(want), len(lines), lines)
	}
	for _, w := range want {
		found := false
		for _, l := range lines {
			if !strings.Contains(strings.ToUpper(l.Name), w.issuer) || l.Quantity != w.qty {
				continue
			}
			found = true
			cost := costestimate.Estimate(l)
			if cost.AvgPrice == nil {
				t.Fatalf("%s: missing avg (line=%+v)", w.issuer, l)
			}
			if math.Abs(*cost.AvgPrice-w.wantAvg) > 5 {
				t.Fatalf("%s avg=%v want~%.0f line=%+v", w.issuer, *cost.AvgPrice, w.wantAvg, l)
			}
			t.Logf("  %q / %.0f주 avg=%.0f", l.Name, l.Quantity, *cost.AvgPrice)
		}
		if !found {
			t.Fatalf("missing %s %v주 in %+v", w.issuer, w.qty, lines)
		}
	}
}
