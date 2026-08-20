package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func f(v float64) *float64 { return &v }

func TestStockSummaryScoreNilWithoutData(t *testing.T) {
	if got := stockSummaryScore(nil, nil); got != nil {
		t.Fatalf("expected nil score with no inputs, got %d", *got)
	}
	if got := stockSummaryScore(&model.StockSummaryView{}, &TechnicalIndicators{}); got != nil {
		t.Fatalf("expected nil score with empty indicators, got %d", *got)
	}
}

func TestStockSummaryScoreRangesAndOrdering(t *testing.T) {
	bullish := &TechnicalIndicators{
		LastClose: f(110), SMA20: f(100), SMA50: f(95),
		RSI14: f(62), MACD: f(2), MACDSignal: f(1),
		PctFromLow52Week: f(80), PctFromHigh52Week: f(-5),
	}
	bearish := &TechnicalIndicators{
		LastClose: f(88), SMA20: f(100), SMA50: f(105),
		RSI14: f(28), MACD: f(-2), MACDSignal: f(1),
		PctFromLow52Week: f(5), PctFromHigh52Week: f(-60),
	}

	bull := stockSummaryScore(nil, bullish)
	bear := stockSummaryScore(nil, bearish)
	if bull == nil || bear == nil {
		t.Fatal("expected scores for populated indicators")
	}
	if *bull <= *bear {
		t.Fatalf("bullish setup (%d) should score above bearish (%d)", *bull, *bear)
	}
	for _, s := range []int{*bull, *bear} {
		if s < 0 || s > 100 {
			t.Fatalf("score %d out of 0..100", s)
		}
	}
}

// An overbought RSI must not read as the strongest possible momentum — the
// curve peaks below 70 and decays above it.
func TestRSIScorePeaksBelowOverbought(t *testing.T) {
	strong := rsiScore(65)
	overbought := rsiScore(85)
	oversold := rsiScore(20)

	if overbought >= strong {
		t.Fatalf("RSI 85 (%.1f) should score below RSI 65 (%.1f)", overbought, strong)
	}
	if oversold >= strong {
		t.Fatalf("RSI 20 (%.1f) should score below RSI 65 (%.1f)", oversold, strong)
	}
	for name, v := range map[string]float64{"strong": strong, "overbought": overbought, "oversold": oversold} {
		if v < 0 || v > 100 {
			t.Fatalf("%s score %.1f out of 0..100", name, v)
		}
	}
}

// A missing component must be dropped, never scored as zero — otherwise absent
// data would read as a maximally bearish signal.
func TestMissingComponentsAreDroppedNotZeroed(t *testing.T) {
	rsiOnly := stockSummaryScore(nil, &TechnicalIndicators{RSI14: f(62)})
	if rsiOnly == nil {
		t.Fatal("expected a score from RSI alone")
	}
	// RSI 62 sits in the strong band, so a re-weighted single component must
	// land there too rather than being diluted toward 0.
	if *rsiOnly < 60 {
		t.Fatalf("single strong component scored %d — missing data was treated as bearish", *rsiOnly)
	}
}

func TestNewsSentimentContributes(t *testing.T) {
	ti := &TechnicalIndicators{RSI14: f(50)}
	positive := stockSummaryScore(&model.StockSummaryView{
		NewsSentiment: &model.NewsSentimentSummary{Total: 10, AverageScore: 80},
	}, ti)
	negative := stockSummaryScore(&model.StockSummaryView{
		NewsSentiment: &model.NewsSentimentSummary{Total: 10, AverageScore: -80},
	}, ti)
	if positive == nil || negative == nil {
		t.Fatal("expected scores")
	}
	if *positive <= *negative {
		t.Fatalf("positive sentiment (%d) should score above negative (%d)", *positive, *negative)
	}
}
