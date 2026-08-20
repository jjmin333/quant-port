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

func TestLiveFuzzyAllOCRNameCandidates(t *testing.T) {
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

	path := filepath.Join("..", "..", "..", "testdata", "import", "toss_domestic_7.png")
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "scan_fuzzy")
	pre := filepath.Join(work, "pre.png")
	result := ParseHoldingsImage(engine, path, pre, work)

	seen := map[string]struct{}{}
	for _, raw := range result.OCRTexts {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			candidates := []string{
				line,
				textparse.NormalizeDisplayName(line),
			}
			// also try first token / hangul chunks
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
					t.Logf("ANTEROGEN MATCH candidate=%q from line=%q", c, line)
				}
			}
		}
	}
}
