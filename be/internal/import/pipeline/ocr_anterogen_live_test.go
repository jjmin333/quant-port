package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/resolver"
	"github.com/enact/quant-port/be/internal/import/textparse"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestLiveResolveEveryValidOCRLine(t *testing.T) {
	if os.Getenv("LIVE_MYSQL") != "1" {
		t.Skip("LIVE_MYSQL=1")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mysql, err := repository.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer mysql.Close()
	r := resolver.New(repository.NewStockRepo(mysql.DB()))
	ctx := context.Background()

	fixtures := []string{
		"toss_domestic_7.png",
		"toss_overseas_4.png",
		"toss_mixed_overseas_domestic.png",
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}

	var parsed []textparse.ParsedLine
	var texts []string
	for _, name := range fixtures {
		path := filepath.Join("..", "..", "..", "testdata", "import", name)
		dir := filepath.Dir(path)
		work := filepath.Join(dir, "scan_"+name)
		pre := filepath.Join(work, "pre.png")
		result := ParseHoldingsImage(engine, path, pre, work)
		parsed = append(parsed, result.Lines...)
		texts = append(texts, result.OCRTexts...)
	}

	check := func(name string, qty float64, src string) {
		display := textparse.NormalizeDisplayName(name)
		if textparse.LooksLikeOCRFragment(display) {
			return
		}
		if !textparse.IsValidParsedLine(name, qty) {
			return
		}
		res := r.ResolveName(ctx, display)
		if res.Symbol == "065660" {
			t.Errorf("[%s] %q qty=%.0f -> %q (%s) conf=%.2f", src, name, qty, res.Name, res.Symbol, res.Confidence)
		}
	}

	for _, l := range textparse.ParseHoldingsText(strings.Join(texts, "\n")) {
		check(l.Name, l.Quantity, "raw")
	}
	merged := textparse.MergeParsedLines(parsed)
	merged = textparse.FilterLinesForLocale(merged, textparse.InferHoldingsLocale(texts, merged))
	merged = textparse.FilterParsedLines(merged)
	for _, l := range merged {
		check(l.Name, l.Quantity, "merged")
	}
}
