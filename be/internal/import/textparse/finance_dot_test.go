package textparse

import "testing"

func TestParseFinanceLineDotThousands(t *testing.T) {
	cases := []struct {
		line   string
		want   float64
		isPnL  bool
		field  string // "eval" or "pnl"
	}{
		{"1.852.010원", 1_852_010, false, "eval"},
		{"8.176.383원", 8_176_383, false, "eval"},
		{"1.994.953원", 1_994_953, false, "eval"},
		{"1.689.162원", 1_689_162, false, "eval"},
		{"1.610.530원", 1_610_530, false, "eval"},
		{"1.274.056원", 1_274_056, false, "eval"},
		{"-222.138원(11.62%:", -222_138, true, "pnl"},
		{"1,544,672원", 1_544_672, false, "eval"},
		{"~4,216,61781 (34.02%)", -4_216_617, true, "pnl"},
	}
	for _, tc := range cases {
		got, ok := parseFinanceLine(tc.line)
		if !ok {
			t.Fatalf("%q: not parsed", tc.line)
		}
		switch tc.field {
		case "eval":
			if got.MarketValue == nil {
				t.Fatalf("%q: missing eval", tc.line)
			}
			if *got.MarketValue != tc.want {
				t.Fatalf("%q: eval=%v want %v", tc.line, *got.MarketValue, tc.want)
			}
		case "pnl":
			if got.ProfitLoss == nil {
				t.Fatalf("%q: missing pnl", tc.line)
			}
			if *got.ProfitLoss != tc.want {
				t.Fatalf("%q: pnl=%v want %v", tc.line, *got.ProfitLoss, tc.want)
			}
		}
	}
}

func TestParseMoneyAmount(t *testing.T) {
	cases := map[string]float64{
		"1,544,672":   1_544_672,
		"1.852.010":   1_852_010,
		"8.176.383":   8_176_383,
		"89.9908":     89.9908,
		"010":         10,
	}
	for raw, want := range cases {
		got, err := parseMoneyAmount(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if got != want {
			t.Fatalf("%q: got %v want %v", raw, got, want)
		}
	}
}
