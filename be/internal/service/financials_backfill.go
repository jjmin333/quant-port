package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

const (
	financialBackfillSymbolPause   = 1100 * time.Millisecond
	financialBackfillDARTReserve   = 5000 // leave headroom for scheduled DART jobs
	financialBackfillDARTPerSymbol = 48   // 전체 갱신: 연간 조회 + 아래 분기 적재
	// 분기 적재만 해도 종목당 20호출이 넘는다 — `FetchQuarterlyHistory`가
	// **(years+1)년 × 보고서 4종 = 16회**, `FetchQuarterlySnapshots`가 4~8회다.
	// 연결(CFS)이 없어 별도(OFS)로 재시도하면 각 호출이 두 번이 된다.
	//
	// 예전 값은 `4`였다. 그 숫자를 믿고 예산 가드(`backfillMaxSymbols`)가 1,151종목
	// 실행을 통과시켰고, 실제로는 2만 회를 넘겨 **DART 일일 한도를 소진**했다
	// (2026-09-02). 가드가 막으라고 있는 일을 못 막은 것이다.
	financialBackfillDARTPeriodsPerSym = 24
	financialBackfillEdgarPerSym       = 2
	// 상세 줄 상한 — 대상이 1,500종목이면 줄마다 남길 수 없다.
	financialBackfillMaxDetails = 200
)

// FinancialsBackfillOptions controls a one-shot empty-cohort migration run.
type FinancialsBackfillOptions struct {
	Market      model.QuoteAssetType
	TrackedOnly bool
	Limit       int
	DryRun      bool
	PeriodsOnly bool // skip fundamentals refresh; ingest quarterly history only
	// HolesOnly는 대상을 「분기는 있는데 그 안에 빈 칸이 있는 종목」으로 바꾼다.
	// 계정명 매칭을 고친 뒤 그 종목만 다시 읽으라는 뜻이며, 분기 적재만 한다
	// (종목당 DART 약 24회 — 전체 갱신 48회의 절반이다).
	HolesOnly   bool
	DARTReserve int
	SymbolPause time.Duration
}

// Normalized fills safe defaults for a backfill run.
func (o FinancialsBackfillOptions) Normalized() FinancialsBackfillOptions {
	if o.DARTReserve <= 0 {
		o.DARTReserve = financialBackfillDARTReserve
	}
	if o.SymbolPause <= 0 {
		o.SymbolPause = financialBackfillSymbolPause
	}
	// 구멍 메우기는 분기 적재만 한다 — 이미 fundamentals가 있는 종목들이고,
	// 전체 갱신은 호출이 두 배라 하루 한도 안에 훨씬 적은 종목만 돈다.
	if o.HolesOnly {
		o.PeriodsOnly = true
	}
	return o
}

type FinancialsBackfillResult struct {
	Attempted int      `json:"attempted"`
	OK        int      `json:"ok"`
	Failed    int      `json:"failed"`
	Skipped   int      `json:"skipped"`
	Details   []string `json:"details,omitempty"`
	Stopped   string   `json:"stopped,omitempty"`
}

// FinancialsBackfillService fills valuation + quarterly financials for symbols
// that have neither. Uses the same ingest paths as production batches.
type FinancialsBackfillService struct {
	stocks       *repository.StockRepo
	fundamentals *FundamentalsService
	fundRepo     *repository.FundamentalsRepo
	periods      *repository.FinancialPeriodRepo
	reference    *ReferenceIngestService
	budget       *apibudget.Manager
}

func NewFinancialsBackfillService(
	stocks *repository.StockRepo,
	fundamentals *FundamentalsService,
	fundRepo *repository.FundamentalsRepo,
	periods *repository.FinancialPeriodRepo,
	reference *ReferenceIngestService,
	budget *apibudget.Manager,
) *FinancialsBackfillService {
	return &FinancialsBackfillService{
		stocks: stocks, fundamentals: fundamentals, fundRepo: fundRepo,
		periods: periods, reference: reference, budget: budget,
	}
}

func (s *FinancialsBackfillService) ListTargets(ctx context.Context, opts FinancialsBackfillOptions) ([]string, error) {
	if s == nil || s.stocks == nil {
		return nil, fmt.Errorf("financials backfill: not configured")
	}
	opts = opts.Normalized()
	filter := repository.FinancialBackfillFilter{
		AssetType:    string(opts.Market),
		TrackedOnly:  opts.TrackedOnly,
		Limit:        opts.Limit,
		ExcludeNoCIK: opts.Market == model.AssetUS && !opts.PeriodsOnly,
	}
	if opts.HolesOnly {
		return s.stocks.ListSymbolsWithFinancialHoles(ctx, filter)
	}
	if opts.PeriodsOnly {
		return s.stocks.ListValuationOkNoPeriodsSymbols(ctx, filter)
	}
	return s.stocks.ListBothEmptyFinancialSymbols(ctx, filter)
}

func (s *FinancialsBackfillService) Run(ctx context.Context, opts FinancialsBackfillOptions) (FinancialsBackfillResult, error) {
	opts = opts.Normalized()
	out := FinancialsBackfillResult{}

	symbols, err := s.ListTargets(ctx, opts)
	if err != nil {
		return out, err
	}
	if len(symbols) == 0 {
		return out, nil
	}

	if opts.DryRun {
		out.Attempted = len(symbols)
		mode := "full"
		if opts.PeriodsOnly {
			mode = "periods-only"
		}
		out.Details = append(out.Details, fmt.Sprintf("dry-run: would backfill %d symbols (%s %s)", len(symbols), opts.Market, mode))
		return out, nil
	}

	provider, perSymbol := backfillProvider(opts.Market, opts.PeriodsOnly)
	maxSymbols := backfillMaxSymbols(ctx, s.budget, provider, opts.DARTReserve, perSymbol, len(symbols))
	if maxSymbols <= 0 {
		out.Stopped = fmt.Sprintf("%s daily headroom exhausted (reserve %d)", provider, opts.DARTReserve)
		return out, nil
	}
	if maxSymbols < len(symbols) {
		out.Stopped = fmt.Sprintf("headroom capped run to %d of %d symbols", maxSymbols, len(symbols))
		symbols = symbols[:maxSymbols]
	}

	for i, symbol := range symbols {
		if ctx.Err() != nil {
			out.Stopped = "context cancelled"
			break
		}
		if err := s.waitBudget(ctx, provider); err != nil {
			out.Stopped = err.Error()
			break
		}

		out.Attempted++
		msg, ok := s.backfillOne(ctx, symbol, opts)
		if ok {
			out.OK++
		} else {
			out.Failed++
			// 한도 소진이면 여기서 끝낸다. 계속 돌아 봐야 남은 종목도 전부
			// 같은 이유로 실패하고, 호출만 태운다.
			if strings.Contains(msg, ErrDartQuotaExceeded.Error()) {
				out.Details = append(out.Details, msg)
				out.Stopped = "DART 일일 한도 소진 — 남은 종목은 다음 날 이어서 돌린다"
				break
			}
		}
		// 성공 쪽도 할 말이 있으면 남긴다 — 구멍 메우기는 「몇 칸이 채워졌는가」가
		// 성패보다 중요한 결과다. 다만 목록이 수천 줄이 되지 않게 상한을 둔다.
		if msg != "" && len(out.Details) < financialBackfillMaxDetails {
			out.Details = append(out.Details, msg)
		}
		log.Printf("financials backfill [%d/%d] %s ok=%v", i+1, len(symbols), symbol, ok)

		if i+1 < len(symbols) {
			select {
			case <-ctx.Done():
			case <-time.After(opts.SymbolPause):
			}
		}
	}
	return out, nil
}

func (s *FinancialsBackfillService) BackfillSymbol(ctx context.Context, symbol string, opts FinancialsBackfillOptions) (detail string, ok bool) {
	return s.backfillOne(ctx, symbol, opts.Normalized())
}

func (s *FinancialsBackfillService) backfillOne(ctx context.Context, symbol string, opts FinancialsBackfillOptions) (detail string, ok bool) {
	stock, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil || stock == nil {
		return symbol + ": not in catalog", false
	}
	if !model.IsFundamentalsApplicable(*stock) {
		return "", true // ETF mis-tagged — skip quietly
	}

	// 구멍 메우기는 「몇 칸이 채워졌는가」로 성패를 가린다 — 대상이 이미 분기를
	// 가진 종목이라 「분기가 있는가」는 늘 참이다.
	holesBefore := -1
	if opts.HolesOnly && s.periods != nil {
		if n, err := s.periods.CountHoles(ctx, symbol); err == nil {
			holesBefore = n
		}
	}

	if !opts.PeriodsOnly && s.fundamentals != nil {
		row, err := s.fundamentals.RefreshSymbol(ctx, symbol)
		if err != nil {
			return symbol + ": fundamentals: " + err.Error(), false
		}
		if row != nil && row.CalcStatus == model.FundamentalsStatusFailed {
			if strings.Contains(row.CalcError, "cik not found") {
				return symbol + ": " + row.CalcError, false
			}
			if opts.Market == model.AssetDomestic && strings.Contains(row.CalcError, "no annual financial") {
				return symbol + ": " + row.CalcError, false
			}
		}
	}

	if s.reference == nil {
		return symbol + ": reference ingest unavailable", false
	}

	var ingestErr error
	switch opts.Market {
	case model.AssetDomestic:
		ingestErr = s.reference.IngestDomesticSymbol(ctx, symbol)
	case model.AssetUS:
		ingestErr = s.reference.IngestOverseasSymbol(ctx, symbol)
	default:
		return symbol + ": unsupported market", false
	}
	// 한도 소진은 이 종목만의 문제가 아니다 — 그대로 두면 남은 수백 종목을
	// 계속 돌면서 아무것도 못 쓰고 전부 ok로 보고한다(2026-09-02 실측:
	// 705종목 ok=705인데 코호트가 한 종목도 안 줄었다).
	if ingestErr != nil {
		return symbol + ": " + ingestErr.Error(), false
	}

	if holesBefore >= 0 {
		after, err := s.periods.CountHoles(ctx, symbol)
		if err != nil {
			return symbol + ": verify holes: " + err.Error(), false
		}
		// 하나도 안 줄었어도 실패는 아니다 — 그 회사가 정말 그 항목을 공시하지
		// 않는 경우가 있다(은행의 영업이익 등). 다만 무엇이 남았는지는 적는다.
		if after >= holesBefore {
			return fmt.Sprintf("%s: 구멍 %d칸 그대로", symbol, after), true
		}
		return fmt.Sprintf("%s: 구멍 %d → %d칸", symbol, holesBefore, after), true
	}

	return s.verifyBackfill(ctx, symbol, stock, opts.PeriodsOnly)
}

func (s *FinancialsBackfillService) verifyBackfill(ctx context.Context, symbol string, stock *model.Stock, periodsOnly bool) (detail string, ok bool) {
	hasPeriods := false
	if s.periods != nil {
		var err error
		hasPeriods, err = s.periods.HasPeriods(ctx, symbol)
		if err != nil {
			return symbol + ": verify periods: " + err.Error(), false
		}
	}
	if periodsOnly {
		if hasPeriods {
			return "", true
		}
		return symbol + ": no periods after ingest", false
	}

	var fund *model.StockFundamentals
	if s.fundRepo != nil {
		var err error
		fund, err = s.fundRepo.GetBySymbol(ctx, symbol)
		if err != nil {
			return symbol + ": verify fundamentals: " + err.Error(), false
		}
	}
	ok, reason := backfillSucceeded(fund, stock, hasPeriods)
	if !ok {
		return symbol + ": " + reason, false
	}
	return "", true
}

// backfillSucceeded reports whether both UI tabs have displayable data.
func backfillSucceeded(fund *model.StockFundamentals, stock *model.Stock, hasPeriods bool) (ok bool, reason string) {
	if !hasPeriods {
		return false, "no periods after ingest"
	}
	if fund == nil {
		return false, "fundamentals row missing after refresh"
	}
	if model.ComputeCoverageTier(*fund, stock) == model.CoverageTierUnavailable {
		return false, "valuation still unavailable (" + string(fund.CalcStatus) + ")"
	}
	return true, ""
}

func (s *FinancialsBackfillService) budgetCheck(ctx context.Context, provider apibudget.Provider) error {
	if s.budget == nil {
		return nil
	}
	return s.budget.Check(ctx, provider)
}

func (s *FinancialsBackfillService) waitBudget(ctx context.Context, provider apibudget.Provider) error {
	const maxTries = 5
	for try := 0; try < maxTries; try++ {
		if err := s.budgetCheck(ctx, provider); err == nil {
			return nil
		} else if !strings.Contains(err.Error(), "min interval") {
			return err
		}
		wait := 350 * time.Millisecond
		if s.budget != nil {
			if d := s.budget.MinInterval(provider); d > wait {
				wait = d
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return s.budgetCheck(ctx, provider)
}

func backfillProvider(market model.QuoteAssetType, periodsOnly bool) (apibudget.Provider, int) {
	if market == model.AssetUS {
		return apibudget.ProviderEdgarBulk, financialBackfillEdgarPerSym
	}
	if periodsOnly {
		return apibudget.ProviderDART, financialBackfillDARTPeriodsPerSym
	}
	return apibudget.ProviderDART, financialBackfillDARTPerSymbol
}

func backfillMaxSymbols(ctx context.Context, budget *apibudget.Manager, provider apibudget.Provider, reserve, perSymbol, want int) int {
	if want <= 0 || perSymbol <= 0 {
		return 0
	}
	if budget == nil {
		return want
	}
	h := budget.Headroom(ctx, provider)
	if h.Calls < 0 {
		return want
	}
	usable := int(h.Calls) - reserve
	if usable <= 0 {
		return 0
	}
	max := usable / perSymbol
	if max <= 0 {
		return 0
	}
	if max > want {
		return want
	}
	return max
}
