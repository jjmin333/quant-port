package textparse

import (
	"strings"
	"testing"
)

func TestParseFinanceLineKiwoomAmounts(t *testing.T) {
	cases := []struct {
		line    string
		wantPnL float64
	}{
		{"~2,885,560 (-17.12%)", -2885560},
		{"-2,885,560 (-17.12%)", -2885560},
		{"-1,225,.870 (-23.54%)", -1225870},
		{"-1,225.870 (-23.54%)", -1225870},
		{"-2.885,560 (-17.12%)", -2885560},
	}
	for _, tc := range cases {
		f, ok := parseFinanceLine(tc.line)
		if !ok {
			t.Fatalf("%q: not parsed", tc.line)
		}
		if f.ProfitLoss == nil {
			t.Fatalf("%q: no pnl", tc.line)
		}
		if *f.ProfitLoss != tc.wantPnL {
			t.Fatalf("%q: pnl=%v want %v", tc.line, *f.ProfitLoss, tc.wantPnL)
		}
	}
	_, fin, ok := parseInlineQtyFinance("785주        -2.885,560 (-17.12%)")
	if !ok || fin.ProfitLoss == nil || *fin.ProfitLoss != -2885560 {
		t.Fatalf("inline qty+pnl: ok=%v fin=%+v", ok, fin)
	}
}

func TestParseKiwoomPSM6Block(t *testing.T) {
	text := `006×미국나스... 13,973,000원 ,
785주        -2.885,560 (-17.12%)
TIGER 미국나스...  3,983,500원 ,
100주                    -1,225.870 (-23.54%)`
	lines := ParseHoldingsText(text)
	if len(lines) != 2 {
		t.Fatalf("want 2 rows got %d: %+v", len(lines), lines)
	}
	assertKiwoomRow(t, lines, "KODEX", 785, 13_973_000, -2_885_560)
	assertKiwoomRow(t, lines, "TIGER", 100, 3_983_500, -1_225_870)
}

func TestParseKiwoomPSM11Tiger(t *testing.T) {
	text := `TIGER 미국나스..
3,983,500원
100주
-1,225,.870 (-23.54%)`
	lines := ParseHoldingsText(text)
	if len(lines) != 1 {
		t.Fatalf("got %d lines: %+v", len(lines), lines)
	}
	assertKiwoomRow(t, lines, "TIGER", 100, 3_983_500, -1_225_870)
}

func TestParseKiwoomEvalReturnOnly(t *testing.T) {
	text := strings.Join([]string{
		"KODEX 미국나스",
		"13,973,000원",
		"(17.12%)",
	}, "\n")
	lines := ParseHoldingsText(text)
	if len(lines) != 1 {
		t.Fatalf("lines=%+v", lines)
	}
	l := lines[0]
	if l.MarketValue == nil || *l.MarketValue != 13_973_000 {
		t.Fatalf("eval=%v", l.MarketValue)
	}
	if l.ReturnRate == nil {
		t.Fatal("expected return rate")
	}
}

func TestTigerDoubleDotNotFragment(t *testing.T) {
	name := "TIGER 미국나스.."
	if looksLikeOCRFragment(name) {
		t.Fatal("double trailing dot should not mark ETF name as fragment")
	}
	if !IsValidParsedLine(NormalizeDisplayName(name), 100) {
		t.Fatal("expected valid parsed line")
	}
}

func assertKiwoomRow(t *testing.T, lines []ParsedLine, issuer string, qty, eval, pnl float64) {
	t.Helper()
	for _, l := range lines {
		if !strings.Contains(strings.ToUpper(l.Name), issuer) {
			continue
		}
		if l.Quantity != qty {
			t.Fatalf("%s qty=%v want %v", issuer, l.Quantity, qty)
		}
		if l.MarketValue == nil || *l.MarketValue != eval {
			t.Fatalf("%s eval=%v want %v", issuer, l.MarketValue, eval)
		}
		if l.ProfitLoss == nil || *l.ProfitLoss != pnl {
			t.Fatalf("%s pnl=%v want %v", issuer, l.ProfitLoss, pnl)
		}
		return
	}
	t.Fatalf("missing %s in %+v", issuer, lines)
}
