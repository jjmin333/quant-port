package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestTossTSMCXOVRFullTextPipeline(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "import", "overseas_toss_tsmc_xovr.png")
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "ocr_toss_tsmc_test")
	scaled := filepath.Join(work, "scaled_x2.png")
	_ = imageutil.UpscaleFile(path, scaled, 2)
	pre := filepath.Join(work, "preprocessed.png")
	_ = imageutil.PreprocessForOCR(scaled, pre)

	result := ParseHoldingsImage(engine, path, pre, work)
	merged := textparse.MergeParsedLines(result.Lines)
	locale := textparse.InferHoldingsLocale(result.OCRTexts, merged)
	t.Logf("locale=%q", locale)
	filtered := textparse.FilterLinesForLocale(merged, locale)
	filtered = textparse.FilterParsedLines(filtered)
	t.Logf("after filter (%d):", len(filtered))
	for _, l := range filtered {
		t.Logf("  %q qty=%.0f", l.Name, l.Quantity)
	}
	combined := strings.ToLower(strings.Join(func() []string {
		var names []string
		for _, l := range filtered {
			names = append(names, l.Name)
		}
		return names
	}(), "|"))
	for _, want := range []string{"tsmc", "xovr"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("missing %q after full pipeline", want)
		}
	}
}
