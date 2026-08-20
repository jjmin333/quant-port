package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestOverseasCombinedMeteRow(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "import", "overseas_holdings.png")
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip()
	}
	work := filepath.Join(filepath.Dir(path), "ocr_mete_debug")
	pre := filepath.Join(work, "pre.png")
	_ = os.MkdirAll(work, 0o755)
	_ = imageutil.UpscaleFile(path, pre, 2)

	var combined []textparse.ParsedLine
	appendPass := func(p string) {
		for _, psm := range []string{"11", "6"} {
			text, err := engine.RecognizeFilePSM(p, psm)
			if err != nil {
				continue
			}
			combined = append(combined, textparse.ParseHoldingsText(text)...)
		}
	}
	appendPass(path)
	appendPass(pre)
	left := filepath.Join(work, "left.png")
	_ = imageutil.CropRelative(path, left, 0, 0, 0.55, 1)
	left2 := filepath.Join(work, "left2.png")
	_ = imageutil.UpscaleFile(left, left2, 3)
	appendPass(left2)

	for _, l := range combined {
		if l.Quantity == 15 {
			t.Logf("qty15: %q mv=%v pnl=%v ret=%v valid=%v frag=%v",
				l.Name, l.MarketValue, l.ProfitLoss, l.ReturnRate,
				textparse.IsValidParsedLine(l.Name, l.Quantity),
				textparse.LooksLikeOCRFragment(l.Name))
		}
	}
}
