package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
백필한 분기에 실제 발표일을 붙이는 잡 — SEC EDGAR 8-K Item 2.02.

Finnhub `/stock/earnings`는 발표일을 안 주고 **달력 분기로 반올림한 기간말**만
준다. 그대로 캘린더에 올리면 네 날짜에 3천 종목씩 뭉친다(migrations/056 헤더).
8-K Item 2.02가 진짜 발표일이고, 종목당 1호출이면 된다.

Finnhub 백필(1시간 40분)보다 훨씬 싸다 — `edgar_bulk`가 300ms 간격이라
3,000종목이 약 15분이다.
*/

// EarningsAnnounceBackfill resolves announcement dates and promotes the dated
// quarters into the calendar table.
type EarningsAnnounceBackfill struct {
	history *repository.EarningsSurpriseHistoryRepo
	edgar   *EdgarClient
	budget  *apibudget.Manager
}

func NewEarningsAnnounceBackfill(
	history *repository.EarningsSurpriseHistoryRepo,
	edgar *EdgarClient,
	budget *apibudget.Manager,
) *EarningsAnnounceBackfill {
	return &EarningsAnnounceBackfill{history: history, edgar: edgar, budget: budget}
}

// AnnounceBackfillOptions controls one run.
type AnnounceBackfillOptions struct {
	Symbols []string
	Limit   int
	// Recheck — 이미 8-K를 받아 본 종목도 다시 부른다(기본 false).
	Recheck bool
	DryRun  bool
	// Promote — 매칭이 끝난 뒤 캘린더 표로 승격할지(기본 true).
	Promote bool
	Pace    time.Duration
}

// AnnounceBackfillResult is what the operator needs afterward.
type AnnounceBackfillResult struct {
	Candidates int
	Attempted  int
	// NoFilings — EDGAR에 8-K가 없는 종목(ADR·해외 발행사 등).
	NoFilings int
	// Rejected — 규칙에 걸려 통째로 버린 종목(단조성 위반 등).
	Rejected int
	Matched  int
	// QuartersDated — 실제로 날짜가 붙은 분기 수.
	QuartersDated int
	Failed        int
	Promoted      int
	Duration      time.Duration
}

// Run walks the symbol list one EDGAR call at a time.
func (b *EarningsAnnounceBackfill) Run(
	ctx context.Context, opts AnnounceBackfillOptions,
) (AnnounceBackfillResult, error) {
	res := AnnounceBackfillResult{}
	if b == nil || b.edgar == nil || !b.edgar.cfg.Enabled() {
		return res, fmt.Errorf("earnings announce backfill: SEC_EDGAR_USER_AGENT 미설정")
	}
	started := time.Now()

	symbols, err := b.candidates(ctx, opts)
	if err != nil {
		return res, err
	}
	res.Candidates = len(symbols)

	pace := opts.Pace
	if pace <= 0 && b.budget != nil {
		pace = b.budget.MinInterval(apibudget.ProviderEdgarBulk)
	}
	log.Printf("earnings announce backfill: %d symbols, pace %v, dryRun=%v (예상 %v)",
		len(symbols), pace, opts.DryRun, time.Duration(len(symbols))*pace)

	consecutiveFail := 0
	for i, sym := range symbols {
		if i > 0 && pace > 0 {
			select {
			case <-ctx.Done():
				res.Duration = time.Since(started)
				return res, ctx.Err()
			case <-time.After(pace):
			}
		}
		res.Attempted++
		if res.Attempted%200 == 0 {
			log.Printf("earnings announce backfill: %d/%d (종목 %d · 분기 %d · 8-K없음 %d · 규칙탈락 %d · 실패 %d · 경과 %v)",
				res.Attempted, len(symbols), res.Matched, res.QuartersDated,
				res.NoFilings, res.Rejected, res.Failed,
				time.Since(started).Truncate(time.Second))
		}

		n, outcome, err := b.resolveSymbol(ctx, sym, opts.DryRun)
		if err != nil {
			if ctx.Err() != nil {
				res.Duration = time.Since(started)
				return res, ctx.Err()
			}
			res.Failed++
			consecutiveFail++
			log.Printf("earnings announce backfill %s: %v", sym, err)
			if consecutiveFail >= backfillFailStreakAbort {
				res.Duration = time.Since(started)
				return res, fmt.Errorf(
					"연속 %d회 실패로 중단 (마지막 %s): %w — 원인 확인 후 이어서 실행",
					consecutiveFail, sym, err)
			}
			continue
		}
		consecutiveFail = 0

		switch outcome {
		case announceNoFilings:
			res.NoFilings++
		case announceRejected:
			res.Rejected++
		case announceMatched:
			res.Matched++
			res.QuartersDated += n
		}
	}

	if opts.Promote && !opts.DryRun {
		promoted, err := b.history.PromoteAnnouncedToCalendar(ctx, string(model.AssetUS))
		if err != nil {
			res.Duration = time.Since(started)
			return res, fmt.Errorf("promote: %w", err)
		}
		res.Promoted = promoted
	}

	res.Duration = time.Since(started)
	return res, nil
}

type announceOutcome int

const (
	announceMatched announceOutcome = iota
	announceNoFilings
	announceRejected
)

func (b *EarningsAnnounceBackfill) resolveSymbol(
	ctx context.Context, symbol string, dryRun bool,
) (int, announceOutcome, error) {
	quarters, err := b.history.QuartersForSymbol(ctx, symbol)
	if err != nil {
		return 0, announceRejected, err
	}
	if len(quarters) == 0 {
		return 0, announceNoFilings, nil
	}

	filings, err := b.edgar.FetchEarningsAnnouncements(ctx, symbol)
	if err != nil {
		// CIK가 없는 종목은 실패가 아니라 「EDGAR에 없음」이다. ADR·해외 발행사가
		// 여기 걸린다 — 3천 종목 중 수백 건이라 실패로 세면 차단기가 오작동한다.
		if isEdgarMissing(err) {
			if !dryRun {
				_, _ = b.history.SaveAnnounceMatches(ctx, symbol, nil)
			}
			return 0, announceNoFilings, nil
		}
		return 0, announceRejected, err
	}
	if len(filings) == 0 {
		if !dryRun {
			_, _ = b.history.SaveAnnounceMatches(ctx, symbol, nil)
		}
		return 0, announceNoFilings, nil
	}

	candidates := make([]model.AnnounceCandidate, 0, len(filings))
	for _, f := range filings {
		candidates = append(candidates, model.AnnounceCandidate{
			Filed:     f.Filed,
			Session:   sessionFromAcceptance(f.Accepted),
			Accession: f.Accession,
		})
	}

	matches := model.MatchAnnouncements(quarters, candidates)
	if len(matches) == 0 {
		// 규칙에 걸려 버린 경우다. **그래도 checked는 찍는다** — 안 찍으면
		// 재실행이 같은 종목을 또 부르고, 결과는 언제나 같다.
		if !dryRun {
			_, _ = b.history.SaveAnnounceMatches(ctx, symbol, nil)
		}
		return 0, announceRejected, nil
	}
	if dryRun {
		return len(matches), announceMatched, nil
	}
	n, err := b.history.SaveAnnounceMatches(ctx, symbol, matches)
	if err != nil {
		return 0, announceRejected, err
	}
	return n, announceMatched, nil
}

// candidates resolves the symbol list for this run.
func (b *EarningsAnnounceBackfill) candidates(
	ctx context.Context, opts AnnounceBackfillOptions,
) ([]string, error) {
	if len(opts.Symbols) > 0 {
		if opts.Limit > 0 && len(opts.Symbols) > opts.Limit {
			return opts.Symbols[:opts.Limit], nil
		}
		return opts.Symbols, nil
	}
	symbols, err := b.history.SymbolsNeedingAnnounceDate(
		ctx, string(model.AssetUS), opts.Recheck)
	if err != nil {
		return nil, err
	}
	if opts.Limit > 0 && len(symbols) > opts.Limit {
		symbols = symbols[:opts.Limit]
	}
	return symbols, nil
}

// isEdgarMissing reports "this company is not in EDGAR", which is a normal
// outcome rather than a failure.
func isEdgarMissing(err error) bool {
	if err == nil {
		return false
	}
	// LookupCIK가 만드는 문자열. 구조화된 에러가 아니라 여기서만 문자열로 본다 —
	// 그 함수는 이 기능보다 먼저 있었고, 시그니처를 바꾸면 호출자 넷이 함께 바뀐다.
	return errors.Is(err, errEdgarRateLimited) == false &&
		containsAny(err.Error(), "cik not found", "company facts not found")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) == 0 {
			continue
		}
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
