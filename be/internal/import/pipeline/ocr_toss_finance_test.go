package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/import/costestimate"
	"github.com/enact/quant-port/be/internal/import/ocr"
)

func TestTossTSMCXOVRFinanceDebug(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "import", "overseas_toss_tsmc_xovr.png")
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}
	engine := ocr.DefaultEngine()
	if !engine.Available() {
		t.Skip("tesseract")
	}
	dir := filepath.Dir(path)
	work := filepath.Join(dir, "ocr_toss_tsmc_test")
	pre := filepath.Join(work, "preprocessed.png")

	result := ParseHoldingsImage(engine, path, pre, work)
	for _, l := range result.Lines {
		cost := costestimate.Estimate(l)
		avg := "nil"
		if cost.AvgPrice != nil {
			avg = fmt.Sprintf("%.2f", *cost.AvgPrice)
		}
		eval, pnl, ret := "nil", "nil", "nil"
		if l.MarketValue != nil {
			eval = fmt.Sprintf("%.2f", *l.MarketValue)
		}
		if l.ProfitLoss != nil {
			pnl = fmt.Sprintf("%.2f", *l.ProfitLoss)
		}
		if l.ReturnRate != nil {
			ret = fmt.Sprintf("%.4f", *l.ReturnRate)
		}
		t.Logf("%q qty=%.0f eval=%s pnl=%s ret=%s cur=%q avg=%s method=%s",
			l.Name, l.Quantity, eval, pnl, ret, l.Currency, avg, cost.Method)
	}

	assertAvg := func(nameHint string, wantMin, wantMax float64, currency string) {
		t.Helper()
		for _, l := range result.Lines {
			if !strings.Contains(l.Name, nameHint) {
				continue
			}
			cost := costestimate.Estimate(l)
			if cost.AvgPrice == nil {
				t.Fatalf("%q: expected avg price", l.Name)
			}
			if cost.Currency != currency {
				t.Fatalf("%q: currency=%q want %q", l.Name, cost.Currency, currency)
			}
			if *cost.AvgPrice < wantMin || *cost.AvgPrice > wantMax {
				t.Fatalf("%q: avg=%.2f want %.2f–%.2f", l.Name, *cost.AvgPrice, wantMin, wantMax)
			}
			return
		}
		t.Fatalf("no row matching %q", nameHint)
	}

	assertAvg("XOVR", 19.5, 21.5, "USD")
	assertAvg("하이닉", 1_400_000, 1_700_000, "KRW")
}
