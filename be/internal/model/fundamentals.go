package model

import (
	"strings"
	"time"
)

type FundamentalsDataSource string

const (
	FundamentalsSourceDART  FundamentalsDataSource = "dart"
	FundamentalsSourceEDGAR FundamentalsDataSource = "edgar"
)

type FundamentalsCalcStatus string

const (
	FundamentalsStatusOK      FundamentalsCalcStatus = "ok"
	FundamentalsStatusPartial FundamentalsCalcStatus = "partial"
	FundamentalsStatusFailed  FundamentalsCalcStatus = "failed"
	FundamentalsStatusPending FundamentalsCalcStatus = "pending"
)

// CoverageTier is the user-facing data quality grade (phase 1 UX).
type CoverageTier string

const (
	CoverageTierFull           CoverageTier = "full"
	CoverageTierBasic          CoverageTier = "basic"
	CoverageTierUnavailable    CoverageTier = "unavailable"
	CoverageTierNotApplicable  CoverageTier = "not_applicable"
)

// IsFundamentalsApplicable reports whether DART/EDGAR-style financial statements apply.
func IsFundamentalsApplicable(st Stock) bool {
	if st.AssetType == AssetETF {
		return false
	}
	sec := strings.ToUpper(strings.TrimSpace(st.SecurityType))
	if strings.Contains(sec, "ETF") || strings.Contains(sec, "ETN") {
		return false
	}
	return true
}

// ComputeCoverageTier maps stored fundamentals to a display tier.
func ComputeCoverageTier(f StockFundamentals, stock *Stock) CoverageTier {
	if stock != nil && !IsFundamentalsApplicable(*stock) {
		return CoverageTierNotApplicable
	}
	if f.CalcStatus == FundamentalsStatusOK {
		return CoverageTierFull
	}
	if f.Revenue != nil || f.NetIncome != nil || f.TotalEquity != nil || f.ROE != nil {
		return CoverageTierBasic
	}
	return CoverageTierUnavailable
}

// FinancialSnapshot holds normalized statement lines used to compute ratios.
type FinancialSnapshot struct {
	FiscalYear         int
	FiscalPeriod       string
	ReportCode         string
	PeriodEnd          *time.Time
	FiledAt            *time.Time
	Revenue            *int64
	OperatingIncome    *int64
	NetIncome          *int64
	TotalAssets        *int64
	TotalEquity        *int64
	TotalLiabilities   *int64
	CurrentAssets      *int64
	CurrentLiabilities *int64
	EPSBasic           *float64
	EPSDiluted         *float64
	SharesOutstanding  *int64
	// Currency — the currencies of the values used, comma-joined ("USD" or "TWD,USD")
	// when the source says so (EDGAR units). "" = unknown — treated as the listing currency.
	Currency string
}

// StockFundamentals is the persisted fundamentals row for a symbol.
// PER/PBR are computed at read time from live market price.
type StockFundamentals struct {
	Symbol string

	DataSource FundamentalsDataSource
	FiscalYear *int
	FiscalPeriod string
	ReportCode   string
	PeriodEnd    *time.Time
	FiledAt      *time.Time

	Revenue            *int64
	OperatingIncome    *int64
	NetIncome          *int64
	TotalAssets        *int64
	TotalEquity        *int64
	TotalLiabilities   *int64
	CurrentAssets      *int64
	CurrentLiabilities *int64
	EPSBasic           *float64
	EPSDiluted         *float64
	SharesOutstanding  *int64

	ROE              *float64
	OperatingMargin  *float64
	RevenueGrowthYoY *float64
	EPSGrowthYoY     *float64
	FwdPER           *float64
	CyPER            *float64

	DebtRatio       *float64
	ROA             *float64
	NetProfitMargin *float64
	CurrentRatio    *float64

	CalcStatus FundamentalsCalcStatus
	CalcError  string
	FetchedAt  *time.Time
	UpdatedAt  time.Time
}

// FundamentalsView adds live valuation metrics to stored fundamentals.
type FundamentalsView struct {
	StockFundamentals
	Price *float64 `json:"price,omitempty"`
	PER   *float64 `json:"per,omitempty"`
	PBR   *float64 `json:"pbr,omitempty"`
	PSR   *float64 `json:"psr,omitempty"`
	// Supplement — EDGAR가 비운 칸을 채운 보조 출처(Finnhub, 2026-09-28). 화면이 「보조」로 밝힌다.
	Supplement *FundamentalsSupplement `json:"supplement,omitempty"`
}

// FundamentalsSupplement says which fields came from the secondary source and when.
type FundamentalsSupplement struct {
	Source string    `json:"source"`
	AsOf   time.Time `json:"asOf"`
	Fields []string  `json:"fields"`
}
