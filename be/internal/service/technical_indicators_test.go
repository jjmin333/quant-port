package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func makeCandles(closes []float64) []model.Candle {
	out := make([]model.Candle, len(closes))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, c := range closes {
		out[i] = model.Candle{
			Time:   base.AddDate(0, 0, i).Format(time.RFC3339),
			Open:   c,
			High:   c * 1.01,
			Low:    c * 0.99,
			Close:  c,
			Volume: 1000,
		}
	}
	return out
}

func TestComputeTechnicalIndicators_InsufficientHistory(t *testing.T) {
	out := ComputeTechnicalIndicators(makeCandles([]float64{100, 101, 102}))
	if out == nil {
		t.Fatal("expected non-nil result even with short history")
	}
	if out.SMA20 != nil {
		t.Error("SMA20 should be nil with only 3 candles")
	}
	if out.RSI14 != nil {
		t.Error("RSI14 should be nil with only 3 candles")
	}
	if out.LastClose == nil || *out.LastClose != 102 {
		t.Errorf("LastClose = %v, want 102", out.LastClose)
	}
}

func TestComputeTechnicalIndicators_MonotonicUptrend(t *testing.T) {
	closes := make([]float64, 260)
	for i := range closes {
		closes[i] = 100 + float64(i)*0.5
	}
	out := ComputeTechnicalIndicators(makeCandles(closes))
	if out.RSI14 == nil {
		t.Fatal("RSI14 should be computable with 260 candles")
	}
	if *out.RSI14 < 90 {
		t.Errorf("RSI14 = %.2f, want close to 100 for a pure uptrend", *out.RSI14)
	}
	if out.SMA20 == nil || out.SMA50 == nil || out.SMA200 == nil {
		t.Fatal("all SMAs should be computable with 260 candles")
	}
	if *out.SMA20 <= *out.SMA50 {
		t.Errorf("SMA20 (%.2f) should be above SMA50 (%.2f) in an uptrend", *out.SMA20, *out.SMA50)
	}
	if out.MACD == nil || out.MACDSignal == nil {
		t.Fatal("MACD should be computable with 260 candles")
	}
	if *out.MACD <= 0 {
		t.Errorf("MACD = %.4f, want positive in a sustained uptrend", *out.MACD)
	}
	if out.ADX14 == nil {
		t.Fatal("ADX14 should be computable with 260 candles")
	}
	if *out.High52Week != closes[len(closes)-1]*1.01 {
		t.Errorf("High52Week should track the last (highest) bar in a monotonic uptrend")
	}
}

func TestComputeTechnicalIndicators_EmptyInput(t *testing.T) {
	if out := ComputeTechnicalIndicators(nil); out != nil {
		t.Errorf("expected nil for empty candle slice, got %+v", out)
	}
}
