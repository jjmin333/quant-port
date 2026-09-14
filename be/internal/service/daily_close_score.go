package service

/*
지표 곡선 — RSI·MACD·추세·52주 위치를 0~100으로 옮기는 함수들.

# 여기 있던 두 번째 점수는 없앴다 (2026-09-03 §6)

`stockSummaryScore`가 이 파일에서 요약 파이프라인 전용 종합 점수를 만들어
`stock_daily_closes.score`에 써 넣고 있었다. 같은 컬럼을 점수 배치도 쓴다
(`SyncDailyCloseScores`). 두 값의 잣대가 아예 달라서 — 이쪽은 기술적 지표 +
뉴스 감성만 보는 절대 기준, 저쪽은 5축을 동종 그룹 백분위로 환산한 값 — AI 분석
탭의 「오늘의 종합 점수」와 바로 아래 같은 날짜 행의 점수가 서로 다른 숫자를
말했다. 종합 점수의 주인은 `stock_scores` 하나다.

남은 곡선들은 **점수 배치가 쓴다**(`stock_score.go`의 `technicalAxis`) —
절대 곡선으로 「좋은 형태」를 정한 뒤 그 값을 동종 그룹 안에서 다시 줄 세운다.
곡선 자체는 그대로 두고, 그 위에 얹혀 있던 두 번째 합산만 걷어냈다.

각 곡선은 중립 50을 중심으로 0~100을 낸다.
*/

type scoreComponent struct {
	value  float64
	weight float64
}

// rsiScore maps RSI to a momentum reading. Deliberately not "high RSI = good":
// above ~70 the move is stretched, so the curve peaks in the strong-but-not-
// overbought band and falls away at both extremes.
func rsiScore(rsi float64) float64 {
	switch {
	case rsi <= 30:
		// Oversold — weak now, but not the worst possible reading.
		return 25 + rsi/30*15 // 25 → 40
	case rsi <= 55:
		return 40 + (rsi-30)/25*35 // 40 → 75
	case rsi <= 70:
		return 75 + (rsi-55)/15*15 // 75 → 90
	default:
		// Overbought — decay back toward neutral as it stretches further.
		return clamp01to100(90 - (rsi-70)/30*45) // 90 → 45
	}
}

// macdScore reads the histogram's sign and size relative to the signal line.
func macdScore(macd, signal float64) float64 {
	diff := macd - signal
	scale := absFloat(signal)
	if scale < 1e-9 {
		scale = absFloat(macd)
	}
	if scale < 1e-9 {
		return 50
	}
	// ±50% of the signal magnitude saturates the component.
	ratio := diff / scale
	return clamp01to100(50 + clampFloat(ratio, -1, 1)*40)
}

// trendScore rewards price above its moving averages and the 20d above the 50d.
func trendScore(last, sma20, sma50 float64) float64 {
	score := 50.0
	if sma20 > 0 {
		score += clampFloat((last-sma20)/sma20*100, -8, 8) * 1.5
	}
	if sma50 > 0 {
		score += clampFloat((last-sma50)/sma50*100, -8, 8) * 1.0
	}
	if sma20 > 0 && sma50 > 0 && sma20 > sma50 {
		score += 6
	}
	return clamp01to100(score)
}

// rangePositionScore places the price inside its 52-week band (0 = at the low,
// 100 = at the high).
func rangePositionScore(pctFromLow, pctFromHigh float64) float64 {
	// pctFromHigh is negative or zero below the high.
	span := pctFromLow + absFloat(pctFromHigh)
	if span <= 0 {
		return 50
	}
	return clamp01to100(pctFromLow / span * 100)
}

func clamp01to100(v float64) float64 { return clampFloat(v, 0, 100) }

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
