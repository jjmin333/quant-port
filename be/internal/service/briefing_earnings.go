package service

import (
	"context"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const (
	briefingKREarningsMax           = 6
	briefingUSEarningsMax           = 10
	briefingEarningsLookbackDays    = 7
	briefingUSEarningsLookaheadDays = 7
	// |서프라이즈|가 이 값을 넘으면 FnGuide/단위 깨진 값으로 보고 그 축은 0점.
	briefingSurpriseBugPct = 200
)

// briefingEarningDTO is one compact row in the LLM user JSON. Keys stay short
// on purpose — this is the budget that used to go to ten truncated summaries.
type briefingEarningDTO struct {
	Sym  string   `json:"sym"`
	Name string   `json:"n,omitempty"`
	D    string   `json:"d"`
	Op   string   `json:"op,omitempty"`
	EPS  string   `json:"eps,omitempty"`
	Sess string   `json:"sess,omitempty"`
	S    *float64 `json:"s,omitempty"`
	RevS *float64 `json:"revS,omitempty"`
	Rxn  *float64 `json:"rxn,omitempty"`
	Imp  int      `json:"imp"`
}

func (s *NewsBriefingService) briefingEarnings(ctx context.Context, market string) []briefingEarningDTO {
	if s == nil || s.earnings == nil {
		return nil
	}
	now := time.Now().In(seoulLocation())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from := today.AddDate(0, 0, -briefingEarningsLookbackDays)
	to := today
	limit := briefingKREarningsMax
	queryFrom := from
	if market == "us" {
		to = today.AddDate(0, 0, briefingUSEarningsLookaheadDays)
		limit = briefingUSEarningsMax
		queryFrom = model.ExpandUSQueryFrom(from, string(model.AssetUS))
	}

	rows, err := s.earnings.events.ListRange(ctx, queryFrom, to, market, nil)
	if err != nil || len(rows) == 0 {
		return nil
	}
	rows = model.CollapseCalendarEvents(rows)
	filtered := make([]model.EarningsEvent, 0, len(rows))
	for _, ev := range rows {
		switch ev.DateStatus {
		case model.EarningsDateConfirmed:
			filtered = append(filtered, ev)
		case model.EarningsDateScheduled:
			if market == "us" {
				filtered = append(filtered, ev)
			}
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	s.earnings.AttachMarketCaps(ctx, filtered)
	s.earnings.AttachReactions(ctx, filtered)
	model.ApplyOverseasDisplayDates(filtered)
	if market == "us" {
		filtered = model.FilterEventsByCalendarDate(filtered, from, to)
	}

	type scored struct {
		ev  model.EarningsEvent
		imp int
		cap float64
	}
	ranked := make([]scored, 0, len(filtered))
	for _, ev := range filtered {
		imp, cap := briefingEarningsScore(ev, today)
		ranked = append(ranked, scored{ev: ev, imp: imp, cap: cap})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].imp != ranked[j].imp {
			return ranked[i].imp > ranked[j].imp
		}
		if ranked[i].cap != ranked[j].cap {
			return ranked[i].cap > ranked[j].cap
		}
		return ranked[i].ev.Symbol < ranked[j].ev.Symbol
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]briefingEarningDTO, 0, len(ranked))
	for _, row := range ranked {
		out = append(out, compactBriefingEarning(row.ev, row.imp))
	}
	return out
}

// briefingEarningsScore returns importance 0–100 and the raw market cap used
// for tie-breaks. Formula: min(100, cap + surprise + reaction + soon).
func briefingEarningsScore(ev model.EarningsEvent, today time.Time) (int, float64) {
	capVal := 0.0
	if ev.MarketCap != nil {
		capVal = *ev.MarketCap
	}
	total := briefingCapScore(ev.Market, capVal) +
		briefingSurpriseScore(ev) +
		briefingReactionScore(ev) +
		briefingSoonScore(ev, today)
	if total > 100 {
		total = 100
	}
	if total < 0 {
		total = 0
	}
	return total, capVal
}

func briefingCapScore(market string, cap float64) int {
	if cap <= 0 {
		return 10
	}
	if market == string(model.AssetUS) {
		switch {
		case cap >= 200e9: // $200B = 2,000억$
			return 80
		case cap >= 20e9:
			return 50
		case cap >= 5e9:
			return 25
		default:
			return 10
		}
	}
	switch {
	case cap >= 100e12: // 100조원
		return 80
	case cap >= 20e12:
		return 50
	case cap >= 5e12:
		return 25
	default:
		return 10
	}
}

func briefingSurpriseScore(ev model.EarningsEvent) int {
	if ev.Surprise == nil || ev.Surprise.Pct == nil {
		return 0
	}
	pct := math.Abs(*ev.Surprise.Pct)
	if pct > briefingSurpriseBugPct {
		return 0
	}
	// WMT EPS +8% → 16. 상한 40.
	v := int(math.Round(pct * 2))
	if v > 40 {
		return 40
	}
	return v
}

func briefingReactionScore(ev model.EarningsEvent) int {
	if ev.Reaction == nil {
		return 0
	}
	// 폭등·폭락 둘 다 가산. |−9%| → 14.
	v := int(math.Round(math.Abs(ev.Reaction.Pct) * 1.5))
	if v > 30 {
		return 30
	}
	return v
}

func briefingSoonScore(ev model.EarningsEvent, today time.Time) int {
	if ev.DateStatus != model.EarningsDateScheduled {
		return 0
	}
	day, err := time.ParseInLocation("2006-01-02", ev.Date, today.Location())
	if err != nil {
		return 0
	}
	days := int(day.Sub(today).Hours() / 24)
	switch {
	case days >= 0 && days <= 2:
		return 25
	case days >= 3 && days <= 7:
		return 10
	default:
		return 0
	}
}

func compactBriefingEarning(ev model.EarningsEvent, imp int) briefingEarningDTO {
	row := briefingEarningDTO{
		Sym:  ev.Symbol,
		D:    shortBriefingDate(ev.Date),
		Sess: ev.Session,
		Imp:  imp,
	}
	if ev.Market != string(model.AssetUS) {
		row.Name = ev.Name
		row.Op = formatMetricPair(ev.OpIncome, true)
	} else if ev.DateStatus == model.EarningsDateConfirmed {
		row.EPS = formatMetricPair(ev.EPS, false)
		if ev.RevenueSurprise != nil && ev.RevenueSurprise.Pct != nil {
			row.RevS = round1Ptr(*ev.RevenueSurprise.Pct)
		}
	}
	if ev.Surprise != nil && ev.Surprise.Pct != nil && math.Abs(*ev.Surprise.Pct) <= briefingSurpriseBugPct {
		row.S = round1Ptr(*ev.Surprise.Pct)
	}
	if ev.Reaction != nil {
		row.Rxn = round1Ptr(ev.Reaction.Pct)
	}
	return row
}

func shortBriefingDate(iso string) string {
	if len(iso) >= 10 {
		return iso[5:7] + "-" + iso[8:10]
	}
	return iso
}

func formatMetricPair(m model.EarningsMetric, krw bool) string {
	if m.Actual == nil && m.Estimate == nil {
		return ""
	}
	format := formatUSDBrief
	if krw {
		format = formatKRWBrief
	}
	switch {
	case m.Actual != nil && m.Estimate != nil:
		return format(*m.Actual) + "/" + format(*m.Estimate)
	case m.Actual != nil:
		return format(*m.Actual)
	default:
		return format(*m.Estimate)
	}
}

func formatKRWBrief(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	switch {
	case v >= 1e12:
		return sign + trimFloat(v/1e12) + "조"
	case v >= 1e8:
		return sign + trimFloat(v/1e8) + "억"
	case v >= 1e4:
		return sign + trimFloat(v/1e4) + "만"
	default:
		return sign + trimFloat(v)
	}
}

func formatUSDBrief(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func trimFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if len(s) >= 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}

func round1Ptr(v float64) *float64 {
	r := math.Round(v*10) / 10
	return &r
}
