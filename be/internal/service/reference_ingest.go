package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

// Financial period bases. 국내는 두 화면이 서로 다른 것을 보고 있어 둘 다 담는다
// (053 마이그레이션 주석 참조).
const (
	FinancialBasisQuarter    = "quarter"
	FinancialBasisCumulative = "cumulative"
)

// ReferenceIngestService fills the three tables that removed the last
// request-time external calls: 분기 재무 · 공시 · 종목 경고.
//
// 세 적재를 한 서비스에 둔 이유는 **DART 게이트를 공유해야 하기 때문**이다.
// 재무 적재와 공시 스윕과 내부자 적재가 같은 DART 한도를 쓰는데,
// apibudget.Check는 간격이 안 지나면 기다리지 않고 에러를 낸다 — 두 잡이 겹치면
// 서로를 실패시킨다. 게이트 하나를 나눠 쓰면 겹칠 때 실패 대신 줄을 선다.
type ReferenceIngestService struct {
	periods     *repository.FinancialPeriodRepo
	disclosures *repository.DisclosureRepo
	warnings    *repository.WarningRepo
	sync        *repository.SyncStateRepo
	stocks      *repository.StockRepo

	dart  *DartClient
	edgar *EdgarClient
	toss  *toss.Client

	// dartGate serialises every DART-calling batch in this process.
	dartGate *providerGate
}

func NewReferenceIngestService(
	periods *repository.FinancialPeriodRepo,
	disclosures *repository.DisclosureRepo,
	warnings *repository.WarningRepo,
	syncState *repository.SyncStateRepo,
	stocks *repository.StockRepo,
	dart *DartClient,
	edgar *EdgarClient,
	tossClient *toss.Client,
	dartGate *providerGate,
) *ReferenceIngestService {
	return &ReferenceIngestService{
		periods: periods, disclosures: disclosures, warnings: warnings,
		sync: syncState, stocks: stocks,
		dart: dart, edgar: edgar, toss: tossClient,
		dartGate: dartGate,
	}
}

// providerGate serialises calls to one upstream across the batches that share
// it, and paces them by the provider's minimum interval.
//
// apibudget이 이미 간격을 알고 있는데 왜 또 필요한가: apibudget.Check는 **판정만**
// 한다. 간격이 안 지났으면 에러를 반환하고 끝이라, 두 배치가 같은 provider를
// 동시에 쓰면 둘 다 계속 실패한다. 게이트는 같은 판정을 "기다림"으로 바꾼다.
type providerGate struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

func newProviderGate(interval time.Duration) *providerGate {
	return &providerGate{interval: interval}
}

// enter blocks until it is this caller's turn, then returns a release func.
func (g *providerGate) enter(ctx context.Context) func() {
	if g == nil {
		return func() {}
	}
	g.mu.Lock()
	if wait := g.interval - time.Since(g.last); wait > 0 {
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
	return func() {
		g.last = time.Now()
		g.mu.Unlock()
	}
}

const (
	// 관심·보유 종목을 다시 받기까지의 간격. 분기 재무는 분기마다 바뀌므로
	// 하루면 충분히 자주다.
	referenceTrackedStaleAfter = 24 * time.Hour

	// 국내 재무는 종목당 DART 호출이 20회를 넘는다(연도 4벌 × 보고서 4종 +
	// 누적 조회). 1초 간격이라 종목 하나에 20초 이상 걸리므로 한 실행의 기본
	// 대상 수가 작다 — 150종목이면 약 한 시간이다.
	financialsDefaultLimitDomestic = 150
	// 해외는 종목당 companyfacts 1호출이라 훨씬 싸다.
	financialsDefaultLimitOverseas = 600

	// 종목 경고는 종목당 Toss 1호출이고 **전 종목 매일**이 목표다.
	warningsDefaultLimit = 20000
	// Toss 경고 조회 간격.
	//
	// 처음에 120ms(초당 8회)로 잡았다가 실측에서 429를 받았다 — 응답 헤더가
	// `limit=5 resetSec=1`, 즉 **초당 5회**다. 250ms는 초당 4회로 그 아래이고,
	// 시세 폴러가 쓸 여유도 남긴다. 17,900종목이면 약 75분이라 새벽 창에 들어간다.
	warningsPause = 250 * time.Millisecond
	// 429를 받았을 때 한 번 더 기다렸다 재시도한다. resetSec이 1이므로 1.5초면
	// 창이 새로 열린다. 재시도하지 않으면 그 종목은 하루를 통째로 건너뛴다.
	warningsRetryPause = 1500 * time.Millisecond

	// 재무 이력 창. DART/EDGAR 모두 이 길이만큼 받아 저장한다.
	financialsYears = 3
)

// ─── 분기 재무 ────────────────────────────────────────────────────────────

// IngestFinancialsDomestic stores per-quarter and cumulative figures for
// domestic symbols.
func (s *ReferenceIngestService) IngestFinancialsDomestic(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.dart == nil || !s.dart.cfg.Enabled() {
		return nil, "", fmt.Errorf("financials ingest: dart api key not configured")
	}
	if limit <= 0 {
		limit = financialsDefaultLimitDomestic
	}
	symbols, err := s.sync.SelectTargets(ctx, model.SyncKindFinancialsDART,
		[]string{string(model.AssetDomestic)}, referenceTrackedStaleAfter, limit)
	if err != nil {
		return nil, "", fmt.Errorf("financials ingest: select targets: %w", err)
	}

	stats := newIngestStats()
	for _, symbol := range symbols {
		if ctx.Err() != nil {
			break
		}
		s.ingestDomesticFinancials(ctx, symbol, stats)
	}
	return stats.finish("financials_dart", len(symbols))
}

func (s *ReferenceIngestService) ingestDomesticFinancials(ctx context.Context, symbol string, stats *ingestStats) {
	release := s.dartGate.enter(ctx)
	quarters, qErr := s.dart.FetchQuarterlyHistory(ctx, symbol, financialsYears)
	release()

	release = s.dartGate.enter(ctx)
	cumulative, cErr := s.dart.FetchQuarterlySnapshots(ctx, symbol)
	release()

	// 둘 다 실패했을 때만 실패다. 하나만 얻어도 그 화면은 살아난다.
	if qErr != nil && cErr != nil {
		status := model.SyncError
		if strings.Contains(qErr.Error(), "corp_code not found") {
			status = model.SyncNotFound
		}
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindFinancialsDART, Symbol: symbol,
			Status: status, Message: qErr.Error(),
		})
		return
	}

	rows := make([]repository.FinancialPeriodRow, 0, len(quarters)+len(cumulative))
	for _, p := range quarters {
		rows = append(rows, repository.FinancialPeriodRow{
			Symbol: symbol, Source: "dart", Basis: FinancialBasisQuarter,
			PeriodLabel: domesticPeriodLabel(p.PeriodIndex),
			ReportCode:  domesticReportCode(p.PeriodIndex),
			Period:      p,
		})
	}
	for _, snap := range cumulative {
		p := model.FinancialHistoryPeriod{
			FiscalYear:  snap.FiscalYear,
			PeriodIndex: domesticPeriodIndex(snap.FiscalPeriod),
			PeriodEnd:   snap.PeriodEnd,
		}
		ApplyPeriodRatios(&p, snap)
		rows = append(rows, repository.FinancialPeriodRow{
			Symbol: symbol, Source: "dart", Basis: FinancialBasisCumulative,
			PeriodLabel: snap.FiscalPeriod, ReportCode: snap.ReportCode, Period: p,
		})
	}

	written, err := s.periods.Upsert(ctx, rows)
	if err != nil {
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindFinancialsDART, Symbol: symbol,
			Status: model.SyncError, Message: err.Error(),
		})
		return
	}
	status := model.SyncEmpty
	if written > 0 {
		status = model.SyncOK
	}
	stats.record(ctx, s.sync, repository.SyncOutcome{
		Kind: model.SyncKindFinancialsDART, Symbol: symbol,
		Status: status, RowsStored: written, ItemsSeen: len(rows),
	})
}

// IngestFinancialsOverseas stores EDGAR quarterly snapshots for US symbols.
//
// 미국은 두 화면(상세 모달·실적 탭)이 원래 같은 EDGAR 응답을 쓰므로 basis를
// 나누지 않는다 — 'quarter' 한 벌만 담는다.
func (s *ReferenceIngestService) IngestFinancialsOverseas(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.edgar == nil || !s.edgar.cfg.Enabled() {
		return nil, "", fmt.Errorf("financials ingest: sec edgar user agent not configured")
	}
	if limit <= 0 {
		limit = financialsDefaultLimitOverseas
	}
	symbols, err := s.sync.SelectTargets(ctx, model.SyncKindFinancialsSEC,
		[]string{string(model.AssetUS)}, referenceTrackedStaleAfter, limit)
	if err != nil {
		return nil, "", fmt.Errorf("financials ingest: select targets: %w", err)
	}

	stats := newIngestStats()
	for _, symbol := range symbols {
		if ctx.Err() != nil {
			break
		}
		s.ingestOverseasFinancials(ctx, symbol, stats)
	}
	return stats.finish("financials_edgar", len(symbols))
}

func (s *ReferenceIngestService) ingestOverseasFinancials(ctx context.Context, symbol string, stats *ingestStats) {
	s.edgar.pace(ctx)
	snaps, err := s.edgar.FetchQuarterlySnapshots(ctx, symbol, financialsYears*4+1)
	if err != nil {
		status := model.SyncError
		if strings.Contains(err.Error(), "cik not found") {
			status = model.SyncNotFound
		}
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindFinancialsSEC, Symbol: symbol,
			Status: status, Message: err.Error(),
		})
		return
	}

	rows := make([]repository.FinancialPeriodRow, 0, len(snaps))
	for _, snap := range snaps {
		p := model.FinancialHistoryPeriod{
			FiscalYear:  snap.FiscalYear,
			PeriodIndex: usPeriodIndex(snap.FiscalPeriod),
			PeriodEnd:   snap.PeriodEnd,
		}
		ApplyPeriodRatios(&p, snap)
		rows = append(rows, repository.FinancialPeriodRow{
			Symbol: symbol, Source: "edgar", Basis: FinancialBasisQuarter,
			PeriodLabel: snap.FiscalPeriod, ReportCode: snap.ReportCode, Period: p,
		})
	}

	written, err := s.periods.Upsert(ctx, rows)
	if err != nil {
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindFinancialsSEC, Symbol: symbol,
			Status: model.SyncError, Message: err.Error(),
		})
		return
	}
	status := model.SyncEmpty
	if written > 0 {
		status = model.SyncOK
	}
	stats.record(ctx, s.sync, repository.SyncOutcome{
		Kind: model.SyncKindFinancialsSEC, Symbol: symbol,
		Status: status, RowsStored: written, ItemsSeen: len(rows),
	})
}

// domesticPeriodLabel / domesticReportCode reconstruct what DART called this
// period. 저장할 때 라벨을 함께 넣어 두면 화면이 period_index를 해석하지 않아도
// 된다 — 국내 2분기는 「반기」(누적)라서 인덱스만으로는 뜻이 통하지 않는다.
func domesticPeriodLabel(index int) string {
	switch index {
	case 1:
		return "1Q"
	case 2:
		return "반기"
	case 3:
		return "3Q"
	default:
		return "연간"
	}
}

func domesticReportCode(index int) string {
	switch index {
	case 1:
		return "11013"
	case 2:
		return "11012"
	case 3:
		return "11014"
	default:
		return "11011"
	}
}

func domesticPeriodIndex(label string) int {
	switch strings.TrimSpace(label) {
	case "1Q":
		return 1
	case "반기":
		return 2
	case "3Q":
		return 3
	default:
		return 4
	}
}

// ─── 공시 ────────────────────────────────────────────────────────────────

// IngestDisclosures sweeps one day's DART filings for the whole market.
//
// 종목별로 돌지 않는 것이 이 잡의 핵심이다. list.json은 corp_code 없이 날짜만
// 받으면 그날 접수분 전체를 페이지로 준다 — 국내 상장 4,400종목을 각각 부르면
// 4,400호출이 드는 일이 페이지 몇 번으로 끝난다(2026-08-18 실측 397건 = 4페이지).
//
// days>1이면 어제까지 거슬러 받는다. 하루를 놓친 날(워커 재시작 등)을 메우는 용도다.
func (s *ReferenceIngestService) IngestDisclosures(ctx context.Context, days int) (map[string]any, string, error) {
	if s == nil || s.dart == nil || !s.dart.cfg.Enabled() {
		return nil, "", fmt.Errorf("disclosures ingest: dart api key not configured")
	}
	if days <= 0 {
		days = 1
	}

	known, err := s.stocks.ExistingSymbolSet(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("disclosures ingest: symbol set: %w", err)
	}

	total, pages, skipped := 0, 0, 0
	var firstErr error
	for d := days - 1; d >= 0; d-- {
		if ctx.Err() != nil {
			break
		}
		day := time.Now().In(model.KST).AddDate(0, 0, -d)

		// 게이트를 여기서 잡지 않는다. 하루치는 여러 페이지이고, 스윕이 전체를
		// 붙잡고 있으면 다른 DART 잡이 그동안 통째로 멈춘다. 페이지 단위로
		// FetchDayDisclosures가 직접 잡고 놓는다.
		//
		// (처음에는 여기서 잡고 안에서 또 잡았다 — Go 뮤텍스는 재진입이 안 되므로
		//  그 자리에서 영구 교착이었다. 스윕과 국내 재무 배치가 10분 넘게 멈춘
		//  것을 실측하고 잡았다.)
		items, pageCount, err := s.dart.FetchDayDisclosures(ctx, day, s.dartGate)
		pages += pageCount
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		rows := make([]repository.DisclosureRow, 0, len(items))
		for _, item := range items {
			symbol := strings.TrimSpace(item.StockCode)
			// 상장 종목이 아닌 제출인(비상장 계열사 등)은 버린다 — 우리 화면은
			// 종목을 통해서만 공시를 보여준다.
			if symbol == "" {
				skipped++
				continue
			}
			if _, ok := known[symbol]; !ok {
				skipped++
				continue
			}
			rows = append(rows, repository.DisclosureRow{
				Symbol:      symbol,
				ReceiptNo:   item.ReceiptNo,
				ReceiptDate: item.ReceiptDate,
				Title:       item.Title,
				Filer:       item.Filer,
			})
		}
		written, err := s.disclosures.Upsert(ctx, rows)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		total += written
	}

	result := map[string]any{
		"days": days, "rows": total, "pages": pages, "skipped": skipped,
	}
	message := fmt.Sprintf("공시 스윕: %d일 · %d페이지 · %d건 적재 (비상장·미등록 %d건 제외)",
		days, pages, total, skipped)
	if firstErr != nil {
		result["firstError"] = firstErr.Error()
		if total == 0 {
			return result, message, firstErr
		}
	}
	return result, message, nil
}

// ─── 종목 경고 ────────────────────────────────────────────────────────────

// IngestWarnings refreshes 지정 상태 for every listed symbol.
//
// 여기만 등급제가 아니라 **전 종목 매일**이다. 투자주의·거래정지는 하루 단위로
// 붙고 풀리므로, 로테이션으로 며칠에 한 번 보면 이미 풀린 경고를 계속 달거나
// 새로 붙은 경고를 놓친다 — 갱신이 잦은 데이터는 주기를 데이터에 맞춘다.
func (s *ReferenceIngestService) IngestWarnings(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.toss == nil || !s.toss.Enabled() {
		return nil, "", fmt.Errorf("warnings ingest: toss not configured")
	}
	if limit <= 0 {
		limit = warningsDefaultLimit
	}
	// staleAfter=0 — 관심·보유 우선순위 없이 마지막 수집이 오래된 순서만.
	symbols, err := s.sync.SelectTargets(ctx, model.SyncKindWarnings,
		[]string{string(model.AssetDomestic), string(model.AssetUS), string(model.AssetETF)}, 0, limit)
	if err != nil {
		return nil, "", fmt.Errorf("warnings ingest: select targets: %w", err)
	}

	stats := newIngestStats()
	for i, symbol := range symbols {
		if ctx.Err() != nil {
			break
		}
		if i > 0 {
			select {
			case <-ctx.Done():
				return stats.finish("warnings", len(symbols))
			case <-time.After(warningsPause):
			}
		}
		s.ingestOneWarning(ctx, symbol, stats)
	}
	return stats.finish("warnings", len(symbols))
}

func (s *ReferenceIngestService) ingestOneWarning(ctx context.Context, symbol string, stats *ingestStats) {
	warnings, err := s.toss.GetStockWarnings(ctx, symbol)
	// 429는 "이 종목에 문제가 있다"가 아니라 "너무 빨랐다"이다. 한 번 기다렸다
	// 다시 묻는다 — 안 그러면 그 종목은 다음 날까지 갱신되지 않는다.
	if err != nil && isRateLimitError(err) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(warningsRetryPause):
		}
		warnings, err = s.toss.GetStockWarnings(ctx, symbol)
	}
	if err != nil {
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindWarnings, Symbol: symbol,
			Status: model.SyncError, Message: err.Error(),
		})
		return
	}

	rows := make([]model.StockWarningRow, 0, len(warnings))
	for _, w := range warnings {
		rows = append(rows, model.StockWarningRow{
			WarningType: w.WarningType,
			Exchange:    w.Exchange,
			StartDate:   w.StartDate,
			EndDate:     w.EndDate,
		})
	}
	// 빈 목록도 반영해야 한다 — 해제된 경고를 지우는 것이 이 호출의 절반이다.
	written, err := s.warnings.ReplaceForSymbol(ctx, symbol, rows)
	if err != nil {
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindWarnings, Symbol: symbol,
			Status: model.SyncError, Message: err.Error(),
		})
		return
	}
	status := model.SyncEmpty
	if written > 0 {
		status = model.SyncOK
	}
	stats.record(ctx, s.sync, repository.SyncOutcome{
		Kind: model.SyncKindWarnings, Symbol: symbol,
		Status: status, RowsStored: written, ItemsSeen: len(rows),
	})
}

// ─── 공통 집계 ────────────────────────────────────────────────────────────

// ingestStats accumulates one run's outcome for the batch console.
type ingestStats struct {
	ok       int
	empty    int
	notFound int
	errored  int
	rows     int
	firstErr string
}

func newIngestStats() *ingestStats { return &ingestStats{} }

func (st *ingestStats) record(ctx context.Context, repo *repository.SyncStateRepo, out repository.SyncOutcome) {
	switch out.Status {
	case model.SyncOK:
		st.ok++
	case model.SyncEmpty:
		st.empty++
	case model.SyncNotFound:
		st.notFound++
	default:
		st.errored++
		if st.firstErr == "" {
			st.firstErr = out.Symbol + ": " + out.Message
		}
	}
	st.rows += out.RowsStored

	// 상태 기록은 취소된 컨텍스트에서도 남겨야 한다. 남기지 않으면 방금 처리한
	// 종목이 last_synced_at 없이 남아 다음 실행이 같은 자리에서 다시 시작한다.
	if err := repo.Mark(context.WithoutCancel(ctx), out); err != nil {
		log.Printf("ingest: mark %s/%s: %v", out.Kind, out.Symbol, err)
	}
}

func (st *ingestStats) finish(label string, attempted int) (map[string]any, string, error) {
	result := map[string]any{
		"kind":      label,
		"attempted": attempted,
		"withData":  st.ok,
		"empty":     st.empty,
		"notFound":  st.notFound,
		"errors":    st.errored,
		"rows":      st.rows,
	}
	message := fmt.Sprintf("%s: %d종목 처리 — %d행 / 내역없음 %d / 소스없음 %d / 실패 %d",
		label, attempted, st.rows, st.empty, st.notFound, st.errored)
	if st.firstErr != "" {
		result["firstError"] = st.firstErr
	}

	// 실패가 있어도 잡 자체는 성공이다 — 개별 종목 실패는 sync_state에 남고
	// 다음 순환에서 재시도된다. 다만 **한 종목도 처리하지 못했다면** 그건
	// 종목의 문제가 아니라 잡의 문제이므로 에러로 올린다.
	if attempted > 0 && st.errored == attempted {
		return result, message, fmt.Errorf("%s: all %d symbols failed (%s)", label, attempted, st.firstErr)
	}
	return result, message, nil
}
