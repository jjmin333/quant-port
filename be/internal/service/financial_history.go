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
	// candles·fundamentals는 분기별 PER·PBR을 실제로 계산하는 데만 쓴다
	// (financial_history_valuation.go). 없으면 그 두 값만 비고 나머지는 그대로다.
	candles      *repository.CandleBarRepo
	fundamentals *repository.FundamentalsRepo
}

func NewFinancialHistoryService(
	periods *repository.FinancialPeriodRepo,
	syncState *repository.SyncStateRepo,
	stocks *repository.StockRepo,
) *FinancialHistoryService {
	return &FinancialHistoryService{periods: periods, sync: syncState, stocks: stocks}
}

// SetValuationSources를 부르면 분기별 PER·PBR이 응답에 실린다.
func (s *FinancialHistoryService) SetValuationSources(
	candles *repository.CandleBarRepo,
	fundamentals *repository.FundamentalsRepo,
) {
	if s == nil {
		return
	}
	s.candles = candles
	s.fundamentals = fundamentals
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
	// 마진·비율도 마찬가지로 읽을 때 계산한다 — `financial_periods`에는 금액만
	// 저장한다(적재 때 계산한 값을 담을 칼럼이 없다). 예전에는 이 단계가 없어
	// 「이익률」 차트와 상세 모달의 비율 선이 통째로 빈 채 나갔다.
	view.Periods = dropMislabeledPeriods(view.Periods)
	applyPeriodRatios(view.Periods)
	if assetType != model.AssetDomestic {
		view.Currency, view.CurrencyMixed = s.reportingCurrency(ctx, stock.Symbol)
	}
	// 공시 통화가 가격 통화와 다르면 분기말 종가 ÷ EPS는 뜻이 없다(TSM 2024 PER 1.42였다).
	if view.Currency == "" {
		s.applyValuation(ctx, stock.Symbol, view.Periods, s.lookupShares(ctx, stock.Symbol))
	}

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

// reportingCurrency — 지표 로테이션이 남긴 통화 표식(`currency_twd_usd`)에서 현지 통화를 읽는다.
func (s *FinancialHistoryService) reportingCurrency(ctx context.Context, symbol string) (string, bool) {
	if s.fundamentals == nil {
		return "", false
	}
	row, err := s.fundamentals.GetBySymbol(ctx, symbol)
	if err != nil || row == nil {
		return "", false
	}
	return foreignCurrencyFromCalcError(row.CalcError, "USD")
}

// foreignCurrencyFromCalcError — "roe,currency_twd_usd" → ("TWD", true). 표식이 없으면 ("", false).
func foreignCurrencyFromCalcError(calcError, listing string) (string, bool) {
	for _, tok := range strings.Split(calcError, ",") {
		tok = strings.TrimSpace(tok)
		if !strings.HasPrefix(tok, "currency_") {
			continue
		}
		codes := strings.Split(strings.TrimPrefix(tok, "currency_"), "_")
		for _, c := range codes {
			if c = strings.ToUpper(c); c != "" && c != listing {
				return c, len(codes) > 1
			}
		}
	}
	return "", false
}

/*
dropMislabeledPeriods — 회계연도 표식이 결산일보다 2년 넘게 어긋난 행을 뺀다. EDGAR 사실의 `fy`는
**그 값을 실은 보고서**의 연도라, 2017년 보고서에 실린 2015년 비교 수치가 「FY2017」로 들어왔다(TSM).
1년 차이는 정상이다(애플 FY2024 1분기 = 2023-12-30, 소매업 1월 결산).
*/
func dropMislabeledPeriods(in []model.FinancialHistoryPeriod) []model.FinancialHistoryPeriod {
	out := in[:0]
	for _, p := range in {
		if p.PeriodEnd != nil && p.FiscalYear != 0 {
			d := p.FiscalYear - p.PeriodEnd.Year()
			if d >= 2 || d <= -2 {
				continue
			}
		}
		out = append(out, p)
	}
	return out
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

// applyPeriodRatios fills each period's own margins and balance-sheet ratios
// from that same period's amounts.
//
// 적재 경로(ApplyPeriodRatios)도 같은 값을 계산하지만 `financial_periods`에는
// 담을 칼럼이 없어 버려진다 — 그래서 읽을 때 다시 만든다. 분모가 0이거나
// 한쪽이 비면 아무것도 넣지 않는다: 0%는 「비율이 0」이라는 거짓말이고,
// 화면은 빈 값을 선의 끊김으로 그려야 한다.
func applyPeriodRatios(periods []model.FinancialHistoryPeriod) {
	for i := range periods {
		p := &periods[i]
		if p.Revenue != nil && *p.Revenue != 0 {
			if p.OperatingIncome != nil {
				p.OperatingMargin = pctRatio(float64(*p.OperatingIncome), float64(*p.Revenue))
			}
			if p.NetIncome != nil {
				p.NetProfitMargin = pctRatio(float64(*p.NetIncome), float64(*p.Revenue))
			}
		}
		if p.TotalEquity != nil && *p.TotalEquity != 0 && p.TotalLiabilities != nil {
			p.DebtRatio = pctRatio(float64(*p.TotalLiabilities), float64(*p.TotalEquity))
		}
		if p.CurrentLiabilities != nil && *p.CurrentLiabilities != 0 && p.CurrentAssets != nil {
			p.CurrentRatio = ratioValue(float64(*p.CurrentAssets), float64(*p.CurrentLiabilities))
		}
	}
}

// lookupShares는 저장된 현재 발행주식수다 — 분기별 주식수 되찾기(순이익÷EPS)가
// 실패했을 때만 쓰는 최후 수단이다. 그 사이 증자가 있었다면 과거 PBR이
// 왜곡되므로 우선순위가 낮다.
func (s *FinancialHistoryService) lookupShares(ctx context.Context, symbol string) *int64 {
	if s == nil || s.fundamentals == nil {
		return nil
	}
	row, err := s.fundamentals.GetBySymbol(ctx, symbol)
	if err != nil || row == nil {
		return nil
	}
	return row.SharesOutstanding
}
