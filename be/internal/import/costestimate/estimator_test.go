package costestimate

import (
	"math"
	"testing"

	"github.com/enact/quant-port/be/internal/import/textparse"
)

func TestEstimateEvalAndPnL(t *testing.T) {
	// Samsung SDI-like: 4 shares, eval 1,544,672, loss -397,328 → avg 485,500
	eval := 1_544_672.0
	pnl := -397_328.0
	res := Estimate(textparse.ParsedLine{
		Name:        "삼성SDI",
		Quantity:    4,
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		Currency:    "KRW",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg price")
	}
	want := 485_500.0
	if mathAbs(*res.AvgPrice-want) > 1 {
		t.Fatalf("avg=%v want~%v", *res.AvgPrice, want)
	}
	if res.Currency != "KRW" {
		t.Fatalf("currency=%s", res.Currency)
	}
}

func TestEstimateEvalAndReturn(t *testing.T) {
	eval := 7_840_644.0
	ret := -0.3854
	res := Estimate(textparse.ParsedLine{
		Name:        "삼성전자",
		Quantity:    35,
		MarketValue: &eval,
		ReturnRate:  &ret,
		Currency:    "KRW",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg price")
	}
	if *res.AvgPrice <= 0 {
		t.Fatalf("unexpected avg: %v", *res.AvgPrice)
	}
}

func TestEstimateInsufficientData(t *testing.T) {
	res := Estimate(textparse.ParsedLine{Name: "삼성전자", Quantity: 10})
	if res.AvgPrice != nil {
		t.Fatalf("expected nil avg, got %v", *res.AvgPrice)
	}
}

func TestEstimateKRWFiveWonTick(t *testing.T) {
	eval := 23_816_500.0
	pnl := -15_343_045.0
	res := Estimate(textparse.ParsedLine{
		Name:        "삼성전자",
		Quantity:    109,
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		Currency:    "KRW",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg price")
	}
	want := 359_260.0
	if *res.AvgPrice != want {
		t.Fatalf("avg=%v want %v", *res.AvgPrice, want)
	}
	if math.Mod(*res.AvgPrice, 5) != 0 {
		t.Fatalf("expected 5-won tick, got %v", *res.AvgPrice)
	}
}

func TestEstimateEvalAndReturnRoundsKRW(t *testing.T) {
	eval := 4_038_279.0
	ret := -0.1825
	res := Estimate(textparse.ParsedLine{
		Name:        "NAVER",
		Quantity:    19,
		MarketValue: &eval,
		ReturnRate:  &ret,
		Currency:    "KRW",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg price")
	}
	if math.Mod(*res.AvgPrice, 1) != 0 {
		t.Fatalf("expected integer KRW avg, got %v", *res.AvgPrice)
	}
}

func TestEstimateKRWUnsignedReturnWithoutPnL(t *testing.T) {
	eval := 13_973_000.0
	ret := 0.1712 // minus dropped by OCR
	res := Estimate(textparse.ParsedLine{
		Name:        "KODEX 미국나스닥100",
		Quantity:    785,
		MarketValue: &eval,
		ReturnRate:  &ret,
		Currency:    "KRW",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg price")
	}
	want := 21_475.0
	if mathAbs(*res.AvgPrice-want) > 5 {
		t.Fatalf("avg=%v want~%v", *res.AvgPrice, want)
	}
}

func TestEstimateUSD(t *testing.T) {
	eval := 1966.68
	pnl := 294.68
	ret := 0.1762
	res := Estimate(textparse.ParsedLine{
		Name:        "어플라이드 머티리얼즈",
		Quantity:    4,
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		ReturnRate:  &ret,
		Currency:    "USD",
	})
	if res.AvgPrice == nil || res.Currency != "USD" {
		t.Fatalf("unexpected: %+v", res)
	}
}

func TestEstimateUSDSmallReturn(t *testing.T) {
	// XOVR-like: tiny P&L and ~0.04% return should still yield avg price.
	eval := 3673.80
	pnl := 1.80
	ret := 0.0004
	res := Estimate(textparse.ParsedLine{
		Name:        "XOVR",
		Quantity:    180,
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		ReturnRate:  &ret,
		Currency:    "USD",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg price for small-return USD row")
	}
	want := 20.40
	if mathAbs(*res.AvgPrice-want) > 0.05 {
		t.Fatalf("avg=%v want~%v method=%s", *res.AvgPrice, want, res.Method)
	}
}

func TestEstimateKRWFromEvalMagnitude(t *testing.T) {
	eval := 36_225_000.0
	pnl := -29_738.0
	ret := 0.4508 // OCR garbage: 45% return with tiny PnL on 36M eval
	res := Estimate(textparse.ParsedLine{
		Name:        "SK하이닉스",
		Quantity:    23,
		MarketValue: &eval,
		ProfitLoss:  &pnl,
		ReturnRate:  &ret,
		Currency:    "USD",
	})
	if res.AvgPrice == nil {
		t.Fatal("expected avg price")
	}
	if res.Currency != "KRW" {
		t.Fatalf("currency=%s want KRW", res.Currency)
	}
	want := 1_576_295.0
	if mathAbs(*res.AvgPrice-want) > 10_000 {
		t.Fatalf("avg=%v want~%v (garbage return must not inflate PnL)", *res.AvgPrice, want)
	}
}

func mathAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
