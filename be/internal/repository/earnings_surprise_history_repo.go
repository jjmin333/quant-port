package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// EarningsSurpriseHistoryRepo persists past consensus-vs-actual quarters that
// arrived **without an announcement date** (Finnhub `/stock/earnings`).
//
// 왜 earnings_events가 아닌 별도 표인지는 migrations/055의 헤더에 적혀 있다 —
// 요약하면 그 소스의 날짜는 발표일이 아니라 회계기간 종료일이고, 그것을
// event_date에 넣으면 이미 맞게 들어 있는 발표일이 덮어써진다.
type EarningsSurpriseHistoryRepo struct {
	db *sql.DB
}

func NewEarningsSurpriseHistoryRepo(db *sql.DB) *EarningsSurpriseHistoryRepo {
	return &EarningsSurpriseHistoryRepo{db: db}
}

// Upsert writes quarters, overwriting the numbers on re-run.
//
// earnings_events의 upsert와 달리 **정정 병합 규칙이 없다**. 이 소스는 회차
// 개념이 없고 언제 물어도 그 시점의 최신 값을 주므로, 마지막 응답이 언제나 옳다.
// (같은 이유로 overseasUpsert도 source_date에 오늘을 넣는다.)
func (r *EarningsSurpriseHistoryRepo) Upsert(
	ctx context.Context, rows []model.EarningsSurpriseRow,
) (int, error) {
	if r == nil || r.db == nil || len(rows) == 0 {
		return 0, nil
	}

	const chunk = 300
	written := 0
	for start := 0; start < len(rows); start += chunk {
		end := start + chunk
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[start:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO earnings_surprise_history
			(symbol, market, fiscal_year, fiscal_period, period_end,
			 eps_est, eps_act, currency, source)
			VALUES `)
		args := make([]any, 0, len(batch)*9)
		for i, row := range batch {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,?)")
			args = append(args,
				row.Symbol,
				row.Market,
				row.FiscalYear,
				row.FiscalPeriod,
				row.PeriodEnd.Format(earningsDateLayout),
				row.EPSEst,
				row.EPSAct,
				row.Currency,
				row.Source,
			)
		}
		sb.WriteString(`
			ON DUPLICATE KEY UPDATE
			  period_end = VALUES(period_end),
			  eps_est    = VALUES(eps_est),
			  eps_act    = VALUES(eps_act),
			  currency   = VALUES(currency),
			  source     = VALUES(source)`)

		if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
			return written, fmt.Errorf("earnings_surprise_history upsert: %w", err)
		}
		written += len(batch)
	}
	return written, nil
}

// ListMarketSince returns stored quarters whose period ended inside the window,
// newest period first.
//
// 발표일이 아니라 period_end로 자르는 것이 이 표의 유일한 시간 축이다. 창을
// 730일로 주면 「최근 2년의 분기」가 되고, 그건 카드가 말하는 것과 같은 뜻이다.
func (r *EarningsSurpriseHistoryRepo) ListMarketSince(
	ctx context.Context, market string, since time.Time,
) ([]model.EarningsSurpriseRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	// stocks를 LEFT JOIN하는 이유는 캘린더 목록과 같다 — 이름이 없다고 행을
	// 감추면 카탈로그 백필이 아직 안 닿은 종목이 사라진다. 심볼이 자리를 지킨다.
	q := `SELECT h.symbol, COALESCE(s.name, ''), h.market,
		     h.fiscal_year, h.fiscal_period, h.period_end,
		     h.eps_est, h.eps_act, h.currency, h.source
		FROM earnings_surprise_history h
		LEFT JOIN stocks s ON s.symbol = h.symbol
		WHERE h.period_end >= ?`
	args := []any{since.Format(earningsDateLayout)}
	if market != "" {
		q += ` AND h.market = ?`
		args = append(args, market)
	}
	q += ` ORDER BY h.period_end DESC`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("earnings_surprise_history list: %w", err)
	}
	defer rows.Close()

	out := make([]model.EarningsSurpriseRow, 0, 512)
	for rows.Next() {
		var row model.EarningsSurpriseRow
		var periodEnd time.Time
		var est, act sql.NullFloat64
		if err := rows.Scan(
			&row.Symbol, &row.Name, &row.Market,
			&row.FiscalYear, &row.FiscalPeriod, &periodEnd,
			&est, &act, &row.Currency, &row.Source,
		); err != nil {
			return nil, err
		}
		row.PeriodEnd = periodEnd
		if est.Valid {
			v := est.Float64
			row.EPSEst = &v
		}
		if act.Valid {
			v := act.Float64
			row.EPSAct = &v
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SymbolsWithHistory is what makes the backfill resumable.
//
// 3,000종목 × 1.5초면 한 시간이 넘고, 그동안 한 번도 안 끊긴다는 보장이 없다.
// 이미 받은 종목을 건너뛰지 못하면 재실행이 매번 처음부터 한 시간을 다시 쓴다.
func (r *EarningsSurpriseHistoryRepo) SymbolsWithHistory(
	ctx context.Context, market string,
) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if r == nil || r.db == nil {
		return out, nil
	}
	q := `SELECT DISTINCT symbol FROM earnings_surprise_history`
	args := []any{}
	if market != "" {
		q += ` WHERE market = ?`
		args = append(args, market)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("earnings_surprise_history symbols: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out[s] = struct{}{}
	}
	return out, rows.Err()
}

// Coverage summarizes what the backfill has landed, for the operator.
func (r *EarningsSurpriseHistoryRepo) Coverage(
	ctx context.Context, market string,
) (rows, symbols int, oldest, newest string, err error) {
	if r == nil || r.db == nil {
		return 0, 0, "", "", nil
	}
	var minD, maxD sql.NullTime
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT symbol), MIN(period_end), MAX(period_end)
		FROM earnings_surprise_history WHERE market = ?`, market,
	).Scan(&rows, &symbols, &minD, &maxD)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("earnings_surprise_history coverage: %w", err)
	}
	if minD.Valid {
		oldest = minD.Time.Format(earningsDateLayout)
	}
	if maxD.Valid {
		newest = maxD.Time.Format(earningsDateLayout)
	}
	return rows, symbols, oldest, newest, nil
}

// QuartersNeedingAnnounceDate returns symbols whose stored quarters still have
// no announcement date, oldest-checked first.
//
// `announce_checked_at`으로 거르는 이유: **매칭에 실패한 종목도 「봤다」는 사실이
// 남아야** 재실행이 같은 종목을 3천 번 다시 묻지 않는다. 날짜가 붙었는지가
// 아니라 물어봤는지가 기준이다.
func (r *EarningsSurpriseHistoryRepo) SymbolsNeedingAnnounceDate(
	ctx context.Context, market string, includeChecked bool,
) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	q := `SELECT DISTINCT symbol FROM earnings_surprise_history WHERE market = ?`
	if !includeChecked {
		q += ` AND announce_checked_at IS NULL`
	}
	q += ` ORDER BY symbol`

	rows, err := r.db.QueryContext(ctx, q, market)
	if err != nil {
		return nil, fmt.Errorf("earnings_surprise_history announce candidates: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 4096)
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// QuartersForSymbol lists one symbol's stored quarters (newest period first).
func (r *EarningsSurpriseHistoryRepo) QuartersForSymbol(
	ctx context.Context, symbol string,
) ([]model.QuarterToMatch, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT fiscal_year, fiscal_period, period_end
		FROM earnings_surprise_history
		WHERE symbol = ? ORDER BY period_end DESC`, symbol)
	if err != nil {
		return nil, fmt.Errorf("earnings_surprise_history quarters: %w", err)
	}
	defer rows.Close()
	out := make([]model.QuarterToMatch, 0, 8)
	for rows.Next() {
		var q model.QuarterToMatch
		if err := rows.Scan(&q.FiscalYear, &q.FiscalPeriod, &q.PeriodEnd); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// SaveAnnounceMatches writes the resolved dates and always stamps the symbol as
// checked — even when nothing matched.
func (r *EarningsSurpriseHistoryRepo) SaveAnnounceMatches(
	ctx context.Context, symbol string, matches []model.AnnounceMatch,
) (int, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	written := 0
	for _, m := range matches {
		res, err := r.db.ExecContext(ctx, `
			UPDATE earnings_surprise_history
			   SET announced_on = ?, announce_session = ?, announce_gap_days = ?,
			       announce_source = 'edgar_8k'
			 WHERE symbol = ? AND fiscal_year = ? AND fiscal_period = ?`,
			m.Filed.Format(earningsDateLayout),
			nullStringArg(m.Session),
			m.GapDays,
			symbol, m.Quarter.FiscalYear, m.Quarter.FiscalPeriod,
		)
		if err != nil {
			return written, fmt.Errorf("earnings_surprise_history announce update: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			written++
		}
	}
	// 매칭 0건이어도 반드시 찍는다 — 안 찍으면 다음 실행이 이 종목을 또 부른다.
	if _, err := r.db.ExecContext(ctx, `
		UPDATE earnings_surprise_history SET announce_checked_at = NOW()
		 WHERE symbol = ?`, symbol); err != nil {
		return written, fmt.Errorf("earnings_surprise_history announce checked: %w", err)
	}
	return written, nil
}

// PromoteAnnouncedToCalendar copies dated quarters into `earnings_events`.
//
// # 기존 캘린더 행은 한 줄도 건드리지 않는다
//
// `INSERT ... ON DUPLICATE KEY UPDATE id = id`는 충돌 시 **아무것도 안 한다**.
// 일반 Upsert를 쓰면 `event_date = LEAST(event_date, VALUES(event_date))`가
// 돌아, 우리가 매칭한 날짜가 조금이라도 이르면 소스가 준 진짜 발표일을 덮는다.
// 이 표에서 그건 되돌릴 수 없는 손상이다(051 헤더).
//
// 그래서 겹치는 분기(대개 가장 최근 것)는 **캘린더 쪽이 항상 이긴다** — 그쪽은
// 소스가 발표일을 직접 준 행이고, 이쪽은 우리가 8-K로 역산한 행이다.
func (r *EarningsSurpriseHistoryRepo) PromoteAnnouncedToCalendar(
	ctx context.Context, market string,
) (int, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO earnings_events
			(symbol, market, event_date, date_status, session,
			 fiscal_year, fiscal_period, period_end, basis,
			 eps_est, eps_act, currency, source, source_date)
		SELECT h.symbol, h.market, h.announced_on, ?, h.announce_session,
		       h.fiscal_year, h.fiscal_period, h.period_end, '',
		       h.eps_est, h.eps_act, h.currency, 'edgar_8k', h.announced_on
		  FROM earnings_surprise_history h
		 WHERE h.market = ?
		   AND h.announced_on IS NOT NULL
		   AND h.eps_est IS NOT NULL
		   AND h.eps_act IS NOT NULL
		ON DUPLICATE KEY UPDATE earnings_events.id = earnings_events.id`,
		model.EarningsDateConfirmed, market,
	)
	if err != nil {
		return 0, fmt.Errorf("earnings_events promote: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// AnnounceCoverage reports how far the 8-K pass has got.
func (r *EarningsSurpriseHistoryRepo) AnnounceCoverage(
	ctx context.Context, market string,
) (checked, dated, quarters int, err error) {
	if r == nil || r.db == nil {
		return 0, 0, 0, nil
	}
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT CASE WHEN announce_checked_at IS NOT NULL THEN symbol END),
		       COUNT(DISTINCT CASE WHEN announced_on IS NOT NULL THEN symbol END),
		       SUM(CASE WHEN announced_on IS NOT NULL THEN 1 ELSE 0 END)
		  FROM earnings_surprise_history WHERE market = ?`, market,
	).Scan(&checked, &dated, &quarters)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("earnings_surprise_history announce coverage: %w", err)
	}
	return checked, dated, quarters, nil
}

// MigrateEarningsSurpriseAnnouncedIfNeeded adds the 8-K columns.
func MigrateEarningsSurpriseAnnouncedIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := columnExists(ctx, db, "earnings_surprise_history", "announced_on")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "056_earnings_surprise_announced.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 056_earnings_surprise_announced.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("056_earnings_surprise_announced.sql: %w", err)
		}
	}
	log.Println("mysql: applied 056_earnings_surprise_announced.sql (실적 발표일 8-K)")
	return nil
}

// MigrateEarningsSurpriseHistoryIfNeeded creates the table on boot.
func MigrateEarningsSurpriseHistoryIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "earnings_surprise_history")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "055_earnings_surprise_history.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 055_earnings_surprise_history.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			// 컨테이너 여섯 개가 함께 부팅해 같은 CREATE TABLE을 동시에 내면
			// 진 쪽이 에러를 받는다 — 이 배포 형태에서는 정상적인 결과다.
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("055_earnings_surprise_history.sql: %w", err)
		}
	}
	log.Println("mysql: applied 055_earnings_surprise_history.sql (과거 실적 서프라이즈 이력)")
	return nil
}
