package pipeline

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestTossUserAllQty19Lines(t *testing.T) {
	fixtures := []string{
		"toss_domestic_7.png",
		"toss_overseas_4.png",
		"toss_mixed_overseas_domestic.png",
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	for _, name := range fixtures {
		path := filepath.Join("..", "..", "..", "testdata", "import", name)
		dir := filepath.Dir(path)
		work := filepath.Join(dir, "scan_"+name)
		pre := filepath.Join(work, "pre.png")
		result := ParseHoldingsImage(engine, path, pre, work)
		for _, l := range result.Lines {
			if l.Quantity == 19 {
				t.Logf("[%s line] %q qty=19", name, l.Name)
			}
		}
		combined := textparse.ParseHoldingsText(strings.Join(result.OCRTexts, "\n"))
		for _, l := range combined {
			if l.Quantity == 19 && l.Name != "" {
				t.Logf("[%s raw] %q valid=%v frag=%v", name, l.Name,
					textparse.IsValidParsedLine(l.Name, l.Quantity),
					textparse.LooksLikeOCRFragment(l.Name))
			}
		}
		for _, raw := range result.OCRTexts {
			for _, line := range strings.Split(raw, "\n") {
				if strings.Contains(line, "19") && (strings.Contains(line, "NAVER") || strings.Contains(line, "안트로") || strings.Contains(line, "065660")) {
					t.Logf("[%s ocr19] %q", name, line)
				}
			}
		}
	}
}
