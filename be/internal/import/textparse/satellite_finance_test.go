package textparse

import "testing"

func TestAttachSatelliteFinanceMete(t *testing.T) {
	named := []ParsedLine{{Name: "맵러빈", Quantity: 15}}
	mv := 4805.76
	pnl := -234.74
	ret := -0.0465
	all := append(named, ParsedLine{
		Name:        "mete!",
		Quantity:    15,
		MarketValue: &mv,
		ProfitLoss:  &pnl,
		ReturnRate:  &ret,
		Currency:    "USD",
	})
	out := AttachSatelliteFinance(named, all)
	if len(out) != 1 || out[0].MarketValue == nil || out[0].ProfitLoss == nil {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestAttachSatelliteFinanceSkipsPortfolioSummary(t *testing.T) {
	mv := 36_225_000.0
	named := []ParsedLine{{Name: "SK하이닉스", Quantity: 48}}
	portfolioPnL := -34_000_000.0
	all := append(named, ParsedLine{
		Name:        "가나다순 ~",
		Quantity:    34,
		MarketValue: &mv,
		ProfitLoss:  &portfolioPnL,
		Currency:    "KRW",
	})
	out := AttachSatelliteFinance(named, all)
	if out[0].ProfitLoss != nil && *out[0].ProfitLoss <= -10_000_000 {
		t.Fatalf("portfolio summary PnL leaked: %+v", out[0])
	}
}

func TestNormalizeDisplayNameSKHynix(t *testing.T) {
	if got := NormalizeDisplayName("5<하이닉스"); got != "SK하이닉스" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeDisplayName("5하미닉스"); got != "SK하이닉스" {
		t.Fatalf("got %q", got)
	}
}
