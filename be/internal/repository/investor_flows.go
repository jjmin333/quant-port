package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

/*
종목별 투자자 매매동향(국내). 표를 왜 이렇게 두는지는 `migrations/084_stock_investor_flows.sql`.
*/

// InvestorFlowRepo owns stock_investor_flows.
type InvestorFlowRepo struct {
	db *sql.DB
}

func NewInvestorFlowRepo(db *sql.DB) *InvestorFlowRepo { return &InvestorFlowRepo{db: db} }

// InvestorFlowRow is one confirmed day for one symbol.
type InvestorFlowRow struct {
	TradeDate       string
	ForeignNet      int64
	InstitutionNet  int64
	IndividualNet   *int64
	ForeignHoldRate *float64
}

// Upsert writes one symbol's days. Later fetches overwrite — 외국인 보유는 다음 영업일
// 오전에 확정치로 한 번 더 바뀐다.
func (r *InvestorFlowRepo) Upsert(ctx context.Context, symbol string, rows []InvestorFlowRow) (int, error) {
	if r == nil || r.db == nil || len(rows) == 0 {
		return 0, nil
	}
	var b strings.Builder
	b.WriteString(`INSERT INTO stock_investor_flows
		(symbol, trade_date, foreign_net, institution_net, individual_net, foreign_hold_rate) VALUES `)
	args := make([]any, 0, len(rows)*6)
	for i, row := range rows {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("(?,?,?,?,?,?)")
		args = append(args, symbol, row.TradeDate, row.ForeignNet, row.InstitutionNet, row.IndividualNet, row.ForeignHoldRate)
	}
	b.WriteString(` ON DUPLICATE KEY UPDATE
		foreign_net = VALUES(foreign_net), institution_net = VALUES(institution_net),
		individual_net = VALUES(individual_net), foreign_hold_rate = VALUES(foreign_hold_rate)`)
	if _, err := r.db.ExecContext(ctx, b.String(), args...); err != nil {
		return 0, fmt.Errorf("investor flows upsert %s: %w", symbol, err)
	}
	return len(rows), nil
}

// LatestDates returns symbol → newest stored trade_date (YYYY-MM-DD).
func (r *InvestorFlowRepo) LatestDates(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	rows, err := r.db.QueryContext(ctx,
		`SELECT symbol, DATE_FORMAT(MAX(trade_date), '%Y-%m-%d') FROM stock_investor_flows GROUP BY symbol`)
	if err != nil {
		return nil, fmt.Errorf("investor flows latest: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sym, d string
		if err := rows.Scan(&sym, &d); err != nil {
			return nil, err
		}
		out[sym] = d
	}
	return out, rows.Err()
}

// DomesticCommonSymbols is the 수급 target set: domestic, not an index product,
// not a 우선주 — the same rule the rankings use (scoreRankableProductClause).
func (r *InvestorFlowRepo) DomesticCommonSymbols(ctx context.Context) (map[string]bool, error) {
	clause, args := scoreRankableProductClause("s")
	rows, err := r.db.QueryContext(ctx,
		`SELECT s.symbol FROM stocks s WHERE s.asset_type = 'domestic'`+clause, args...)
	if err != nil {
		return nil, fmt.Errorf("investor flow targets: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		out[sym] = true
	}
	return out, rows.Err()
}

// InvestorFlowAgg is one symbol's net buying over a window.
type InvestorFlowAgg struct {
	// Net is 외국인 + 기관 순매수 주식 수.
	Net int64
	// Volume is 같은 날들의 거래량 합(stock_daily_closes). 0이면 비율을 낼 수 없다.
	Volume int64
	Days   int
}

/*
FlowWindow — tradeDate까지 **수급이 적재된 최근 sessions개 거래일**의 순매수 합과 거래량 합.

창을 달력이 아니라 적재된 거래일로 잡는다 — 연휴가 끼면 20일 달력 창이 12거래일이 된다.
거래량은 같은 날의 종가 행에서 온다(없는 날은 분모에서도 빠진다 — 한쪽만 세면 비율이 튄다).
*/
func (r *InvestorFlowRepo) FlowWindow(ctx context.Context, tradeDate string, sessions int) (map[string]InvestorFlowAgg, error) {
	out := map[string]InvestorFlowAgg{}
	var from sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(MIN(d), '%Y-%m-%d') FROM (
		  SELECT DISTINCT trade_date AS d FROM stock_investor_flows
		   WHERE trade_date <= ? ORDER BY trade_date DESC LIMIT ?
		) t`, tradeDate, sessions).Scan(&from)
	if err != nil {
		return nil, fmt.Errorf("investor flow window: %w", err)
	}
	if !from.Valid {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT f.symbol, SUM(f.foreign_net + f.institution_net), SUM(c.volume), COUNT(*)
		  FROM stock_investor_flows f
		  JOIN stock_daily_closes c ON c.symbol = f.symbol AND c.trade_date = f.trade_date
		 WHERE f.trade_date BETWEEN ? AND ?
		 GROUP BY f.symbol`, from.String, tradeDate)
	if err != nil {
		return nil, fmt.Errorf("investor flow window: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sym string
		var a InvestorFlowAgg
		if err := rows.Scan(&sym, &a.Net, &a.Volume, &a.Days); err != nil {
			return nil, err
		}
		out[sym] = a
	}
	return out, rows.Err()
}

// Prune deletes rows older than the cutoff date.
func (r *InvestorFlowRepo) Prune(ctx context.Context, before string) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM stock_investor_flows WHERE trade_date < ?`, before)
	if err != nil {
		return 0, fmt.Errorf("investor flows prune: %w", err)
	}
	return res.RowsAffected()
}

// MigrateInvestorFlowsIfNeeded creates the table on an existing database.
func MigrateInvestorFlowsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_investor_flows")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "084_stock_investor_flows.sql", "종목별 투자자 매매동향")
}
