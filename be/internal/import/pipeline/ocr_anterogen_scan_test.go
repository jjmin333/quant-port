package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestTossUserAnterogenScan(t *testing.T) {
	fixtures := []string{
		"toss_domestic_7.png",
		"toss_overseas_4.png",
		"toss_mixed_overseas_domestic.png",
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	var all []textparse.ParsedLine
	var texts []string
	for _, name := range fixtures {
		path := filepath.Join("..", "..", "..", "testdata", "import", name)
		if _, err := os.Stat(path); err != nil {
			t.Skip(path)
		}
		dir := filepath.Dir(path)
		work := filepath.Join(dir, "scan_"+name)
		pre := filepath.Join(work, "pre.png")
		result := ParseHoldingsImage(engine, path, pre, work)
		all = append(all, result.Lines...)
		texts = append(texts, result.OCRTexts...)
		for _, raw := range result.OCRTexts {
			for _, line := range strings.Split(raw, "\n") {
				if strings.Contains(line, "안트로") || strings.Contains(line, "065660") ||
					strings.Contains(strings.ToUpper(line), "NAVER") || strings.Contains(line, "®") {
					t.Logf("[%s ocr] %q", name, line)
				}
			}
		}
	}
	for _, l := range textparse.ParseHoldingsText(strings.Join(texts, "\n")) {
		if l.Quantity > 0 && l.Name != "" {
			n := textparse.NormalizeDisplayName(l.Name)
			if strings.Contains(n, "안트로") || strings.Contains(n, "NAVER") || strings.Contains(n, "065660") {
				t.Logf("raw parsed %q qty=%.0f valid=%v frag=%v",
					l.Name, l.Quantity,
					textparse.IsValidParsedLine(l.Name, l.Quantity),
					textparse.LooksLikeOCRFragment(l.Name))
			}
		}
	}
	merged := textparse.MergeParsedLines(all)
	merged = textparse.FilterLinesForLocale(merged, textparse.InferHoldingsLocale(texts, merged))
	merged = textparse.FilterParsedLines(merged)
	for _, l := range merged {
		if strings.Contains(l.Name, "안트로") || strings.Contains(l.Name, "NAVER") {
			t.Logf("merged %q qty=%.0f", l.Name, l.Quantity)
		}
	}
}
