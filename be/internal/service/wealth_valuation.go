package service

import (
	"math"
	"sort"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
순자산 평가 엔진 — 순수 함수만 (2026-10-01, docs/ASSET_EXPANSION_RESEARCH.md §6·§8).

부동산 비교사례 평가는 PoC(래미안대치팰리스·헬리오시티·마포래미안푸르지오)에서 배운 두 규칙을 따른다:
  - 직거래·해제 거래는 뺀다 — 가족 간 직거래(시세의 60%)가 중앙값을 흔들었다.
  - 최근 3개월(없으면 6개월) 비교가 5건 이상이면 **그 창만** 쓰고 지수 보정을 하지 않는다. 지수 보정은 거래가 모자랄 때
    12개월 창의 「틈」만 메운다 — 구 지수가 오른 만큼 오래된 거래를 올리면 그 단지의 실제 최근 거래보다 높아졌다.

모든 금액은 원.
*/

// ReComp — 평가에 쓴 비교 거래 한 건(화면이 근거로 보여 준다).
type ReComp struct {
	Date     string  `json:"date"`
	Area     float64 `json:"area"`
	Floor    *int    `json:"floor,omitempty"`
	Price    float64 `json:"price"`
	Adjusted float64 `json:"adjusted"`
}

// ReEstimate — 부동산 추정 결과.
type ReEstimate struct {
	Value          float64  `json:"value"`
	Low            float64  `json:"low"`
	High           float64  `json:"high"`
	Confidence     string   `json:"confidence"`
	Method         string   `json:"method"` // comparable · region_unit
	N              int      `json:"n"`
	WindowMonths   int      `json:"windowMonths"`
	IndexAdjusted  bool     `json:"indexAdjusted"`
	LastTrade      *ReComp  `json:"lastTrade,omitempty"`
	LastTradeDays  int      `json:"lastTradeDays"`
	Recent3Avg     *float64 `json:"recent3Avg,omitempty"`
	Recent3N       int      `json:"recent3N"`
	SameAreaMedian *float64 `json:"sameAreaMedian,omitempty"`
	ExcludedDirect int      `json:"excludedDirect"`
	ExcludedCancel int      `json:"excludedCancelled"`
	Comps          []ReComp `json:"comps"`
	UnitPerM2      *float64 `json:"unitPerM2,omitempty"`
	// 같은 건물·면적 전세 보증금 중앙값(정보용 — 추정가에 섞지 않는다).
	Jeonse *jeonseInfo `json:"jeonse,omitempty"`
}

// IndexAt returns the regional price index for a date (ok=false면 없음).
type IndexAt func(t time.Time) (float64, bool)

func areaTolerance(area float64) float64 { return math.Max(3, area*0.04) }

func parseDay(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func daysBetween(a, b time.Time) int { return int(b.Sub(a).Hours() / 24) }

// EstimateComparable values a unit from same-building sale trades. false면 비교 거래 없음.
func EstimateComparable(trades []repository.ReTrade, area float64, today time.Time, idx IndexAt) (ReEstimate, bool) {
	out := ReEstimate{Method: "comparable"}
	type cand struct {
		t    repository.ReTrade
		date time.Time
	}
	var all []cand
	tol := areaTolerance(area)
	for _, t := range trades {
		if t.DealType != "sale" || t.Price == nil || *t.Price <= 0 || math.Abs(t.Area-area) > tol {
			continue
		}
		if t.Cancelled {
			out.ExcludedCancel++
			continue
		}
		if t.Direct {
			out.ExcludedDirect++
			continue
		}
		d := parseDay(t.DealDate)
		if daysBetween(d, today) > 366 {
			continue
		}
		all = append(all, cand{t, d})
	}
	if len(all) == 0 {
		return out, false
	}
	sort.Slice(all, func(i, j int) bool { return all[i].date.After(all[j].date) })

	// 창은 짧은 것부터: 3개월에 5건이면 3개월, 아니면 6개월에 5건, 아니면 12개월 + 지수 보정.
	// 거래가 많은 단지에서 6개월 중앙값은 오르는 장에서 뒤처졌다(헬리오시티 84㎡: 6개월 28.1억 vs 최근 3개월 평균 29.7억).
	within := func(days int) []cand {
		var out []cand
		for _, c := range all {
			if daysBetween(c.date, today) <= days {
				out = append(out, c)
			}
		}
		return out
	}
	use, adjust := all, true
	out.WindowMonths = 12
	if r := within(92); len(r) >= 5 {
		use, adjust = r, false
		out.WindowMonths = 3
	} else if r := within(183); len(r) >= 5 {
		use, adjust = r, false
		out.WindowMonths = 6
	}
	nowIdx, haveNow := 0.0, false
	if idx != nil {
		nowIdx, haveNow = idx(today)
	}
	var adjusted, raw []float64
	for _, c := range use {
		p := float64(*c.t.Price)
		a := p
		if c.t.Area > 0 {
			a = p / c.t.Area * area // 면적 차이는 ㎡ 단가로 맞춘다
		}
		if adjust && haveNow {
			if then, ok := idx(c.date); ok && then > 0 {
				a = a * nowIdx / then
				out.IndexAdjusted = true
			}
		}
		a = math.Round(a)
		adjusted = append(adjusted, a)
		raw = append(raw, p)
		out.Comps = append(out.Comps, ReComp{Date: c.t.DealDate, Area: c.t.Area, Floor: c.t.Floor, Price: p, Adjusted: a})
	}
	out.N = len(adjusted)
	out.Value = median(adjusted)
	out.Low, out.High = spread(adjusted)
	m := median(raw)
	out.SameAreaMedian = &m
	last := out.Comps[0]
	out.LastTrade = &last
	out.LastTradeDays = daysBetween(parseDay(last.Date), today)
	var sum float64
	for _, c := range all {
		if daysBetween(c.date, today) <= 92 {
			sum += float64(*c.t.Price)
			out.Recent3N++
		}
	}
	if out.Recent3N > 0 {
		avg := math.Round(sum / float64(out.Recent3N))
		out.Recent3Avg = &avg
	}
	out.Confidence = confidenceOf(out.N, out.LastTradeDays, out.Low, out.High, out.Value)
	if len(out.Comps) > 30 {
		out.Comps = out.Comps[:30]
	}
	return out, true
}

func confidenceOf(n, lastDays int, low, high, value float64) string {
	width := 1.0
	if value > 0 {
		width = (high - low) / value
	}
	switch {
	case n >= 5 && lastDays <= 90 && width <= 0.08:
		return "high"
	case n >= 2 && lastDays <= 180:
		return "medium"
	}
	return "low"
}

// EstimateRegionUnit values by the median ㎡ price of similar-size sales nearby (단지 거래가 없을 때). 신뢰도는 늘 낮음.
func EstimateRegionUnit(trades []repository.ReTrade, area float64, today time.Time) (ReEstimate, bool) {
	out := ReEstimate{Method: "region_unit", WindowMonths: 12, Confidence: "low"}
	var units, similar []float64
	for _, t := range trades {
		if t.DealType != "sale" || t.Price == nil || *t.Price <= 0 || t.Area <= 0 || t.Cancelled || t.Direct {
			continue
		}
		if daysBetween(parseDay(t.DealDate), today) > 366 {
			continue
		}
		u := float64(*t.Price) / t.Area
		units = append(units, u)
		if area > 0 && math.Abs(t.Area-area) <= area*0.3 {
			similar = append(similar, u)
		}
	}
	if len(similar) >= 5 {
		units = similar
	}
	if len(units) == 0 || area <= 0 {
		return out, false
	}
	unit := median(units)
	out.UnitPerM2 = &unit
	out.N = len(units)
	out.Value = math.Round(unit * area)
	lo, hi := spread(units)
	out.Low, out.High = math.Round(lo*area), math.Round(hi*area)
	return out, true
}

// JeonseEstimate — 같은 건물·면적 전세(월세 0) 보증금 중앙값. 6개월에 3건 미만이면 12개월.
func JeonseEstimate(rents []repository.ReTrade, area float64, today time.Time) (median_ float64, n int) {
	tol := areaTolerance(area)
	pick := func(days int) []float64 {
		var out []float64
		for _, r := range rents {
			if r.DealType != "rent" || r.Deposit == nil || *r.Deposit <= 0 || (r.Monthly != nil && *r.Monthly > 0) {
				continue
			}
			if math.Abs(r.Area-area) > tol || daysBetween(parseDay(r.DealDate), today) > days {
				continue
			}
			out = append(out, float64(*r.Deposit))
		}
		return out
	}
	v := pick(183)
	if len(v) < 3 {
		v = pick(366)
	}
	if len(v) == 0 {
		return 0, 0
	}
	return median(v), len(v)
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	m := len(s) / 2
	if len(s)%2 == 1 {
		return s[m]
	}
	return math.Round((s[m-1] + s[m]) / 2)
}

// spread — 4건 이상이면 25~75%, 그보다 적으면 최소~최대.
func spread(v []float64) (float64, float64) {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s) < 4 {
		return s[0], s[len(s)-1]
	}
	q := func(p float64) float64 { return s[int(math.Floor(float64(len(s)-1)*p))] }
	return q(0.25), q(0.75)
}

// ── 고정수익 ──────────────────────────────────────────────────────────────────

// DepositValue — 정기예금 원금 + 오늘까지 경과이자(세후). compound면 월복리.
func DepositValue(principal, ratePct float64, start, maturity, today time.Time, compound bool, taxPct float64) (value, interest float64) {
	end := today
	if !maturity.IsZero() && maturity.Before(today) {
		end = maturity
	}
	if end.Before(start) || principal <= 0 {
		return principal, 0
	}
	days := end.Sub(start).Hours() / 24
	r := ratePct / 100
	if compound {
		interest = principal * (math.Pow(1+r/12, days/30.4375) - 1)
	} else {
		interest = principal * r * days / 365
	}
	interest *= 1 - taxPct/100
	return math.Round(principal + interest), math.Round(interest)
}

// SavingValue — 정액 적금(매월 같은 날 납입, 단리). 낸 원금 + 회차별 경과이자(세후).
func SavingValue(monthly, ratePct float64, start time.Time, months int, today time.Time, taxPct float64) (value, paid, interest float64) {
	if monthly <= 0 || months <= 0 || today.Before(start) {
		return 0, 0, 0
	}
	maturity := start.AddDate(0, months, 0)
	end := today
	if maturity.Before(today) {
		end = maturity
	}
	r := ratePct / 100
	for i := 0; i < months; i++ {
		pay := start.AddDate(0, i, 0)
		if pay.After(end) {
			break
		}
		paid += monthly
		interest += monthly * r * end.Sub(pay).Hours() / 24 / 365
	}
	interest *= 1 - taxPct/100
	return math.Round(paid + interest), paid, math.Round(interest)
}

// BondValue — 남은 현금흐름(쿠폰 + 액면)을 yieldPct로 할인한 값(경과이자 포함, dirty).
// freq는 연 이자 횟수(0이면 만기일시 할인채).
func BondValue(face, couponPct float64, freq int, maturity, today time.Time, yieldPct float64) float64 {
	if face <= 0 || !maturity.After(today) {
		if face > 0 && !maturity.After(today) {
			return face
		}
		return 0
	}
	y := yieldPct / 100
	disc := func(t time.Time) float64 {
		years := t.Sub(today).Hours() / 24 / 365
		if freq <= 0 {
			return math.Pow(1+y, -years)
		}
		return math.Pow(1+y/float64(freq), -years*float64(freq))
	}
	pv := face * disc(maturity)
	if freq > 0 && couponPct > 0 {
		step := 12 / freq
		cpn := face * couponPct / 100 / float64(freq)
		for d := maturity; d.After(today); d = d.AddDate(0, -step, 0) {
			pv += cpn * disc(d)
		}
	}
	return math.Round(pv)
}

// CurveYield interpolates a yield curve (만기 년 → 금리 %) at years. 점이 없으면 false.
func CurveYield(curve map[float64]float64, years float64) (float64, bool) {
	if len(curve) == 0 {
		return 0, false
	}
	keys := make([]float64, 0, len(curve))
	for k := range curve {
		keys = append(keys, k)
	}
	sort.Float64s(keys)
	if years <= keys[0] {
		return curve[keys[0]], true
	}
	for i := 1; i < len(keys); i++ {
		if years <= keys[i] {
			a, b := keys[i-1], keys[i]
			w := (years - a) / (b - a)
			return curve[a]*(1-w) + curve[b]*w, true
		}
	}
	return curve[keys[len(keys)-1]], true
}

// GoldValue — 그램 × 국제 금 현물(달러/트로이온스) × 원/달러.
func GoldValue(grams, xauUSD, usdKRW float64) float64 {
	return math.Round(grams / 31.1034768 * xauUSD * usdKRW)
}

// ── 부채 ──────────────────────────────────────────────────────────────────────

// ScheduledLoan — 약정대로 갚았을 때 오늘의 잔액과 월 상환액(이자 포함, 이번 달 기준).
func ScheduledLoan(principal, ratePct float64, repayment string, start, maturity, today time.Time) (balance, payment float64, ok bool) {
	if principal <= 0 || start.IsZero() || !maturity.After(start) {
		return 0, 0, false
	}
	n := monthsBetween(start, maturity)
	if n <= 0 {
		return 0, 0, false
	}
	k := monthsBetween(start, today) // 지난 회차 수
	if k < 0 {
		k = 0
	}
	if k > n {
		k = n
	}
	r := ratePct / 100 / 12
	switch repayment {
	case "equal_payment":
		if r == 0 {
			payment = principal / float64(n)
			balance = principal - payment*float64(k)
		} else {
			payment = principal * r * math.Pow(1+r, float64(n)) / (math.Pow(1+r, float64(n)) - 1)
			balance = principal*math.Pow(1+r, float64(k)) - payment*(math.Pow(1+r, float64(k))-1)/r
		}
	case "equal_principal":
		per := principal / float64(n)
		balance = principal - per*float64(k)
		payment = per + balance*r
	case "bullet":
		balance = principal
		if k >= n {
			balance = 0
		}
		payment = principal * r
	default:
		return 0, 0, false
	}
	return math.Max(0, math.Round(balance)), math.Round(payment), true
}

func monthsBetween(a, b time.Time) int {
	m := (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
	if b.Day() < a.Day() {
		m--
	}
	return m
}
