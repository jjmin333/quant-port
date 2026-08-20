package service

import (
	"math"

	"github.com/enact/quant-port/be/internal/model"
)

// TechnicalIndicators is a snapshot of standard technical-analysis figures
// computed from a symbol's daily candle history. Every field is a pointer —
// nil means "not enough history to compute this one," never a fabricated
// value. Computed purely from model.Candle slices, no I/O.
type TechnicalIndicators struct {
	LastClose *float64 `json:"lastClose,omitempty"`

	SMA20  *float64 `json:"sma20,omitempty"`
	SMA50  *float64 `json:"sma50,omitempty"`
	SMA200 *float64 `json:"sma200,omitempty"`

	RSI14 *float64 `json:"rsi14,omitempty"`

	MACD          *float64 `json:"macd,omitempty"`
	MACDSignal    *float64 `json:"macdSignal,omitempty"`
	MACDHistogram *float64 `json:"macdHistogram,omitempty"`

	ADX14 *float64 `json:"adx14,omitempty"`

	High52Week        *float64 `json:"high52Week,omitempty"`
	Low52Week         *float64 `json:"low52Week,omitempty"`
	PctFromHigh52Week *float64 `json:"pctFromHigh52Week,omitempty"`
	PctFromLow52Week  *float64 `json:"pctFromLow52Week,omitempty"`

	RecentSwingHigh *float64 `json:"recentSwingHigh,omitempty"`
	RecentSwingLow  *float64 `json:"recentSwingLow,omitempty"`

	// Trailing returns, used by the AI 요약 facts to say how the price actually
	// moved rather than making the model infer it from moving averages.
	Return5D  *float64 `json:"return5d,omitempty"`
	Return20D *float64 `json:"return20d,omitempty"`
}

const (
	tiRSIPeriod         = 14
	tiADXPeriod         = 14
	tiMACDFast          = 12
	tiMACDSlow          = 26
	tiMACDSignal        = 9
	tiSwingLookback     = 20
	tiTradingDaysInYear = 252
)

// ComputeTechnicalIndicators derives indicators from candles, which must be
// ordered oldest-first (the shape CandleBarRepo.ListBefore returns).
func ComputeTechnicalIndicators(candles []model.Candle) *TechnicalIndicators {
	n := len(candles)
	if n == 0 {
		return nil
	}

	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i, c := range candles {
		closes[i] = c.Close
		highs[i] = c.High
		lows[i] = c.Low
	}

	out := &TechnicalIndicators{LastClose: floatPtr(closes[n-1])}
	out.SMA20 = smaLast(closes, 20)
	out.SMA50 = smaLast(closes, 50)
	out.SMA200 = smaLast(closes, 200)
	out.RSI14 = wilderRSI(closes, tiRSIPeriod)
	out.MACD, out.MACDSignal, out.MACDHistogram = macd(closes, tiMACDFast, tiMACDSlow, tiMACDSignal)
	out.ADX14 = wilderADX(highs, lows, closes, tiADXPeriod)

	yearStart := 0
	if n > tiTradingDaysInYear {
		yearStart = n - tiTradingDaysInYear
	}
	high52, low52 := extremes(highs[yearStart:], lows[yearStart:])
	out.High52Week = floatPtr(high52)
	out.Low52Week = floatPtr(low52)
	last := closes[n-1]
	if high52 > 0 {
		v := (last - high52) / high52 * 100
		out.PctFromHigh52Week = &v
	}
	if low52 > 0 {
		v := (last - low52) / low52 * 100
		out.PctFromLow52Week = &v
	}

	out.Return5D = trailingReturn(closes, 5)
	out.Return20D = trailingReturn(closes, 20)

	swingStart := 0
	if n > tiSwingLookback {
		swingStart = n - tiSwingLookback
	}
	swingHigh, swingLow := extremes(highs[swingStart:], lows[swingStart:])
	out.RecentSwingHigh = floatPtr(swingHigh)
	out.RecentSwingLow = floatPtr(swingLow)

	return out
}

func floatPtr(v float64) *float64 { return &v }

// trailingReturn is the percent change over the last `bars` sessions. nil when
// there is not enough history, never 0 — an unknown move must not read as flat.
func trailingReturn(closes []float64, bars int) *float64 {
	if len(closes) <= bars || bars <= 0 {
		return nil
	}
	prev := closes[len(closes)-1-bars]
	if prev <= 0 {
		return nil
	}
	v := (closes[len(closes)-1] - prev) / prev * 100
	return &v
}

func smaLast(values []float64, period int) *float64 {
	if len(values) < period || period <= 0 {
		return nil
	}
	window := values[len(values)-period:]
	sum := 0.0
	for _, v := range window {
		sum += v
	}
	avg := sum / float64(period)
	return &avg
}

// emaSeries returns an EMA series the same length as values; entries before
// index period-1 are 0 and must not be read (seeded via SMA of the first
// `period` values, per the standard EMA convention).
func emaSeries(values []float64, period int) []float64 {
	if len(values) < period || period <= 0 {
		return nil
	}
	out := make([]float64, len(values))
	seed := 0.0
	for i := 0; i < period; i++ {
		seed += values[i]
	}
	seed /= float64(period)
	out[period-1] = seed
	k := 2.0 / float64(period+1)
	ema := seed
	for i := period; i < len(values); i++ {
		ema = values[i]*k + ema*(1-k)
		out[i] = ema
	}
	return out
}

func wilderRSI(closes []float64, period int) *float64 {
	if len(closes) < period+1 {
		return nil
	}
	var gainSum, lossSum float64
	for i := 1; i <= period; i++ {
		diff := closes[i] - closes[i-1]
		if diff > 0 {
			gainSum += diff
		} else {
			lossSum += -diff
		}
	}
	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)
	for i := period + 1; i < len(closes); i++ {
		diff := closes[i] - closes[i-1]
		gain, loss := 0.0, 0.0
		if diff > 0 {
			gain = diff
		} else {
			loss = -diff
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
	}
	if avgLoss == 0 {
		v := 100.0
		return &v
	}
	rs := avgGain / avgLoss
	v := 100 - (100 / (1 + rs))
	return &v
}

func macd(closes []float64, fast, slow, signal int) (macdLine, signalLine, histogram *float64) {
	if len(closes) < slow+signal {
		return nil, nil, nil
	}
	emaFast := emaSeries(closes, fast)
	emaSlow := emaSeries(closes, slow)
	start := slow - 1
	macdSeries := make([]float64, 0, len(closes)-start)
	for i := start; i < len(closes); i++ {
		macdSeries = append(macdSeries, emaFast[i]-emaSlow[i])
	}
	if len(macdSeries) < signal {
		return nil, nil, nil
	}
	signalSeries := emaSeries(macdSeries, signal)
	lastMacd := macdSeries[len(macdSeries)-1]
	lastSignal := signalSeries[len(signalSeries)-1]
	hist := lastMacd - lastSignal
	return &lastMacd, &lastSignal, &hist
}

// wilderADX implements the standard Average Directional Index: smoothed
// +DM/-DM/TR over `period`, DX from the smoothed DI values, then ADX as the
// Wilder-smoothed average of DX. Needs at least ~2*period+1 bars.
func wilderADX(highs, lows, closes []float64, period int) *float64 {
	n := len(highs)
	if n < period*2+1 {
		return nil
	}

	trs := make([]float64, 0, n-1)
	plusDMs := make([]float64, 0, n-1)
	minusDMs := make([]float64, 0, n-1)
	for i := 1; i < n; i++ {
		upMove := highs[i] - highs[i-1]
		downMove := lows[i-1] - lows[i]
		plusDM, minusDM := 0.0, 0.0
		if upMove > downMove && upMove > 0 {
			plusDM = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM = downMove
		}
		tr := math.Max(highs[i]-lows[i], math.Max(math.Abs(highs[i]-closes[i-1]), math.Abs(lows[i]-closes[i-1])))
		trs = append(trs, tr)
		plusDMs = append(plusDMs, plusDM)
		minusDMs = append(minusDMs, minusDM)
	}
	if len(trs) < period*2 {
		return nil
	}

	sumFirst := func(vals []float64, k int) float64 {
		s := 0.0
		for i := 0; i < k; i++ {
			s += vals[i]
		}
		return s
	}
	smoothTR := sumFirst(trs, period)
	smoothPlusDM := sumFirst(plusDMs, period)
	smoothMinusDM := sumFirst(minusDMs, period)

	var dxValues []float64
	for i := period; i < len(trs); i++ {
		smoothTR = smoothTR - smoothTR/float64(period) + trs[i]
		smoothPlusDM = smoothPlusDM - smoothPlusDM/float64(period) + plusDMs[i]
		smoothMinusDM = smoothMinusDM - smoothMinusDM/float64(period) + minusDMs[i]
		if smoothTR == 0 {
			continue
		}
		plusDI := 100 * smoothPlusDM / smoothTR
		minusDI := 100 * smoothMinusDM / smoothTR
		sumDI := plusDI + minusDI
		if sumDI == 0 {
			continue
		}
		dxValues = append(dxValues, 100*math.Abs(plusDI-minusDI)/sumDI)
	}
	if len(dxValues) < period {
		return nil
	}

	adx := 0.0
	for i := 0; i < period; i++ {
		adx += dxValues[i]
	}
	adx /= float64(period)
	for i := period; i < len(dxValues); i++ {
		adx = (adx*float64(period-1) + dxValues[i]) / float64(period)
	}
	return &adx
}

func extremes(highs, lows []float64) (high, low float64) {
	if len(highs) == 0 {
		return 0, 0
	}
	high, low = highs[0], lows[0]
	for i := 1; i < len(highs); i++ {
		if highs[i] > high {
			high = highs[i]
		}
		if lows[i] < low {
			low = lows[i]
		}
	}
	return high, low
}
