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
}

// SymbolsWithoutCloses lists active master symbols that have no stored close at
// or after `since`, oldest-listed first for a stable resume order.
// Empty assetTypes means every market.
func (s *DailyCloseService) SymbolsWithoutCloses(ctx context.Context, since string, limit int, assetTypes ...model.QuoteAssetType) ([]string, int, error) {
	if s == nil || s.db == nil {
		return nil, 0, nil
	}

	const countQuery = `
		SELECT COUNT(*) FROM stocks st
		 WHERE st.status = 'NORMAL'
		   AND NOT EXISTS (
		       SELECT 1 FROM stock_daily_closes dc
		        WHERE dc.symbol = st.symbol AND dc.trade_date >= ?
		   )`
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

	const from = `
		  FROM stocks st
		  LEFT JOIN (
		      SELECT symbol, MAX(trade_date) AS newest
		        FROM stock_daily_closes GROUP BY symbol
		  ) d ON d.symbol = st.symbol
		 WHERE st.status = 'NORMAL'
		   AND (d.newest IS NULL OR d.newest < ?)`
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
		if err != nil {
			res.Failed++
			log.Printf("daily close fetch %s: %v", symbol, err)
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
		if err != nil {
			res.Failed++
			log.Printf("daily close history fetch %s: %v", symbol, err)
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
	recentDailyCloseBars   = 40
	historicalCandlePage   = 200
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
