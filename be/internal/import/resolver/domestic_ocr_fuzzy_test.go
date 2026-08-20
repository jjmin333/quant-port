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
	"github.com/enact/quant-port/be/internal/repository"
)

func TestLiveFuzzyDomesticOCRAgainstAnterogen(t *testing.T) {
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

	path := filepath.Join("..", "..", "..", "testdata", "import", "toss_domestic_7.png")
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "fuzzy_dom")
	pre := filepath.Join(work, "pre.png")
	result := pipeline.ParseHoldingsImage(engine, path, pre, work)

	seen := map[string]struct{}{}
	for _, raw := range result.OCRTexts {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			for _, c := range []string{line, strings.Fields(line)[0]} {
				if len(c) < 2 {
					continue
				}
				if _, ok := seen[c]; ok {
					continue
				}
				seen[c] = struct{}{}
				res := r.ResolveName(ctx, c)
				if res.Symbol == "065660" {
					t.Errorf("OCR candidate %q -> anterogen (conf=%.2f)", c, res.Confidence)
				}
			}
		}
	}
}
