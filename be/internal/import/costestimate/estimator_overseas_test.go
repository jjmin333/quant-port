package costestimate

import (
	"math"
	"testing"

	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestEstimateOracleSmallReturn(t *testing.T) {
	eval := 5763.69
	pnl := 23.69
	ret := 0.0041
	res := Estimate(textparse.ParsedLine{
		Name:        "오라클",
		Quantity:    40,
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		ReturnRate:  &ret,
		Currency:    "USD",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg for small return overseas row")
	}
	want := 143.5
	if math.Abs(*res.AvgPrice-want) > 2 {
		t.Fatalf("avg=%v want~%v", *res.AvgPrice, want)
	}
}

func TestEstimateAppLovinInflatedPnL(t *testing.T) {
	eval := 4805.76
	pnl := -6234.74
	ret := -0.0465
	res := Estimate(textparse.ParsedLine{
		Name:        "맵러빈",
		Quantity:    15,
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		ReturnRate:  &ret,
		Currency:    "USD",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg")
	}
	want := 335.77
	if math.Abs(*res.AvgPrice-want) > 1 {
		t.Fatalf("avg=%v want~%v method=%s", *res.AvgPrice, want, res.Method)
	}
}

func TestEstimateAppLovinLoss(t *testing.T) {
	eval := 4805.76
	pnl := -234.74
	ret := -0.0465
	res := Estimate(textparse.ParsedLine{
		Name:        "맵러빈",
		Quantity:    15,
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		ReturnRate:  &ret,
		Currency:    "USD",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg")
	}
	want := 335.77
	if math.Abs(*res.AvgPrice-want) > 1 {
		t.Fatalf("avg=%v want~%v", *res.AvgPrice, want)
	}
}
