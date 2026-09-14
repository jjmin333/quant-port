package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
과거 실적 서프라이즈 백필 — Finnhub `/stock/earnings`.

# 왜 캘린더로는 안 되는가 (2026-08-21 실측)

무료 티어의 `/calendar/earnings`는 **과거 30일까지만** 준다. 오류가 아니라 빈
배열이라 더 고약하다:

	2026-08-10~08-16  1,246행
	2026-07-27~08-02    955행
	2026-07-20~07-26    370행 (07-22부터만 — 경계)
	2026-07-13~07-19      0행   ← HTTP 200
	2025-08-04~08-10      0행
	2024-08-05~08-11      0행

그래서 「꾸준히 기대를 넘는 기업」 카드는 최소 표본 4분기를 채우는 해외 종목이
**0개**였다(국내는 FnGuide가 2년치를 줘서 403종목).

# 이 엔드포인트가 주는 것과 안 주는 것

준다: 종목당 **정확히 4분기**의 (회계연도·분기·기간종료일·EPS 컨센서스·EPS 실적).
`limit` 파라미터는 무시된다 — 8이나 16을 줘도 4행이다.

안 준다: **발표일**, 매출(‌`/stock/revenue-estimate`는 무료 티어에서 403).
그래서 결과는 earnings_events가 아니라 earnings_surprise_history로 간다
(migrations/055 헤더 참고).
*/

// FinnhubSurpriseRow is one `/stock/earnings` entry.
type FinnhubSurpriseRow struct {
	Symbol string
	// Period — 회계기간 종료일("2026-06-30"). 발표일이 아니다.
	Period   string
	Quarter  int
	Year     int
	Estimate *float64
	Actual   *float64
}

// UseBackfillBudget switches this client to the one-off backfill quota bucket.
//
// ProviderFinnhubEarnings의 20초 간격은 하루 수십 호출짜리 캘린더 배치를 위한
// 값이라, 3,000종목을 도는 이 잡이 쓰면 17시간이 걸리고 그동안 캘린더 배치를
// 막는다. 나누는 것은 한도가 아니라 "누가 언제 부르는지"다.
func (e *EarningsClient) UseBackfillBudget() *EarningsClient {
	if e != nil {
		e.provider = apibudget.ProviderFinnhubBackfill
	}
	return e
}

// FetchSurpriseHistory returns the last four quarters of consensus vs actual.
func (e *EarningsClient) FetchSurpriseHistory(
	ctx context.Context, symbol string,
) ([]FinnhubSurpriseRow, error) {
	if e == nil || !e.cfg.Enabled() {
		return nil, fmt.Errorf("finnhub surprises: disabled")
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil, nil
	}
	if e.budget != nil {
		if err := e.budget.Check(ctx, e.provider); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf(
		"%s/stock/earnings?symbol=%s&token=%s",
		e.base(), url.QueryEscape(sym), url.QueryEscape(e.cfg.FinnhubAPIKey),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := e.httpClient.Do(req)
	if err != nil {
		// **토큰이 URL에 있으므로 반드시 지운다.** 실측으로 겪었다 — 백필 중
		// VLY 타임아웃 한 건이 키를 평문으로 로그 파일에 남겼다(earnings.go
		// redactFinnhubToken 참고).
		return nil, redactFinnhubToken(err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		if e.budget != nil {
			e.budget.BlockFor(ctx, e.provider, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, errFinnhubRateLimited
	}
	// 401/403은 재시도해도 소용없고, 3,000번 반복하면 그냥 3,000번 거절당한다.
	// 호출자가 그 자리에서 멈출 수 있도록 구분되는 에러로 올린다.
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w (status %d)", ErrFinnhubNotEntitled, res.StatusCode)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub surprises %s: status %d", sym, res.StatusCode)
	}

	var payload []struct {
		Actual   *float64 `json:"actual"`
		Estimate *float64 `json:"estimate"`
		Period   string   `json:"period"`
		Quarter  float64  `json:"quarter"`
		Year     float64  `json:"year"`
		Symbol   string   `json:"symbol"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("finnhub surprises %s: %w", sym, err)
	}
	if e.budget != nil {
		e.budget.Record(ctx, e.provider)
	}

	out := make([]FinnhubSurpriseRow, 0, len(payload))
	for _, row := range payload {
		period := strings.TrimSpace(row.Period)
		if period == "" {
			continue
		}
		rowSym := strings.ToUpper(strings.TrimSpace(row.Symbol))
		if rowSym == "" {
			rowSym = sym
		}
		out = append(out, FinnhubSurpriseRow{
			Symbol:   rowSym,
			Period:   period,
			Quarter:  int(row.Quarter),
			Year:     int(row.Year),
			Estimate: row.Estimate,
			Actual:   row.Actual,
		})
	}
	return out, nil
}

// backfillFailStreakAbort — 몇 번 연속 실패하면 잡 자체가 잘못됐다고 볼지.
//
// 20인 것은 산발적 실패와 구조적 실패를 가르는 선이다. 개별 종목의 일시적 오류는
// 3,000개 중 드문드문 나오지 스무 번 잇달아 나오지 않는다. 반대로 한도 소진이나
// 키 문제는 **첫 실패부터 전부** 실패한다.
const backfillFailStreakAbort = 20

// ErrFinnhubNotEntitled means the API key's plan does not cover the endpoint.
// 백필은 이 에러를 만나면 즉시 멈춘다 — 티어 문제는 재시도로 풀리지 않는다.
var ErrFinnhubNotEntitled = errors.New("finnhub: endpoint not available on this plan")

// errFinnhubRateLimited is retried once after the budget's backoff.
var errFinnhubRateLimited = errors.New("finnhub: status 429")

// surpriseRows converts one symbol's API response into storable rows.
//
// **정렬을 여기서 한다.** `/stock/earnings`는 기간순으로 오지 않는다 — 실측 TGT는
// 2026-09-30, 2025-09-30, 2026-06-30, 2026-03-31 순으로 왔다. 응답 순서를 믿고
// 앞에서 N개를 자르면 최근 분기가 아니라 아무 분기나 집는다.
func surpriseRows(symbol string, rows []FinnhubSurpriseRow) []model.EarningsSurpriseRow {
	out := make([]model.EarningsSurpriseRow, 0, len(rows))
	for _, row := range rows {
		periodEnd, err := time.ParseInLocation("2006-01-02", row.Period, time.UTC)
		if err != nil {
			continue
		}
		year := row.Year
		if year <= 0 {
			year = periodEnd.Year()
		}
		out = append(out, model.EarningsSurpriseRow{
			Symbol:       symbol,
			Market:       string(model.AssetUS),
			FiscalYear:   year,
			FiscalPeriod: model.FiscalPeriodFromQuarter(row.Quarter),
			PeriodEnd:    periodEnd,
			EPSEst:       row.Estimate,
			EPSAct:       row.Actual,
			Currency:     "USD",
			Source:       "finnhub",
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].PeriodEnd.After(out[j].PeriodEnd)
	})
	return out
}

// EarningsSurpriseBackfill is the one-off job behind
// cmd/backfill-earnings-surprises.
//
// 관리자 배치 콘솔에 넣지 않았다. 한 번 받으면 되는 데이터이고, 정기 잡으로
// 두면 매일 3,000호출을 쓰면서 같은 4분기를 다시 받는다 — 새 분기는 캘린더
// 배치가 발표일과 함께 이미 넣는다.
type EarningsSurpriseBackfill struct {
	history *repository.EarningsSurpriseHistoryRepo
	events  *repository.EarningsEventRepo
	finnhub *EarningsClient
	budget  *apibudget.Manager
}

func NewEarningsSurpriseBackfill(
	history *repository.EarningsSurpriseHistoryRepo,
	events *repository.EarningsEventRepo,
	finnhub *EarningsClient,
	budget *apibudget.Manager,
) *EarningsSurpriseBackfill {
	return &EarningsSurpriseBackfill{history: history, events: events, finnhub: finnhub, budget: budget}
}

// BackfillOptions controls one run.
type BackfillOptions struct {
	// Symbols — 비우면 「해외에서 실적을 한 번이라도 발표한 종목」 전체.
	Symbols []string
	// Limit — 0이면 제한 없음. 시험 실행에 쓴다.
	Limit int
	// Resume — 이미 이력이 있는 종목을 건너뛴다(기본 true).
	Resume bool
	// DryRun — 호출은 하되 저장하지 않는다.
	DryRun bool
	// Pace — 호출 간격. 0이면 예산 버킷의 MinInterval.
	Pace time.Duration
}

// BackfillResult is what the operator needs to know afterward.
type BackfillResult struct {
	Candidates int
	Attempted  int
	Fetched    int
	Empty      int
	Failed     int
	RowsSaved  int
	Duration   time.Duration
}

// Run walks the symbol list one HTTP call at a time.
func (b *EarningsSurpriseBackfill) Run(
	ctx context.Context, opts BackfillOptions,
) (BackfillResult, error) {
	res := BackfillResult{}
	if b == nil || b.finnhub == nil || !b.finnhub.cfg.Enabled() {
		return res, fmt.Errorf("earnings surprise backfill: finnhub disabled")
	}
	started := time.Now()

	symbols, err := b.candidates(ctx, opts)
	if err != nil {
		return res, err
	}
	res.Candidates = len(symbols)
	if len(symbols) == 0 {
		return res, nil
	}

	pace := opts.Pace
	if pace <= 0 && b.budget != nil {
		pace = b.budget.MinInterval(apibudget.ProviderFinnhubBackfill)
	}

	log.Printf("earnings surprise backfill: %d symbols, pace %v, dryRun=%v (예상 %v)",
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
		// 진행 로그를 루프 **앞**에서 찍는다. 뒤에 두면 빈 응답·실패로 continue한
		// 회차에서 건너뛰어, 하필 100번째가 빈 응답이면 그 구간이 통째로 조용해진다.
		// 한 시간짜리 잡에서 "아직 도는 중인가"를 알 수 있어야 한다.
		if res.Attempted%100 == 0 {
			log.Printf("earnings surprise backfill: %d/%d (저장 %d행, 빈응답 %d, 실패 %d, 경과 %v)",
				res.Attempted, len(symbols), res.RowsSaved, res.Empty, res.Failed,
				time.Since(started).Truncate(time.Second))
		}

		rows, err := b.fetchWithRetry(ctx, sym, pace)
		if err != nil {
			// 티어 문제면 남은 종목도 전부 같은 답을 받는다. 3,000번 거절당하며
			// 한 시간을 태우는 대신 여기서 멈추고, 지금까지 받은 것은 남긴다.
			if errorsIsNotEntitled(err) {
				res.Duration = time.Since(started)
				return res, fmt.Errorf("aborted at %s: %w", sym, err)
			}
			if ctx.Err() != nil {
				res.Duration = time.Since(started)
				return res, ctx.Err()
			}
			res.Failed++
			consecutiveFail++
			log.Printf("earnings surprise backfill %s: %v", sym, err)
			// 연속 실패 차단기. 개별 종목 실패는 건너뛰면 되지만, **모두가 같은
			// 이유로 실패하는 상황**(일일 한도 소진 → Check가 계속 거부, 키 회수,
			// 네트워크 단절)에서는 남은 3,000종목을 75분간 헛돌게 된다. 그때는
			// 멈추고 사람이 원인을 보는 편이 낫다 — 받은 것은 이미 저장돼 있고
			// -resume이 이어받는다.
			if consecutiveFail >= backfillFailStreakAbort {
				res.Duration = time.Since(started)
				return res, fmt.Errorf(
					"연속 %d회 실패로 중단 (마지막 %s): %w — 원인 확인 후 -resume으로 이어서 실행",
					consecutiveFail, sym, err)
			}
			continue
		}
		// 여기까지 왔으면 이번 종목은 API 응답을 받았다. 저장 단계는 아래에서
		// 따로 센다.
		consecutiveFail = 0
		if len(rows) == 0 {
			res.Empty++
			continue
		}
		res.Fetched++

		if opts.DryRun {
			res.RowsSaved += len(rows)
			continue
		}
		n, err := b.history.Upsert(ctx, rows)
		if err != nil {
			// DB 실패도 같은 차단기에 넣는다 — MySQL이 내려가면 남은 종목의
			// API 호출만 쓰고 저장은 한 건도 못 하는 상태로 끝까지 돈다.
			res.Failed++
			consecutiveFail++
			log.Printf("earnings surprise backfill %s (upsert): %v", sym, err)
			if consecutiveFail >= backfillFailStreakAbort {
				res.Duration = time.Since(started)
				return res, fmt.Errorf(
					"연속 %d회 저장 실패로 중단 (마지막 %s): %w", consecutiveFail, sym, err)
			}
			continue
		}
		consecutiveFail = 0
		res.RowsSaved += n
	}

	res.Duration = time.Since(started)
	return res, nil
}

// fetchWithRetry retries exactly once, and only for 429.
//
// 429는 기다리면 풀리는 제약이라 실패로 다루지 않는다. 그 밖의 오류를 재시도하지
// 않는 이유는 이 잡이 3,000종목을 돌기 때문이다 — 한 종목에 매달리는 것보다
// 건너뛰고 다음 실행에서 다시 받는 편이 낫다(-resume이 그래서 있다).
func (b *EarningsSurpriseBackfill) fetchWithRetry(
	ctx context.Context, symbol string, pace time.Duration,
) ([]model.EarningsSurpriseRow, error) {
	rows, err := b.finnhub.FetchSurpriseHistory(ctx, symbol)
	if err != nil && errorsIsRateLimited(err) {
		backoff := 60 * time.Second
		if pace > backoff {
			backoff = pace
		}
		log.Printf("earnings surprise backfill %s: 429 — %v 후 1회 재시도", symbol, backoff)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		rows, err = b.finnhub.FetchSurpriseHistory(ctx, symbol)
	}
	if err != nil {
		return nil, err
	}
	return surpriseRows(symbol, rows), nil
}

// candidates resolves the symbol list for this run.
func (b *EarningsSurpriseBackfill) candidates(
	ctx context.Context, opts BackfillOptions,
) ([]string, error) {
	var symbols []string
	if len(opts.Symbols) > 0 {
		symbols = append(symbols, opts.Symbols...)
	} else {
		// 「해외에서 실적을 한 번이라도 확정 발표한 종목」이 기본 대상이다. 예정만
		// 있는 종목(4,550 중 1,507)은 과거 분기가 없을 가능성이 높아 호출이 헛돈다.
		var err error
		symbols, err = b.events.ConfirmedSymbols(ctx, string(model.AssetUS))
		if err != nil {
			return nil, err
		}
	}

	if opts.Resume {
		done, err := b.history.SymbolsWithHistory(ctx, string(model.AssetUS))
		if err != nil {
			return nil, err
		}
		kept := symbols[:0:0]
		for _, s := range symbols {
			if _, ok := done[s]; ok {
				continue
			}
			kept = append(kept, s)
		}
		if skipped := len(symbols) - len(kept); skipped > 0 {
			log.Printf("earnings surprise backfill: %d종목은 이미 이력이 있어 건너뜀 (-resume=false로 재수집)", skipped)
		}
		symbols = kept
	}

	if opts.Limit > 0 && len(symbols) > opts.Limit {
		symbols = symbols[:opts.Limit]
	}
	return symbols, nil
}

// 문자열 매칭이 아니라 errors.Is로 가른다 — 메시지에 "429"가 우연히 들어간
// 다른 오류(예: 심볼에 429가 있는 종목)를 rate limit으로 오인하면, 그 종목마다
// 60초를 자며 잡이 몇 시간 늘어난다.
func errorsIsRateLimited(err error) bool { return errors.Is(err, errFinnhubRateLimited) }

func errorsIsNotEntitled(err error) bool { return errors.Is(err, ErrFinnhubNotEntitled) }
