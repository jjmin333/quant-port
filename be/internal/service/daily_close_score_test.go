package service

import "testing"

func f(v float64) *float64 { return &v }

/*
이 파일이 재던 `stockSummaryScore`는 없앴다 (2026-09-03 §6) — 종합 점수를 두
군데서 서로 다른 잣대로 만들던 쪽이었다. 남은 곡선들은 점수 배치의 `technicalAxis`가
쓰므로, 검사도 곡선 자체의 성질로 옮긴다.
*/

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

// MACD above its signal is momentum turning up; below is turning down. The
// technical axis leans on this ordering, so it is worth pinning.
func TestMACDScoreOrdering(t *testing.T) {
	up := macdScore(2, 1)
	down := macdScore(-2, 1)
	if up <= down {
		t.Fatalf("MACD above signal (%.1f) should score above MACD below signal (%.1f)", up, down)
	}
	for name, v := range map[string]float64{"up": up, "down": down} {
		if v < 0 || v > 100 {
			t.Fatalf("%s score %.1f out of 0..100", name, v)
		}
	}
}

// Price above both moving averages is a stronger setup than price below both.
func TestTrendScoreOrdering(t *testing.T) {
	above := trendScore(110, 100, 95)
	below := trendScore(88, 100, 105)
	if above <= below {
		t.Fatalf("price above its MAs (%.1f) should score above price below them (%.1f)", above, below)
	}
	for name, v := range map[string]float64{"above": above, "below": below} {
		if v < 0 || v > 100 {
			t.Fatalf("%s score %.1f out of 0..100", name, v)
		}
	}
}
