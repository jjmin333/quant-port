package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/import/costestimate"
	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
)

func TestParseHoldingsImageKiwoomSamsungFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "import", "domestic_kiwoom_samsung.png")
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "ocr_kiwoom_samsung_test")
	_ = os.MkdirAll(work, 0o755)
	scaled := filepath.Join(work, "scaled_x2.png")
	_ = imageutil.UpscaleFile(path, scaled, 2)
	pre := filepath.Join(work, "preprocessed.png")
	if err := imageutil.PreprocessForOCR(scaled, pre); err != nil {
		pre = scaled
	}

	lines := ParseHoldingsImage(engine, path, pre, work).Lines
	if len(lines) != 1 {
		t.Fatalf("want 1 row got %d: %+v", len(lines), lines)
	}
	l := lines[0]
	if !strings.Contains(l.Name, "삼성전자") || l.Quantity != 109 {
		t.Fatalf("unexpected row: %+v", l)
	}
	cost := costestimate.Estimate(l)
	if cost.AvgPrice == nil {
		t.Fatal("missing avg")
	}
	want := 359_260.0
	if *cost.AvgPrice != want {
		t.Fatalf("avg=%v want %v", *cost.AvgPrice, want)
	}
	t.Logf("삼성전자 109주 avg=%.0f", *cost.AvgPrice)
}
