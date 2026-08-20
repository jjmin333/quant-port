package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// FinancialPeriodRepo persists 분기 재무 — the series behind both the
// 재무제표 상세 모달 and the 종목 실적 탭「분기 실적」.
//
// 두 화면이 같은 표를 읽는 것이 핵심이다. 예전에는 각자 DART를 따로 불러
// 같은 분기에 대해 서로 다른 숫자를 말할 수 있었다.
type FinancialPeriodRepo struct {
	db *sql.DB
}

func NewFinancialPeriodRepo(db *sql.DB) *FinancialPeriodRepo {
	return &FinancialPeriodRepo{db: db}
}

// FinancialPeriodRow is one stored quarter.
type FinancialPeriodRow struct {
	Symbol      string
	Source      string
	Basis       string
	PeriodLabel string
	ReportCode  string
	Period      model.FinancialHistoryPeriod
}

func (r *FinancialPeriodRepo) Upsert(ctx context.Context, rows []FinancialPeriodRow) (int, error) {
	if r == nil || r.db == nil || len(rows) == 0 {
		return 0, nil
	}

	var sb strings.Builder
	sb.WriteString(`INSERT INTO financial_periods
		(symbol, source, basis, fiscal_year, period_index, period_label, report_code, period_end,
		 revenue, operating_income, net_income, eps_basic,
		 total_assets, total_liabilities, total_equity, current_assets, current_liabilities)
		VALUES `)
	args := make([]any, 0, len(rows)*17)
	written := 0
	for _, row := range rows {
		p := row.Period
		if p.FiscalYear <= 0 || p.PeriodIndex < 1 || p.PeriodIndex > 4 {
			// 연도나 분기를 모르면 유니크 키가 성립하지 않는다 — 넣으면 서로
			// 다른 분기가 한 행으로 뭉친다.
			continue
		}
		if written > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
		args = append(args,
			strings.ToUpper(strings.TrimSpace(row.Symbol)),
			row.Source,
			row.Basis,
			p.FiscalYear,
			p.PeriodIndex,
			row.PeriodLabel,
			row.ReportCode,
			nullDate(p.PeriodEnd),
			nullInt64(p.Revenue),
			nullInt64(p.OperatingIncome),
			nullInt64(p.NetIncome),
			nullFloat64(p.EPSBasic),
			nullInt64(p.TotalAssets),
			nullInt64(p.TotalLiabilities),
			nullInt64(p.TotalEquity),
			nullInt64(p.CurrentAssets),
			nullInt64(p.CurrentLiabilities),
		)
		written++
	}
	if written == 0 {
		return 0, nil
	}
	sb.WriteString(`
		ON DUPLICATE KEY UPDATE
		  source = VALUES(source),
		  period_label = VALUES(period_label),
		  report_code = VALUES(report_code),
		  period_end = VALUES(period_end),
		  revenue = VALUES(revenue),
		  operating_income = VALUES(operating_income),
		  net_income = VALUES(net_income),
		  eps_basic = VALUES(eps_basic),
		  total_assets = VALUES(total_assets),
		  total_liabilities = VALUES(total_liabilities),
		  total_equity = VALUES(total_equity),
		  current_assets = VALUES(current_assets),
		  current_liabilities = VALUES(current_liabilities)`)

	if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
		return 0, fmt.Errorf("financial periods upsert: %w", err)
	}
	return written, nil
}

// ListForSymbol returns up to `limit` most recent quarters of one basis,
// **oldest first** (the order the history chart and the QoQ growth calculation
// both want).
func (r *FinancialPeriodRepo) ListForSymbol(ctx context.Context, symbol, basis string, limit int) ([]FinancialPeriodRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT source, fiscal_year, period_index, period_label, report_code, period_end,
		       revenue, operating_income, net_income, eps_basic,
		       total_assets, total_liabilities, total_equity, current_assets, current_liabilities
		FROM financial_periods
		WHERE symbol = ? AND basis = ?
		ORDER BY fiscal_year DESC, period_index DESC
		LIMIT ?`,
		strings.ToUpper(strings.TrimSpace(symbol)), basis, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]FinancialPeriodRow, 0, limit)
	for rows.Next() {
		var (
			row    FinancialPeriodRow
			end    sql.NullTime
			period model.FinancialHistoryPeriod
		)
		if err := rows.Scan(
			&row.Source, &period.FiscalYear, &period.PeriodIndex, &row.PeriodLabel, &row.ReportCode, &end,
			&period.Revenue, &period.OperatingIncome, &period.NetIncome, &period.EPSBasic,
			&period.TotalAssets, &period.TotalLiabilities, &period.TotalEquity,
			&period.CurrentAssets, &period.CurrentLiabilities,
		); err != nil {
			return nil, err
		}
		if end.Valid {
			t := end.Time
			period.PeriodEnd = &t
		}
		row.Symbol = symbol
		row.Basis = basis
		row.Period = period
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 질의는 최신순으로 자르고(LIMIT이 최근 N분기를 뜻해야 한다), 반환은
	// 오래된 순이다 — 화면과 성장률 계산이 둘 다 시간순을 전제한다.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// DisclosureRepo persists DART 공시 (시장 전체 일별 스윕 결과).
type DisclosureRepo struct {
	db *sql.DB
}

func NewDisclosureRepo(db *sql.DB) *DisclosureRepo {
	return &DisclosureRepo{db: db}
}

// DisclosureRow is one filing.
type DisclosureRow struct {
	Symbol      string
	ReceiptNo   string
	ReceiptDate string // YYYY-MM-DD
	Title       string
	Filer       string
}

func (r *DisclosureRepo) Upsert(ctx context.Context, rows []DisclosureRow) (int, error) {
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
		sb.WriteString(`INSERT INTO stock_disclosures
			(symbol, receipt_no, receipt_date, title, filer) VALUES `)
		args := make([]any, 0, len(batch)*5)
		for i, row := range batch {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?)")
			args = append(args,
				strings.ToUpper(strings.TrimSpace(row.Symbol)),
				row.ReceiptNo, row.ReceiptDate, truncate(row.Title, 500), truncate(row.Filer, 200))
		}
		// 접수번호가 자연키라 재실행은 갱신이다. 제목이 바뀌는 일은 없지만,
		// DO NOTHING 대신 갱신으로 둔 것은 정정 표기가 붙는 경우를 위해서다.
		sb.WriteString(` ON DUPLICATE KEY UPDATE
			symbol = VALUES(symbol), title = VALUES(title), filer = VALUES(filer)`)

		if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
			return written, fmt.Errorf("disclosures upsert: %w", err)
		}
		written += len(batch)
	}
	return written, nil
}

// ListForSymbolsOnDate returns filings for the given symbols on one date.
func (r *DisclosureRepo) ListForSymbolsOnDate(ctx context.Context, symbols []string, date time.Time) ([]DisclosureRow, error) {
	if r == nil || r.db == nil || len(symbols) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(symbols)), ",")
	args := make([]any, 0, len(symbols)+1)
	for _, s := range symbols {
		args = append(args, strings.ToUpper(strings.TrimSpace(s)))
	}
	args = append(args, date.Format("2006-01-02"))

	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, receipt_no, receipt_date, title, filer
		FROM stock_disclosures
		WHERE symbol IN (`+placeholders+`) AND receipt_date = ?
		ORDER BY receipt_no DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDisclosures(rows)
}

// LatestDate reports the most recent date the sweep has stored, so the admin
// console can show whether today's run has landed.
func (r *DisclosureRepo) LatestDate(ctx context.Context) (string, int, error) {
	if r == nil || r.db == nil {
		return "", 0, nil
	}
	var (
		latest sql.NullTime
		total  int
	)
	if err := r.db.QueryRowContext(ctx, `
		SELECT MAX(receipt_date), COUNT(*) FROM stock_disclosures`).Scan(&latest, &total); err != nil {
		return "", 0, err
	}
	if !latest.Valid {
		return "", total, nil
	}
	return latest.Time.Format("2006-01-02"), total, nil
}

func scanDisclosures(rows *sql.Rows) ([]DisclosureRow, error) {
	var out []DisclosureRow
	for rows.Next() {
		var (
			row  DisclosureRow
			date time.Time
		)
		if err := rows.Scan(&row.Symbol, &row.ReceiptNo, &date, &row.Title, &row.Filer); err != nil {
			return nil, err
		}
		row.ReceiptDate = date.Format("2006-01-02")
		out = append(out, row)
	}
	return out, rows.Err()
}

// WarningRepo persists 종목 경고 (투자주의·경고·거래정지 등 지정 상태).
type WarningRepo struct {
	db *sql.DB
}

func NewWarningRepo(db *sql.DB) *WarningRepo {
	return &WarningRepo{db: db}
}

// ReplaceForSymbol swaps one symbol's warnings for the current set.
//
// 갱신이 아니라 **교체**여야 한다 — 경고는 해제되는 것이 정상이고, upsert만 하면
// 이미 풀린 지정이 표에 영원히 남는다. 그러면 화면이 정상 종목에 경고를 단다.
func (r *WarningRepo) ReplaceForSymbol(ctx context.Context, symbol string, warnings []model.StockWarningRow) (int, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM stock_warnings WHERE symbol = ?`, sym); err != nil {
		return 0, err
	}
	written := 0
	for _, w := range warnings {
		wtype := strings.TrimSpace(w.WarningType)
		if wtype == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_warnings (symbol, warning_type, exchange, start_date, end_date)
			VALUES (?,?,?,?,?)
			ON DUPLICATE KEY UPDATE
			  exchange = VALUES(exchange),
			  start_date = VALUES(start_date),
			  end_date = VALUES(end_date)`,
			sym, truncate(wtype, 64), w.Exchange, w.StartDate, w.EndDate); err != nil {
			return written, err
		}
		written++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return written, nil
}

// ListForSymbol returns one symbol's current warnings.
func (r *WarningRepo) ListForSymbol(ctx context.Context, symbol string) ([]model.StockWarningRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT warning_type, exchange, start_date, end_date FROM stock_warnings
		WHERE symbol = ? ORDER BY warning_type`,
		strings.ToUpper(strings.TrimSpace(symbol)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.StockWarningRow
	for rows.Next() {
		var w model.StockWarningRow
		if err := rows.Scan(&w.WarningType, &w.Exchange, &w.StartDate, &w.EndDate); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ListForSymbols returns warnings for many symbols in one query — the insights
// dashboard needs every held symbol at once.
func (r *WarningRepo) ListForSymbols(ctx context.Context, symbols []string) (map[string][]model.StockWarningRow, error) {
	out := map[string][]model.StockWarningRow{}
	if r == nil || r.db == nil || len(symbols) == 0 {
		return out, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(symbols)), ",")
	args := make([]any, 0, len(symbols))
	for _, s := range symbols {
		args = append(args, strings.ToUpper(strings.TrimSpace(s)))
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, warning_type, exchange, start_date, end_date FROM stock_warnings
		WHERE symbol IN (`+placeholders+`) ORDER BY symbol, warning_type`, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			symbol string
			w      model.StockWarningRow
		)
		if err := rows.Scan(&symbol, &w.WarningType, &w.Exchange, &w.StartDate, &w.EndDate); err != nil {
			return out, err
		}
		out[symbol] = append(out[symbol], w)
	}
	return out, rows.Err()
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	// 바이트로 자르면 한글이 깨지므로 룬 경계에서 자른다.
	runes := []rune(s)
	for len(string(runes)) > max && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

// ListRecentForSymbol returns one symbol's filings over the last `days`,
// newest first, capped at `limit`.
//
// 종목 실적 탭의 「최근 공시」가 쓰는 질의다. 예전에는 이 화면이 DART를 직접
// 불렀다 — 종목 하나에 list.json 1회, 즉 사용자 수 × 종목 수의 호출이었다.
func (r *DisclosureRepo) ListRecentForSymbol(ctx context.Context, symbol string, days, limit int) ([]DisclosureRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if days <= 0 {
		days = 365
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, receipt_no, receipt_date, title, filer
		FROM stock_disclosures
		WHERE symbol = ? AND receipt_date >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
		ORDER BY receipt_date DESC, receipt_no DESC
		LIMIT ?`,
		strings.ToUpper(strings.TrimSpace(symbol)), days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDisclosures(rows)
}
