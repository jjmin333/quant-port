package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

type StockRepo struct {
	db *sql.DB
}

func NewStockRepo(db *sql.DB) *StockRepo {
	return &StockRepo{db: db}
}

func (r *StockRepo) Upsert(ctx context.Context, s model.Stock) error {
	var listDate, delistDate sql.NullTime
	if s.ListDate != nil {
		listDate = sql.NullTime{Time: *s.ListDate, Valid: true}
	}
	if s.DelistDate != nil {
		delistDate = sql.NullTime{Time: *s.DelistDate, Valid: true}
	}
	var shares sql.NullInt64
	if s.SharesOutstanding != nil {
		shares = sql.NullInt64{Int64: *s.SharesOutstanding, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stocks (
			symbol, name, english_name, isin_code, market, security_type,
			currency, asset_type, status, shares_outstanding, list_date, delist_date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			english_name = VALUES(english_name),
			isin_code = VALUES(isin_code),
			market = VALUES(market),
			security_type = VALUES(security_type),
			currency = VALUES(currency),
			asset_type = VALUES(asset_type),
			status = VALUES(status),
			shares_outstanding = VALUES(shares_outstanding),
			list_date = VALUES(list_date),
			delist_date = VALUES(delist_date)
	`,
		strings.ToUpper(strings.TrimSpace(s.Symbol)),
		s.Name,
		s.EnglishName,
		s.ISINCode,
		s.Market,
		s.SecurityType,
		string(s.Currency),
		string(s.AssetType),
		s.Status,
		shares,
		listDate,
		delistDate,
	)
	return err
}

func (r *StockRepo) UpsertBatch(ctx context.Context, stocks []model.Stock) error {
	if len(stocks) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const query = `
		INSERT INTO stocks (
			symbol, name, english_name, isin_code, market, security_type,
			currency, asset_type, status, shares_outstanding, list_date, delist_date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			english_name = VALUES(english_name),
			isin_code = VALUES(isin_code),
			market = VALUES(market),
			security_type = VALUES(security_type),
			currency = VALUES(currency),
			asset_type = VALUES(asset_type),
			status = VALUES(status),
			shares_outstanding = VALUES(shares_outstanding),
			list_date = VALUES(list_date),
			delist_date = VALUES(delist_date)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range stocks {
		var listDate, delistDate sql.NullTime
		if s.ListDate != nil {
			listDate = sql.NullTime{Time: *s.ListDate, Valid: true}
		}
		if s.DelistDate != nil {
			delistDate = sql.NullTime{Time: *s.DelistDate, Valid: true}
		}
		var shares sql.NullInt64
		if s.SharesOutstanding != nil {
			shares = sql.NullInt64{Int64: *s.SharesOutstanding, Valid: true}
		}

		if _, err := stmt.ExecContext(ctx,
			strings.ToUpper(strings.TrimSpace(s.Symbol)),
			s.Name,
			s.EnglishName,
			s.ISINCode,
			s.Market,
			s.SecurityType,
			string(s.Currency),
			string(s.AssetType),
			s.Status,
			shares,
			listDate,
			delistDate,
		); err != nil {
			return fmt.Errorf("upsert %s: %w", s.Symbol, err)
		}
	}

	return tx.Commit()
}

func (r *StockRepo) TotalCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stocks`).Scan(&n)
	return n, err
}

func (r *StockRepo) ListBySymbols(ctx context.Context, symbols []string) ([]model.Stock, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	seen := map[string]struct{}{}
	args := make([]any, 0, len(symbols))
	placeholders := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, sym)
	}
	if len(args) == 0 {
		return nil, nil
	}

	query := fmt.Sprintf(`
		SELECT symbol, name, english_name, isin_code, market, security_type,
		       currency, asset_type, status, shares_outstanding, list_date, delist_date, updated_at
		FROM stocks
		WHERE symbol IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Stock
	for rows.Next() {
		s, err := scanStockRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *StockRepo) GetBySymbol(ctx context.Context, symbol string) (*model.Stock, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	row := r.db.QueryRowContext(ctx, `
		SELECT symbol, name, english_name, isin_code, market, security_type,
		       currency, asset_type, status, shares_outstanding, list_date, delist_date, updated_at
		FROM stocks WHERE symbol = ?
	`, symbol)

	var s model.Stock
	var currency, assetType string
	var shares sql.NullInt64
	var listDate, delistDate sql.NullTime
	if err := row.Scan(
		&s.Symbol, &s.Name, &s.EnglishName, &s.ISINCode, &s.Market, &s.SecurityType,
		&currency, &assetType, &s.Status, &shares, &listDate, &delistDate, &s.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.Currency = model.QuoteCurrency(currency)
	s.AssetType = model.QuoteAssetType(assetType)
	if shares.Valid {
		v := shares.Int64
		s.SharesOutstanding = &v
	}
	if listDate.Valid {
		t := listDate.Time
		s.ListDate = &t
	}
	if delistDate.Valid {
		t := delistDate.Time
		s.DelistDate = &t
	}
	return &s, nil
}

func (r *StockRepo) Search(ctx context.Context, p model.StockSearchParams) ([]model.Stock, error) {
	limit := normalizeSearchLimit(p.Limit)
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}

	q := strings.TrimSpace(p.Query)
	latinOnly := q != "" && IsLatinOnlyQuery(q)

	candidates, err := r.searchCandidates(ctx, p, latinOnly, searchCandidateCap)
	if err != nil {
		return nil, err
	}

	ranked := rankSearchResults(candidates, q, latinOnly)
	return paginateStocks(ranked, limit, offset), nil
}

func (r *StockRepo) searchCandidates(ctx context.Context, p model.StockSearchParams, latinOnly bool, cap int) ([]model.Stock, error) {
	if cap <= 0 {
		cap = searchCandidateCap
	}

	query := `
		SELECT symbol, name, english_name, isin_code, market, security_type,
		       currency, asset_type, status, shares_outstanding, list_date, delist_date, updated_at
		FROM stocks
		WHERE status = 'NORMAL'
	`
	args := make([]any, 0, 8)

	if p.AssetType != "" {
		query += ` AND asset_type = ?`
		args = append(args, string(p.AssetType))
	} else if p.Market == "domestic" {
		query += ` AND asset_type = 'domestic'`
	} else if p.Market == "us" {
		query += ` AND asset_type IN ('us', 'etf')`
	}

	appendSearchQueryFilter(&query, &args, p.Query, latinOnly)

	query += ` LIMIT ?`
	args = append(args, cap)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Stock
	for rows.Next() {
		s, err := scanStockRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListSymbols returns all active stock symbols for batch jobs (no 200-row search cap).
func (r *StockRepo) ListSymbols(ctx context.Context, assetType string, limit int) ([]string, error) {
	query := `SELECT symbol FROM stocks WHERE status = 'NORMAL'`
	args := make([]any, 0, 2)

	switch strings.ToLower(strings.TrimSpace(assetType)) {
	case "domestic":
		query += ` AND asset_type = ?`
		args = append(args, string(model.AssetDomestic))
	case "us":
		query += ` AND asset_type IN ('us', 'etf')`
	case "", "all":
	default:
		return nil, fmt.Errorf("invalid asset type %q", assetType)
	}

	query += ` ORDER BY symbol ASC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
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

// ListSymbolsMissingKoreanName returns US/ETF symbols whose display name has no Hangul (Toss sync pending).
func (r *StockRepo) ListSymbolsMissingKoreanName(ctx context.Context, assetType string, limit int) ([]string, error) {
	query := `
		SELECT symbol FROM stocks
		WHERE status = 'NORMAL'
		  AND symbol NOT LIKE '%^%'
		  AND name NOT REGEXP '[가-힣]'
	`
	args := make([]any, 0, 2)

	switch strings.ToLower(strings.TrimSpace(assetType)) {
	case "domestic":
		query += ` AND asset_type = ?`
		args = append(args, string(model.AssetDomestic))
	case "us", "":
		query += ` AND asset_type IN ('us', 'etf')`
	case "all":
	default:
		return nil, fmt.Errorf("invalid asset type %q", assetType)
	}

	query += ` ORDER BY symbol ASC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
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

func (r *StockRepo) Count(ctx context.Context, p model.StockSearchParams) (int, error) {
	q := strings.TrimSpace(p.Query)
	if q == "" {
		query := `SELECT COUNT(*) FROM stocks WHERE status = 'NORMAL'`
		args := make([]any, 0, 4)

		if p.AssetType != "" {
			query += ` AND asset_type = ?`
			args = append(args, string(p.AssetType))
		} else if p.Market == "domestic" {
			query += ` AND asset_type = 'domestic'`
		} else if p.Market == "us" {
			query += ` AND asset_type IN ('us', 'etf')`
		}

		var n int
		if err := r.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			return 0, err
		}
		return n, nil
	}

	latinOnly := IsLatinOnlyQuery(q)
	candidates, err := r.searchCandidates(ctx, p, latinOnly, searchCandidateCap)
	if err != nil {
		return 0, err
	}
	return CountRankedSearch(candidates, q, latinOnly), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanStockRow(row rowScanner) (model.Stock, error) {
	var s model.Stock
	var currency, assetType string
	var shares sql.NullInt64
	var listDate, delistDate sql.NullTime
	if err := row.Scan(
		&s.Symbol, &s.Name, &s.EnglishName, &s.ISINCode, &s.Market, &s.SecurityType,
		&currency, &assetType, &s.Status, &shares, &listDate, &delistDate, &s.UpdatedAt,
	); err != nil {
		return model.Stock{}, fmt.Errorf("scan stock: %w", err)
	}
	s.Currency = model.QuoteCurrency(currency)
	s.AssetType = model.QuoteAssetType(assetType)
	if shares.Valid {
		v := shares.Int64
		s.SharesOutstanding = &v
	}
	if listDate.Valid {
		t := listDate.Time
		s.ListDate = &t
	}
	if delistDate.Valid {
		t := delistDate.Time
		s.DelistDate = &t
	}
	return s, nil
}

// ParseListDate parses YYYY-MM-DD from Toss or CSV seed files.
func ParseListDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
