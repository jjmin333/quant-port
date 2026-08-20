package pipeline

import (
	"context"
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

func TestTossUserLivePreviewSimulation(t *testing.T) {
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

	for _, line := range merged {
		display := textparse.NormalizeDisplayName(line.Name)
		if textparse.LooksLikeOCRFragment(display) {
			continue
		}
		res := r.ResolveName(ctx, display)
		cost := costestimate.Estimate(line)
		t.Logf("%q qty=%.0f -> %q (%s) conf=%.2f avg=%v",
			line.Name, line.Quantity, res.Name, res.Symbol, res.Confidence, cost.AvgPrice)
		if res.Symbol == "065660" {
			t.Fatalf("merged line resolved to anterogen: %q", line.Name)
		}
	}

	// Direct NAVER resolve check
	res := r.ResolveName(ctx, "NAVER")
	if res.Symbol != "035420" {
		t.Fatalf("NAVER -> %+v", res)
	}
	t.Logf("NAVER direct: %+v", res)
}

func TestResolveNAVERDirect(t *testing.T) {
	if os.Getenv("LIVE_MYSQL") != "1" {
		t.Skip("LIVE_MYSQL=1")
	}
	cfg, _ := config.Load()
	mysql, _ := repository.NewMySQL(cfg.MySQLDSN)
	defer mysql.Close()
	r := resolver.New(repository.NewStockRepo(mysql.DB()))
	for _, raw := range []string{"NAVER", "안트로젠", "®  NAVER", "4,294,248"} {
		res := r.ResolveName(context.Background(), raw)
		t.Logf("%q -> sym=%q name=%q err=%q conf=%.2f", raw, res.Symbol, res.Name, res.Error, res.Confidence)
	}
}
