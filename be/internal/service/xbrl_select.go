package service

import (
	"math"
	"sort"
	"strings"
	"time"
)

// XBRLFact is one reported value from SEC companyfacts JSON.
type XBRLFact struct {
	End   string
	Start string
	Val   float64
	FY    int
	FP    string
	Form  string
	Filed string
}

var revenueConceptFallbacks = []string{
	"RevenueFromContractWithCustomerExcludingAssessedTax",
	"Revenues",
	"SalesRevenueNet",
	"SalesRevenueGoodsNet",
	"Revenue",
}

var sharesConceptFallbacks = []string{
	"CommonStockSharesOutstanding",
	"EntityCommonStockSharesOutstanding",
	"CommonSharesOutstanding",
	"CommonStockSharesIssued",
}

var operatingIncomeConceptFallbacks = []string{
	"OperatingIncomeLoss",
	"OperatingProfitLoss",
}

var netIncomeConceptFallbacks = []string{
	"NetIncomeLoss",
	"ProfitLoss",
	"NetIncomeLossAvailableToCommonStockholdersBasic",
}

var equityConceptFallbacks = []string{
	"StockholdersEquity",
	"StockholdersEquityIncludingPortionAttributableToNoncontrollingInterest",
}

var assetsConceptFallbacks = []string{
	"Assets",
}

var liabilitiesConceptFallbacks = []string{
	"Liabilities",
}

var currentAssetsConceptFallbacks = []string{
	"AssetsCurrent",
}

var currentLiabilitiesConceptFallbacks = []string{
	"LiabilitiesCurrent",
}

var epsBasicConceptFallbacks = []string{
	"EarningsPerShareBasic",
	"BasicEarningsLossPerShare",
}

var epsDilutedConceptFallbacks = []string{
	"EarningsPerShareDiluted",
	"DilutedEarningsLossPerShare",
}

func pickConceptFacts(facts map[string]map[string][]XBRLFact, tags []string) []XBRLFact {
	for _, tag := range tags {
		if items := lookupConceptFacts(facts, tag); len(items) > 0 {
			return items
		}
	}
	return nil
}

func lookupConceptFacts(facts map[string]map[string][]XBRLFact, tag string) []XBRLFact {
	for _, taxonomy := range []string{"us-gaap", "ifrs-full", "dei"} {
		if byTag, ok := facts[taxonomy]; ok {
			if items, ok := byTag[tag]; ok && len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

// PickSharesOutstanding returns the latest shares fact using SEC tag fallbacks.
func PickSharesOutstanding(facts map[string]map[string][]XBRLFact) *int64 {
	items := pickConceptFacts(facts, sharesConceptFallbacks)
	if len(items) == 0 {
		return nil
	}
	latest := pickLatestByEnd(items)
	if latest == nil {
		return nil
	}
	v := int64(math.Round(latest.Val))
	return &v
}

func pickLatestAnnualPair(items []XBRLFact) (current, prior *XBRLFact) {
	annual := filterAnnualFacts(items)
	if len(annual) == 0 {
		return nil, nil
	}
	sort.Slice(annual, func(i, j int) bool {
		if annual[i].FY != annual[j].FY {
			return annual[i].FY > annual[j].FY
		}
		return annual[i].End > annual[j].End
	})
	current = cloneFact(annual[0])
	if len(annual) > 1 {
		prior = cloneFact(annual[1])
	}
	return current, prior
}

func filterAnnualFacts(items []XBRLFact) []XBRLFact {
	var out []XBRLFact
	for _, item := range items {
		if isAnnualFact(item) {
			out = append(out, item)
		}
	}
	return out
}

func isAnnualFact(item XBRLFact) bool {
	fp := strings.ToUpper(strings.TrimSpace(item.FP))
	form := strings.ToUpper(strings.TrimSpace(item.Form))
	if fp == "FY" {
		return true
	}
	switch form {
	case "10-K", "20-F", "40-F":
		if fp == "" || fp == "FY" {
			return true
		}
	}
	if item.Start != "" && item.End != "" {
		start, err1 := time.Parse("2006-01-02", item.Start)
		end, err2 := time.Parse("2006-01-02", item.End)
		if err1 == nil && err2 == nil {
			days := end.Sub(start).Hours() / 24
			if days >= 330 {
				return true
			}
		}
	}
	return false
}

// xbrlPeriodKey identifies one reported fiscal period across concepts.
// FY is display-only (see recentReportPeriods) — matching uses FP+End only,
// since SEC's own `fy` tag is not reliable enough to match on (see below).
type xbrlPeriodKey struct {
	FY  int
	FP  string
	End string
}

// recentReportPeriods returns up to limit distinct (FP,End) periods from
// 10-Q/10-K/20-F/40-F filings, most recent End first. Used to drive the
// quarterly 실적 tab and financial-history modal for US symbols — only real
// SEC-reported periods, no derived/synthesized quarters.
//
// Deliberately dedupes on (FP,End) and NOT `fy`: the same physical quarter
// is often reported under two different `fy` tags — once in its own filing,
// again as a prior-year comparative column in the next year's filing — with
// identical End/values but a different fy. Keying on fy too would silently
// produce duplicate periods (observed live for AAPL: 2024-09-28 appearing as
// both "FY2024 Q4" and "FY2025 Q4"). FY on the returned key is display-only,
// taken from whichever occurrence is encountered first.
func recentReportPeriods(items []XBRLFact, limit int) []xbrlPeriodKey {
	type dedupKey struct{ FP, End string }
	seen := make(map[dedupKey]bool)
	var keys []xbrlPeriodKey
	for _, item := range items {
		if !isReportPeriod(item) {
			continue
		}
		fp := strings.ToUpper(strings.TrimSpace(item.FP))
		dk := dedupKey{FP: fp, End: item.End}
		if seen[dk] {
			continue
		}
		seen[dk] = true
		keys = append(keys, xbrlPeriodKey{FY: item.FY, FP: fp, End: item.End})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].End > keys[j].End })
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys
}

// isReportPeriod accepts only standalone-quarter or full-year durations.
// SEC XBRL 10-Qs commonly tag BOTH a ~3-month standalone fact and a
// cumulative YTD fact (e.g. "six months ended") under the same FY/FP/End —
// without this duration check, Q2/Q3 would pick up 6-/9-month cumulative
// totals and look like implausible quarter-over-quarter revenue spikes.
func isReportPeriod(item XBRLFact) bool {
	fp := strings.ToUpper(strings.TrimSpace(item.FP))
	form := strings.ToUpper(strings.TrimSpace(item.Form))
	if item.End == "" || fp == "" {
		return false
	}
	switch form {
	case "10-Q", "10-K", "20-F", "40-F":
	default:
		return false
	}
	switch fp {
	case "Q1", "Q2", "Q3":
		return isStandaloneQuarterDuration(item)
	case "Q4", "FY":
		return true
	}
	return false
}

// isStandaloneQuarterDuration reports whether item covers roughly one
// quarter (60-100 days), as opposed to a cumulative year-to-date span.
func isStandaloneQuarterDuration(item XBRLFact) bool {
	if item.Start == "" || item.End == "" {
		return false
	}
	start, err1 := time.Parse("2006-01-02", item.Start)
	end, err2 := time.Parse("2006-01-02", item.End)
	if err1 != nil || err2 != nil {
		return false
	}
	days := end.Sub(start).Hours() / 24
	return days >= 60 && days <= 100
}

// pickFactForPeriod finds the fact matching key within items (a different
// concept's fact list than the one recentReportPeriods was built from). For
// Q1-Q3 it additionally requires a standalone-quarter duration, so a
// concept that only has a cumulative YTD fact for that End date is treated
// as missing rather than silently substituting the wrong span.
func pickFactForPeriod(items []XBRLFact, key xbrlPeriodKey) *XBRLFact {
	for _, item := range items {
		// Match on End+FP only — not FY, which the same physical period can
		// carry inconsistently across filings (see recentReportPeriods).
		if item.End != key.End || !strings.EqualFold(strings.TrimSpace(item.FP), key.FP) {
			continue
		}
		switch key.FP {
		case "Q1", "Q2", "Q3":
			if !isStandaloneQuarterDuration(item) {
				continue
			}
		}
		return cloneFact(item)
	}
	return nil
}

// pickInstantFactForPeriod finds a point-in-time (balance sheet) fact whose
// End date matches key.End. Unlike pickFactForPeriod, no duration check is
// applied — instant facts (Assets, StockholdersEquity, ...) have no
// standalone-vs-cumulative distinction to make; the balance simply is what
// it is as of that date.
func pickInstantFactForPeriod(items []XBRLFact, key xbrlPeriodKey) *XBRLFact {
	for _, item := range items {
		if item.End == key.End {
			return cloneFact(item)
		}
	}
	return nil
}

func pickLatestByEnd(items []XBRLFact) *XBRLFact {
	if len(items) == 0 {
		return nil
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].End != items[j].End {
			return items[i].End > items[j].End
		}
		return items[i].Filed > items[j].Filed
	})
	return cloneFact(items[0])
}

func cloneFact(item XBRLFact) *XBRLFact {
	x := item
	return &x
}

func factInt64(f *XBRLFact) *int64 {
	if f == nil {
		return nil
	}
	v := int64(math.Round(f.Val))
	return &v
}

func factFloat64(f *XBRLFact) *float64 {
	if f == nil {
		return nil
	}
	v := f.Val
	return &v
}

func parseFactDate(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil
	}
	return &t
}

func pctChange(current, prior float64) *float64 {
	if prior == 0 {
		return nil
	}
	v := (current - prior) / math.Abs(prior) * 100
	return &v
}

func pctRatio(numerator, denominator float64) *float64 {
	if denominator == 0 {
		return nil
	}
	v := numerator / denominator * 100
	return &v
}

func ratioValue(numerator, denominator float64) *float64 {
	if denominator == 0 {
		return nil
	}
	v := numerator / denominator
	return &v
}

func averageTwo(current, prior *int64) *float64 {
	if current == nil {
		return nil
	}
	if prior == nil || *prior == 0 {
		v := float64(*current)
		return &v
	}
	v := (float64(*current) + float64(*prior)) / 2
	return &v
}

func int64Value(v *int64) float64 {
	if v == nil {
		return 0
	}
	return float64(*v)
}

func float64Value(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}
