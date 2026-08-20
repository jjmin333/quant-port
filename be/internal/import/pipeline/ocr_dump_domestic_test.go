package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/import/ocr"
)

func TestDumpDomesticOCRLines(t *testing.T) {
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	path := filepath.Join("..", "..", "..", "testdata", "import", "toss_domestic_7.png")
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "scan_dump")
	pre := filepath.Join(work, "pre.png")
	result := ParseHoldingsImage(engine, path, pre, work)
	for i, raw := range result.OCRTexts {
		t.Logf("=== pass %d ===", i)
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			t.Logf("%q", line)
		}
	}
}
