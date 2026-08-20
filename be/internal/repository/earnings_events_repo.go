package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const earningsDateLayout = "2006-01-02"

// EarningsEventRepo persists 실적 발표 이벤트 (국내 FnGuide · 해외 Finnhub).
//
// MySQL이 source of truth다. 이 데이터는 배치로만 들어오고, 유실되면
// 다음 배치까지 화면이 비는 게 아니라 **과거 발표 이력이 통째로 사라진다** —
// FnGuide는 약 2년만 롤링으로 보관하므로 우리가 지운 옛 달은 다시 받을 수 없다.
type EarningsEventRepo struct {
	db *sql.DB
}

func NewEarningsEventRepo(db *sql.DB) *EarningsEventRepo {
	return &EarningsEventRepo{db: db}
}

// Upsert writes announcements, merging revisions onto the row they revise.
//
// 병합 규칙이 이 함수의 핵심이다. FnGuide는 정정 공시를 **별 행으로** 준다 —
// 삼성전자 2026 2Q는 07/07 원 발표와 07/30 `[정정]`이 같은 달 응답에 함께 있다.
// 순진하게 넣으면 캘린더에 같은 분기가 두 줄로 뜨고, 순진하게 덮으면 발표일이
// 정정일로 바뀐다. 그래서 **날짜는 이른 쪽, 수치는 최신 회차**로 나눈다.
func (r *EarningsEventRepo) Upsert(ctx context.Context, rows []model.EarningsEventUpsert) (int, error) {
	if r == nil || r.db == nil || len(rows) == 0 {
		return 0, nil
	}

	// 같은 (종목·기간)의 여러 회차가 한 배치에 있으면 **오래된 것부터** 적용해야
	// 최신 정정이 마지막에 이긴다. MySQL은 다중 VALUES의 중복 키를 행 단위로
	// 순차 처리하므로 정렬만으로 결정된다.
	ordered := make([]model.EarningsEventUpsert, len(rows))
	copy(ordered, rows)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].SourceDate.Before(ordered[j].SourceDate)
	})

	const chunk = 300
	written := 0
	for start := 0; start < len(ordered); start += chunk {
		end := start + chunk
		if end > len(ordered) {
			end = len(ordered)
		}
		batch := ordered[start:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO earnings_events
			(symbol, market, event_date, date_status, session,
			 fiscal_year, fiscal_period, period_end, basis,
			 revenue_est, revenue_act, op_income_est, op_income_act,
			 net_income_est, net_income_act, eps_est, eps_act,
			 revenue_qoq, op_income_qoq, net_income_qoq,
			 currency, source, source_date, revised_at)
			VALUES `)
		args := make([]any, 0, len(batch)*24)
		for i, row := range batch {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")

			// revised_at은 컬럼으로 오지 않는다 — 정정 여부는 종목명 접두사로
			// 판정하므로, 정정 회차일 때만 그 회차 날짜를 실어 보낸다.
			var revisedAt any
			if row.Revised {
				revisedAt = row.SourceDate.Format(earningsDateLayout)
			}
			args = append(args,
				row.Symbol,
				row.Market,
				row.EventDate.Format(earningsDateLayout),
				row.DateStatus,
				nullStringArg(row.Session),
				row.FiscalYear,
				row.FiscalPeriod,
				nullDateArg(row.PeriodEnd),
				row.Basis,
				row.RevenueEst, row.RevenueAct,
				row.OpIncomeEst, row.OpIncomeAct,
				row.NetIncomeEst, row.NetIncomeAct,
				row.EPSEst, row.EPSAct,
				row.RevenueQoQ, row.OpIncomeQoQ, row.NetIncomeQoQ,
				row.Currency,
				row.Source,
				row.SourceDate.Format(earningsDateLayout),
				revisedAt,
			)
		}

		// 대입은 왼쪽에서 오른쪽으로 평가되고 뒤 항목은 앞에서 바뀐 값을 본다.
		// `fresh`를 source_date 갱신 **전에** 다 쓰고, source_date를 맨 마지막에
		// 올린다 — 순서를 바꾸면 모든 IF가 항상 참이 된다.
		sb.WriteString(`
			ON DUPLICATE KEY UPDATE
			  event_date     = LEAST(event_date, VALUES(event_date)),
			  date_status    = IF(VALUES(source_date) >= source_date, VALUES(date_status), date_status),
			  session        = IF(VALUES(source_date) >= source_date, VALUES(session), session),
			  period_end     = IF(VALUES(source_date) >= source_date, VALUES(period_end), period_end),
			  revenue_est    = IF(VALUES(source_date) >= source_date, VALUES(revenue_est), revenue_est),
			  revenue_act    = IF(VALUES(source_date) >= source_date, VALUES(revenue_act), revenue_act),
			  op_income_est  = IF(VALUES(source_date) >= source_date, VALUES(op_income_est), op_income_est),
			  op_income_act  = IF(VALUES(source_date) >= source_date, VALUES(op_income_act), op_income_act),
			  net_income_est = IF(VALUES(source_date) >= source_date, VALUES(net_income_est), net_income_est),
			  net_income_act = IF(VALUES(source_date) >= source_date, VALUES(net_income_act), net_income_act),
			  eps_est        = IF(VALUES(source_date) >= source_date, VALUES(eps_est), eps_est),
			  eps_act        = IF(VALUES(source_date) >= source_date, VALUES(eps_act), eps_act),
			  revenue_qoq    = IF(VALUES(source_date) >= source_date, VALUES(revenue_qoq), revenue_qoq),
			  op_income_qoq  = IF(VALUES(source_date) >= source_date, VALUES(op_income_qoq), op_income_qoq),
			  net_income_qoq = IF(VALUES(source_date) >= source_date, VALUES(net_income_qoq), net_income_qoq),
			  source         = IF(VALUES(source_date) >= source_date, VALUES(source), source),
			  revised_at     = COALESCE(VALUES(revised_at), revised_at),
			  source_date    = GREATEST(source_date, VALUES(source_date))`)

		if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
			return written, fmt.Errorf("earnings_events upsert: %w", err)
		}
		written += len(batch)
	}
	return written, nil
}

// selectColumns keeps the read projection in one place — five callers below
// scan the same shape, and a column added to one but not the others is the
// classic way these functions drift apart.
const earningsSelectColumns = `
	e.symbol, COALESCE(s.name, ''), e.market, e.event_date, e.date_status,
	COALESCE(e.session, ''), e.fiscal_year, e.fiscal_period, e.period_end,
	e.basis, e.currency, e.source, e.revised_at,
	e.revenue_est, e.revenue_act, e.op_income_est, e.op_income_act,
	e.net_income_est, e.net_income_act, e.eps_est, e.eps_act,
	e.revenue_qoq, e.op_income_qoq, e.net_income_qoq`

// ListRange returns detail rows for a date window, optionally narrowed to a
// market and/or an explicit symbol set (「내 종목」 필터).
func (r *EarningsEventRepo) ListRange(
	ctx context.Context, from, to time.Time, market string, symbols []string,
) ([]model.EarningsEvent, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	// stocks를 LEFT JOIN한다 — 이름이 없다고 발표를 감추면, 카탈로그 백필이 아직
	// 안 닿은 신규 상장이 캘린더에서 사라진다. 이름 자리는 심볼이 대신한다.
	q := `SELECT ` + earningsSelectColumns + `
		FROM earnings_events e
		LEFT JOIN stocks s ON s.symbol = e.symbol
		WHERE e.event_date BETWEEN ? AND ?`
	args := []any{from.Format(earningsDateLayout), to.Format(earningsDateLayout)}

	if market != "" {
		q += ` AND e.market = ?`
		args = append(args, market)
	}
	if len(symbols) > 0 {
		q += ` AND e.symbol IN (` + placeholders(len(symbols)) + `)`
		for _, s := range symbols {
			args = append(args, s)
		}
	}
	q += ` ORDER BY e.event_date, e.symbol`

	return r.query(ctx, q, args...)
}

// SummaryRange counts announcements per day for the month heatmap.
//
// 상세 행을 세어 만들지 않는다 — 월 전체가 2,000행이라 히트맵 하나에 그만큼을
// 내리는 셈이 된다(AGENT_STATE 응답 속도 원칙 #4). 집계는 인덱스
// idx_earnings_date_market 안에서 끝난다.
func (r *EarningsEventRepo) SummaryRange(
	ctx context.Context, from, to time.Time, market string,
) (map[string]*model.EarningsDaySummary, error) {
	if r == nil || r.db == nil {
		return map[string]*model.EarningsDaySummary{}, nil
	}
	q := `SELECT event_date, date_status, COUNT(*)
		FROM earnings_events
		WHERE event_date BETWEEN ? AND ?`
	args := []any{from.Format(earningsDateLayout), to.Format(earningsDateLayout)}
	if market != "" {
		q += ` AND market = ?`
		args = append(args, market)
	}
	q += ` GROUP BY event_date, date_status`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("earnings_events summary: %w", err)
	}
	defer rows.Close()

	out := map[string]*model.EarningsDaySummary{}
	for rows.Next() {
		var day time.Time
		var status string
		var n int
		if err := rows.Scan(&day, &status, &n); err != nil {
			return nil, err
		}
		key := day.Format(earningsDateLayout)
		cell := out[key]
		if cell == nil {
			cell = &model.EarningsDaySummary{Date: key}
			out[key] = cell
		}
		switch status {
		case model.EarningsDateScheduled:
			cell.Scheduled += n
		default:
			cell.Confirmed += n
		}
	}
	return out, rows.Err()
}

// ListForSymbols returns every stored announcement for the given symbols,
// newest first — the 종목 실적 탭과 홈 배치 카드가 함께 쓴다.
func (r *EarningsEventRepo) ListForSymbols(
	ctx context.Context, symbols []string,
) (map[string][]model.EarningsEvent, error) {
	out := map[string][]model.EarningsEvent{}
	if r == nil || r.db == nil || len(symbols) == 0 {
		return out, nil
	}
	q := `SELECT ` + earningsSelectColumns + `
		FROM earnings_events e
		LEFT JOIN stocks s ON s.symbol = e.symbol
		WHERE e.symbol IN (` + placeholders(len(symbols)) + `)
		ORDER BY e.event_date DESC`
	args := make([]any, 0, len(symbols))
	for _, s := range symbols {
		args = append(args, s)
	}
	events, err := r.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	for _, ev := range events {
		out[ev.Symbol] = append(out[ev.Symbol], ev)
	}
	return out, nil
}

// LastYearConfirmed returns confirmed domestic announcements inside a window,
// the raw material for 「작년 동기」 예정일 추정.
func (r *EarningsEventRepo) LastYearConfirmed(
	ctx context.Context, from, to time.Time,
) ([]model.EarningsEvent, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	q := `SELECT ` + earningsSelectColumns + `
		FROM earnings_events e
		LEFT JOIN stocks s ON s.symbol = e.symbol
		WHERE e.market = ? AND e.date_status = ?
		  AND e.event_date BETWEEN ? AND ?
		ORDER BY e.event_date, e.symbol`
	return r.query(ctx, q,
		string(model.AssetDomestic), model.EarningsDateConfirmed,
		from.Format(earningsDateLayout), to.Format(earningsDateLayout),
	)
}

// ConfirmedPeriodKeys returns the (symbol|fiscalYear|period) keys that already
// have a real announcement in a window — 추정이 실제 발표를 덮지 않게 걸러낸다.
func (r *EarningsEventRepo) ConfirmedPeriodKeys(
	ctx context.Context, from, to time.Time,
) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if r == nil || r.db == nil {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, fiscal_year, fiscal_period
		FROM earnings_events
		WHERE market = ? AND event_date BETWEEN ? AND ?`,
		string(model.AssetDomestic),
		from.Format(earningsDateLayout), to.Format(earningsDateLayout),
	)
	if err != nil {
		return nil, fmt.Errorf("earnings_events confirmed keys: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sym, period string
		var year int
		if err := rows.Scan(&sym, &year, &period); err != nil {
			return nil, err
		}
		out[EarningsPeriodKey(sym, year, period)] = struct{}{}
	}
	return out, rows.Err()
}

// DailyClose is one (date, close) point used to measure post-earnings reaction.
type DailyClose struct {
	Date  time.Time
	Close float64
}

// ClosesForSymbols returns each symbol's closes inside a window, oldest first.
//
// **한 번의 쿼리로 받아 Go에서 짝을 맞춘다.** 행마다 상관 서브쿼리를 두면
// (발표일 이전 종가 / 이후 종가) 두 번씩 타므로 한 달 3,089행에 6,178번의
// 인덱스 탐색이 되고, 실측에서 0.145초가 붙었다. 여기서는 심볼 목록 + 날짜
// 범위 한 번이고, PK가 `(symbol, trade_date)`라 심볼당 범위 스캔으로 끝난다.
func (r *EarningsEventRepo) ClosesForSymbols(
	ctx context.Context, symbols []string, from, to time.Time,
) (map[string][]DailyClose, error) {
	out := map[string][]DailyClose{}
	if r == nil || r.db == nil || len(symbols) == 0 {
		return out, nil
	}
	q := `SELECT symbol, trade_date, close_price
		FROM stock_daily_closes
		WHERE symbol IN (` + placeholders(len(symbols)) + `)
		  AND trade_date BETWEEN ? AND ?
		ORDER BY symbol, trade_date`
	args := make([]any, 0, len(symbols)+2)
	for _, s := range symbols {
		args = append(args, s)
	}
	args = append(args, from.Format(earningsDateLayout), to.Format(earningsDateLayout))

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("earnings closes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var symbol string
		var point DailyClose
		if err := rows.Scan(&symbol, &point.Date, &point.Close); err != nil {
			return nil, err
		}
		out[symbol] = append(out[symbol], point)
	}
	return out, rows.Err()
}

// MarketCapsFor returns 발행주식수 × 최근 종가 for each symbol that has both.
//
// **한 쿼리로 끝낸다.** 행마다 「이 종목의 최근 종가」를 상관 서브쿼리로 찾으면
// 300행에 300번의 정렬이 붙는다. 심볼별 최댓값을 먼저 구해 조인하면 PK
// `(symbol, trade_date)` 위에서 인덱스 탐색으로 끝난다.
//
// 시가총액을 저장하지 않는 이유: 종가가 매일 바뀌므로 저장하면 매일 갱신해야
// 하는 파생값이 하나 더 생긴다. 읽을 때 곱하는 편이 늘 최신이다.
func (r *EarningsEventRepo) MarketCapsFor(ctx context.Context, symbols []string) (map[string]float64, error) {
	out := map[string]float64{}
	if r == nil || r.db == nil || len(symbols) == 0 {
		return out, nil
	}
	ph := placeholders(len(symbols))
	q := `SELECT s.symbol, s.shares_outstanding * d.close_price
		FROM stocks s
		JOIN (
			SELECT symbol, MAX(trade_date) AS latest
			FROM stock_daily_closes
			WHERE symbol IN (` + ph + `)
			GROUP BY symbol
		) x ON x.symbol = s.symbol
		JOIN stock_daily_closes d ON d.symbol = x.symbol AND d.trade_date = x.latest
		WHERE s.symbol IN (` + ph + `) AND s.shares_outstanding > 0`

	args := make([]any, 0, len(symbols)*2)
	for range 2 {
		for _, s := range symbols {
			args = append(args, s)
		}
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("earnings market caps: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var symbol string
		var cap float64
		if err := rows.Scan(&symbol, &cap); err != nil {
			return nil, err
		}
		if cap > 0 {
			out[symbol] = cap
		}
	}
	return out, rows.Err()
}

// Coverage summarizes what a market's ingest has actually landed — the admin
// console's only way to tell "배치가 돌았다"와 "돌았지만 0건이었다"를 구분한다.
func (r *EarningsEventRepo) Coverage(ctx context.Context, market string) (rows, symbols int, latest string, err error) {
	if r == nil || r.db == nil {
		return 0, 0, "", nil
	}
	var latestDate sql.NullTime
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT symbol), MAX(event_date)
		FROM earnings_events WHERE market = ?`, market,
	).Scan(&rows, &symbols, &latestDate)
	if err != nil {
		return 0, 0, "", fmt.Errorf("earnings_events coverage: %w", err)
	}
	if latestDate.Valid {
		latest = latestDate.Time.Format(earningsDateLayout)
	}
	return rows, symbols, latest, nil
}

// EarningsPeriodKey is the identity of "this company's this reporting period".
func EarningsPeriodKey(symbol string, year int, period string) string {
	return fmt.Sprintf("%s|%d|%s", symbol, year, period)
}

// Announced returns confirmed rows in a window that carry both an estimate and
// an actual — the surprise 랭킹의 후보 집합.
func (r *EarningsEventRepo) Announced(
	ctx context.Context, from, to time.Time, market string,
) ([]model.EarningsEvent, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	// 서프라이즈 계산 가능 여부(기대·발표 둘 다 있음)를 SQL에서 먼저 거른다.
	// 전부 읽어 Go에서 버리면 시즌 달에는 400행 중 대부분이 헛일이다.
	q := `SELECT ` + earningsSelectColumns + `
		FROM earnings_events e
		LEFT JOIN stocks s ON s.symbol = e.symbol
		WHERE e.event_date BETWEEN ? AND ?
		  AND e.date_status = ?
		  AND ((e.op_income_est IS NOT NULL AND e.op_income_act IS NOT NULL)
		    OR (e.eps_est IS NOT NULL AND e.eps_act IS NOT NULL))`
	args := []any{
		from.Format(earningsDateLayout), to.Format(earningsDateLayout),
		model.EarningsDateConfirmed,
	}
	if market != "" {
		q += ` AND e.market = ?`
		args = append(args, market)
	}
	q += ` ORDER BY e.event_date DESC`
	return r.query(ctx, q, args...)
}

func (r *EarningsEventRepo) query(ctx context.Context, q string, args ...any) ([]model.EarningsEvent, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("earnings_events query: %w", err)
	}
	defer rows.Close()

	var out []model.EarningsEvent
	for rows.Next() {
		var (
			ev        model.EarningsEvent
			eventDate time.Time
			periodEnd sql.NullTime
			revisedAt sql.NullTime
		)
		if err := rows.Scan(
			&ev.Symbol, &ev.Name, &ev.Market, &eventDate, &ev.DateStatus,
			&ev.Session, &ev.FiscalYear, &ev.FiscalPeriod, &periodEnd,
			&ev.Basis, &ev.Currency, &ev.Source, &revisedAt,
			&ev.Revenue.Estimate, &ev.Revenue.Actual,
			&ev.OpIncome.Estimate, &ev.OpIncome.Actual,
			&ev.NetIncome.Estimate, &ev.NetIncome.Actual,
			&ev.EPS.Estimate, &ev.EPS.Actual,
			&ev.Revenue.QoQ, &ev.OpIncome.QoQ, &ev.NetIncome.QoQ,
		); err != nil {
			return nil, err
		}
		ev.Date = eventDate.Format(earningsDateLayout)
		if periodEnd.Valid {
			ev.PeriodEnd = periodEnd.Time.Format(earningsDateLayout)
		}
		if revisedAt.Valid {
			ev.RevisedAt = revisedAt.Time.Format(earningsDateLayout)
		}
		if ev.Name == "" {
			ev.Name = ev.Symbol
		}
		// 서프라이즈는 저장하지 않고 읽을 때 만든다 — 계산이 한 곳(model)에만
		// 있으면 목록·상세·홈이 같은 숫자를 말한다.
		ev.Surprise = ev.HeadlineSurprise()
		// 매출 서프라이즈는 헤드라인과 **별개 열**이다. 이익이 기대를 넘었는데
		// 매출이 밑돌면 비용 절감이지 성장이 아니라는 뜻이라, 둘을 나란히 두는
		// 것이 이 열의 존재 이유다.
		ev.RevenueSurprise = model.ComputeSurprise(model.EarningsMetricRevenue, ev.Revenue)
		out = append(out, ev)
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	if n <= 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func nullStringArg(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullDateArg(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.Format(earningsDateLayout)
}

// MigrateEarningsEventsIfNeeded applies 051_earnings_events.sql.
func MigrateEarningsEventsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "earnings_events")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "051_earnings_events.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 051_earnings_events.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			// 컨테이너 여섯 개가 함께 부팅해 같은 CREATE TABLE을 동시에 내면
			// 진 쪽이 에러를 받는다 — 이 배포 형태에서는 정상적인 결과다.
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("051_earnings_events.sql: %w", err)
		}
	}
	log.Println("mysql: applied 051_earnings_events.sql (실적 발표 이벤트)")
	return nil
}
