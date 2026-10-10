package service

import (
	"math"
	"sort"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// Verdict rules (docs/RESEARCH_CALL_DESIGN.md §8).
//
// Two decisions carry the whole design:
//
//  1. Judgement reads *closes only*. An intraday wick through the target on a
//     thin symbol is not a thesis coming true, and using highs would hand every
//     illiquid small cap a free hit.
//  2. A horizon that has not produced a post-publish close yet is nil, never 0.
//     "No data" and "no move" are different answers.

// researchHoldBandPct is how far a 관망 call may drift and still be right.
const researchHoldBandPct = 3.0

// researchMinSample is the number of resolved calls below which a hit rate is
// reported but not treated as meaningful (leaderboard §9 uses the same value).
const researchMinSample = 10

// researchHorizons are the fixed buckets every call reports, independent of its
// own horizon.
var researchHorizons = []string{"1W", "1M", "3M"}

func researchHorizonTargetDate(base time.Time, label string) time.Time {
	switch label {
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

// researchVerdict decides a call whose horizon has elapsed, using the plain
// price return. Alpha is reported next to it but does not decide — a rule the
// author can reproduce in their head is worth more than a marginally better one.
func researchVerdict(direction model.ResearchDirection, ret *float64) model.ResearchStatus {
	if ret == nil {
		return model.StatusExpiredUnknown
	}
	switch direction {
	case model.DirectionBuy:
		if *ret > 0 {
			return model.StatusExpiredHit
		}
	case model.DirectionSell:
		if *ret < 0 {
			return model.StatusExpiredHit
		}
	case model.DirectionHold:
		if *ret > -researchHoldBandPct && *ret < researchHoldBandPct {
			return model.StatusExpiredHit
		}
	}
	return model.StatusExpiredMiss
}

// researchBoundHit finds the first close that reached the call's target or stop,
// whichever came first. Only closes strictly after the publish date count.
//
// The direction decides which side of a bound counts: a buy call's target is
// reached from below, a sell call's from above.
func researchBoundHit(
	c model.ResearchCall, prices closeSeries, until time.Time,
) (model.ResearchStatus, repository.DailyClosePoint, bool) {
	if prices.empty() {
		return "", repository.DailyClosePoint{}, false
	}
	deadline := until.Format(seriesDateLayout)
	reached := func(bound float64, above bool, close float64) bool {
		if above {
			return close >= bound
		}
		return close <= bound
	}
	// For a hold call neither bound is directional, so nothing auto-resolves
	// early — it is judged at expiry like any drift question.
	targetAbove := c.Direction == model.DirectionBuy
	stopAbove := c.Direction == model.DirectionSell

	for _, p := range prices.points {
		if p.Date <= c.TradeDate || p.Date > deadline {
			continue
		}
		if c.Direction != model.DirectionHold {
			if c.TargetPrice != nil && *c.TargetPrice > 0 && reached(*c.TargetPrice, targetAbove, p.Close) {
				return model.StatusTargetHit, p, true
			}
			if c.StopPrice != nil && *c.StopPrice > 0 && reached(*c.StopPrice, stopAbove, p.Close) {
				return model.StatusStopHit, p, true
			}
		}
	}
	return "", repository.DailyClosePoint{}, false
}

// buildResearchReturn resolves one horizon bucket. A nil Return means the
// horizon has not produced a close after publication yet — the UI renders "—"
// and must never substitute 0.
func buildResearchReturn(
	label string,
	baseTradeDate string,
	basePrice float64,
	target time.Time,
	prices closeSeries,
	benchmarkID string,
	benchmark closeSeries,
) model.ResearchReturn {
	out := model.ResearchReturn{Horizon: label, BenchmarkID: benchmarkID}
	if basePrice <= 0 {
		return out
	}
	point, ok := resolveHorizonClose(prices, baseTradeDate, target)
	if !ok {
		return out
	}
	ret, ok := closePctChange(basePrice, point.Close)
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
	base, okBase := benchmark.onOrBefore(baseTradeDate)
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

// researchReturnAt builds the return/alpha pair for one resolved date.
func researchReturnAt(
	c model.ResearchCall, point repository.DailyClosePoint, benchmark closeSeries,
) (ret *float64, alpha *float64) {
	r, ok := closePctChange(c.BasePrice, point.Close)
	if !ok {
		return nil, nil
	}
	ret = &r
	if c.BenchmarkID == "" || benchmark.empty() {
		return ret, nil
	}
	base, okBase := benchmark.onOrBefore(c.TradeDate)
	end, okEnd := benchmark.onOrBefore(point.Date)
	if !okBase || !okEnd || end.Date <= base.Date {
		return ret, nil
	}
	benchRet, okRate := closePctChange(base.Close, end.Close)
	if !okRate {
		return ret, nil
	}
	a := r - benchRet
	return ret, &a
}

// ResearchJudgement is what the verdict batch decides for one call.
type ResearchJudgement struct {
	Status model.ResearchStatus
	Date   string
	Return *float64
	Alpha  *float64
	// Changed is false when the call is still open and nothing should be written.
	Changed bool
}

// judgeResearchCall resolves an open call against stored closes. It returns
// Changed=false while the call is genuinely still running, so the batch writes
// nothing rather than churning rows.
func judgeResearchCall(
	c model.ResearchCall, now time.Time, prices, benchmark closeSeries,
) ResearchJudgement {
	if c.Status != model.StatusOpen {
		return ResearchJudgement{}
	}
	dueAt := c.PublishedAt.AddDate(0, 0, c.HorizonDays)

	// A bound may have been hit before the horizon ran out. Search only up to
	// the earlier of "now" and the deadline so a late-arriving backfill cannot
	// resolve a call using a close from after it should have expired.
	boundLimit := dueAt
	if now.Before(boundLimit) {
		boundLimit = now
	}
	if status, point, ok := researchBoundHit(c, prices, boundLimit); ok {
		ret, alpha := researchReturnAt(c, point, benchmark)
		return ResearchJudgement{Status: status, Date: point.Date, Return: ret, Alpha: alpha, Changed: true}
	}

	if now.Before(dueAt) {
		return ResearchJudgement{}
	}

	point, ok := resolveHorizonClose(prices, c.TradeDate, dueAt)
	if !ok {
		// The horizon elapsed but we have no close to judge with. Record it as
		// unknown so it stops being polled, and never count it as a hit or miss.
		return ResearchJudgement{Status: model.StatusExpiredUnknown, Changed: true}
	}
	ret, alpha := researchReturnAt(c, point, benchmark)
	return ResearchJudgement{
		Status: researchVerdict(c.Direction, ret), Date: point.Date,
		Return: ret, Alpha: alpha, Changed: true,
	}
}

// buildResearchView assembles the read-time half of a call.
func buildResearchView(
	c model.ResearchCall,
	now time.Time,
	livePrice float64,
	prices, benchmark closeSeries,
	authorHitRate *float64,
) model.ResearchView {
	view := model.ResearchView{ResearchCall: c}
	view.Returns = make([]model.ResearchReturn, 0, len(researchHorizons))
	for _, label := range researchHorizons {
		view.Returns = append(view.Returns, buildResearchReturn(
			label, c.TradeDate, c.BasePrice, researchHorizonTargetDate(c.PublishedAt, label),
			prices, c.BenchmarkID, benchmark,
		))
	}

	view.DueAt = c.PublishedAt.AddDate(0, 0, c.HorizonDays)
	view.Due = !now.Before(view.DueAt)
	view.Progress = researchProgress(c.PublishedAt, view.DueAt, now)

	horizon := buildResearchReturn(
		"horizon", c.TradeDate, c.BasePrice, view.DueAt, prices, c.BenchmarkID, benchmark,
	)
	view.Horizon = &horizon

	view.Current = buildResearchCurrent(c, livePrice, prices)
	if view.Current != nil && view.Current.Price > 0 {
		if c.TargetPrice != nil && *c.TargetPrice > 0 {
			if pct, ok := closePctChange(view.Current.Price, *c.TargetPrice); ok {
				view.ToTarget = &pct
			}
		}
		if c.StopPrice != nil && *c.StopPrice > 0 {
			if pct, ok := closePctChange(view.Current.Price, *c.StopPrice); ok {
				view.ToStop = &pct
			}
		}
	}
	view.Expected = researchExpectedValue(c, authorHitRate)
	return view
}

// researchProgress is the elapsed fraction of the horizon, clamped to 0..1.
func researchProgress(from, to, now time.Time) float64 {
	total := to.Sub(from)
	if total <= 0 {
		return 1
	}
	elapsed := now.Sub(from)
	switch {
	case elapsed <= 0:
		return 0
	case elapsed >= total:
		return 1
	}
	return elapsed.Seconds() / total.Seconds()
}

func buildResearchCurrent(c model.ResearchCall, livePrice float64, prices closeSeries) *model.ResearchCurrent {
	cur := &model.ResearchCurrent{}
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
	if rate, ok := closePctChange(c.BasePrice, cur.Price); ok {
		cur.ChangeRate = &rate
	}
	return cur
}

// researchExpectedValue turns target/stop into a number twice — once with the
// author's stated conviction, once with the hit rate they have actually earned.
// Showing both is the point: a 90% conviction next to a 45% record is the
// single most informative thing on the page.
func researchExpectedValue(c model.ResearchCall, authorHitRate *float64) *model.ResearchExpectedValue {
	if c.BasePrice <= 0 || c.TargetPrice == nil || *c.TargetPrice <= 0 {
		return nil
	}
	up, ok := closePctChange(c.BasePrice, *c.TargetPrice)
	if !ok {
		return nil
	}
	out := &model.ResearchExpectedValue{UpsidePct: up}
	if c.StopPrice != nil && *c.StopPrice > 0 {
		if down, ok := closePctChange(c.BasePrice, *c.StopPrice); ok {
			out.DownsidePct = down
			if math.Abs(down) > 0 {
				rr := math.Abs(up) / math.Abs(down)
				out.RiskReward = &rr
			}
		}
	}

	ev := func(p float64) float64 {
		return out.UpsidePct*p + out.DownsidePct*(1-p)
	}
	if c.Conviction > 0 {
		v := ev(float64(c.Conviction) / 100)
		out.Stated = &v
	}
	if authorHitRate != nil {
		rate := *authorHitRate
		v := ev(rate / 100)
		out.Historical = &v
		out.HitRateUsed = &rate
	}
	return out
}

// summarizeResearch builds the author scorecard. Retracted calls are excluded
// from the hit rate but counted in the retract rate — hiding losers has to cost
// something visible, or the hit rate can be manufactured.
func summarizeResearch(calls []model.ResearchCall) model.ResearchSummary {
	out := model.ResearchSummary{Total: len(calls), MinSample: researchMinSample}
	symbols := map[string]struct{}{}
	var (
		retSum, alphaSum float64
		retN, alphaN     int
		brierSum         float64
		brierN           int
	)
	for _, c := range calls {
		// 분석 리포트(부동산·금융상품)는 판정이 없어 성적에 들지 않는다(2026-10-09 요청 9).
		if c.Status == model.StatusNote {
			out.Total--
			continue
		}
		symbols[c.Symbol] = struct{}{}
		switch {
		case c.Status == model.StatusRetracted:
			out.Retracted++
		case c.Status == model.StatusOpen:
			out.Open++
		}
		if !c.Status.Resolved() {
			continue
		}
		out.Resolved++
		hit := c.Status.Hit()
		if hit {
			out.Hits++
		}
		if c.Contrarian {
			out.ContrarianResolved++
			if hit {
				out.ContrarianHits++
			}
		}
		// Brier score: squared error between the stated probability and the
		// outcome. Lower is better, 0 is perfect.
		outcome := 0.0
		if hit {
			outcome = 1
		}
		p := float64(c.Conviction) / 100
		brierSum += (p - outcome) * (p - outcome)
		brierN++

		if c.VerdictReturn != nil {
			retSum += *c.VerdictReturn
			retN++
		}
		if c.VerdictAlpha != nil {
			alphaSum += *c.VerdictAlpha
			alphaN++
		}
	}
	out.Symbols = len(symbols)
	if out.Resolved > 0 {
		rate := float64(out.Hits) / float64(out.Resolved) * 100
		out.HitRate = &rate
	}
	if out.ContrarianResolved > 0 {
		rate := float64(out.ContrarianHits) / float64(out.ContrarianResolved) * 100
		out.ContrarianHitRate = &rate
	}
	if out.Total > 0 {
		rate := float64(out.Retracted) / float64(out.Total) * 100
		out.RetractRate = &rate
	}
	if retN > 0 {
		avg := retSum / float64(retN)
		out.AvgReturn = &avg
	}
	if alphaN > 0 {
		avg := alphaSum / float64(alphaN)
		out.AvgAlpha = &avg
	}
	if brierN > 0 {
		b := brierSum / float64(brierN)
		out.Calibration = &b
	}
	return out
}

// researchTargetOdds answers "has this symbol ever actually moved that far in
// this much time?" from stored daily closes, using every overlapping window.
//
// It never blocks a target. It only makes the claim's difficulty visible — and
// when history is too short to say anything, it says that instead of returning
// a small number that reads like a real probability.
func researchTargetOdds(
	points []repository.DailyClosePoint, basePrice, targetPrice float64, horizonDays int,
) *model.ResearchTargetOdds {
	if basePrice <= 0 || targetPrice <= 0 || horizonDays <= 0 {
		return nil
	}
	required, ok := closePctChange(basePrice, targetPrice)
	if !ok {
		return nil
	}
	out := &model.ResearchTargetOdds{HorizonDays: horizonDays, RequiredPct: required}
	if len(points) > 0 {
		out.FromDate = points[0].Date
		out.ToDate = points[len(points)-1].Date
	}

	// Trading days, not calendar days: the series is a trading-day series, so
	// the window has to be expressed in the same unit. ~252 sessions a year.
	window := int(math.Round(float64(horizonDays) * 252.0 / 365.0))
	if window < 1 {
		window = 1
	}
	if len(points) <= window {
		out.Insufficient = true
		return out
	}

	moves := make([]float64, 0, len(points)-window)
	for i := 0; i+window < len(points); i++ {
		from := points[i].Close
		to := points[i+window].Close
		if from <= 0 {
			continue
		}
		moves = append(moves, (to-from)/from*100)
	}
	out.Windows = len(moves)
	// Fewer than 30 overlapping windows is noise, not a distribution.
	if out.Windows < 30 {
		out.Insufficient = true
		return out
	}

	up := required >= 0
	for _, m := range moves {
		if (up && m >= required) || (!up && m <= required) {
			out.Reached++
		}
	}
	prob := float64(out.Reached) / float64(out.Windows) * 100
	out.Probability = &prob

	sorted := append([]float64(nil), moves...)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]
	out.MedianPct = &median
	best := sorted[len(sorted)-1]
	if !up {
		best = sorted[0]
	}
	out.BestPct = &best
	return out
}
