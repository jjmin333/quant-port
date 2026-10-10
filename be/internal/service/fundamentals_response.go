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
	// 화면이 「FY2025」를 사람 말로 옮기려면 회계연도만으로는 부족하다 — 연간
	// 보고서인지 어느 분기인지("FY"/"1Q"…), 그 기간이 언제 끝났는지를 함께 줘야
	// 「올해 건지 작년 건지」를 사용자가 알 수 있다.
	FiscalPeriod string  `json:"fiscalPeriod,omitempty"`
	PeriodEnd    *string `json:"periodEnd,omitempty"`
	CoverageTier string  `json:"coverageTier"`
	CalcStatus   string  `json:"calcStatus,omitempty"`
	CalcError    string  `json:"calcError,omitempty"`
	FetchedAt    *string `json:"fetchedAt,omitempty"`

	Price *float64 `json:"price,omitempty"`
	PER   *float64 `json:"per,omitempty"`
	PBR   *float64 `json:"pbr,omitempty"`
	PSR   *float64 `json:"psr,omitempty"`
	// Supplement — EDGAR 빈 칸을 채운 보조 출처(Finnhub). 화면이 출처·날짜를 밝힌다.
	Supplement *model.FundamentalsSupplement `json:"supplement,omitempty"`

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
	// 보조(Finnhub)만 있는 종목(TSM처럼 20-F를 비운 경우)도 「데이터 없음」 카드로 가리지 않는다.
	if tier == model.CoverageTierUnavailable && view.Supplement != nil {
		tier = model.CoverageTierBasic
	}

	resp := FundamentalsResponse{
		Symbol:            f.Symbol,
		DataSource:        string(f.DataSource),
		FiscalYear:        f.FiscalYear,
		FiscalPeriod:      f.FiscalPeriod,
		// 결산일은 날짜만 쓴다 — 시각을 붙이면 화면이 시간대에 따라 하루 어긋난
		// 라벨("2025년 12월" vs "2026년 1월")을 그린다.
		PeriodEnd: formatDateOnly(f.PeriodEnd),
		CoverageTier:      string(tier),
		CalcStatus:        string(f.CalcStatus),
		CalcError:         f.CalcError,
		FetchedAt:         formatTimeRFC3339(f.FetchedAt),
		Price:             view.Price,
		PER:               view.PER,
		PBR:               view.PBR,
		PSR:               view.PSR,
		Supplement:        view.Supplement,
		ROE:               f.ROE,
		OperatingMargin:   f.OperatingMargin,
		RevenueGrowthYoY:  f.RevenueGrowthYoY,
		EPSGrowthYoY:      f.EPSGrowthYoY,
		FwdPER:            f.FwdPER,
		CyPER:             f.CyPER,
		DebtRatio:         f.DebtRatio,
		ROA:               f.ROA,
		NetProfitMargin:   f.NetProfitMargin,
		CurrentRatio:      f.CurrentRatio,
		Revenue:           f.Revenue,
		NetIncome:         f.NetIncome,
		TotalEquity:       f.TotalEquity,
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

// formatDateOnly는 "2025-12-31" 꼴로 낸다. 결산일에는 시각이 없으므로 RFC3339로
// 내보내면 화면이 UTC로 되읽으며 하루를 잃는다(국내 12월 31일 → 12월 30일).
func formatDateOnly(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}
