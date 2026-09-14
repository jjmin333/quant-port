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
	"github.com/enact/quant-port/be/internal/toss"
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
	// sync는 「Toss가 이 종목을 모른다」를 기억하는 자리다. nil이면 기록도 건너뛰기도
	// 하지 않는다 — 예전 동작 그대로다(SetSyncState 머리글 참조).
	sync *repository.SyncStateRepo
}

/*
SetSyncState — 구멍 메우기가 헛도는 것을 멈추게 한다 (2026-09-11 사용자 요청 7).

# 실측

해외 구멍 메우기(`daily_close_fetch_stale`)의 원장을 보면 같은 모양이 매번 반복된다:

	09-10 05:20  시도 3,350 · 채움    85 · 실패 3,254 → 종가 보유 10,168/13,507 (75%)
	09-11 13:15  시도 8,655 · 채움 5,386 · 실패 3,258 → 종가 보유 10,163/13,506 (75%)

국내 같은 잡은 4,412 시도에 실패 138(97% 보유)이다. 해외만 **매 회차 3,250여 건이
실패하고, 다음 회차에 같은 종목을 또 시도한다** — 하루 두 번이니 6,500여 회의 Toss
호출이 「내일도 같은 답이 올 질문」이다. 표본 조사에서 워런트(`…W`)·우선주(`…^C`)·
라이츠(`…R`)가 `stock-not-found`로 돌아왔다: 우리 카탈로그의 NORMAL 종목이 Toss가
취급하는 종목의 **상위집합**이라서다.

# 어떻게 거르나 — 조건 셋을 **모두** 만족해야 건너뛴다

종가는 크리티컬 데이터라(AGENT_STATE 개발 원칙) 「한 번 실패했으니 빼자」로는 부족하다.

 1. **오류 코드가 `stock-not-found`일 때만.** 429·타임아웃·5xx는 절대 아니다 —
    그쪽은 다시 물어야 나아진다. Toss 장애는 종목별 `stock-not-found`가 아니라
    5xx·타임아웃으로 온다.
 2. **한 번도 성공한 적 없는 종목만**(`last_success_at IS NULL`). 예전에 종가를 준
    적이 있는 종목이 지금 404를 주면 그건 상대 쪽 일시 문제일 수 있으므로 `error`로
    남겨 계속 묻는다.
 3. **30일 쿨다운.** 영구가 아니다(`syncNotFoundCooldown`). 신규 상장이나 상대의
    카탈로그 변경이 30일 안에 돌아온다.

성공·빈응답도 기록한다 — 그래야 조건 2가 의미를 갖고, 한때 404였던 종목이 한 번
성공하면 그 뒤로는 절대 건너뛰지 않는다.
*/
func (s *DailyCloseService) SetSyncState(sync *repository.SyncStateRepo) {
	if s != nil {
		s.sync = sync
	}
}

/*
markFetchOutcome — 한 종목의 Toss 조회 결과를 기억한다.

`skip=true`는 「이 종목을 당분간 대상에서 빼라」는 뜻이고, 위 조건 셋을 모두
만족했을 때만이다.
*/
func (s *DailyCloseService) markFetchOutcome(
	ctx context.Context, symbol string, rows int, err error,
) (skipped bool) {
	if s == nil || s.sync == nil {
		return false
	}
	kind := model.SyncKindDailyCloseToss
	switch {
	case err == nil && rows > 0:
		_ = s.sync.Mark(ctx, repository.SyncOutcome{
			Kind: kind, Symbol: symbol, Status: model.SyncOK, RowsStored: rows,
		})
	case err == nil:
		// 응답은 왔는데 봉이 없다 — 거래가 없는 종목이다. 상대가 아는 종목이므로
		// `empty`는 성공으로 센다(SyncStateRepo.Mark 참조). 다음 회차에 다시 온다.
		_ = s.sync.Mark(ctx, repository.SyncOutcome{
			Kind: kind, Symbol: symbol, Status: model.SyncEmpty,
		})
	case toss.IsStockNotFound(err):
		everWorked, wasErr := s.sync.WasSynced(ctx, kind, symbol)
		if wasErr != nil || everWorked {
			// 조건 2 탈락 — 예전에 종가를 준 종목이다. 계속 묻는다.
			_ = s.sync.Mark(ctx, repository.SyncOutcome{
				Kind: kind, Symbol: symbol, Status: model.SyncError, Message: err.Error(),
			})
			return false
		}
		_ = s.sync.Mark(ctx, repository.SyncOutcome{
			Kind: kind, Symbol: symbol, Status: model.SyncNotFound, Message: err.Error(),
		})
		return true
	default:
		_ = s.sync.Mark(ctx, repository.SyncOutcome{
			Kind: kind, Symbol: symbol, Status: model.SyncError, Message: err.Error(),
		})
	}
	return false
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
// session bars. ai_summary defaults to the empty string on insert and is absent
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
// [since, until] (until empty = no upper bound).
//
// # 날짜 경계를 안쪽 스캔에도 건다
//
// 예전에는 바깥 WHERE에만 걸었다(`dailyCloseSessionBars`에 빈 문자열). 그러면
// **candle_bars 전체**에 ROW_NUMBER()·LAG()를 돌린 뒤에야 날짜로 거르게 된다 —
// 한 달을 뽑든 다섯 달을 뽑든 비용이 같고, 실측 250만 행에서 **15분을 넘겨도
// 안 끝났다**(readTimeout에 잘림). 월 단위로 쪼개도 소용이 없는 이유이자,
// 쪼갠 만큼 같은 전체 스캔을 반복하게 되는 이유였다.
//
// IngestStockCloses(일일 잡)는 처음부터 안쪽에 필터를 걸어 빨랐다. 같은 방식을
// 여기에도 쓴다.
//
// **30일 여유(lead-in)가 정확성의 핵심이다.** LAG는 필터링된 집합 안에서만
// 이전 행을 보므로, 안쪽 하한을 `since`에 딱 맞추면 구간 첫날의 prev_close가
// NULL이 되어 등락률이 통째로 빈다. 30일을 앞서 읽어 진짜 직전 세션을 보게 한다
// (휴장이 길어도 덮는 값 — IngestStockCloses가 같은 값을 쓴다).
func (s *DailyCloseService) BackfillStocksFromCandleBarsRange(ctx context.Context, since, until string, assetTypes ...model.QuoteAssetType) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}

	// 안쪽(서브쿼리) 인자가 먼저, 바깥 WHERE 인자가 나중, assetType 인자가 마지막.
	// 순서가 어긋나면 조용히 엉뚱한 구간을 넣는다.
	innerFilter := "AND cb.bar_time >= DATE_SUB(?, INTERVAL 30 DAY)"
	args := []any{since}
	if until != "" {
		// 상한은 여유가 필요 없다 — 그 뒤 세션은 이 구간의 어떤 행에도 안 쓰인다.
		// 하루를 더 두는 것은 bar_time이 자정이 아닌 경우를 위한 것이다.
		innerFilter += " AND cb.bar_time < DATE_ADD(?, INTERVAL 1 DAY)"
		args = append(args, until)
	}
	sessionBars := fmt.Sprintf(dailyCloseSessionBars, innerFilter)

	filter, filterArgs := assetTypeFilter(assetTypes)
	where := "d.trade_date >= ?"
	args = append(args, since)
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

	rows := make([]repository.IndexDailyCloseRow, 0, len(indices))
	for _, idx := range indices {
		if dailyCloseIndexSchedules[idx.ID] != schedule {
			continue
		}
		if idx.Value <= 0 {
			continue
		}
		/*
			행의 날짜는 **시세가 스스로 말하는 거래일**로 잡는다 — 세션이 끝난 날이 아니라.

			FRED 계열(나스닥·S&P·다우·VIX)은 관측치를 하루 늦게 공표한다. 금요일 장이
			끝난 뒤 값을 받아 오면 그건 목요일 종가인데, 여기서 그것을 금요일 자리에
			쓰면 **같은 값이 이틀에 걸쳐 두 번** 저장된다. 실측 08-31 기준으로 나스닥·
			나스닥100·S&P 500·다우가 08-27과 08-28에 똑같은 종가를 들고 있었고, 08-28
			행의 전일 종가는 자기 자신이라 등락률이 0%였다. 화면에서는 이게 "금요일에
			나스닥이 보합이었다"로 읽힌다.

			`indexLiveDate`는 앞당기기만 한다 — 시세가 세션보다 뒤를 가리키면(자정 근처에
			도는 24/7 자산) 세션 날짜를 그대로 둔다. 그 자리는 그 세션의 몫이다.
		*/
		tradeDate := indexLiveDate(idx.ID, idx.UpdatedAt, day.In(loc))
		rowDate := tradeDate.Format("2006-01-02")

		row := repository.IndexDailyCloseRow{
			IndexID:   idx.ID,
			TradeDate: tradeDate,
			Close:     idx.Value,
			Currency:  string(idx.Currency),
			Source:    "index_poller",
		}
		if prev, ok, err := s.indices.LastCloseBefore(ctx, idx.ID, rowDate); err == nil && ok && prev > 0 {
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
			LLMNote:   row.LLMNote,
			LLMStance: row.LLMStance,
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
	// 한 슬롯은 한 번만 (SlotGate 머리글 — 실측으로 예약 잡이 매일 두 번 돌았다).
	// 이 잡은 특히 중요하다: 두 번 돌면 종가 적재 뒤에 구멍 메우기까지 두 번 큐에 든다.
	gate := NewSlotGate()
	for {
		next, schedule := s.nextTrigger(time.Now(), schedules)
		if !WaitForSlot(ctx, next) {
			return
		}
		if !gate.Allow(next) {
			continue
		}
		RunScheduledJob(ctx, runs, jobID, func(ctx context.Context) (map[string]any, string, error) {
			stocks, indices := s.ingestSession(ctx, schedule, next, assetTypes)
			return map[string]any{"stockRows": stocks, "indexRows": indices},
				fmt.Sprintf("%s 세션 적재 완료 (종목 %d행, 지수 %d행)", schedule, stocks, indices), nil
		})
		s.enqueueStaleFetch(ctx, runs, schedule, next)
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
		/*
			catch-up은 **그 세션이 실제로 비어 있을 때만** 구멍 메우기를 큐에 넣는다.

			넣어야 하는 이유: `ingestSession`은 candle_bars를 투영할 뿐이라, 재시작
			때문에 정규 트리거를 놓쳤다면 그 세션의 봉을 받아 온 적이 없어 재적재가
			0행으로 끝나고 다음 세션 마감까지 그 하루가 빈 채로 남는다.

			조건을 다는 이유: catch-up은 **프로세스가 뜰 때마다** 돈다. 이 프로젝트는
			BE를 고칠 때마다 재시작하므로, 무조건 넣으면 재시작 한 번이 전 종목
			일봉 재수집(국내 약 27분·해외 약 90분)을 깨운다. 커버리지를 먼저 재서
			이미 채워져 있으면 건드리지 않는다 — 조회 한 번이면 알 수 있다.
		*/
		if s.sessionNeedsFetch(ctx, schedule, at, assetTypes) {
			s.enqueueStaleFetch(ctx, runs, schedule, at)
		}
	}
}

/*
sessionCatchUpCoverage — catch-up이 「이미 채워졌다」고 볼 커버리지 하한.

90%로 둔다. 100%를 요구하면 절대 만족되지 않는다 — 마스터에는 상장폐지 직전·거래
정지·Toss에 없는 종목이 늘 섞여 있어(실측 국내 4,412 중 404가 `stock-not-found`)
세션이 정상적으로 끝나도 몇 %는 비어 있다. 반대로 적재가 실제로 실패한 날은 훨씬
크게 벌어진다(실측 08-31 국내 391/4,412 = 8.9%).
*/
const sessionCatchUpCoverage = 0.9

// sessionNeedsFetch reports whether `at`의 세션 종가가 아직 대부분 비어 있는지.
// 조회에 실패하면 **넣는 쪽**으로 답한다 — 못 재서 건너뛰는 것보다 한 번 더 도는
// 편이 낫다(구멍 메우기는 멱등하고 재개 가능하다).
func (s *DailyCloseService) sessionNeedsFetch(
	ctx context.Context, schedule calendar.ScheduleID, at time.Time, assetTypes []model.QuoteAssetType,
) bool {
	switch schedule {
	case calendar.ScheduleKRX, calendar.ScheduleUS:
	default:
		// 보조 스케줄(환율·코인)은 주식 일봉과 무관하다 — enqueueStaleFetch도 무시한다.
		return false
	}
	date := at.In(calendar.LocationFor(schedule)).Format("2006-01-02")
	have, total, err := s.SessionCloseCoverage(ctx, date, assetTypes...)
	if err != nil {
		log.Printf("daily close: catch-up coverage %s: %v", date, err)
		return true
	}
	if total == 0 {
		return false
	}
	ratio := float64(have) / float64(total)
	if ratio >= sessionCatchUpCoverage {
		log.Printf("daily close: catch-up %s 종가 보유 %d/%d — 구멍 메우기 생략", date, have, total)
		return false
	}
	log.Printf("daily close: catch-up %s 종가 보유 %d/%d — 구멍 메우기 필요", date, have, total)
	return true
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
/*
staleFetchDailyLimit — 마감 후 자동 구멍 메우기가 한 번에 다룰 종목 수.

`ingestSession`은 **투영만** 한다(candle_bars → stock_daily_closes). Toss를 부르지
않으므로 candle_bars에 그날 봉이 없는 종목은 투영할 것도 없다. 그리고 candle_bars에
1일봉이 들어오는 경로는 **사용자가 그 종목을 조회했을 때**뿐이었다 — 세 개의
fetch 잡이 전부 수동이었기 때문이다.

그래서 아무도 안 본 종목은 그날 종가가 **아예 안 들어왔다.** 삭제가 아니라 누락이라
일봉 차트에 띄엄띄엄 구멍이 생긴다(`SymbolsStaleBefore` 주석의 "the gap a user
notices"가 정확히 이 증상이다).

실측(2026-08-24): 전체 NORMAL 종목 17,907 중 최신 거래일 종가를 못 가진 것이
**16,160개**. 20,000이면 그 전부를 한 실행에 담는다. `fetchSymbols`는 재개 가능하고
심볼마다 `fetchDelay`로 페이싱하므로 중간에 끊겨도 진행분이 남는다.

대상은 각 워커의 시장으로 좁힌다 — 국내 마감 뒤에 미국 종목까지 훑을 이유가 없고,
두 워커가 같은 Toss 예산을 동시에 쓰지 않게 된다.
*/
const staleFetchDailyLimit = 20000

/*
enqueueStaleFetch — 세션 적재 직후 「구멍 메우기」를 큐에 넣는다.

직접 실행하지 않고 큐에 넣는 이유: 이 잡은 Toss 캔들 예산을 쓰는데, worker-fetch가
그 예산을 쓰는 세 잡을 **뮤텍스로 직렬화**하고 있다. 여기서 따로 돌리면 그 직렬화
밖으로 새어 나가 한도를 두 배로 태운다.

`HasPending`으로 중복 적재를 막는다 — 앞선 실행이 아직 큐에 있거나 도는 중이면
넣지 않는다. 배치 큐가 이미 원자적 클레임(`SELECT … FOR UPDATE`)을 하므로 여기서는
「같은 일을 두 번 줄 세우지 않는다」만 보장하면 된다.
*/
func (s *DailyCloseService) enqueueStaleFetch(
	ctx context.Context, runs *repository.BatchJobRunRepo, schedule calendar.ScheduleID, at time.Time,
) {
	if runs == nil {
		return
	}

	/*
		**주 시장 스케줄에서만** 넣는다.

		각 워커는 보조 스케줄도 함께 받는다 — 국내는 `weekdays_kst`(KRW/USD가
		평일 연속 호가), 해외는 `always`(BTC 등 상시 거래). 그것들은 환율·코인용
		트리거라 **주식 일봉 구멍 메우기와 무관**하고, 여기에 섞으면 하루에 두 번
		큐에 넣으려 시도하게 된다(HasPending이 막긴 하지만 로그만 시끄러워진다).
	*/
	var market string
	switch schedule {
	case calendar.ScheduleKRX:
		market = "domestic"
	case calendar.ScheduleUS:
		market = "us"
	default:
		return
	}

	pending, err := runs.HasPending(ctx, string(model.BatchDailyCloseFetchStale))
	if err != nil {
		log.Printf("daily close: stale fetch enqueue check: %v", err)
		return
	}
	if pending {
		log.Printf("daily close: stale fetch already queued — skipping %s enqueue", market)
		return
	}

	/*
		**방금 끝난 세션의 날짜를 함께 넘긴다.**

		이걸 넘기지 않으면 워커가 `LatestStoredTradeDate`(= 저장된 종가 중 가장 늦은
		날)를 기준으로 「밀린 종목」을 고른다. 그런데 여기서 메우려는 것이 바로 **아직
		아무도 저장하지 않은 그 세션**이라, 기준이 언제나 한 세션 뒤에 있다 — 조건이
		`newest < asOf`이므로 그날 종가가 없는 종목이 전부 빠진다.

		실측(2026-08-31 15:50 KST, 국내 마감 직후): 저장된 최신 종가가 08-28이라
		asOf=08-28이 되어 대상이 **516종목**으로 잡혔다. 실제로 08-31 종가가 없는
		국내 종목은 4,287개였다. 그 결과 08-31 국내 종가가 391개만 채워졌고, 홈의
		AI 추천 랭킹은 08-31 점수 옆에 **08-28 등락률**을 달고 있었다(전 종목 회색).

		세션 날짜는 스케줄의 표준시로 읽는다 — `at`은 마감 + 지연이라 같은 날 안에
		있다(국내 15:30+지연, 미국 16:00 ET+지연).
	*/
	sessionDate := at.In(calendar.LocationFor(schedule)).Format("2006-01-02")

	if _, err := runs.Enqueue(ctx, string(model.BatchDailyCloseFetchStale), map[string]any{
		"market": market,
		"limit":  staleFetchDailyLimit,
		"asOf":   sessionDate,
	}); err != nil {
		log.Printf("daily close: stale fetch enqueue (%s): %v", market, err)
		return
	}
	log.Printf("daily close: queued stale fetch (market=%s asOf=%s limit=%d)", market, sessionDate, staleFetchDailyLimit)
}

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

/*
backfillProjectionWindowDays — 투영을 한 번에 처리하는 구간 길이(일).

`BackfillStocksFromCandleBarsRange`는 안쪽 스캔에 30일 여유(lead-in)를 붙이므로 한
구간의 실제 스캔 폭은 이 값 + 30일이다. 45일이면 75일치 — 전 종목이라도 한 문장이
readTimeout(30초) 안에 끝난다.
*/
const backfillProjectionWindowDays = 45

/*
BackfillStocksFromCandleBarsChunked — 긴 구간을 나눠서 투영한다.

한 문장으로 `since`부터 끝까지 투영하면 **전 종목 fetch 직후에 터진다.** 실측
2026-08-31 국내 4,021종목 재수집 뒤 (since=2026-01-01):

	[mysql] read tcp …:3306: i/o timeout
	worker-fetch: project after fetch: backfill stock daily closes: invalid connection

여덟 달치 1일봉에 ROW_NUMBER()·LAG()를 한 번에 돌리다 readTimeout에 잘린 것이다.
그 전까지 이 경로가 멀쩡했던 이유는 **구멍 메우기가 실제로 전 종목을 돈 적이 없어서**다
(대상 선정 버그로 매번 수백 종목만 처리했다 — `enqueueStaleFetch` 주석).

나눠 돌리면 스캔한 행 수 자체는 늘지만(구간마다 lead-in 30일을 다시 읽는다) 각
문장이 짧게 끝나 연결이 끊기지 않는다. 투영은 멱등한 upsert라 구간이 겹쳐도 안전하다.
한 구간이 실패해도 나머지는 계속 간다 — 부분 진행이 낭비가 아니라 진전이다.
*/
func (s *DailyCloseService) BackfillStocksFromCandleBarsChunked(
	ctx context.Context, since string, assetTypes ...model.QuoteAssetType,
) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	start, err := time.Parse("2006-01-02", since)
	if err != nil {
		return 0, err
	}
	// 상한은 내일로 둔다 — 자정 근처에 도는 24/7 자산의 오늘 봉까지 덮는다.
	end := time.Now().AddDate(0, 0, 1)

	var total int64
	var firstErr error
	for from := start; from.Before(end); from = from.AddDate(0, 0, backfillProjectionWindowDays) {
		to := from.AddDate(0, 0, backfillProjectionWindowDays-1)
		if to.After(end) {
			to = end
		}
		n, err := s.BackfillStocksFromCandleBarsRange(
			ctx, from.Format("2006-01-02"), to.Format("2006-01-02"), assetTypes...)
		if err != nil {
			log.Printf("daily close: project %s~%s: %v",
				from.Format("2006-01-02"), to.Format("2006-01-02"), err)
			if firstErr == nil {
				firstErr = err
			}
			if ctx.Err() != nil {
				break
			}
			continue
		}
		total += n
	}
	return total, firstErr
}
