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
