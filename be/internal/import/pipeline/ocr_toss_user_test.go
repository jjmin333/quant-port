package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestTossUserThreeScreensMerged(t *testing.T) {
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
	var ocrTexts []string
	for _, name := range fixtures {
		path := filepath.Join("..", "..", "..", "testdata", "import", name)
		if _, err := os.Stat(path); err != nil {
			t.Skip(path)
		}
		dir := filepath.Dir(path)
		work := filepath.Join(dir, "ocr_"+name)
		pre := filepath.Join(work, "pre.png")
		result := ParseHoldingsImage(engine, path, pre, work)
		all = append(all, result.Lines...)
		ocrTexts = append(ocrTexts, result.OCRTexts...)
	}
	merged := textparse.MergeParsedLines(all)
	merged = textparse.FilterLinesForLocale(merged, textparse.InferHoldingsLocale(ocrTexts, merged))
	merged = textparse.FilterParsedLines(merged)

	lgFound := false
	armQty := 0.0
	for _, l := range merged {
		if textparse.NormalizeDisplayName(l.Name) == "LG전자" {
			lgFound = true
			if l.Quantity != 7 {
				t.Fatalf("LG qty=%v want 7", l.Quantity)
			}
		}
		if l.Name == "ARM 홀딩스(ADR)" {
			armQty = l.Quantity
		}
		if strings.Contains(l.Name, "SPOL") || strings.HasPrefix(l.Name, "-") {
			t.Fatalf("garbage row: %q", l.Name)
		}
	}
	if !lgFound {
		names := make([]string, len(merged))
		for i, l := range merged {
			names[i] = fmt.Sprintf("%s(%.0f)", l.Name, l.Quantity)
		}
		t.Fatalf("LG전자 missing from merged: %v", names)
	}
	if armQty != 50 {
		t.Fatalf("ARM qty=%v want 50", armQty)
	}
}
