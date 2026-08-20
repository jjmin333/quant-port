package repository

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// ExistingSymbolSet returns all NORMAL status symbols in the stocks table.
func (r *StockRepo) ExistingSymbolSet(ctx context.Context) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT symbol FROM stocks WHERE status = 'NORMAL'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]struct{}{}
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym != "" {
			out[sym] = struct{}{}
		}
	}
	return out, rows.Err()
}

// ListETPStocks returns ETF/ETN/FOREIGN_ETP catalog rows for logo sync.
func (r *StockRepo) ListETPStocks(ctx context.Context) ([]model.Stock, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, name, english_name, isin_code, market, security_type,
		       currency, asset_type, status, shares_outstanding, list_date, delist_date, updated_at
		FROM stocks
		WHERE status = 'NORMAL'
		  AND (
		    asset_type = 'etf'
		    OR UPPER(security_type) IN ('ETF', 'ETN', 'FOREIGN_ETF')
		  )
		ORDER BY asset_type ASC, symbol ASC
	`)
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

// ListDomesticNameCatalog returns every listed KRX equity's symbol and name,
// for the news pipeline's 종목명 → 심볼 index. Ordered by symbol so a duplicate
// name (a preferred share carrying the same 표시 이름) resolves to the lower
// code, which is the ordinary share.
//
// Derivatives and ETPs are excluded: their names ("KODEX 200선물인버스2X") never
// appear in news prose, and including them only adds index entries to scan.
func (r *StockRepo) ListDomesticNameCatalog(ctx context.Context) ([]model.Stock, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, name
		FROM stocks
		WHERE status = 'NORMAL'
		  AND asset_type = 'domestic'
		  AND name <> ''
		ORDER BY symbol ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Stock
	for rows.Next() {
		var s model.Stock
		if err := rows.Scan(&s.Symbol, &s.Name); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListUSNameCatalog returns every listed US equity's symbol and names for the
// news pipeline's company-name → ticker index (mirrors ListDomesticNameCatalog).
func (r *StockRepo) ListUSNameCatalog(ctx context.Context) ([]model.Stock, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, name, english_name
		FROM stocks
		WHERE status = 'NORMAL'
		  AND asset_type = 'us'
		  AND currency = 'USD'
		  AND (english_name <> '' OR name <> '')
		ORDER BY symbol ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Stock
	for rows.Next() {
		var s model.Stock
		if err := rows.Scan(&s.Symbol, &s.Name, &s.EnglishName); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListNameCatalog returns symbol and display names for import OCR fuzzy matching.
func (r *StockRepo) ListNameCatalog(ctx context.Context) ([]model.Stock, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, name, english_name, isin_code, market, security_type,
		       currency, asset_type, status, shares_outstanding, list_date, delist_date, updated_at
		FROM stocks
		WHERE status = 'NORMAL'
		ORDER BY symbol ASC
	`)
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
