package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// DailyCloseBackfillStart is the first trade date the 종가 store covers.
// 실적 「발표 후 주가」는 2026-01-01부터 숫자로 보여준다.
const DailyCloseBackfillStart = "2026-01-01"

// dailyCloseIngestDelay pads the wall-clock trigger past the official close so
// the final print has settled and the poller has written the day's 1d bar.
const dailyCloseIngestDelay = 20 * time.Minute

// dailyCloseIndexSchedules maps each tracked index to the session whose close
// defines its trade date.
var dailyCloseIndexSchedules = map[string]calendar.ScheduleID{
	"kospi":     calendar.ScheduleKRX,
	"kosdaq":    calendar.ScheduleKRX,
	"nasdaq":    calendar.ScheduleUS,
	"nasdaq100": calendar.ScheduleUS,
	"nq100":     calendar.ScheduleUS,
	"sp500":     calendar.ScheduleUS,
	"dow":       calendar.ScheduleUS,
	"vix":       calendar.ScheduleUS,
	"sox":       calendar.ScheduleUS,
	"gold":      calendar.ScheduleUS,
	"usd-krw":   calendar.ScheduleWeekdays,
	"btc":       calendar.ScheduleAlways,
}

// DailyCloseService owns the 일별 종가 store: a read-optimised projection of
// confirmed daily closes for stocks and indices, carrying the per-date LLM
// score and summary shown next to each row.
//
// candle_bars and the index poller stay the upstream source of truth; this
// service only ever restates their numbers through an idempotent upsert keyed
// on (symbol, trade_date), so a re-run can never invent or drift a price.
type DailyCloseService struct {
	db      *sql.DB
	stocks  *repository.StockDailyCloseRepo
	indices *repository.IndexDailyCloseRepo
	master  *repository.StockRepo
	index   *MarketIndexStore
}

func NewDailyCloseService(
	db *sql.DB,
	stocks *repository.StockDailyCloseRepo,
	indices *repository.IndexDailyCloseRepo,
	master *repository.StockRepo,
	index *MarketIndexStore,
) *DailyCloseService {
	return &DailyCloseService{db: db, stocks: stocks, indices: indices, master: master, index: index}
}

// dailyCloseSessionBars normalises candle_bars' 1d rows into exactly one
// session bar per (symbol, trade_date), with that date's previous close.
//
// Two defects in the upstream table make the normalisation necessary:
//
//   - Many dates carry TWO 1d rows, one at 00:00 and one at 15:00 KST, and the
//     00:00 row repeats the *previous* session's close. Taking rows as-is
//     doubles the series and produces change rates alternating between 0% and
//     a doubled move. ROW_NUMBER keeps the latest bar of each date, which is
//     the real session bar.
//   - Some rows are dated on a weekend, which no equity market trades. Those
//     are upstream noise and must not surface as a 종가.
//
// prev_close is taken with LAG over the de-duplicated series, so it is the
// previous *stored session*, not merely the previous row.
const dailyCloseSessionBars = `
	SELECT
		x.symbol,
		x.trade_date,
		x.close_price,
		x.volume,
		LAG(x.close_price) OVER (PARTITION BY x.symbol ORDER BY x.trade_date) AS prev_close
	FROM (
		SELECT
			cb.symbol,
			DATE(cb.bar_time) AS trade_date,
			cb.close_price,
			cb.volume,
			ROW_NUMBER() OVER (
				PARTITION BY cb.symbol, DATE(cb.bar_time)
				ORDER BY cb.bar_time DESC
			) AS rn
		FROM candle_bars cb
		WHERE cb.bar_interval = '1d'
		  AND DAYOFWEEK(cb.bar_time) NOT IN (1, 7)
		  %s
	) x
	WHERE x.rn = 1`

// dailyCloseInsert is the shared projection INSERT. The %s is the source of
// session bars. ai_summary defaults to '' on insert and is deliberately absent
// from the UPDATE list so a stored analysis survives a re-run.
const dailyCloseInsert = `
INSERT INTO stock_daily_closes
	(symbol, trade_date, close_price, prev_close, change_amount, change_rate, volume, currency, ai_summary, source)
SELECT
	d.symbol,
	d.trade_date,
	d.close_price,
	d.prev_close,
	CASE WHEN d.prev_close IS NULL OR d.prev_close = 0 THEN NULL
	     ELSE d.close_price - d.prev_close END,
	CASE WHEN d.prev_close IS NULL OR d.prev_close = 0 THEN NULL
	     ELSE (d.close_price - d.prev_close) / d.prev_close * 100 END,
	d.volume,
	COALESCE(st.currency, 'KRW'),
	'',
	'candle_bars'
FROM (%s) d
LEFT JOIN stocks st ON st.symbol = d.symbol
WHERE %s
ON DUPLICATE KEY UPDATE
	close_price = VALUES(close_price),
	prev_close = VALUES(prev_close),
	change_amount = VALUES(change_amount),
	change_rate = VALUES(change_rate),
	volume = VALUES(volume),
	currency = VALUES(currency),
	source = VALUES(source)`

// assetTypeFilter builds the WHERE fragment and args restricting a projection
// to specific markets. Empty assetTypes means every market — used by the
// combined CLI backfill, never by a market-specific worker.
//
// Splitting stock ingestion by asset_type is what makes the domestic and
// overseas daily-close workers independent: a slow or stuck US run must not
// block or delay the KRX one, and vice versa, since they now run in separate
// processes on separate market-close triggers.
func assetTypeFilter(assetTypes []model.QuoteAssetType) (string, []any) {
	if len(assetTypes) == 0 {
		return "", nil
	}
	placeholders := make([]string, len(assetTypes))
	args := make([]any, len(assetTypes))
	for i, t := range assetTypes {
		placeholders[i] = "?"
		args[i] = string(t)
	}
	return " AND st.asset_type IN (" + strings.Join(placeholders, ",") + ")", args
}

// ParseDailyCloseMarket maps CLI/admin market scope to asset types.
// Empty or "all" means every market; "us"/"overseas" means US+ETF only.
func ParseDailyCloseMarket(raw string) ([]model.QuoteAssetType, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "all":
		return nil, nil
	case "domestic", "kr", "krx":
		return []model.QuoteAssetType{model.AssetDomestic}, nil
	case "us", "overseas", "global":
		return []model.QuoteAssetType{model.AssetUS, model.AssetETF}, nil
	default:
		return nil, fmt.Errorf("unknown market %q (use all, domestic, or us)", raw)
	}
}

// BackfillStocksFromCandleBars projects every confirmed 1d session bar at or
// after `since` into stock_daily_closes in a single SQL pass, restricted to
// `assetTypes` (empty = every market).
//
// This is the whole stock backfill: candle_bars already holds the closes, so
// there is nothing to re-fetch from an upstream API. Symbols with no daily bars
// simply have no close to load — see FetchMissingCloses for widening that.
func (s *DailyCloseService) BackfillStocksFromCandleBars(ctx context.Context, since string, assetTypes ...model.QuoteAssetType) (int64, error) {
	return s.BackfillStocksFromCandleBarsRange(ctx, since, "", assetTypes...)
}

// BackfillStocksFromCandleBarsRange projects session bars with trade_date in
// [since, until] (until empty = no upper bound). Chunking by month keeps each
// SQL pass short enough to run safely alongside the live app.
func (s *DailyCloseService) BackfillStocksFromCandleBarsRange(ctx context.Context, since, until string, assetTypes ...model.QuoteAssetType) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}

	sessionBars := fmt.Sprintf(dailyCloseSessionBars, "")
	filter, filterArgs := assetTypeFilter(assetTypes)
	where := "d.trade_date >= ?"
	args := []any{since}
	if until != "" {
		where += " AND d.trade_date <= ?"
		args = append(args, until)
	}
	query := fmt.Sprintf(dailyCloseInsert, sessionBars, where+filter)

	args = append(args, filterArgs...)
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("backfill stock daily closes: %w", err)
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

// PruneWeekendRows removes stored closes dated on a Saturday or Sunday. No
// equity market trades then, so such a row can only have come from a bad
// upstream bar — and because the projection now skips weekends, an already
// stored bad row would never be corrected by a re-run.
func (s *DailyCloseService) PruneWeekendRows(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM stock_daily_closes WHERE DAYOFWEEK(trade_date) IN (1, 7)`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// IngestStockCloses writes the confirmed closes for one trading day from the
// day's 1d bars, restricted to `assetTypes` (empty = every market). Used by the
// post-close scheduler; `day` is interpreted in the schedule's own timezone.
func (s *DailyCloseService) IngestStockCloses(ctx context.Context, day time.Time, schedule calendar.ScheduleID, assetTypes ...model.QuoteAssetType) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	date := day.In(calendar.LocationFor(schedule)).Format("2006-01-02")

	// Same normalisation as the backfill, narrowed to one date. The inner scan
	// keeps a 30-day lead-in so LAG still sees the genuine previous session
	// rather than the first row of the filtered set.
	sessionBars := fmt.Sprintf(dailyCloseSessionBars,
		"AND cb.bar_time >= DATE_SUB(?, INTERVAL 30 DAY)")
	filter, filterArgs := assetTypeFilter(assetTypes)
	query := fmt.Sprintf(dailyCloseInsert, sessionBars, "d.trade_date = ?"+filter)

	args := append([]any{date, date}, filterArgs...)
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("ingest stock closes %s: %w", date, err)
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

// IngestIndexCloses snapshots the current index values as `day`'s close. Called
// after each market's regular session ends, when market_indices holds the
// settled figure.
func (s *DailyCloseService) IngestIndexCloses(ctx context.Context, day time.Time, schedule calendar.ScheduleID) (int, error) {
	if s == nil || s.indices == nil || s.index == nil {
		return 0, nil
	}
	indices, _ := s.index.List(ctx)

	loc := calendar.LocationFor(schedule)
	date := day.In(loc).Format("2006-01-02")

	rows := make([]repository.IndexDailyCloseRow, 0, len(indices))
	for _, idx := range indices {
		if dailyCloseIndexSchedules[idx.ID] != schedule {
			continue
		}
		if idx.Value <= 0 {
			continue
		}
		row := repository.IndexDailyCloseRow{
			IndexID:   idx.ID,
			TradeDate: day.In(loc),
			Close:     idx.Value,
			Currency:  string(idx.Currency),
			Source:    "index_poller",
		}
		if prev, ok, err := s.indices.LastCloseBefore(ctx, idx.ID, date); err == nil && ok && prev > 0 {
			row.PrevClose = sql.NullFloat64{Float64: prev, Valid: true}
			row.ChangeAmount = sql.NullFloat64{Float64: idx.Value - prev, Valid: true}
			row.ChangeRate = sql.NullFloat64{Float64: (idx.Value - prev) / prev * 100, Valid: true}
		}
		rows = append(rows, row)
	}

	if err := s.indices.UpsertBatch(ctx, rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// AttachAnalysis stores the score and AI summary for one symbol/date. Returns
// false when no close row exists for that date yet — the caller should not
// treat that as an error, the price ingestion simply has not run.
func (s *DailyCloseService) AttachAnalysis(ctx context.Context, a model.DailyCloseAnalysis) (bool, error) {
	if s == nil || s.stocks == nil {
		return false, nil
	}
	return s.stocks.UpsertAnalysis(ctx, a)
}

// StockSeries returns a symbol's daily closes, newest first.
func (s *DailyCloseService) StockSeries(ctx context.Context, symbol string, limit int) (*model.DailyCloseSeries, error) {
	if s == nil || s.stocks == nil {
		return &model.DailyCloseSeries{Rows: []model.DailyCloseRow{}}, nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	rows, err := s.stocks.List(ctx, symbol, limit)
	if err != nil {
		return nil, err
	}

	out := &model.DailyCloseSeries{
		Symbol:   symbol,
		Currency: "KRW",
		Rows:     make([]model.DailyCloseRow, 0, len(rows)),
	}
	if s.master != nil {
		if stock, err := s.master.GetBySymbol(ctx, symbol); err == nil && stock != nil {
			out.Name = stock.Name
			out.Currency = string(stock.Currency)
		}
	}

	for i, row := range rows {
		item := model.DailyCloseRow{
			TradeDate:      row.TradeDate.Format("2006-01-02"),
			Weekday:        model.WeekdayKo(int(row.TradeDate.Weekday())),
			Close:          row.Close,
			Volume:         row.Volume,
			AISummary:      row.AISummary,
			AnalysisSource: row.AnalysisSource,
		}
		if row.Currency != "" {
			out.Currency = row.Currency
		}
		if row.Score.Valid {
			score := int(row.Score.Int64)
			item.Score = &score
		}

		prev, ok := row.PrevClose.Float64, row.PrevClose.Valid
		// Rows are newest-first, so the next element is the previous session —
		// a usable fallback when the stored prev_close is missing.
		if !ok && i+1 < len(rows) {
			prev, ok = rows[i+1].Close, rows[i+1].Close > 0
		}
		if ok && prev > 0 {
			change := row.Close - prev
			rate := change / prev * 100
			item.PrevClose = &prev
			item.ChangeAmount = &change
			item.ChangeRate = &rate
		}
		out.Rows = append(out.Rows, item)
	}
	return out, nil
}

// IndexSeries returns an index's daily closes, newest first.
func (s *DailyCloseService) IndexSeries(ctx context.Context, indexID string, limit int) (*model.DailyCloseSeries, error) {
	if s == nil || s.indices == nil {
		return &model.DailyCloseSeries{Rows: []model.DailyCloseRow{}}, nil
	}
	indexID = strings.ToLower(strings.TrimSpace(indexID))
	rows, err := s.indices.List(ctx, indexID, limit)
	if err != nil {
		return nil, err
	}

	out := &model.DailyCloseSeries{
		IndexID:  indexID,
		Currency: "KRW",
		Rows:     make([]model.DailyCloseRow, 0, len(rows)),
	}
	for i, row := range rows {
		item := model.DailyCloseRow{
			TradeDate: row.TradeDate.Format("2006-01-02"),
			Weekday:   model.WeekdayKo(int(row.TradeDate.Weekday())),
			Close:     row.Close,
			AISummary: row.AISummary,
		}
		if row.Score.Valid {
			score := int(row.Score.Int64)
			item.Score = &score
		}
		if row.Currency != "" {
			out.Currency = row.Currency
		}
		prev, ok := row.PrevClose.Float64, row.PrevClose.Valid
		if !ok && i+1 < len(rows) {
			prev, ok = rows[i+1].Close, rows[i+1].Close > 0
		}
		if ok && prev > 0 {
			change := row.Close - prev
			rate := change / prev * 100
			item.PrevClose = &prev
			item.ChangeAmount = &change
			item.ChangeRate = &rate
		}
		out.Rows = append(out.Rows, item)
	}
	return out, nil
}

// RunMarketScheduler ingests closes for a restricted set of (schedule,
// assetType) pairs, once each session ends. It wakes on the earliest upcoming
// trigger rather than polling on a fixed tick, so a restart mid-day still
// lands on the right next close.
//
// Callers pass exactly the schedules that belong to their market: the domestic
// worker passes {KRX, Weekdays} with assetTypes={domestic} (KRX equities plus
// the KRW/USD rate, which is quoted continuously on weekdays); the overseas
// worker passes {US, Always} with assetTypes={us, etf} (US equities plus
// always-on instruments like BTC). Each worker is a separate process, so a
// stuck or slow run on one market can never delay or block the other.
func (s *DailyCloseService) RunMarketScheduler(
	ctx context.Context, jobID model.BatchJobID, runs *repository.BatchJobRunRepo,
	schedules []calendar.ScheduleID, assetTypes []model.QuoteAssetType,
) {
	if s == nil || s.db == nil {
		return
	}
	s.catchUpMissedSessions(ctx, jobID, runs, schedules, assetTypes)
	for {
		next, schedule := s.nextTrigger(time.Now(), schedules)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			RunScheduledJob(ctx, runs, jobID, func(ctx context.Context) (map[string]any, string, error) {
				stocks, indices := s.ingestSession(ctx, schedule, next, assetTypes)
				return map[string]any{"stockRows": stocks, "indexRows": indices},
					fmt.Sprintf("%s 세션 적재 완료 (종목 %d행, 지수 %d행)", schedule, stocks, indices), nil
			})
		}
	}
}

// catchUpMissedSessions runs once when the scheduler starts, so a session
// whose close+delay trigger already elapsed while this process was not
// running (a Docker restart landing between market close and the next
// session, which this project's "restart after every BE change" workflow
// makes routine) is not silently skipped forever. nextTrigger only ever
// searches forward from "now", so without this a missed window is gone for
// good — the ingest for that date never runs again on its own, which is
// exactly how stock_daily_closes got stuck several sessions behind while
// candle_bars (fed independently) kept moving, and every downstream reader
// keyed off "the newest stored close" — the 종합 점수 batch included — quietly
// kept scoring that stale date.
//
// Safe to run unconditionally on every start: ingestion is an idempotent
// upsert with no upstream API call, so re-running an already-current session
// costs one bounded query and changes nothing.
func (s *DailyCloseService) catchUpMissedSessions(
	ctx context.Context, jobID model.BatchJobID, runs *repository.BatchJobRunRepo,
	schedules []calendar.ScheduleID, assetTypes []model.QuoteAssetType,
) {
	now := time.Now()
	for _, schedule := range schedules {
		at, ok := s.lastCompletedTrigger(now, schedule)
		if !ok {
			continue
		}
		schedule := schedule
		RunScheduledJob(ctx, runs, jobID, func(ctx context.Context) (map[string]any, string, error) {
			stocks, indices := s.ingestSession(ctx, schedule, at, assetTypes)
			return map[string]any{"stockRows": stocks, "indexRows": indices},
				fmt.Sprintf("%s 세션 재적재(catch-up) 완료 (종목 %d행, 지수 %d행)", schedule, stocks, indices), nil
		})
	}
}

// lastCompletedTrigger finds the most recent already-elapsed post-close
// ingestion moment for `schedule`, searching backward up to 10 days — the
// mirror image of nextTrigger's forward search.
func (s *DailyCloseService) lastCompletedTrigger(now time.Time, schedule calendar.ScheduleID) (time.Time, bool) {
	loc := calendar.LocationFor(schedule)
	local := now.In(loc)
	for i := 0; i < 10; i++ {
		day := local.AddDate(0, 0, -i)
		if !calendar.IsTradingDay(schedule, day) {
			continue
		}
		at := calendar.SessionCloseAt(schedule, day).Add(dailyCloseIngestDelay)
		if at.After(now) {
			continue
		}
		return at, true
	}
	return time.Time{}, false
}

// nextTrigger picks the soonest post-close ingestion moment across schedules.
func (s *DailyCloseService) nextTrigger(now time.Time, schedules []calendar.ScheduleID) (time.Time, calendar.ScheduleID) {
	var bestAt time.Time
	var bestSchedule calendar.ScheduleID

	for _, schedule := range schedules {
		loc := calendar.LocationFor(schedule)
		local := now.In(loc)
		for i := 0; i < 10; i++ {
			day := local.AddDate(0, 0, i)
			if !calendar.IsTradingDay(schedule, day) {
				continue
			}
			at := calendar.SessionCloseAt(schedule, day).Add(dailyCloseIngestDelay)
			if !at.After(now) {
				continue
			}
			if bestAt.IsZero() || at.Before(bestAt) {
				bestAt, bestSchedule = at, schedule
			}
			break
		}
	}

	if bestAt.IsZero() {
		fallback := calendar.ScheduleKRX
		if len(schedules) > 0 {
			fallback = schedules[0]
		}
		return now.Add(time.Hour), fallback
	}
	return bestAt, bestSchedule
}

func (s *DailyCloseService) ingestSession(ctx context.Context, schedule calendar.ScheduleID, triggeredAt time.Time, assetTypes []model.QuoteAssetType) (stockRows int64, indexRows int) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// triggeredAt is already past the close, so its local calendar day is the
	// session that just ended.
	day := triggeredAt.In(calendar.LocationFor(schedule))

	stocks, err := s.IngestStockCloses(ctx, day, schedule, assetTypes...)
	if err != nil {
		log.Printf("daily close: stock ingest %s %s: %v", schedule, day.Format("2006-01-02"), err)
	}
	indices, err := s.IngestIndexCloses(ctx, day, schedule)
	if err != nil {
		log.Printf("daily close: index ingest %s %s: %v", schedule, day.Format("2006-01-02"), err)
	}
	log.Printf("daily close: %s %s — %d stock rows, %d index rows",
		schedule, day.Format("2006-01-02"), stocks, indices)
	return stocks, indices
}

// CoverageSummary answers the operator's question directly: how many symbols
// have closes, out of how many active ones, and how recent is the newest.
func (s *DailyCloseService) CoverageSummary(ctx context.Context, assetTypes ...model.QuoteAssetType) (symbols, activeSymbols int, latestTradeDate string, err error) {
	if s == nil || s.db == nil {
		return 0, 0, "", nil
	}
	filter, filterArgs := assetTypeFilter(assetTypes)
	// assetTypeFilter aliases the table as "st"; join it in for this read too.
	var latest sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT d.symbol), DATE_FORMAT(MAX(d.trade_date), '%Y-%m-%d')
		  FROM stock_daily_closes d
		  JOIN stocks st ON st.symbol = d.symbol
		 WHERE 1=1`+filter, filterArgs...).Scan(&symbols, &latest)
	if err != nil {
		return 0, 0, "", err
	}
	activeQuery := `SELECT COUNT(*) FROM stocks st WHERE st.status = 'NORMAL'` + filter
	if err = s.db.QueryRowContext(ctx, activeQuery, filterArgs...).Scan(&activeSymbols); err != nil {
		return symbols, 0, latest.String, err
	}
	return symbols, activeSymbols, latest.String, nil
}

// IndexCoverageSummary is the same question for the index store.
func (s *DailyCloseService) IndexCoverageSummary(ctx context.Context) (indices, rows int, latestTradeDate string, err error) {
	if s == nil || s.db == nil {
		return 0, 0, "", nil
	}
	var latest sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT index_id), COUNT(*), DATE_FORMAT(MAX(trade_date), '%Y-%m-%d')
		  FROM index_daily_closes`).Scan(&indices, &rows, &latest)
	if err != nil {
		return 0, 0, "", err
	}
	return indices, rows, latest.String, nil
}

// LatestStoredTradeDate is the newest session the close store holds, used as the
// "should be present" date for the stale-session fetch.
func (s *DailyCloseService) LatestStoredTradeDate(ctx context.Context) (string, error) {
	if s == nil || s.db == nil {
		return "", nil
	}
	var latest sql.NullString
	if err := s.db.QueryRowContext(ctx,
		`SELECT DATE_FORMAT(MAX(trade_date), '%Y-%m-%d') FROM stock_daily_closes`).Scan(&latest); err != nil {
		return "", err
	}
	return latest.String, nil
}
