package model

import "time"

// FinancialHistoryPeriod is one standalone quarter (or annual, for markets
// without a clean quarterly source) in a multi-period financial history.
// Flow items (Revenue/OperatingIncome/NetIncome/EPSBasic) are standalone
// period values, never cumulative-within-fiscal-year. Balance-sheet items
// (TotalAssets/TotalLiabilities/TotalEquity/CurrentAssets/CurrentLiabilities)
// are point-in-time balances as of PeriodEnd.
type FinancialHistoryPeriod struct {
	FiscalYear  int        `json:"fiscalYear"`
	PeriodIndex int        `json:"periodIndex"` // 1..4
	PeriodEnd   *time.Time `json:"periodEnd,omitempty"`

	Revenue         *int64   `json:"revenue,omitempty"`
	OperatingIncome *int64   `json:"operatingIncome,omitempty"`
	NetIncome       *int64   `json:"netIncome,omitempty"`
	EPSBasic        *float64 `json:"epsBasic,omitempty"`

	TotalAssets        *int64 `json:"totalAssets,omitempty"`
	TotalLiabilities   *int64 `json:"totalLiabilities,omitempty"`
	TotalEquity        *int64 `json:"totalEquity,omitempty"`
	CurrentAssets      *int64 `json:"currentAssets,omitempty"`
	CurrentLiabilities *int64 `json:"currentLiabilities,omitempty"`

	OperatingMargin *float64 `json:"operatingMargin,omitempty"`
	NetProfitMargin *float64 `json:"netProfitMargin,omitempty"`
	DebtRatio       *float64 `json:"debtRatio,omitempty"`
	CurrentRatio    *float64 `json:"currentRatio,omitempty"`

	RevenueGrowthQoQ         *float64 `json:"revenueGrowthQoq,omitempty"`
	OperatingIncomeGrowthQoQ *float64 `json:"operatingIncomeGrowthQoq,omitempty"`
	NetIncomeGrowthQoQ       *float64 `json:"netIncomeGrowthQoq,omitempty"`
}

// FinancialHistoryView is the response for GET /stocks/{symbol}/financials/history.
// Periods are ordered oldest-first.
type FinancialHistoryView struct {
	Symbol    string                   `json:"symbol"`
	AssetType QuoteAssetType           `json:"assetType"`
	Periods   []FinancialHistoryPeriod `json:"periods"`
	// Note는 비어 있을 때의 이유를 말한다 — "아직 수집 전"과 "데이터 없음"을
	// 사용자가 구분할 수 있어야 한다.
	Note string `json:"note,omitempty"`
}
