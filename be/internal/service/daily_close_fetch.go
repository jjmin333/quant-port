package service

import (
	"context"
	"database/sql"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

// FetchMissingResult reports one widen-coverage run.
type FetchMissingResult struct {
	Attempted int
	Filled    int
	Empty     int
	Failed    int
	Remaining int
	/*
		Unknown은 **Toss가 취급하지 않는 종목** 수다(`Failed`의 부분집합).

		`Failed`와 갈라 적는 이유는 두 수가 다른 것을 뜻하기 때문이다. `Failed`는
		「다시 해 보면 될 수도 있다」이고 Unknown은 「내일도 같은 답이 온다」다.
		갈라 놓지 않아서 해외 구멍 메우기가 매 회차 「실패 3,254」를 적고 있었고,
		그 수를 보고도 고장인지 정상인지 알 수 없었다(2026-09-11).
	*/
	Unknown int
}

// SymbolsWithoutCloses lists active master symbols that have no stored close at
// or after `since`, oldest-listed first for a stable resume order.
// Empty assetTypes means every market.
/*
tossUnknownFilter — 「Toss가 모른다고 답한 종목」을 대상에서 뺀다.

`markFetchOutcome`이 조건 셋을 모두 만족했을 때만 그 상태를 쓰므로(그 머리글 참조),
여기서는 그 기록을 그대로 신뢰한다. 30일이 지난 기록은 다시 대상이 된다 —
`syncNotFoundCooldown`과 같은 값이고, 그 상수의 이유도 같다.

`sync`가 배선되지 않았으면 아무것도 빼지 않는다(빈 문자열) — 예전 동작 그대로다.
*/
func (s *DailyCloseService) tossUnknownFilter() string {
	if s == nil || s.sync == nil {
		return ""
	}
	/*
		두 겹으로 거른다.

		 1. **유니버스**(`toss_tradable`) — `toss_universe` 잡이 하루 7호출로 받은
		    「토스가 취급하는 종목」이다. 권위 있는 답이고, 404를 한 번도 안 맞고 안다.
		 2. **404 기록**(`symbol_sync_state`) — 유니버스에는 있는데 캔들만 없는 종목이
		    남는다. 1번이 못 거른 것을 이쪽이 받는다.

		2번을 남겨 두는 이유: 유니버스는 「거래 지원」이고 이 잡이 묻는 것은 「일봉이
		있나」라서 완전히 같은 집합이 아니다. 상장 첫날 종목처럼 거래는 되는데 봉이
		없는 경우가 실제로 있다.
	*/
	return `
		   AND ` + repository.TossTradableFilter("st") + `
		   AND NOT EXISTS (
		       SELECT 1 FROM symbol_sync_state ss
		        WHERE ss.symbol = st.symbol
		          AND ss.kind = '` + model.SyncKindDailyCloseToss + `'
		          AND ss.status = '` + model.SyncNotFound + `'
		          AND ss.last_synced_at > NOW() - INTERVAL 30 DAY
		   )`
}

func (s *DailyCloseService) SymbolsWithoutCloses(ctx context.Context, since string, limit int, assetTypes ...model.QuoteAssetType) ([]string, int, error) {
	if s == nil || s.db == nil {
		return nil, 0, nil
	}

	countQuery := `
		SELECT COUNT(*) FROM stocks st
		 WHERE st.status = 'NORMAL'
		   AND NOT EXISTS (
		       SELECT 1 FROM stock_daily_closes dc
		        WHERE dc.symbol = st.symbol AND dc.trade_date >= ?
		   )` + s.tossUnknownFilter()
	filter, filterArgs := assetTypeFilter(assetTypes)
	countSQL := countQuery + filter
	args := append([]any{since}, filterArgs...)

	var remaining int
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&remaining); err != nil {
		return nil, 0, err
	}

	query := strings.Replace(countSQL, "SELECT COUNT(*) FROM stocks st", "SELECT st.symbol FROM stocks st", 1)
	query += " ORDER BY st.symbol ASC"
	if limit > 0 {
		query += " LIMIT " + strconv.Itoa(limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, remaining, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, remaining, err
		}
		out = append(out, symbol)
	}
	return out, remaining, rows.Err()
}

// SymbolsStaleBefore lists active master symbols whose most recent stored close
// predates `asOf` — including symbols with no close at all.
//
// This is deliberately a different question from SymbolsWithoutCloses. A symbol
// that already has July closes but stopped at 08-06 is invisible to that query
// (it has *some* row in the window), yet it is exactly the gap a user notices:
// the latest session is missing while older ones are fine. Selecting on the
// newest stored date instead of on "has any row" catches both cases.
func (s *DailyCloseService) SymbolsStaleBefore(ctx context.Context, asOf string, limit int, assetTypes ...model.QuoteAssetType) ([]string, int, error) {
	if s == nil || s.db == nil {
		return nil, 0, nil
	}

	from := `
		  FROM stocks st
		  LEFT JOIN (
		      SELECT symbol, MAX(trade_date) AS newest
		        FROM stock_daily_closes GROUP BY symbol
		  ) d ON d.symbol = st.symbol
		 WHERE st.status = 'NORMAL'
		   AND (d.newest IS NULL OR d.newest < ?)` + s.tossUnknownFilter()
	filter, filterArgs := assetTypeFilter(assetTypes)
	fromSQL := from + filter
	args := append([]any{asOf}, filterArgs...)

	var remaining int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+fromSQL, args...).Scan(&remaining); err != nil {
		return nil, 0, err
	}

	query := "SELECT st.symbol" + fromSQL + " ORDER BY st.symbol ASC"
	if limit > 0 {
		query += " LIMIT " + strconv.Itoa(limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, remaining, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, remaining, err
		}
		out = append(out, symbol)
	}
	return out, remaining, rows.Err()
}

// SymbolsHistoryIncomplete lists active symbols whose stored history does not
// reach `since` — they have some closes, but the oldest is too recent for
// earnings reactions in that window (e.g. Jul-only rows when Feb reactions need
// Mar closes). fetch-missing and fetch-stale do not catch this gap.
//
// A 14-day grace after `since` ignores New-Year-style gaps: if the oldest
// stored close is within two weeks of since, we treat history as starting OK.
func (s *DailyCloseService) SymbolsHistoryIncomplete(ctx context.Context, since string, limit int, assetTypes ...model.QuoteAssetType) ([]string, int, error) {
	if s == nil || s.db == nil {
		return nil, 0, nil
	}

	const from = `
		  FROM stocks st
		 INNER JOIN (
		      SELECT symbol, MIN(trade_date) AS oldest
		        FROM stock_daily_closes
		       GROUP BY symbol
		  ) d ON d.symbol = st.symbol
		 WHERE st.status = 'NORMAL'
		   AND d.oldest > DATE_ADD(?, INTERVAL 14 DAY)`
	filter, filterArgs := assetTypeFilter(assetTypes)
	fromSQL := from + filter
	args := append([]any{since}, filterArgs...)

	var remaining int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+fromSQL, args...).Scan(&remaining); err != nil {
		return nil, 0, err
	}

	query := "SELECT st.symbol" + fromSQL + " ORDER BY st.symbol ASC"
	if limit > 0 {
		query += " LIMIT " + strconv.Itoa(limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, remaining, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, remaining, err
		}
		out = append(out, symbol)
	}
	return out, remaining, rows.Err()
}

// HistoryIncompleteCount returns how many active symbols still need -fetch-history.
func (s *DailyCloseService) HistoryIncompleteCount(ctx context.Context, since string, assetTypes ...model.QuoteAssetType) (int, error) {
	_, remaining, err := s.SymbolsHistoryIncomplete(ctx, since, 0, assetTypes...)
	return remaining, err
}

// FetchHistoryCloses walks symbols whose oldest stored close is after `since`
// and pages Toss daily candles backward until that date is covered.
func (s *DailyCloseService) FetchHistoryCloses(
	ctx context.Context,
	client *toss.Client,
	bars *repository.CandleBarRepo,
	since string,
	limit int,
	delay time.Duration,
	logEvery int,
	assetTypes ...model.QuoteAssetType,
) (FetchMissingResult, error) {
	var res FetchMissingResult
	if s == nil || client == nil || bars == nil {
		return res, nil
	}

	symbols, remaining, err := s.SymbolsHistoryIncomplete(ctx, since, limit, assetTypes...)
	if err != nil {
		return res, err
	}
	res.Remaining = remaining

	sinceDate, err := time.Parse("2006-01-02", since)
	if err != nil {
		return res, err
	}
	return s.fetchSymbolsDeep(ctx, client, bars, symbols, sinceDate, delay, logEvery, res)
}

// FetchHistoryForSymbols pages daily candles backward for an explicit symbol
// list, instead of "every symbol whose history starts too late".
//
// 왜 목록을 받는가: `SymbolsHistoryIncomplete`는 조건에 맞는 **전 종목**을 잡는다
// (실측 미국 13,000개 이상). 하지만 메우려는 구멍이 특정 종목 몇 백 개로 정해져
// 있을 때가 있다 — 예를 들어 「2025년 실적 발표일에 종가가 없는 종목」. 그때
// 전 종목을 도는 것은 Toss 호출을 수십 배 쓰는 일이다.
//
// 페이싱·재개 방식은 FetchHistoryCloses와 같다. 한 종목이 실패해도 나머지는
// 계속하고, 다시 실행하면 이미 채워진 종목은 빨리 지나간다.
func (s *DailyCloseService) FetchHistoryForSymbols(
	ctx context.Context,
	client *toss.Client,
	bars *repository.CandleBarRepo,
	symbols []string,
	since string,
	delay time.Duration,
	logEvery int,
) (FetchMissingResult, error) {
	var res FetchMissingResult
	if s == nil || client == nil || bars == nil || len(symbols) == 0 {
		return res, nil
	}
	sinceDate, err := time.Parse("2006-01-02", since)
	if err != nil {
		return res, err
	}
	res.Remaining = len(symbols)
	return s.fetchSymbolsDeep(ctx, client, bars, symbols, sinceDate, delay, logEvery, res)
}

// FetchStaleCloses refreshes symbols whose newest stored close predates `asOf`,
// so a missed session is filled in rather than left as a hole under rows that
// already exist. Same pacing and resumability as FetchMissingCloses.
func (s *DailyCloseService) FetchStaleCloses(
	ctx context.Context,
	client *toss.Client,
	bars *repository.CandleBarRepo,
	asOf string,
	since string,
	limit int,
	delay time.Duration,
	logEvery int,
	assetTypes ...model.QuoteAssetType,
) (FetchMissingResult, error) {
	var res FetchMissingResult
	if s == nil || client == nil || bars == nil {
		return res, nil
	}

	symbols, remaining, err := s.SymbolsStaleBefore(ctx, asOf, limit, assetTypes...)
	if err != nil {
		return res, err
	}
	res.Remaining = remaining

	sinceDate, err := time.Parse("2006-01-02", since)
	if err != nil {
		return res, err
	}
	return s.fetchSymbols(ctx, client, bars, symbols, sinceDate, delay, logEvery, res)
}

// FetchMissingCloses pulls daily candles from Toss for symbols that have none
// stored and writes both candle_bars and the daily-close projection.
//
// Paced deliberately: Toss allows only a handful of candle requests per second
// app-wide and the live app shares that budget, so this walks symbols one at a
// time with `delay` between them. It is resumable — every run re-queries which
// symbols are still missing — so a partial run is progress, not waste.
func (s *DailyCloseService) FetchMissingCloses(
	ctx context.Context,
	client *toss.Client,
	bars *repository.CandleBarRepo,
	since string,
	limit int,
	delay time.Duration,
	logEvery int,
	assetTypes ...model.QuoteAssetType,
) (FetchMissingResult, error) {
	var res FetchMissingResult
	if s == nil || client == nil || bars == nil {
		return res, nil
	}

	symbols, remaining, err := s.SymbolsWithoutCloses(ctx, since, limit, assetTypes...)
	if err != nil {
		return res, err
	}
	res.Remaining = remaining

	sinceDate, err := time.Parse("2006-01-02", since)
	if err != nil {
		return res, err
	}
	return s.fetchSymbols(ctx, client, bars, symbols, sinceDate, delay, logEvery, res)
}

// fetchSymbols walks one symbol list, pacing requests and tallying outcomes.
// Shared by the missing-symbol and stale-date entry points so both honour the
// same rate-limit pacing and cancellation behaviour.
func (s *DailyCloseService) fetchSymbols(
	ctx context.Context,
	client *toss.Client,
	bars *repository.CandleBarRepo,
	symbols []string,
	since time.Time,
	delay time.Duration,
	logEvery int,
	res FetchMissingResult,
) (FetchMissingResult, error) {
	if len(symbols) > 0 {
		log.Printf("fetch batch: %d symbols this run (%d total remaining before limit)", len(symbols), res.Remaining)
	}
	for i, symbol := range symbols {
		if ctx.Err() != nil {
			break
		}
		if i > 0 && delay > 0 {
			select {
			case <-ctx.Done():
				return res, ctx.Err()
			case <-time.After(delay):
			}
		}
		res.Attempted++

		rows, err := s.fetchSymbolDailyCloses(ctx, client, bars, symbol, since)
		// **결과를 기억한다** — 안 그러면 상대가 모르는 종목을 매 회차 다시 묻는다
		// (2026-09-11 실측: 해외 회차마다 3,250여 건, 하루 두 번). 무엇을 근거로
		// 빼는지는 `SetSyncState` 머리글에 있다.
		skipped := s.markFetchOutcome(ctx, symbol, rows, err)
		if err != nil {
			res.Failed++
			if skipped {
				res.Unknown++
				// 한 줄씩 찍으면 회차마다 3,000줄이 된다. 합계만 남긴다.
			} else {
				log.Printf("daily close fetch %s: %v", symbol, err)
			}
		} else if rows == 0 {
			res.Empty++
		} else {
			res.Filled++
			log.Printf("fetch ok %s: %d rows", symbol, rows)
		}
		if logEvery > 0 && (i+1)%logEvery == 0 {
			log.Printf("fetch progress: %d/%d in batch · filled=%d empty=%d failed=%d",
				i+1, len(symbols), res.Filled, res.Empty, res.Failed)
		}
	}
	return res, nil
}

func (s *DailyCloseService) fetchSymbolsDeep(
	ctx context.Context,
	client *toss.Client,
	bars *repository.CandleBarRepo,
	symbols []string,
	since time.Time,
	delay time.Duration,
	logEvery int,
	res FetchMissingResult,
) (FetchMissingResult, error) {
	if len(symbols) > 0 {
		log.Printf("fetch-history batch: %d symbols this run (%d incomplete before limit)", len(symbols), res.Remaining)
	}
	for i, symbol := range symbols {
		if ctx.Err() != nil {
			break
		}
		if i > 0 && delay > 0 {
			select {
			case <-ctx.Done():
				return res, ctx.Err()
			case <-time.After(delay):
			}
		}
		res.Attempted++

		rows, err := s.fetchSymbolDailyClosesDeep(ctx, client, bars, symbol, since)
		// 얕은 경로와 **같은 기억을 쓴다** — 두 잡이 같은 종목 집합을 도는데
		// 한쪽만 기억하면 다른 쪽이 그 종목을 계속 묻는다.
		skipped := s.markFetchOutcome(ctx, symbol, rows, err)
		if err != nil {
			res.Failed++
			if skipped {
				res.Unknown++
			} else {
				log.Printf("daily close history fetch %s: %v", symbol, err)
			}
		} else if rows == 0 {
			res.Empty++
		} else {
			res.Filled++
			log.Printf("fetch-history ok %s: %d rows", symbol, rows)
		}
		if logEvery > 0 && (i+1)%logEvery == 0 {
			log.Printf("fetch-history progress: %d/%d in batch · filled=%d empty=%d failed=%d",
				i+1, len(symbols), res.Filled, res.Empty, res.Failed)
		}
	}
	return res, nil
}

// fetchSymbolDailyCloses fetches one symbol's recent daily candles, persists
// them to candle_bars, and projects the ones at or after `since`.
func (s *DailyCloseService) fetchSymbolDailyCloses(
	ctx context.Context,
	client *toss.Client,
	bars *repository.CandleBarRepo,
	symbol string,
	since time.Time,
) (int, error) {
	params := toss.GetCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1d,
		Count:    recentDailyCloseBars,
	}
	page, err := s.getCandlesWithRetry(ctx, client, params)
	if err != nil {
		return 0, err
	}
	if page == nil || len(page.Candles) == 0 {
		return 0, nil
	}
	return s.persistDailyCandles(ctx, bars, symbol, since, page.Candles)
}

const (
	recentDailyCloseBars     = 40
	historicalCandlePage     = 200
	maxHistoricalCandlePages = 8
)

// fetchSymbolDailyClosesDeep pages Toss 1d candles backward until `since` is
// covered (or history ends). One symbol may cost several API calls.
func (s *DailyCloseService) fetchSymbolDailyClosesDeep(
	ctx context.Context,
	client *toss.Client,
	bars *repository.CandleBarRepo,
	symbol string,
	since time.Time,
) (int, error) {
	var all []toss.Candle
	before := ""
	reachedSince := false

	for pageNum := 0; pageNum < maxHistoricalCandlePages; pageNum++ {
		params := toss.GetCandlesParams{
			Symbol:   symbol,
			Interval: toss.CandleInterval1d,
			Count:    historicalCandlePage,
		}
		if before != "" {
			params.Before = before
		}
		page, err := s.getCandlesWithRetry(ctx, client, params)
		if err != nil {
			return 0, err
		}
		if page == nil || len(page.Candles) == 0 {
			break
		}
		all = append(all, page.Candles...)

		oldest, ok := oldestCandleDay(page.Candles)
		if ok && !oldest.After(since) {
			reachedSince = true
			break
		}
		if page.NextBefore == nil {
			break
		}
		before = *page.NextBefore
	}

	if len(all) == 0 {
		return 0, nil
	}
	rows, err := s.persistDailyCandles(ctx, bars, symbol, since, all)
	if err != nil {
		return 0, err
	}
	if !reachedSince && rows > 0 {
		// Partial history is still progress — caller re-runs until Remaining=0.
		return rows, nil
	}
	return rows, nil
}

func (s *DailyCloseService) getCandlesWithRetry(
	ctx context.Context, client *toss.Client, params toss.GetCandlesParams,
) (*toss.CandlePageResponse, error) {
	page, err := client.GetCandles(ctx, params)
	for attempt := 0; attempt < 3 && err != nil; attempt++ {
		wait, limited := tossRateLimitWait(err)
		if !limited {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		page, err = client.GetCandles(ctx, params)
	}
	return page, err
}

type dailyClosePoint struct {
	day    time.Time
	close  float64
	volume int64
}

func oldestCandleDay(candles []toss.Candle) (time.Time, bool) {
	var oldest time.Time
	ok := false
	for _, c := range candles {
		ts, err := time.Parse(time.RFC3339, c.Timestamp)
		if err != nil {
			continue
		}
		if !ok || ts.Before(oldest) {
			oldest = ts
			ok = true
		}
	}
	return oldest, ok
}

func (s *DailyCloseService) persistDailyCandles(
	ctx context.Context,
	bars *repository.CandleBarRepo,
	symbol string,
	since time.Time,
	raw []toss.Candle,
) (int, error) {
	candles := make([]model.Candle, 0, len(raw))
	parsed := make([]dailyClosePoint, 0, len(raw))

	for _, c := range raw {
		ts, err := time.Parse(time.RFC3339, c.Timestamp)
		if err != nil {
			continue
		}
		closePrice := parseFloatOrZero(c.ClosePrice)
		if closePrice <= 0 {
			continue
		}
		candles = append(candles, model.Candle{
			Time:   c.Timestamp,
			Open:   parseFloatOrZero(c.OpenPrice),
			High:   parseFloatOrZero(c.HighPrice),
			Low:    parseFloatOrZero(c.LowPrice),
			Close:  closePrice,
			Volume: int64(parseFloatOrZero(c.Volume)),
		})
		parsed = append(parsed, dailyClosePoint{
			day: ts, close: closePrice, volume: int64(parseFloatOrZero(c.Volume)),
		})
	}
	if len(candles) == 0 {
		return 0, nil
	}

	if err := bars.UpsertBatch(ctx, symbol, model.Interval1d, candles); err != nil {
		return 0, err
	}

	sort.Slice(parsed, func(i, j int) bool { return parsed[i].day.Before(parsed[j].day) })

	currency := "KRW"
	if s.master != nil {
		if stock, err := s.master.GetBySymbol(ctx, symbol); err == nil && stock != nil && stock.Currency != "" {
			currency = string(stock.Currency)
		}
	}

	out := make([]repository.StockDailyCloseRow, 0, len(parsed))
	for i, p := range parsed {
		if p.day.Before(since) {
			continue
		}
		row := repository.StockDailyCloseRow{
			Symbol:    strings.ToUpper(symbol),
			TradeDate: p.day,
			Close:     p.close,
			Volume:    p.volume,
			Currency:  currency,
			Source:    "toss",
		}
		if i > 0 && parsed[i-1].close > 0 {
			prev := parsed[i-1].close
			row.PrevClose = sql.NullFloat64{Float64: prev, Valid: true}
			row.ChangeAmount = sql.NullFloat64{Float64: p.close - prev, Valid: true}
			row.ChangeRate = sql.NullFloat64{Float64: (p.close - prev) / prev * 100, Valid: true}
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		return 0, nil
	}
	if err := s.stocks.UpsertBatch(ctx, out); err != nil {
		return 0, err
	}
	return len(out), nil
}

func parseFloatOrZero(raw string) float64 {
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(raw), ",", ""), 64)
	if err != nil {
		return 0
	}
	return v
}

/*
SessionCloseCoverage — 한 세션 날짜의 종가를 실제로 몇 종목이 가지고 있는지 센다.

구멍 메우기 잡의 「시도·채움」 숫자는 **대상 선정이 맞았을 때만** 성과를 뜻한다.
선정이 어긋나면 적은 대상을 전부 채우고 done으로 끝나면서 그날 종가는 비어 있다.
결과를 직접 재는 이 숫자가 그 차이를 드러낸다.

# 분모에서 **Toss가 취급하지 않는 종목을 뺀다** (2026-09-11 사용자 요청 7)

예전 `total`은 마스터의 활성 종목 수 전부였다. 그래서 해외 커버리지가 구조적으로
75%에 붙어 있었고(실측 10,163/13,506), 「90% 미만」 경고가 **고쳐질 수 없는 조건으로
매 회차 울렸다** — 울리는 경고는 곧 안 보는 경고가 된다.

빠지는 것은 우리가 **직접 물어보고 「그런 종목 없다」는 답을 받은** 종목뿐이다
(`markFetchOutcome`의 조건 셋 + 30일 쿨다운). 상장폐지 직전·거래정지처럼 상대가
알고는 있는 종목은 그대로 분모에 남는다 — 그것들이 비어 있는 것은 진짜 구멍이다.

`sync`가 배선되지 않았으면 예전과 같은 수가 나온다.
*/
func (s *DailyCloseService) SessionCloseCoverage(
	ctx context.Context, tradeDate string, assetTypes ...model.QuoteAssetType,
) (have, total int, err error) {
	if s == nil || s.db == nil || tradeDate == "" {
		return 0, 0, nil
	}
	filter, args := assetTypeFilter(assetTypes)
	query := `
		SELECT COUNT(*),
		       SUM(EXISTS (SELECT 1 FROM stock_daily_closes c
		                    WHERE c.symbol = st.symbol AND c.trade_date = ?))
		  FROM stocks st
		 WHERE st.status = 'NORMAL'` + filter + s.tossUnknownFilter()
	var haveNull sql.NullInt64
	if err := s.db.QueryRowContext(ctx, query, append([]any{tradeDate}, args...)...).
		Scan(&total, &haveNull); err != nil {
		return 0, 0, err
	}
	return int(haveNull.Int64), total, nil
}
