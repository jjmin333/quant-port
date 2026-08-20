package pipeline

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/enact/quant-port/be/internal/import/costestimate"
	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
)

func TestOverseasFixtureFinanceDebug(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "import", "overseas_holdings.png")
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	work := filepath.Join(filepath.Dir(path), "ocr_fin_debug")
	_ = os.MkdirAll(work, 0o755)
	pre := filepath.Join(work, "pre.png")
	_ = imageutil.UpscaleFile(path, pre, 2)
	lines := ParseHoldingsImage(engine, path, pre, work).Lines
	wantAvg := map[string]float64{
		"어플라이드 머티리얼즈": 418,
		"아이렌":          37.2,
		"ARM 홀딩스(ADR)": 217.7,
		"맵러빈":          335.77,
		"오라클":          143.5,
	}
	for _, l := range lines {
		cost := costestimate.Estimate(l)
		var avg float64
		if cost.AvgPrice != nil {
			avg = *cost.AvgPrice
		}
		t.Logf("%q qty=%.0f avg=%.2f %s", l.Name, l.Quantity, avg, cost.Method)
		if want, ok := wantAvg[l.Name]; ok {
			if cost.AvgPrice == nil || math.Abs(avg-want) > 2 {
				t.Fatalf("%s avg=%v want~%.2f", l.Name, cost.AvgPrice, want)
			}
		}
	}
	if len(lines) != len(wantAvg) {
		t.Fatalf("want %d rows got %d: %+v", len(wantAvg), len(lines), lines)
	}
}
