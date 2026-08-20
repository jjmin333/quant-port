package costestimate

import (
	"math"
	"testing"

	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestReconcileNAVERFields(t *testing.T) {
	text := `NAVER

4,047,794

19주

-892,2218! (18.06%)`
	lines := textparse.FilterLinesForLocale(
		textparse.MergeMultiOCRPass(textparse.ParseHoldingsText(text)),
		"domestic",
	)
	line := lines[0]
	t.Logf("raw eval=%v pnl=%v rate=%v", line.MarketValue, line.ProfitLoss, line.ReturnRate)
	reconciled := reconcileParsedFinance(line)
	t.Logf("reconciled pnl=%v rate=%v", reconciled.ProfitLoss, reconciled.ReturnRate)
	if reconciled.MarketValue != nil && reconciled.ProfitLoss != nil {
		avg := (*reconciled.MarketValue - *reconciled.ProfitLoss) / 19
		t.Logf("implied avg=%v", avg)
	}
	res := Estimate(line)
	t.Logf("final avg=%v method=%s", res.AvgPrice, res.Method)
}

func TestReconcileParsedFinanceOCRSigns(t *testing.T) {
	eval := 1_852_010.0
	pnl := 89_990.0
	rate := 0.0463
	line := reconcileParsedFinance(textparse.ParsedLine{
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		ReturnRate:  &rate,
		Quantity:    4,
	})
	if line.ProfitLoss == nil || *line.ProfitLoss >= 0 {
		t.Fatalf("expected negative pnl, got %+v", line.ProfitLoss)
	}
	if line.ReturnRate == nil || *line.ReturnRate >= 0 {
		t.Fatalf("expected negative return, got %+v", line.ReturnRate)
	}
	res := Estimate(line)
	if res.AvgPrice == nil || math.Abs(*res.AvgPrice-485_500) > 1 {
		t.Fatalf("avg=%v want 485500", res.AvgPrice)
	}
}

func TestReconcileNAVERReturnSign(t *testing.T) {
	eval := 4_047_794.0
	pnl := -892_221.0
	rate := 0.1806
	line := reconcileParsedFinance(textparse.ParsedLine{
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		ReturnRate:  &rate,
		Quantity:    19,
	})
	if line.ReturnRate == nil || *line.ReturnRate >= 0 {
		t.Fatalf("expected negative return, got %+v", line.ReturnRate)
	}
	res := Estimate(line)
	if res.AvgPrice == nil || math.Abs(*res.AvgPrice-260_000) > 1 {
		t.Fatalf("avg=%v want 260000", *res.AvgPrice)
	}
}
