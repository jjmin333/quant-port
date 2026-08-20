package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestParseHoldingsImageRejectsNonBrokerageChatFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "import", "non_brokerage_chat.png")
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "ocr_non_broker_test")
	_ = os.MkdirAll(work, 0o755)
	scaled := filepath.Join(work, "scaled_x2.png")
	_ = imageutil.UpscaleFile(path, scaled, 2)
	pre := filepath.Join(work, "preprocessed.png")
	if err := imageutil.PreprocessForOCR(scaled, pre); err != nil {
		pre = scaled
	}

	result := ParseHoldingsImage(engine, path, pre, work)
	t.Logf("OCR texts (%d passes):", len(result.OCRTexts))
	for i, txt := range result.OCRTexts {
		t.Logf("--- pass %d ---\n%s", i, txt)
	}
	t.Logf("parsed lines (%d):", len(result.Lines))
	for _, l := range result.Lines {
		t.Logf("  %+v", l)
	}
	if msg := textparse.RejectNonBrokerageScreenshot(result.OCRTexts, result.Lines); msg == "" {
		t.Fatalf("expected reject for chat/non-brokerage screenshot, got accept")
	}
}
