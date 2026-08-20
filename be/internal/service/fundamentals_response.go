package service

import (
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

type FundamentalsResponse struct {
	Symbol       string  `json:"symbol"`
	AssetType    string  `json:"assetType,omitempty"`
	ProductLabel string  `json:"productLabel,omitempty"`
	Message      string  `json:"message,omitempty"`
	DataSource   string  `json:"dataSource,omitempty"`
	FiscalYear   *int    `json:"fiscalYear,omitempty"`
	CoverageTier string  `json:"coverageTier"`
	CalcStatus   string  `json:"calcStatus,omitempty"`
	CalcError    string  `json:"calcError,omitempty"`
	FetchedAt    *string `json:"fetchedAt,omitempty"`

	Price *float64 `json:"price,omitempty"`
	PER   *float64 `json:"per,omitempty"`
	PBR   *float64 `json:"pbr,omitempty"`

	ROE              *float64 `json:"roe,omitempty"`
	OperatingMargin  *float64 `json:"operatingMargin,omitempty"`
	RevenueGrowthYoY *float64 `json:"revenueGrowthYoy,omitempty"`
	EPSGrowthYoY     *float64 `json:"epsGrowthYoy,omitempty"`
	FwdPER           *float64 `json:"fwdPer,omitempty"`
	CyPER            *float64 `json:"cyPer,omitempty"`
	DebtRatio        *float64 `json:"debtRatio,omitempty"`
	ROA              *float64 `json:"roa,omitempty"`
	NetProfitMargin  *float64 `json:"netProfitMargin,omitempty"`
	CurrentRatio     *float64 `json:"currentRatio,omitempty"`

	Revenue     *int64 `json:"revenue,omitempty"`
	NetIncome   *int64 `json:"netIncome,omitempty"`
	TotalEquity *int64 `json:"totalEquity,omitempty"`

	SharesOutstanding *int64   `json:"sharesOutstanding,omitempty"`
	EPSBasic          *float64 `json:"epsBasic,omitempty"`
	EPSDiluted        *float64 `json:"epsDiluted,omitempty"`
}

func BuildFundamentalsResponse(view *model.FundamentalsView, stock *model.Stock) FundamentalsResponse {
	f := view.StockFundamentals
	tier := model.ComputeCoverageTier(f, stock)

	resp := FundamentalsResponse{
		Symbol:           f.Symbol,
		DataSource:       string(f.DataSource),
		FiscalYear:       f.FiscalYear,
		CoverageTier:     string(tier),
		CalcStatus:       string(f.CalcStatus),
		CalcError:        f.CalcError,
		FetchedAt:        formatTimeRFC3339(f.FetchedAt),
		Price:            view.Price,
		PER:              view.PER,
		PBR:              view.PBR,
		ROE:              f.ROE,
		OperatingMargin:  f.OperatingMargin,
		RevenueGrowthYoY: f.RevenueGrowthYoY,
		EPSGrowthYoY:     f.EPSGrowthYoY,
		FwdPER:           f.FwdPER,
		CyPER:            f.CyPER,
		DebtRatio:        f.DebtRatio,
		ROA:              f.ROA,
		NetProfitMargin:  f.NetProfitMargin,
		CurrentRatio:     f.CurrentRatio,
		Revenue:          f.Revenue,
		NetIncome:        f.NetIncome,
		TotalEquity:      f.TotalEquity,
		SharesOutstanding: f.SharesOutstanding,
		EPSBasic:          f.EPSBasic,
		EPSDiluted:        f.EPSDiluted,
	}

	if stock != nil {
		resp.AssetType = string(stock.AssetType)
		resp.ProductLabel = productLabel(*stock)
		if tier == model.CoverageTierNotApplicable {
			resp.Message = fundamentalsNotApplicableMessage(*stock)
		}
	}
	if resp.Symbol == "" && stock != nil {
		resp.Symbol = stock.Symbol
	}
	return resp
}

func formatTimeRFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
