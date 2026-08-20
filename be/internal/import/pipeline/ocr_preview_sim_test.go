package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/import/costestimate"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/resolver"
	"github.com/enact/quant-port/be/internal/import/textparse"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestFullImportPreviewSimulation(t *testing.T) {
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

	merged := textparse.MergeParsedLines(parsed)
	merged = textparse.FilterLinesForLocale(merged, textparse.InferHoldingsLocale(texts, merged))
	merged = textparse.FilterParsedLines(merged)

	type row struct {
		raw, name, sym string
		qty            float64
		conf           float64
	}
	var preview []row
	for _, line := range merged {
		display := textparse.NormalizeDisplayName(line.Name)
		if textparse.LooksLikeOCRFragment(display) {
			continue
		}
		res := r.ResolveName(ctx, display)
		cost := costestimate.Estimate(line)
		preview = append(preview, row{
			raw: line.Name, name: res.Name, sym: res.Symbol,
			qty: line.Quantity, conf: res.Confidence,
		})
		_ = cost
	}

	// dedupe by symbol like import_job
	bySym := map[string]row{}
	for _, p := range preview {
		if p.sym == "" {
			t.Logf("unresolved raw=%q qty=%.0f", p.raw, p.qty)
			continue
		}
		if existing, ok := bySym[p.sym]; ok {
			t.Logf("DUP SYM %s: existing=%+v new raw=%q qty=%.0f", p.sym, existing, p.raw, p.qty)
		}
		bySym[p.sym] = p
		t.Logf("PREVIEW %s (%s) raw=%q qty=%.0f conf=%.2f", p.name, p.sym, p.raw, p.qty, p.conf)
		if p.sym == "065660" {
			t.Errorf("phantom anterogen: raw=%q qty=%.0f", p.raw, p.qty)
		}
	}
	if len(bySym) == 0 {
		t.Fatal("empty preview")
	}
	fmt.Println("count", len(bySym))
}
