package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

const financialHistoryMaxYears = 5

// FinancialHistoryService assembles the 재무제표 상세 모달's multi-period
// time series.
//
// 외부 API를 부르지 않는다. 예전에는 요청마다 DART(종목당 16호출) 또는 EDGAR를
// 직접 불렀다 — 국내 한 종목을 여는 데 DART 1초 간격으로 16초가 들었고, 그 부담이
// 사용자 수에 비례했다. 이제 worker-filings가 `financial_periods`를 채우고
// 여기서는 표만 읽는다.
type FinancialHistoryService struct {
	periods *repository.FinancialPeriodRepo
	sync    *repository.SyncStateRepo
	stocks  *repository.StockRepo
}

func NewFinancialHistoryService(
	periods *repository.FinancialPeriodRepo,
	syncState *repository.SyncStateRepo,
	stocks *repository.StockRepo,
) *FinancialHistoryService {
	return &FinancialHistoryService{periods: periods, sync: syncState, stocks: stocks}
}

func (s *FinancialHistoryService) Get(ctx context.Context, symbol string, years int) (*model.FinancialHistoryView, error) {
	if years <= 0 {
		years = 3
	}
	if years > financialHistoryMaxYears {
		years = financialHistoryMaxYears
	}

	stock, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return nil, fmt.Errorf("financial history: unknown symbol %s", symbol)
	}

	assetType := ResolveFundamentalsAssetType(stock)
	view := &model.FinancialHistoryView{Symbol: stock.Symbol, AssetType: assetType}

	// 상세 모달은 **분기값**을 본다 — 국내 DART가 보고하는 누적이 아니라, 배치가
	// 4분기에서 3분기 누적을 빼 만든 값이다(053 마이그레이션 주석 참조).
	rows, err := s.periods.ListForSymbol(ctx, stock.Symbol, FinancialBasisQuarter, years*4)
	if err != nil {
		return nil, err
	}
	periods := make([]model.FinancialHistoryPeriod, 0, len(rows))
	for _, row := range rows {
		periods = append(periods, row.Period)
	}
	view.Periods = periods
	if view.Periods == nil {
		view.Periods = []model.FinancialHistoryPeriod{}
	}
	// 성장률은 저장하지 않고 읽을 때 계산한다 — 직전 분기가 나중에 채워지면
	// 저장된 값이 틀린 채로 남는다.
	applyQoQGrowth(view.Periods)

	if len(view.Periods) == 0 {
		synced, err := s.sync.WasSynced(ctx, financialsSyncKind(assetType), stock.Symbol)
		if err != nil {
			return nil, err
		}
		if !synced {
			view.Note = referencePendingNote
		}
	}
	return view, nil
}

// financialsSyncKind maps the screen to the batch that fills it.
func financialsSyncKind(assetType model.QuoteAssetType) string {
	if assetType == model.AssetDomestic {
		return model.SyncKindFinancialsDART
	}
	return model.SyncKindFinancialsSEC
}

// referencePendingNote — 배치가 아직 이 종목에 오지 않았을 때의 안내.
// "데이터 없음"과 구분해야 한다: 사용자에게는 똑같이 빈 화면이지만, 하나는
// 사실이고 하나는 대기 상태다.
const referencePendingNote = "아직 수집되지 않은 종목입니다. 배치가 순차로 수집하며, 관심 종목에 추가하면 다음 실행에서 우선 수집됩니다."

func usPeriodIndex(fp string) int {
	switch strings.ToUpper(strings.TrimSpace(fp)) {
	case "Q1":
		return 1
	case "Q2":
		return 2
	case "Q3":
		return 3
	default:
		return 4 // FY / Q4
	}
}

// applyQoQGrowth fills growth fields by comparing each period to the one
// immediately before it in the (already chronologically ordered) slice —
// this is quarter-over-quarter, not year-over-year.
func applyQoQGrowth(periods []model.FinancialHistoryPeriod) {
	for i := 1; i < len(periods); i++ {
		prev, cur := periods[i-1], periods[i]
		if cur.Revenue != nil && prev.Revenue != nil {
			periods[i].RevenueGrowthQoQ = pctChange(float64(*cur.Revenue), float64(*prev.Revenue))
		}
		if cur.OperatingIncome != nil && prev.OperatingIncome != nil {
			periods[i].OperatingIncomeGrowthQoQ = pctChange(float64(*cur.OperatingIncome), float64(*prev.OperatingIncome))
		}
		if cur.NetIncome != nil && prev.NetIncome != nil {
			periods[i].NetIncomeGrowthQoQ = pctChange(float64(*cur.NetIncome), float64(*prev.NetIncome))
		}
	}
}
