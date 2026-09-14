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
	// 검색 동점 처리용 시가총액 스냅샷. 15분 TTL 캐시라 요청마다 조인하지 않는다
	// (`stock_market_caps.go` 헤더에 비용 실측을 남겼다).
	marketCaps *MarketCapIndex
}

func NewStockRepo(db *sql.DB) *StockRepo {
	return &StockRepo{db: db, marketCaps: NewMarketCapIndex(db)}
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

	ranked := rankSearchResultsWithCaps(candidates, q, latinOnly, r.marketCaps.Lookup(ctx))
	return paginateStocks(ranked, limit, offset), nil
}

func (r *StockRepo) searchCandidates(ctx context.Context, p model.StockSearchParams, latinOnly bool, cap int) ([]model.Stock, error) {
	if cap <= 0 {
		cap = searchCandidateCap
	}

	/*
		**토스가 취급하지 않는 종목은 검색에서 빠진다** (2026-09-12 사용자 결정).

		우리 카탈로그는 토스 유니버스의 상위집합이라(워런트·라이츠·우선주 일부·
		상장폐지 직전) 검색 결과에 **열 수도 없는 종목**이 섞였다. 눌러도 시세가
		없고 종가가 없으니, 검색에 있는 것 자체가 거짓이다.

		판정은 `toss_universe` 잡이 하루 7호출로 확정한다. 조건이 「1」이 아니라
		`IS NULL OR = 1`인 이유는 NULL이 「아직 확인 안 했다」이기 때문이다 —
		배치가 처음 돌기 전이나 새로 들어온 종목을 숨기면 화면이 통째로 빈다
		(migration 069 머리글).

		**보유·관심 화면은 이 조건을 쓰지 않는다.** 이미 담아 둔 종목이 사라지면
		사용자의 기록이 사라진 것처럼 보인다 — 숨기는 것은 **발견**(검색·목록)이고
		가진 것은 계속 보여야 한다.
	*/
	query := `
		SELECT symbol, name, english_name, isin_code, market, security_type,
		       currency, asset_type, status, shares_outstanding, list_date, delist_date, updated_at
		FROM stocks
		WHERE status = 'NORMAL'
		  AND ` + TossTradableFilter("") + `
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

	// 표시 순서와 무관하다 — 순위는 Go(`rankSearchResults`)가 정한다. 여기 정렬은
	// `LIMIT`이 잘라내는 창을 고정하고, 강한 매치를 그 창 안에 넣기 위한 것이다.
	appendSearchOrder(&query, &args, p.Query)
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

// FinancialBackfillFilter selects symbols whose valuation and quarterly financial
// tabs are both empty (HPSP-like). ETF/ETN rows are excluded — fundamentals N/A.
type FinancialBackfillFilter struct {
	AssetType    string // model.AssetDomestic | model.AssetUS
	TrackedOnly  bool   // watchlist or user_holdings
	Limit        int
	ExcludeNoCIK bool   // skip US rows already marked cik-not-found failures
}

const financialBackfillStockOnlySQL = `
		  AND s.asset_type <> 'etf'
		  AND UPPER(COALESCE(s.security_type, '')) NOT LIKE '%ETF%'
		  AND UPPER(COALESCE(s.security_type, '')) NOT LIKE '%ETN%'`

const financialBackfillBothEmptySQL = `
		  AND NOT EXISTS (SELECT 1 FROM financial_periods fp WHERE fp.symbol = s.symbol)
		  AND (
		    fund.symbol IS NULL
		    OR fund.calc_status IN ('failed', 'pending')
		    OR (
		      fund.calc_status = 'partial'
		      AND fund.revenue IS NULL AND fund.net_income IS NULL
		      AND fund.total_equity IS NULL AND fund.roe IS NULL
		    )
		  )`

const financialBackfillValuationOkSQL = `
		  AND fund.calc_status = 'ok'
		  AND NOT EXISTS (SELECT 1 FROM financial_periods fp WHERE fp.symbol = s.symbol)`

// financialBackfillHolesSQL — 분기는 이미 있는데 그 안에 빈 칸이 있는 종목.
//
// 「없는 종목」과 대상이 완전히 다르다. 이쪽은 적재는 됐지만 **파서가 계정명을
// 못 읽어** 매출·영업이익·순이익·EPS 중 일부가 NULL로 남은 경우다. 계정명 매칭을
// 고친 뒤 그 종목만 다시 읽으면 채워진다(dart_account_match.go 참조).
const financialBackfillHolesSQL = `
		  AND EXISTS (
		    SELECT 1 FROM financial_periods fp
		    WHERE fp.symbol = s.symbol AND fp.basis = 'quarter'
		      AND (
		        fp.revenue IS NULL OR fp.operating_income IS NULL
		        OR fp.net_income IS NULL OR fp.eps_basic IS NULL
		      )
		  )`

func (r *StockRepo) appendFinancialBackfillOrder(query string, limit int, args []any) (string, []any) {
	query += `
		ORDER BY
		  CASE WHEN EXISTS (SELECT 1 FROM watchlist_items w WHERE w.symbol = s.symbol)
		            OR EXISTS (SELECT 1 FROM user_holdings h WHERE h.symbol = s.symbol)
		       THEN 0 ELSE 1 END,
		  s.symbol ASC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	return query, args
}

func (r *StockRepo) ListBothEmptyFinancialSymbols(ctx context.Context, f FinancialBackfillFilter) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	asset := strings.TrimSpace(f.AssetType)
	if asset == "" {
		return nil, fmt.Errorf("list empty financials: asset_type required")
	}

	query := `
		SELECT s.symbol
		FROM stocks s
		LEFT JOIN stock_fundamentals fund ON fund.symbol = s.symbol
		WHERE s.status = 'NORMAL'
		  AND s.asset_type = ?` + financialBackfillStockOnlySQL + financialBackfillBothEmptySQL
	args := []any{asset}

	if f.TrackedOnly {
		query += `
		  AND (
		    EXISTS (SELECT 1 FROM watchlist_items w WHERE w.symbol = s.symbol)
		    OR EXISTS (SELECT 1 FROM user_holdings h WHERE h.symbol = s.symbol)
		  )`
	}
	if f.ExcludeNoCIK && asset == string(model.AssetUS) {
		// fund.symbol IS NULL must stay in the cohort (never seeded). A bare
		// NOT (calc_status = 'failed' AND …) drops NULL rows in SQL WHERE.
		query += `
		  AND (
		    fund.symbol IS NULL
		    OR NOT (
		      fund.calc_status = 'failed'
		      AND fund.calc_error LIKE '%cik not found%'
		    )
		  )`
	}
	query, args = r.appendFinancialBackfillOrder(query, f.Limit, args)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 64)
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		out = append(out, sym)
	}
	return out, rows.Err()
}

// ListValuationOkNoPeriodsSymbols returns symbols with ok fundamentals but no
// financial_periods rows (valuation tab filled, financials tab empty).
func (r *StockRepo) ListValuationOkNoPeriodsSymbols(ctx context.Context, f FinancialBackfillFilter) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	asset := strings.TrimSpace(f.AssetType)
	if asset == "" {
		return nil, fmt.Errorf("list val_ok no periods: asset_type required")
	}

	query := `
		SELECT s.symbol
		FROM stocks s
		JOIN stock_fundamentals fund ON fund.symbol = s.symbol
		WHERE s.status = 'NORMAL'
		  AND s.asset_type = ?` + financialBackfillStockOnlySQL + financialBackfillValuationOkSQL
	args := []any{asset}

	if f.TrackedOnly {
		query += `
		  AND (
		    EXISTS (SELECT 1 FROM watchlist_items w WHERE w.symbol = s.symbol)
		    OR EXISTS (SELECT 1 FROM user_holdings h WHERE h.symbol = s.symbol)
		  )`
	}
	query, args = r.appendFinancialBackfillOrder(query, f.Limit, args)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 64)
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		out = append(out, sym)
	}
	return out, rows.Err()
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

// ListSymbolsWithFinancialHoles returns symbols whose stored quarters have at
// least one missing flow/EPS figure — the re-read cohort for a parser fix.
//
// 관심·보유 종목이 먼저 나온다(appendFinancialBackfillOrder) — 사용자가 실제로
// 여는 종목부터 메워지는 편이 낫다.
func (r *StockRepo) ListSymbolsWithFinancialHoles(ctx context.Context, f FinancialBackfillFilter) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	asset := strings.TrimSpace(f.AssetType)
	if asset == "" {
		return nil, fmt.Errorf("list financial holes: asset_type required")
	}

	query := `
		SELECT s.symbol
		FROM stocks s
		WHERE s.status = 'NORMAL'
		  AND s.asset_type = ?` + financialBackfillStockOnlySQL + financialBackfillHolesSQL
	args := []any{asset}

	if f.TrackedOnly {
		query += `
		  AND (
		    EXISTS (SELECT 1 FROM watchlist_items w WHERE w.symbol = s.symbol)
		    OR EXISTS (SELECT 1 FROM user_holdings h WHERE h.symbol = s.symbol)
		  )`
	}
	query, args = r.appendFinancialBackfillOrder(query, f.Limit, args)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 64)
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		out = append(out, sym)
	}
	return out, rows.Err()
}
