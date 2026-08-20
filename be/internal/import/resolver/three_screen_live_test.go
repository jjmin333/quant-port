package resolver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/pipeline"
	"github.com/enact/quant-port/be/internal/import/textparse"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestLiveResolveAllThreeScreensMerged(t *testing.T) {
	if os.Getenv("LIVE_MYSQL") != "1" {
		t.Skip("LIVE_MYSQL=1")
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
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
	r := New(repository.NewStockRepo(mysql.DB()))
	ctx := context.Background()

	fixtures := []string{
		"toss_domestic_7.png",
		"toss_overseas_4.png",
		"toss_mixed_overseas_domestic.png",
	}
	var parsed []textparse.ParsedLine
	var texts []string
	for _, name := range fixtures {
		path := filepath.Join("..", "..", "..", "testdata", "import", name)
		dir := filepath.Dir(path)
		work := filepath.Join(dir, "live3_"+name)
		pre := filepath.Join(work, "pre.png")
		result := pipeline.ParseHoldingsImage(engine, path, pre, work)
		parsed = append(parsed, result.Lines...)
		texts = append(texts, result.OCRTexts...)
	}
	merged := textparse.MergeParsedLines(parsed)
	merged = textparse.FilterLinesForLocale(merged, textparse.InferHoldingsLocale(texts, merged))
	merged = textparse.FilterParsedLines(merged)

	for _, line := range merged {
		display := textparse.NormalizeDisplayName(line.Name)
		if textparse.LooksLikeOCRFragment(display) {
			continue
		}
		res := r.ResolveName(ctx, display)
		t.Logf("%q -> %q (%s) qty=%.0f conf=%.2f", line.Name, res.Name, res.Symbol, line.Quantity, res.Confidence)
		if res.Symbol == "065660" {
			t.Fatalf("phantom anterogen from %q", line.Name)
		}
	}

	// Also scan every OCR token from all passes
	seen := map[string]struct{}{}
	for _, raw := range texts {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			candidates := []string{line}
			if parts := strings.Fields(line); len(parts) > 0 {
				candidates = append(candidates, parts...)
			}
			for _, c := range candidates {
				c = strings.TrimSpace(c)
				if len([]rune(c)) < 2 {
					continue
				}
				if _, ok := seen[c]; ok {
					continue
				}
				seen[c] = struct{}{}
				res := r.ResolveName(ctx, c)
				if res.Symbol == "065660" {
					t.Errorf("OCR token %q -> anterogen conf=%.2f", c, res.Confidence)
				}
			}
		}
	}
}
