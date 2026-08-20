package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestParseHoldingsImageOverseasFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "import", "overseas_holdings.png")
	if _, err := os.Stat(path); err != nil {
		t.Skip("fixture missing:", path)
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract not available")
	}
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "ocr_test_work")
	_ = os.MkdirAll(work, 0o755)
	scaled := filepath.Join(work, "scaled_x2.png")
	_ = imageutil.UpscaleFile(path, scaled, 2)
	pre := filepath.Join(work, "preprocessed.png")
	if err := imageutil.PreprocessForOCR(scaled, pre); err != nil {
		pre = scaled
	}

	lines := ParseHoldingsImage(engine, path, pre, work).Lines
	want := map[string]float64{
		"어플라이드 머티리얼즈": 4,
		"오라클":         40,
		"아이렌":          55,
		"ARM 홀딩스(ADR)": 50,
		"맵러빈":          15,
	}
	got := map[string]float64{}
	for _, l := range lines {
		got[l.Name] = l.Quantity
	}
	for name, qty := range want {
		if got[name] != qty {
			t.Errorf("want %s=%v, got map=%v, lines=%+v", name, qty, got, lines)
		}
	}
	if len(lines) != len(want) {
		t.Errorf("want %d rows, got %d: %+v", len(want), len(lines), lines)
	}
}

func TestParseHoldingsTextOverseasMergedPasses(t *testing.T) {
	// Simulates full-frame + left-column OCR from overseas_holdings.png fixture.
	full := `어플라이드 머티리얼즈
$2,093.76
+$421.76 (25.22%)
aR
아이렌
$2,168.84
55주
+6122.84(6.00%
.404홀딩스(600)
$13,392.21
50%
++$2,507.07 (23.03%)
mete!
$4,805.76
15주
-6234.74(4.65%
오라클
$5,763.69
40%
+$23.69 (0.41%)`
	left := `해외주
머플라이드 머티리얼즈
4주
마미렌
55주
ARM #3-(ADR)
50주
맵러빈
15주
오라클
40`
	combined := append(textparse.ParseHoldingsText(full), textparse.ParseHoldingsText(left)...)
	merged := textparse.CollapseSimilarRows(textparse.FilterParsedLines(textparse.FuzzyUnifyParsedLines(textparse.MergeMultiOCRPass(combined))))
	merged = textparse.FilterLinesForLocale(merged, "overseas")
	want := map[string]float64{
		"어플라이드 머티리얼즈": 4,
		"오라클":         40,
		"아이렌":          55,
		"ARM 홀딩스(ADR)": 50,
		"맵러빈":          15,
	}
	got := map[string]float64{}
	for _, l := range merged {
		got[l.Name] = l.Quantity
	}
	for name, qty := range want {
		if got[name] != qty {
			t.Fatalf("want %s=%v got %v full=%+v", name, qty, got, merged)
		}
	}
	if len(merged) != len(want) {
		t.Fatalf("want %d rows got %d: %+v", len(want), len(merged), merged)
	}
}
