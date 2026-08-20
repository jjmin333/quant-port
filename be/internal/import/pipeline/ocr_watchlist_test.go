package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
)

func TestParseHoldingsImageOverseasWatchlistFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "import", "overseas_watchlist_quotes.png")
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "ocr_watchlist_test")
	_ = os.MkdirAll(work, 0o755)
	scaled := filepath.Join(work, "scaled_x2.png")
	_ = imageutil.UpscaleFile(path, scaled, 2)
	pre := filepath.Join(work, "preprocessed.png")
	if err := imageutil.PreprocessForOCR(scaled, pre); err != nil {
		pre = scaled
	}

	lines := ParseHoldingsImage(engine, path, pre, work).Lines
	if len(lines) < 4 {
		t.Fatalf("want >=4 quote names got %d: %+v", len(lines), lines)
	}
	for _, l := range lines {
		if l.Quantity != 0 {
			t.Fatalf("watchlist row should have empty qty: %+v", l)
		}
		if l.MarketValue != nil || l.ProfitLoss != nil {
			t.Fatalf("watchlist row should have no finance: %+v", l)
		}
		t.Logf("  %q", l.Name)
	}
}
