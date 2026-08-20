package service

import (
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// neutralBandPct is how far a "관망" stamp may drift and still count as right.
// Same number that decides whether a target price reads as 관망 in the first
// place (model.StampNeutralBandPct) — two constants here would let a stamp be
// filed as 관망 and then judged by a different band.
const neutralBandPct = model.StampNeutralBandPct

// stampHorizons are the fixed buckets every stamp reports, independent of the
// user's own verification horizon.
var stampHorizons = []string{"1D", "1W", "1M", "3M"}

// horizonTargetDate maps a bucket label to its target calendar date.
func horizonTargetDate(base time.Time, label string) time.Time {
	switch label {
	case "1D":
		return base.AddDate(0, 0, 1)
	case "1W":
		return base.AddDate(0, 0, 7)
	case "1M":
		return base.AddDate(0, 1, 0)
	case "3M":
		return base.AddDate(0, 3, 0)
	default:
		return base
	}
}

// buildStampReturn resolves one horizon bucket. A nil Return means the horizon
// has not produced a close after the stamp yet — the UI renders "—" and must
// never substitute 0.
func buildStampReturn(
	label string,
	stampTradeDate string,
	stampPrice float64,
	target time.Time,
	prices closeSeries,
	benchmarkID string,
	benchmark closeSeries,
) model.StampReturn {
	out := model.StampReturn{Horizon: label, BenchmarkID: benchmarkID}
	if stampPrice <= 0 {
		return out
	}
	point, ok := resolveHorizonClose(prices, stampTradeDate, target)
	if !ok {
		return out
	}
	ret, ok := closePctChange(stampPrice, point.Close)
	if !ok {
		return out
	}
	out.AsOfDate = point.Date
	price := point.Close
	out.Price = &price
	out.Return = &ret

	if benchmarkID == "" || benchmark.empty() {
		return out
	}
	base, okBase := benchmark.onOrBefore(stampTradeDate)
	end, okEnd := benchmark.onOrBefore(point.Date)
	if !okBase || !okEnd || end.Date <= base.Date {
		return out
	}
	benchRet, okRate := closePctChange(base.Close, end.Close)
	if !okRate {
		return out
	}
	alpha := ret - benchRet
	out.BenchmarkRet = &benchRet
	out.Alpha = &alpha
	return out
}

// stampVerdict judges the user's own hypothesis. Judgement uses plain price
// return (alpha is reported alongside but does not decide) — a rule the user
// can reproduce in their head is worth more than a marginally better one.
func stampVerdict(stance model.StampStance, ret *float64) model.StampVerdict {
	if ret == nil {
		return model.VerdictUnknown
	}
	switch stance {
	case model.StanceBullish:
		if *ret > 0 {
			return model.VerdictHit
		}
	case model.StanceBearish:
		if *ret < 0 {
			return model.VerdictHit
		}
	case model.StanceNeutral:
		if *ret > -neutralBandPct && *ret < neutralBandPct {
			return model.VerdictHit
		}
	}
	return model.VerdictMiss
}

// buildStampView assembles the read-time half of a stamp: current price, the
// fixed horizon buckets, the user's own deadline and its verdict.
//
// extremes are the daily highs/lows since the stamp (may be nil). They decide
// early hits and feed the peak/trough report.
func buildStampView(
	s model.Stamp,
	now time.Time,
	livePrice float64,
	prices closeSeries,
	benchmark closeSeries,
	extremes []repository.DailyExtreme,
) model.StampView {
	view := model.StampView{Stamp: s}
	view.Returns = make([]model.StampReturn, 0, len(stampHorizons))

	stampedAt := s.StampedAt
	for _, label := range stampHorizons {
		view.Returns = append(view.Returns, buildStampReturn(
			label, s.TradeDate, s.Price, horizonTargetDate(stampedAt, label),
			prices, s.BenchmarkID, benchmark,
		))
	}

	view.Opened = s.OpenedAt != nil
	view.ExpectedReturn = expectedReturn(s.Price, s.TargetPrice)

	horizon := buildStampReturn(
		"horizon", s.TradeDate, s.Price, s.DueAt,
		prices, s.BenchmarkID, benchmark,
	)
	view.Horizon = &horizon

	// An intraday touch of the target settles the stamp there and then. Waiting
	// for the deadline would file "목표에 닿았다가 되돌아왔다" as a miss, which is
	// not what the user predicted wrong.
	view.TargetHit = findTargetHit(s, extremes, now)
	view.PeakReturn, view.TroughReturn = extremeReturns(s.Price, extremes)

	switch {
	case view.TargetHit != nil:
		view.Verdict = model.VerdictHit
		// Reaching the target is exactly on target, so the error is zero — the
		// stamp is not penalised for whatever the price did afterwards.
		zero := 0.0
		view.TargetAccuracy = &zero
		view.Due = true
	case now.Before(s.DueAt):
		view.Verdict = model.VerdictPending
		view.Due = false
	default:
		view.Due = true
		view.Verdict = stampVerdict(s.Stance, horizon.Return)
		view.TargetAccuracy = targetError(s.TargetPrice, horizon.Price)
	}

	view.Current = buildStampCurrent(s, livePrice, prices)
	if view.Current != nil {
		view.TargetProgress = targetProgress(s, view.Current.Price)
	}
	return view
}

// expectedReturn is the move the target implies from the captured price — the
// size of the prediction, shown next to the result so a lucky +0.4% against a
// +18% target reads as the near-miss it is.
func expectedReturn(price, target float64) *float64 {
	if price <= 0 || target <= 0 {
		return nil
	}
	v := (target - price) / price * 100
	return &v
}

// findTargetHit returns the first day the price reached the target inside the
// verification window, or nil.
//
// 관망 has no direction to reach, so it never hits early: a sideways call is
// only answerable at the deadline.
func findTargetHit(s model.Stamp, extremes []repository.DailyExtreme, now time.Time) *model.StampTargetHit {
	if s.TargetPrice <= 0 || s.Stance == model.StanceNeutral || len(extremes) == 0 {
		return nil
	}
	// The stamp day itself is excluded: the day's high may predate the stamp,
	// and crediting a hit the price already made before the user called it
	// would be scoring the past.
	deadline := s.DueAt
	if now.Before(deadline) {
		deadline = now
	}
	last := marketTradeDate(deadline, s.AssetType)

	for _, e := range extremes {
		if e.Date <= s.TradeDate || e.Date > last {
			continue
		}
		switch s.Stance {
		case model.StanceBullish:
			if e.High >= s.TargetPrice {
				return &model.StampTargetHit{Date: e.Date, Price: e.High}
			}
		case model.StanceBearish:
			if e.Low <= s.TargetPrice {
				return &model.StampTargetHit{Date: e.Date, Price: e.Low}
			}
		}
	}
	return nil
}

// extremeReturns reports the best and worst the price got to since the stamp,
// in percent. "적중이지만 도중에 -6%를 버텨야 했다" is a fact the closing price
// alone hides.
func extremeReturns(price float64, extremes []repository.DailyExtreme) (*float64, *float64) {
	if price <= 0 || len(extremes) == 0 {
		return nil, nil
	}
	high, low := extremes[0].High, extremes[0].Low
	for _, e := range extremes[1:] {
		if e.High > high {
			high = e.High
		}
		if e.Low < low {
			low = e.Low
		}
	}
	peak := (high - price) / price * 100
	trough := (low - price) / price * 100
	return &peak, &trough
}

// targetError is |settled − target| ÷ target, in percent.
func targetError(target float64, settled *float64) *float64 {
	if target <= 0 || settled == nil || *settled <= 0 {
		return nil
	}
	diff := *settled - target
	if diff < 0 {
		diff = -diff
	}
	v := diff / target * 100
	return &v
}

// targetProgress is how much of the predicted move has happened: 100 means the
// target was reached, negative means the price went the other way. 관망 stamps
// get nil — there is no distance to cover.
func targetProgress(s model.Stamp, current float64) *float64 {
	if s.Price <= 0 || s.TargetPrice <= 0 || current <= 0 || s.Stance == model.StanceNeutral {
		return nil
	}
	span := s.TargetPrice - s.Price
	if span == 0 {
		return nil
	}
	v := (current - s.Price) / span * 100
	return &v
}

func buildStampCurrent(s model.Stamp, livePrice float64, prices closeSeries) *model.StampCurrent {
	cur := &model.StampCurrent{}
	if livePrice > 0 {
		cur.Price = livePrice
		cur.Source = "quote"
	} else if point, ok := prices.latest(); ok {
		cur.Price = point.Close
		cur.TradeDate = point.Date
		cur.Source = "close"
	} else {
		return nil
	}
	if rate, ok := closePctChange(s.Price, cur.Price); ok {
		cur.ChangeRate = &rate
	}
	return cur
}

// summarizeStamps aggregates the header stats. Only judged stamps (horizon
// elapsed with a real close) count toward the hit rate — pending ones would
// otherwise drag it toward zero and make the number meaningless early on.
func summarizeStamps(views []model.StampView) model.StampSummary {
	out := model.StampSummary{Total: len(views)}
	symbols := map[string]struct{}{}
	var (
		retSum, alphaSum, errSum float64
		retN, alphaN, errN       int
	)
	for _, v := range views {
		symbols[v.Symbol] = struct{}{}
		if v.Due && !v.Opened {
			out.DueCount++
		}
		if v.TargetHit != nil {
			out.EarlyHits++
		}
		if v.TargetAccuracy != nil {
			errSum += *v.TargetAccuracy
			errN++
			if *v.TargetAccuracy <= neutralBandPct {
				out.WithinBand++
			}
		}
		if v.Verdict == model.VerdictHit || v.Verdict == model.VerdictMiss {
			out.Judged++
			if v.Verdict == model.VerdictHit {
				out.Hits++
			}
		}
		if v.Horizon != nil {
			if v.Horizon.Return != nil {
				retSum += *v.Horizon.Return
				retN++
			}
			if v.Horizon.Alpha != nil {
				alphaSum += *v.Horizon.Alpha
				alphaN++
			}
		}
	}
	out.Symbols = len(symbols)
	if out.Judged > 0 {
		rate := float64(out.Hits) / float64(out.Judged) * 100
		out.HitRate = &rate
	}
	if retN > 0 {
		avg := retSum / float64(retN)
		out.AvgReturn = &avg
	}
	if alphaN > 0 {
		avg := alphaSum / float64(alphaN)
		out.AvgAlpha = &avg
	}
	if errN > 0 {
		avg := errSum / float64(errN)
		out.AvgTargetError = &avg
	}
	if len(views) > 0 {
		last := views[0].StampedAt
		for _, v := range views {
			if v.StampedAt.After(last) {
				last = v.StampedAt
			}
		}
		formatted := last.UTC().Format(time.RFC3339)
		out.LastStamped = &formatted
	}
	return out
}
