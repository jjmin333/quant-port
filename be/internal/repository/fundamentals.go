package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

type FundamentalsRepo struct {
	db *sql.DB
}

func NewFundamentalsRepo(db *sql.DB) *FundamentalsRepo {
	return &FundamentalsRepo{db: db}
}

func (r *FundamentalsRepo) Upsert(ctx context.Context, f model.StockFundamentals) error {
	symbol := strings.ToUpper(strings.TrimSpace(f.Symbol))
	if symbol == "" {
		return fmt.Errorf("upsert fundamentals: empty symbol")
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stock_fundamentals (
			symbol, data_source, fiscal_year, fiscal_period, report_code,
			period_end_date, filed_at,
			revenue, operating_income, net_income, total_assets, total_equity,
			total_liabilities, current_assets, current_liabilities,
			eps_basic, eps_diluted, shares_outstanding,
			roe, operating_margin, revenue_growth_yoy, eps_growth_yoy,
			fwd_per, cy_per,
			debt_ratio, roa, net_profit_margin, current_ratio,
			calc_status, calc_error, fetched_at
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?,
			?, ?,
			?, ?, ?, ?,
			?, ?, ?
		)
		ON DUPLICATE KEY UPDATE
			data_source = VALUES(data_source),
			fiscal_year = VALUES(fiscal_year),
			fiscal_period = VALUES(fiscal_period),
			report_code = VALUES(report_code),
			period_end_date = VALUES(period_end_date),
			filed_at = VALUES(filed_at),
			revenue = VALUES(revenue),
			operating_income = VALUES(operating_income),
			net_income = VALUES(net_income),
			total_assets = VALUES(total_assets),
			total_equity = VALUES(total_equity),
			total_liabilities = VALUES(total_liabilities),
			current_assets = VALUES(current_assets),
			current_liabilities = VALUES(current_liabilities),
			eps_basic = VALUES(eps_basic),
			eps_diluted = VALUES(eps_diluted),
			shares_outstanding = VALUES(shares_outstanding),
			roe = VALUES(roe),
			operating_margin = VALUES(operating_margin),
			revenue_growth_yoy = VALUES(revenue_growth_yoy),
			eps_growth_yoy = VALUES(eps_growth_yoy),
			fwd_per = VALUES(fwd_per),
			cy_per = VALUES(cy_per),
			debt_ratio = VALUES(debt_ratio),
			roa = VALUES(roa),
			net_profit_margin = VALUES(net_profit_margin),
			current_ratio = VALUES(current_ratio),
			calc_status = VALUES(calc_status),
			calc_error = VALUES(calc_error),
			fetched_at = VALUES(fetched_at)
	`,
		symbol,
		string(f.DataSource),
		nullInt(f.FiscalYear),
		nullString(f.FiscalPeriod),
		nullString(f.ReportCode),
		nullDate(f.PeriodEnd),
		nullDate(f.FiledAt),
		nullInt64(f.Revenue),
		nullInt64(f.OperatingIncome),
		nullInt64(f.NetIncome),
		nullInt64(f.TotalAssets),
		nullInt64(f.TotalEquity),
		nullInt64(f.TotalLiabilities),
		nullInt64(f.CurrentAssets),
		nullInt64(f.CurrentLiabilities),
		nullFloat64(f.EPSBasic),
		nullFloat64(f.EPSDiluted),
		nullInt64(f.SharesOutstanding),
		nullFloat64(f.ROE),
		nullFloat64(f.OperatingMargin),
		nullFloat64(f.RevenueGrowthYoY),
		nullFloat64(f.EPSGrowthYoY),
		nullFloat64(f.FwdPER),
		nullFloat64(f.CyPER),
		nullFloat64(f.DebtRatio),
		nullFloat64(f.ROA),
		nullFloat64(f.NetProfitMargin),
		nullFloat64(f.CurrentRatio),
		string(f.CalcStatus),
		nullString(f.CalcError),
		nullTime(f.FetchedAt),
	)
	return err
}

func (r *FundamentalsRepo) ListSymbolSlice(ctx context.Context) ([]string, error) {
	set, err := r.ListSymbols(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(set))
	for sym := range set {
		out = append(out, sym)
	}
	sort.Strings(out)
	return out, nil
}

func (r *FundamentalsRepo) ListSymbols(ctx context.Context) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT symbol FROM stock_fundamentals`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		out[strings.ToUpper(strings.TrimSpace(symbol))] = struct{}{}
	}
	return out, rows.Err()
}

func (r *FundamentalsRepo) ListSymbolsNotOK(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol FROM stock_fundamentals
		WHERE calc_status <> ?
		ORDER BY symbol
	`, string(model.FundamentalsStatusOK))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		out = append(out, symbol)
	}
	return out, rows.Err()
}

func (r *FundamentalsRepo) GetBySymbol(ctx context.Context, symbol string) (*model.StockFundamentals, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	row := r.db.QueryRowContext(ctx, `
		SELECT symbol, data_source, fiscal_year, fiscal_period, report_code,
		       period_end_date, filed_at,
		       revenue, operating_income, net_income, total_assets, total_equity,
		       total_liabilities, current_assets, current_liabilities,
		       eps_basic, eps_diluted, shares_outstanding,
		       roe, operating_margin, revenue_growth_yoy, eps_growth_yoy,
		       fwd_per, cy_per,
		       debt_ratio, roa, net_profit_margin, current_ratio,
		       calc_status, calc_error, fetched_at, updated_at
		FROM stock_fundamentals
		WHERE symbol = ?
	`, symbol)

	var f model.StockFundamentals
	var dataSource, calcStatus string
	var fiscalYear sql.NullInt64
	var fiscalPeriod, reportCode, calcError sql.NullString
	var periodEnd, filedAt, fetchedAt sql.NullTime
	var revenue, operatingIncome, netIncome sql.NullInt64
	var totalAssets, totalEquity, totalLiabilities sql.NullInt64
	var currentAssets, currentLiabilities, shares sql.NullInt64
	var epsBasic, epsDiluted sql.NullFloat64
	var roe, opMargin, revGrowth, epsGrowth sql.NullFloat64
	var fwdPER, cyPER sql.NullFloat64
	var debtRatio, roa, netMargin, currentRatio sql.NullFloat64

	err := row.Scan(
		&f.Symbol, &dataSource, &fiscalYear, &fiscalPeriod, &reportCode,
		&periodEnd, &filedAt,
		&revenue, &operatingIncome, &netIncome, &totalAssets, &totalEquity,
		&totalLiabilities, &currentAssets, &currentLiabilities,
		&epsBasic, &epsDiluted, &shares,
		&roe, &opMargin, &revGrowth, &epsGrowth,
		&fwdPER, &cyPER,
		&debtRatio, &roa, &netMargin, &currentRatio,
		&calcStatus, &calcError, &fetchedAt, &f.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	f.DataSource = model.FundamentalsDataSource(dataSource)
	f.FiscalPeriod = fiscalPeriod.String
	f.ReportCode = reportCode.String
	f.CalcStatus = model.FundamentalsCalcStatus(calcStatus)
	f.CalcError = calcError.String
	if fiscalYear.Valid {
		v := int(fiscalYear.Int64)
		f.FiscalYear = &v
	}
	f.PeriodEnd = nullTimePtr(periodEnd)
	f.FiledAt = nullTimePtr(filedAt)
	f.FetchedAt = nullTimePtr(fetchedAt)
	f.Revenue = nullInt64Ptr(revenue)
	f.OperatingIncome = nullInt64Ptr(operatingIncome)
	f.NetIncome = nullInt64Ptr(netIncome)
	f.TotalAssets = nullInt64Ptr(totalAssets)
	f.TotalEquity = nullInt64Ptr(totalEquity)
	f.TotalLiabilities = nullInt64Ptr(totalLiabilities)
	f.CurrentAssets = nullInt64Ptr(currentAssets)
	f.CurrentLiabilities = nullInt64Ptr(currentLiabilities)
	f.EPSBasic = nullFloat64Ptr(epsBasic)
	f.EPSDiluted = nullFloat64Ptr(epsDiluted)
	f.SharesOutstanding = nullInt64Ptr(shares)
	f.ROE = nullFloat64Ptr(roe)
	f.OperatingMargin = nullFloat64Ptr(opMargin)
	f.RevenueGrowthYoY = nullFloat64Ptr(revGrowth)
	f.EPSGrowthYoY = nullFloat64Ptr(epsGrowth)
	f.FwdPER = nullFloat64Ptr(fwdPER)
	f.CyPER = nullFloat64Ptr(cyPER)
	f.DebtRatio = nullFloat64Ptr(debtRatio)
	f.ROA = nullFloat64Ptr(roa)
	f.NetProfitMargin = nullFloat64Ptr(netMargin)
	f.CurrentRatio = nullFloat64Ptr(currentRatio)
	return &f, nil
}

func (r *FundamentalsRepo) RecalcAllStatuses(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT symbol FROM stock_fundamentals ORDER BY symbol`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return updated, err
		}
		f, err := r.GetBySymbol(ctx, symbol)
		if err != nil || f == nil {
			continue
		}
		status, calcErr := calcStatusFromStored(*f)
		if status == f.CalcStatus && calcErr == f.CalcError {
			continue
		}
		_, err = r.db.ExecContext(ctx, `
			UPDATE stock_fundamentals SET calc_status = ?, calc_error = ? WHERE symbol = ?
		`, string(status), nullString(calcErr), symbol)
		if err != nil {
			return updated, err
		}
		updated++
	}
	return updated, rows.Err()
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func nullFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func nullString(v string) sql.NullString {
	if strings.TrimSpace(v) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullDate(v *time.Time) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *v, Valid: true}
}

func nullTime(v *time.Time) sql.NullTime {
	return nullDate(v)
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func nullFloat64Ptr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	x := v.Float64
	return &x
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func calcStatusFromStored(f model.StockFundamentals) (model.FundamentalsCalcStatus, string) {
	var missing []string
	if f.ROE == nil {
		missing = append(missing, "roe")
	}
	if f.OperatingMargin == nil {
		missing = append(missing, "operating_margin")
	}
	if f.RevenueGrowthYoY == nil {
		missing = append(missing, "revenue_growth_yoy")
	}
	if f.EPSGrowthYoY == nil {
		missing = append(missing, "eps_growth_yoy")
	}
	core := 4
	missingCore := len(missing)
	switch {
	case missingCore == 0:
		return model.FundamentalsStatusOK, ""
	case missingCore < core:
		return model.FundamentalsStatusPartial, strings.Join(missing, ",")
	default:
		return model.FundamentalsStatusFailed, strings.Join(missing, ",")
	}
}
