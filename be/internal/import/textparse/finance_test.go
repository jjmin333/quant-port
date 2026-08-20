package textparse

import "testing"

func TestParseFinanceLineDomestic(t *testing.T) {
	eval, ok := parseFinanceLine("1,544,672원")
	if !ok || eval.MarketValue == nil || *eval.MarketValue != 1_544_672 {
		t.Fatalf("eval: %+v ok=%v", eval, ok)
	}
	if eval.Currency != "KRW" {
		t.Fatalf("currency=%s", eval.Currency)
	}

	pnl, ok := parseFinanceLine("-397,328원 (20.45%)")
	if !ok || pnl.ProfitLoss == nil || *pnl.ProfitLoss > -397_000 || *pnl.ProfitLoss < -398_000 {
		t.Fatalf("pnl: %+v", pnl)
	}
	if pnl.ReturnRate == nil {
		t.Fatal("expected return rate")
	}
}

func TestParseFinanceLineUSDBareLoss(t *testing.T) {
	fin, ok := parseFinanceLine("-6234.74(4.65%")
	if !ok || fin.ProfitLoss == nil || fin.ReturnRate == nil {
		t.Fatalf("unexpected: %+v ok=%v", fin, ok)
	}
	if *fin.ProfitLoss >= 0 {
		t.Fatalf("pnl=%v", *fin.ProfitLoss)
	}
	if fin.Currency != "USD" {
		t.Fatalf("currency=%s", fin.Currency)
	}
}

func TestParseHoldingsTextAppLovinBlock(t *testing.T) {
	text := `mete!
$4,805.76
15주
-6234.74(4.65%`
	lines := ParseHoldingsText(text)
	if len(lines) != 1 {
		t.Fatalf("lines=%+v", lines)
	}
	if lines[0].MarketValue == nil || lines[0].ReturnRate == nil {
		t.Fatalf("missing finance: %+v", lines[0])
	}
}

func TestParseHoldingsTextDomesticWithFinance(t *testing.T) {
	text := `삼성SDI
4주
1,544,672원
-397,328원 (20.45%)
삼성전자
35주
7,840,644원
-4,916,856원 (38.54%)`
	lines := ParseHoldingsText(text)
	if len(lines) != 2 {
		t.Fatalf("lines=%+v", lines)
	}
	for _, l := range lines {
		if l.MarketValue == nil || l.ProfitLoss == nil {
			t.Fatalf("missing finance on %+v", l)
		}
	}
}

func TestParseHoldingsTextQtyOnlyNoFinance(t *testing.T) {
	text := `삼성전자
10주`
	lines := ParseHoldingsText(text)
	if len(lines) != 1 {
		t.Fatalf("lines=%+v", lines)
	}
	if lines[0].MarketValue != nil {
		t.Fatalf("unexpected finance: %+v", lines[0])
	}
}
