package service

import (
	"math"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

type StoredFundamentals struct {
	ROE              *float64
	OperatingMargin  *float64
	RevenueGrowthYoY *float64
	EPSGrowthYoY     *float64
	DebtRatio        *float64
	ROA              *float64
	NetProfitMargin  *float64
	CurrentRatio     *float64
	Missing          []string
}

type ValuationMetrics struct {
	PER *float64
	PBR *float64
}

func enrichDerivedEPS(snap *model.FinancialSnapshot) {
	if snap == nil {
		return
	}
	if snap.EPSDiluted != nil || snap.EPSBasic != nil {
		return
	}
	if snap.NetIncome == nil || snap.SharesOutstanding == nil || *snap.SharesOutstanding == 0 {
		return
	}
	eps := float64(*snap.NetIncome) / float64(*snap.SharesOutstanding)
	snap.EPSBasic = &eps
	snap.EPSDiluted = &eps
}

func mergeSnapshot(base model.FinancialSnapshot, shares *int64) model.FinancialSnapshot {
	if shares != nil {
		base.SharesOutstanding = shares
	}
	return base
}

func ComputeStoredFundamentals(current, prior model.FinancialSnapshot, sharesOverride *int64) StoredFundamentals {
	current = mergeSnapshot(current, sharesOverride)
	prior = mergeSnapshot(prior, sharesOverride)
	enrichDerivedEPS(&current)
	enrichDerivedEPS(&prior)

	out := StoredFundamentals{}

	eps := current.EPSDiluted
	if eps == nil {
		eps = current.EPSBasic
	}
	priorEPS := prior.EPSDiluted
	if priorEPS == nil {
		priorEPS = prior.EPSBasic
	}

	if avgEquity := averageTwo(current.TotalEquity, prior.TotalEquity); avgEquity != nil && current.NetIncome != nil {
		out.ROE = pctRatio(float64(*current.NetIncome), *avgEquity)
	} else {
		out.Missing = append(out.Missing, "roe")
	}

	if current.Revenue != nil && current.OperatingIncome != nil && *current.Revenue != 0 {
		out.OperatingMargin = pctRatio(float64(*current.OperatingIncome), float64(*current.Revenue))
	} else {
		out.Missing = append(out.Missing, "operating_margin")
	}

	if current.Revenue != nil && prior.Revenue != nil {
		out.RevenueGrowthYoY = pctChange(float64(*current.Revenue), float64(*prior.Revenue))
	} else {
		out.Missing = append(out.Missing, "revenue_growth_yoy")
	}

	if eps != nil && priorEPS != nil {
		out.EPSGrowthYoY = pctChange(*eps, *priorEPS)
	} else {
		out.Missing = append(out.Missing, "eps_growth_yoy")
	}

	if current.TotalEquity != nil && current.TotalLiabilities != nil && *current.TotalEquity != 0 {
		out.DebtRatio = pctRatio(float64(*current.TotalLiabilities), float64(*current.TotalEquity))
	} else {
		out.Missing = append(out.Missing, "debt_ratio")
	}

	if avgAssets := averageTwo(current.TotalAssets, prior.TotalAssets); avgAssets != nil && current.NetIncome != nil {
		out.ROA = pctRatio(float64(*current.NetIncome), *avgAssets)
	} else {
		out.Missing = append(out.Missing, "roa")
	}

	if current.Revenue != nil && current.NetIncome != nil && *current.Revenue != 0 {
		out.NetProfitMargin = pctRatio(float64(*current.NetIncome), float64(*current.Revenue))
	} else {
		out.Missing = append(out.Missing, "net_profit_margin")
	}

	if current.CurrentAssets != nil && current.CurrentLiabilities != nil && *current.CurrentLiabilities != 0 {
		out.CurrentRatio = ratioValue(float64(*current.CurrentAssets), float64(*current.CurrentLiabilities))
	} else {
		out.Missing = append(out.Missing, "current_ratio")
	}

	return out
}

// ApplyPeriodRatios fills a FinancialHistoryPeriod's flow/balance-sheet
// fields and per-period margins/ratios from its own snapshot — independent
// per period, unlike ComputeStoredFundamentals which compares current vs.
// prior year. Shared by the KR (DART) and US (EDGAR) history builders.
func ApplyPeriodRatios(p *model.FinancialHistoryPeriod, snap model.FinancialSnapshot) {
	p.Revenue = snap.Revenue
	p.OperatingIncome = snap.OperatingIncome
	p.NetIncome = snap.NetIncome
	p.EPSBasic = snap.EPSBasic
	p.TotalAssets = snap.TotalAssets
	p.TotalLiabilities = snap.TotalLiabilities
	p.TotalEquity = snap.TotalEquity
	p.CurrentAssets = snap.CurrentAssets
	p.CurrentLiabilities = snap.CurrentLiabilities

	if snap.Revenue != nil && snap.OperatingIncome != nil && *snap.Revenue != 0 {
		p.OperatingMargin = pctRatio(float64(*snap.OperatingIncome), float64(*snap.Revenue))
	}
	if snap.Revenue != nil && snap.NetIncome != nil && *snap.Revenue != 0 {
		p.NetProfitMargin = pctRatio(float64(*snap.NetIncome), float64(*snap.Revenue))
	}
	if snap.TotalEquity != nil && snap.TotalLiabilities != nil && *snap.TotalEquity != 0 {
		p.DebtRatio = pctRatio(float64(*snap.TotalLiabilities), float64(*snap.TotalEquity))
	}
	if snap.CurrentAssets != nil && snap.CurrentLiabilities != nil && *snap.CurrentLiabilities != 0 {
		p.CurrentRatio = ratioValue(float64(*snap.CurrentAssets), float64(*snap.CurrentLiabilities))
	}
}

func ComputeValuationMetrics(price *float64, current model.FinancialSnapshot, shares *int64) ValuationMetrics {
	out := ValuationMetrics{}
	if price == nil {
		return out
	}

	current = mergeSnapshot(current, shares)
	enrichDerivedEPS(&current)

	eps := current.EPSDiluted
	if eps == nil {
		eps = current.EPSBasic
	}
	if eps != nil && *eps != 0 {
		out.PER = ratioValue(*price, *eps)
	}

	sharesVal := shares
	if sharesVal == nil {
		sharesVal = current.SharesOutstanding
	}
	if bvps := bookValuePerShare(current.TotalEquity, sharesVal); bvps != nil && *bvps != 0 {
		out.PBR = ratioValue(*price, *bvps)
	}
	return out
}

func calcStatus(computed StoredFundamentals) model.FundamentalsCalcStatus {
	core := []string{"roe", "operating_margin", "revenue_growth_yoy", "eps_growth_yoy"}
	missingCore := 0
	for _, key := range core {
		for _, item := range computed.Missing {
			if item == key {
				missingCore++
				break
			}
		}
	}
	switch {
	case missingCore == 0:
		return model.FundamentalsStatusOK
	case missingCore < len(core):
		return model.FundamentalsStatusPartial
	default:
		return model.FundamentalsStatusFailed
	}
}

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}

func bookValuePerShare(equity, shares *int64) *float64 {
	if equity == nil || shares == nil || *shares == 0 {
		return nil
	}
	v := float64(*equity) / float64(*shares)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
