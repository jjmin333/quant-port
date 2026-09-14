package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

type FundamentalsService struct {
	dart        *DartClient
	edgar       *EdgarClient
	toss        *toss.Client
	stocks      *repository.StockRepo
	repo        *repository.FundamentalsRepo
	marketStore *MarketDataStore
	// 등급제 로테이션의 커서와 DART 예산. 둘 다 없으면 회전 갱신은 아무것도 하지
	// 않는다 — 예전처럼 「전 종목 통째로」로 되돌아가는 길을 남기지 않는다.
	sync   *repository.SyncStateRepo
	budget *apibudget.Manager
}

func NewFundamentalsService(
	dart *DartClient,
	edgar *EdgarClient,
	tossClient *toss.Client,
	stocks *repository.StockRepo,
	repo *repository.FundamentalsRepo,
) *FundamentalsService {
	return &FundamentalsService{
		dart:   dart,
		edgar:  edgar,
		toss:   tossClient,
		stocks: stocks,
		repo:   repo,
	}
}

func (s *FundamentalsService) SetMarketStore(store *MarketDataStore) {
	s.marketStore = store
}

// SetRotation wires the graded-rotation cursor and the API budget. Without
// both, RefreshRotation is a no-op — see its header for why that is the safe
// default rather than falling back to a full sweep.
func (s *FundamentalsService) SetRotation(sync *repository.SyncStateRepo, budget *apibudget.Manager) {
	s.sync = sync
	s.budget = budget
}

// fundamentalsAssetType resolves DART vs EDGAR source; guards against stale us mis-tags on KOSPI rows.
func (s *FundamentalsService) fundamentalsAssetType(stock *model.Stock) model.QuoteAssetType {
	return ResolveFundamentalsAssetType(stock)
}

// ResolveFundamentalsAssetType resolves DART vs EDGAR source; guards against
// stale us mis-tags on KOSPI rows. Shared by FundamentalsService and
// EarningsTabService — both need the same domestic/US split.
func ResolveFundamentalsAssetType(stock *model.Stock) model.QuoteAssetType {
	if stock == nil {
		return model.AssetUS
	}
	if at := assetTypeFromMarket(stock.Market, stock.SecurityType); at == model.AssetDomestic {
		return model.AssetDomestic
	}
	if stock.Currency == model.CurrencyKRW || isDomesticKRSymbol(stock.Symbol) {
		return model.AssetDomestic
	}
	return stock.AssetType
}

func (s *FundamentalsService) RefreshSymbol(ctx context.Context, symbol string) (*model.StockFundamentals, error) {
	stock, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return nil, fmt.Errorf("fundamentals: unknown symbol %s", symbol)
	}

	shares := s.resolveShares(ctx, stock, nil)

	var current, prior model.FinancialSnapshot
	var source model.FundamentalsDataSource
	var fetchErr error

	switch s.fundamentalsAssetType(stock) {
	case model.AssetDomestic:
		source = model.FundamentalsSourceDART
		current, prior, fetchErr = s.fetchDartSnapshots(ctx, stock.Symbol)
		if fetchErr == nil {
			if edgarCurrent, edgarPrior, edgarShares, ok := s.tryFresherEdgarSnapshot(ctx, stock.Symbol, current); ok {
				current, prior = edgarCurrent, edgarPrior
				source = model.FundamentalsSourceEDGAR
				shares = s.resolveShares(ctx, stock, edgarShares)
			}
		}
	default:
		source = model.FundamentalsSourceEDGAR
		var edgarShares *int64
		current, prior, edgarShares, fetchErr = s.edgar.FetchFinancialSnapshots(ctx, stock.Symbol)
		shares = s.resolveShares(ctx, stock, edgarShares)
	}
	if fetchErr != nil {
		// Keep last good row when refresh fails (bulk repair must not wipe DART ok data).
		if existing, _ := s.repo.GetBySymbol(ctx, stock.Symbol); existing != nil &&
			existing.CalcStatus == model.FundamentalsStatusOK {
			return existing, nil
		}
		row := model.StockFundamentals{
			Symbol:     stock.Symbol,
			DataSource: source,
			CalcStatus: model.FundamentalsStatusFailed,
			CalcError:  fetchErr.Error(),
			FetchedAt:  nowPtr(),
		}
		if err := s.repo.Upsert(ctx, row); err != nil {
			return nil, err
		}
		return &row, nil
	}

	current = mergeSnapshot(current, shares)
	computed := ComputeStoredFundamentals(current, prior, shares)

	row := model.StockFundamentals{
		Symbol:             stock.Symbol,
		DataSource:         source,
		FiscalYear:         intPtr(current.FiscalYear),
		FiscalPeriod:       current.FiscalPeriod,
		ReportCode:         current.ReportCode,
		PeriodEnd:          current.PeriodEnd,
		FiledAt:            current.FiledAt,
		Revenue:            current.Revenue,
		OperatingIncome:    current.OperatingIncome,
		NetIncome:          current.NetIncome,
		TotalAssets:        current.TotalAssets,
		TotalEquity:        current.TotalEquity,
		TotalLiabilities:   current.TotalLiabilities,
		CurrentAssets:      current.CurrentAssets,
		CurrentLiabilities: current.CurrentLiabilities,
		EPSBasic:           current.EPSBasic,
		EPSDiluted:         current.EPSDiluted,
		SharesOutstanding:  shares,
		ROE:                computed.ROE,
		OperatingMargin:    computed.OperatingMargin,
		RevenueGrowthYoY:   computed.RevenueGrowthYoY,
		EPSGrowthYoY:       computed.EPSGrowthYoY,
		DebtRatio:          computed.DebtRatio,
		ROA:                computed.ROA,
		NetProfitMargin:    computed.NetProfitMargin,
		CurrentRatio:       computed.CurrentRatio,
		CalcStatus:         calcStatus(computed),
		FetchedAt:          nowPtr(),
	}
	if len(computed.Missing) > 0 {
		row.CalcError = strings.Join(computed.Missing, ",")
	}

	if err := s.repo.Upsert(ctx, row); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *FundamentalsService) GetBySymbol(ctx context.Context, symbol string) (*model.StockFundamentals, error) {
	return s.repo.GetBySymbol(ctx, symbol)
}

func (s *FundamentalsService) LookupStock(ctx context.Context, symbol string) (*model.Stock, error) {
	return s.resolveStock(ctx, symbol)
}

func (s *FundamentalsService) GetWithValuation(ctx context.Context, symbol string) (*model.FundamentalsView, error) {
	stock, err := s.resolveStock(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return nil, nil
	}

	if !model.IsFundamentalsApplicable(*stock) {
		return &model.FundamentalsView{
			StockFundamentals: model.StockFundamentals{
				Symbol: stock.Symbol,
			},
		}, nil
	}

	stored, err := s.repo.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return &model.FundamentalsView{
			StockFundamentals: model.StockFundamentals{
				Symbol: stock.Symbol,
			},
		}, nil
	}

	price := s.fetchPrice(ctx, symbol)
	view := &model.FundamentalsView{
		StockFundamentals: *stored,
		Price:             price,
	}
	if price == nil {
		return view, nil
	}

	snap := model.FinancialSnapshot{
		TotalEquity:       stored.TotalEquity,
		EPSBasic:          stored.EPSBasic,
		EPSDiluted:        stored.EPSDiluted,
		SharesOutstanding: stored.SharesOutstanding,
		NetIncome:         stored.NetIncome,
	}
	val := ComputeValuationMetrics(price, snap, stored.SharesOutstanding)
	view.PER = val.PER
	view.PBR = val.PBR
	return view, nil
}

// krxToEdgarTicker lists domestic (KRX) symbols confirmed — via SEC's own
// company_tickers.json — to also file 20-F with the SEC under a US ticker.
// Unsponsored ADRs (e.g. Samsung Electronics, SK Hynix) have no SEC filing
// obligation and are correctly absent here; symbols not in this map always
// use DART only, unchanged from before.
var krxToEdgarTicker = map[string]string{
	"017670": "SKM", // SK Telecom
	"105560": "KB",  // KB Financial Group
	"055550": "SHG", // Shinhan Financial Group
	"316140": "WF",  // Woori Financial Group
	"005490": "PKX", // POSCO Holdings
	"015760": "KEP", // Korea Electric Power (KEPCO)
	"034220": "LPL", // LG Display
}

// tryFresherEdgarSnapshot best-effort checks whether a dual-listed domestic
// symbol's EDGAR filing is more recent than what DART already returned.
// Returns ok=false (keep DART) on any missing mapping, lookup/fetch failure,
// or when EDGAR is not strictly newer — 20-F is annual-only, so in practice
// DART's quarterly cadence usually wins; this only matters right after a
// fresh 20-F filing outruns DART's latest annual report.
func (s *FundamentalsService) tryFresherEdgarSnapshot(
	ctx context.Context,
	symbol string,
	dartCurrent model.FinancialSnapshot,
) (current, prior model.FinancialSnapshot, shares *int64, ok bool) {
	if s.edgar == nil {
		return current, prior, nil, false
	}
	ticker, mapped := krxToEdgarTicker[symbol]
	if !mapped {
		return current, prior, nil, false
	}
	edgarCurrent, edgarPrior, edgarShares, err := s.edgar.FetchFinancialSnapshots(ctx, ticker)
	if err != nil {
		return current, prior, nil, false
	}
	if edgarCurrent.PeriodEnd == nil || dartCurrent.PeriodEnd == nil {
		return current, prior, nil, false
	}
	if !edgarCurrent.PeriodEnd.After(*dartCurrent.PeriodEnd) {
		return current, prior, nil, false // DART wins ties and when EDGAR is older
	}
	return edgarCurrent, edgarPrior, edgarShares, true
}

func (s *FundamentalsService) fetchDartSnapshots(ctx context.Context, symbol string) (current, prior model.FinancialSnapshot, err error) {
	current, prior, err = s.dart.FindLatestAnnualSnapshots(ctx, symbol)
	if err == nil {
		return current, prior, nil
	}
	if alt, ok := commonStockCandidate(symbol); ok {
		cur, pri, altErr := s.dart.FindLatestAnnualSnapshots(ctx, alt)
		if altErr == nil {
			return cur, pri, nil
		}
	}
	return current, prior, err
}

func commonStockCandidate(symbol string) (string, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if len(symbol) != 6 {
		return "", false
	}
	last := rune(symbol[5])
	if last == '0' {
		return "", false
	}
	if unicode.IsDigit(last) || unicode.IsLetter(last) {
		return symbol[:5] + "0", true
	}
	return "", false
}

func (s *FundamentalsService) resolveShares(ctx context.Context, stock *model.Stock, edgarShares *int64) *int64 {
	if edgarShares != nil && *edgarShares > 0 {
		return edgarShares
	}
	if stock.SharesOutstanding != nil && *stock.SharesOutstanding > 0 {
		return stock.SharesOutstanding
	}
	if s.toss == nil || !s.toss.Enabled() {
		return nil
	}
	info, err := s.toss.GetStock(ctx, stock.Symbol)
	if err != nil || info == nil {
		return nil
	}
	if n := parseDecimalInt(info.SharesOutstanding); n > 0 {
		return &n
	}
	return nil
}

func (s *FundamentalsService) fetchPrice(ctx context.Context, symbol string) *float64 {
	if s.marketStore != nil {
		if v, ok := s.marketStore.EnsurePrice(ctx, symbol); ok {
			return &v
		}
	}
	return nil
}

const fundamentalsRefreshPause = 1100 * time.Millisecond // DART apibudget MinInterval 1s

/*
지표 갱신을 **등급제 로테이션**으로 돌린다 (2026-09-11 사용자 요청 6).

# 무엇이 터졌나

2026-09-10, DART 일일 한도 18,000이 **19:29에 이미 비어** 있었다 — 그날 예정된 DART
잡(19:30 공시 · 20:00 내부자 · 22:00 재무) 셋이 **한 종목도 못 부르고** 전부 실패했다.
범인은 배치가 아니라 이 함수의 예전 모습이었다: `RefreshAllStored`가 `stock_fundamentals`
**전 종목을 한 번에** 돌았다(실측 로그 「refreshed fundamentals for 12037 symbols」,
03:56~11:04 7시간). 국내 2,800종목 × 실측 6.4호출 ≈ 18,000 — 하루치 전부다.

그 호출이 특히 나빴던 이유는 세 가지다.

 1. **보이지 않았다.** 원장(`batch_job_runs`)에 없고 `BATCH_SCHEDULE.md`에도 없어
    관리자 화면에서는 아무 일도 안 일어난 것처럼 보였다.
 2. **예산 개념이 없었다.** 같은 저장소의 `financials_backfill`은 5,000콜을 예약해
    두고 그만큼 남기는데(그 상수 머리글에 2026-09-02의 같은 사고가 적혀 있다),
    이 경로만 한도를 통째로 쓸 수 있었다.
 3. **시각이 api 기동 시점에 묶여 있었다.** 24시간 주기가 부팅 시각을 따라가므로
    어느 시간대에든 떨어질 수 있었고, 그날은 저녁 DART 잡 바로 앞이었다.

# 무엇으로 바꿨나

이 저장소에는 이미 「누구를 다음에 받을까」의 정답이 하나 있다 — `SyncStateRepo`다.
그 머리글이 직접 말한다: 「배치마다 이 로직을 복제하지 않는 이유: SelectTargets의
ORDER BY가 **등급제의 전부**다.」 종목 단위 적재 셋(내부자 · 분기 재무 · 경고)이
모두 그것을 쓰는데 **지표 갱신만 빠져 있었다.** 이제 같은 줄에 선다:

  - 대상은 `SelectTargets`가 고른다 — 관심·보유가 앞서고, 그다음은 오래 안 받은 순.
  - 한 회차의 상한이 있다(`limit`). 전 종목을 한 번에 도는 길은 없어졌다.
  - 성공도 실패도 `Mark`로 기록한다. 실패한 종목이 큐 뒤로 가야 하나가 배치를
    영원히 막지 않는다(그 함수 머리글의 규칙).
  - **예산을 먼저 본다.** 국내는 DART 잔여에서 예약분을 뺀 만큼만 돈다.

# 왜 sync/budget이 없으면 아무것도 안 하나

예전 코드로 되돌아가는 길을 남기지 않기 위해서다. 「배선이 안 됐으면 전 종목이라도
돌자」는 폴백은 정확히 사고 당일의 동작이다.
*/

/*
fundamentalsDARTPerSymbol — 국내 한 종목 갱신의 DART 호출 수(실측).

`FindLatestAnnualSnapshots`는 최근 5개 연도를 차례로 시도하고 연결(CFS)이 비면
별도(OFS)로 한 번 더 부르므로 한 종목이 1~12호출 사이다. 2026-09-10 실측은
2,800종목에 18,000호출 = **6.4**였다. 예산 계산에는 그 평균을 올려 잡은 7을 쓴다 —
낮게 잡으면 가드가 통과시킨 회차가 한도를 넘는다. 그것이 바로 같은 사고에서
`financials_backfill`의 예전 값 4가 1,151종목을 통과시킨 방식이었다.
*/
const fundamentalsDARTPerSymbol = 7

/*
fundamentalsDARTReserve — 저녁 DART 잡들에게 남겨 두는 몫.

실측(2026-09-09 정상일): 하루 DART 사용량 7,895호출 = 내부자 국내 1,500(종목당 1)
+ 재무 국내 400(종목당 16 = 6,400) + 공시 스윕 몇 건. 9,000은 그 위에 재시도와
관리자 수동 실행 한 번의 여유를 둔 값이다.
*/
const fundamentalsDARTReserve = 9000

/*
fundamentalsStaleAfter — 관심·보유 종목을 우선순위 0으로 올리는 나이.

지표는 **분기에 한 번** 바뀌는 값이라(연간 보고서 기준) 잦은 갱신에 값이 없다.
7일은 분기 경계를 며칠 안에 따라가면서 한 바퀴에 드는 비용을 낮게 유지하는 값이다.
*/
const fundamentalsStaleAfter = 7 * 24 * time.Hour

// FundamentalsRotationResult is what one rotation pass did, for the log line.
type FundamentalsRotationResult struct {
	Attempted int
	Updated   int
	Failed    int
	Stopped   string
}

// RefreshRotation refreshes one graded-rotation slice of a market's
// fundamentals. See the block comment above for why this replaced the
// all-symbols sweep.
func (s *FundamentalsService) RefreshRotation(
	ctx context.Context,
	market model.QuoteAssetType,
	limit int,
) (FundamentalsRotationResult, error) {
	out := FundamentalsRotationResult{}
	if s == nil || s.repo == nil || s.sync == nil || s.budget == nil {
		out.Stopped = "rotation not wired (sync/budget)"
		return out, nil
	}
	if limit <= 0 {
		return out, nil
	}

	kind := model.SyncKindFundamentals
	symbols, err := s.sync.SelectTargets(
		ctx, kind, []string{string(market)}, fundamentalsStaleAfter, limit,
	)
	if err != nil {
		return out, fmt.Errorf("fundamentals rotation: select targets: %w", err)
	}
	if len(symbols) == 0 {
		return out, nil
	}

	// 국내만 DART를 쓴다. 해외는 EDGAR이고 그 버킷에는 일일 한도가 없다
	// (providers.go의 ProviderEdgar — 분당 제한만 있다).
	if market == model.AssetDomestic {
		max := backfillMaxSymbols(
			ctx, s.budget, apibudget.ProviderDART,
			fundamentalsDARTReserve, fundamentalsDARTPerSymbol, len(symbols),
		)
		if max <= 0 {
			out.Stopped = fmt.Sprintf(
				"dart headroom exhausted (reserve %d)", fundamentalsDARTReserve,
			)
			return out, nil
		}
		if max < len(symbols) {
			out.Stopped = fmt.Sprintf("headroom capped run to %d of %d", max, len(symbols))
			symbols = symbols[:max]
		}
	}

	for _, sym := range symbols {
		if ctx.Err() != nil {
			out.Stopped = "context cancelled"
			return out, nil
		}
		stock, err := s.stocks.GetBySymbol(ctx, sym)
		if err != nil || stock == nil || !model.IsFundamentalsApplicable(*stock) {
			// 대상이 아닌 종목도 **기록은 남긴다** — 안 그러면 다음 회차의 맨 앞에
			// 다시 서서 슬롯을 먹는다.
			_ = s.sync.Mark(ctx, repository.SyncOutcome{
				Kind: kind, Symbol: sym, Status: model.SyncNotFound,
				Message: "fundamentals not applicable",
			})
			continue
		}
		out.Attempted++
		row, refreshErr := s.RefreshSymbol(ctx, sym)
		switch {
		case refreshErr != nil:
			out.Failed++
			_ = s.sync.Mark(ctx, repository.SyncOutcome{
				Kind: kind, Symbol: sym, Status: model.SyncError, Message: refreshErr.Error(),
			})
		case row != nil && row.CalcStatus == model.FundamentalsStatusFailed:
			out.Failed++
			status := model.SyncError
			if strings.Contains(row.CalcError, "corp_code not found") {
				status = model.SyncNotFound
			}
			_ = s.sync.Mark(ctx, repository.SyncOutcome{
				Kind: kind, Symbol: sym, Status: status, Message: row.CalcError,
			})
		default:
			out.Updated++
			_ = s.sync.Mark(ctx, repository.SyncOutcome{
				Kind: kind, Symbol: sym, Status: model.SyncOK, RowsStored: 1,
			})
		}
		select {
		case <-ctx.Done():
			out.Stopped = "context cancelled"
			return out, nil
		case <-time.After(fundamentalsRefreshPause):
		}
	}
	return out, nil
}

// RefreshFailedDomestic re-fetches DART fundamentals for domestic symbols stuck in failed state.
func (s *FundamentalsService) RefreshFailedDomestic(ctx context.Context) (int, error) {
	if s.repo == nil || s.stocks == nil {
		return 0, nil
	}
	symbols, err := s.repo.ListSymbolsNotOK(ctx)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, sym := range symbols {
		stock, err := s.stocks.GetBySymbol(ctx, sym)
		if err != nil || stock == nil {
			continue
		}
		if s.fundamentalsAssetType(stock) != model.AssetDomestic {
			continue
		}
		if _, err := s.RefreshSymbol(ctx, sym); err != nil {
			continue
		}
		updated++
		select {
		case <-ctx.Done():
			return updated, ctx.Err()
		case <-time.After(fundamentalsRefreshPause):
		}
	}
	return updated, nil
}

func intPtr(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}
