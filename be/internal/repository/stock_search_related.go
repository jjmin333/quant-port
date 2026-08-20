package repository

import (
	"context"
	"sort"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

var englishNameStopWords = map[string]struct{}{
	"inc": {}, "inc.": {}, "corp": {}, "corp.": {}, "corporation": {}, "co": {}, "co.": {},
	"ltd": {}, "ltd.": {}, "limited": {}, "plc": {}, "common": {}, "stock": {}, "stocks": {},
	"adr": {}, "ads": {}, "etf": {}, "trust": {}, "fund": {}, "holdings": {}, "holding": {},
	"company": {}, "companies": {}, "the": {}, "and": {}, "of": {}, "class": {}, "shares": {},
}

// SearchByNameAnchors finds US/ETF rows whose display or English name contains any anchor.
func (r *StockRepo) SearchByNameAnchors(
	ctx context.Context,
	anchors []string,
	excludeSymbols []string,
	limit int,
) ([]model.Stock, error) {
	anchors = normalizeAnchors(anchors)
	if len(anchors) == 0 || limit <= 0 {
		return nil, nil
	}

	query := `
		SELECT symbol, name, english_name, isin_code, market, security_type,
		       currency, asset_type, status, shares_outstanding, list_date, delist_date, updated_at
		FROM stocks
		WHERE status = 'NORMAL'
		  AND asset_type IN ('us', 'etf')
	`
	args := make([]any, 0, len(anchors)*2+len(excludeSymbols)+1)

	parts := make([]string, 0, len(anchors)*2)
	for _, anchor := range anchors {
		like := "%" + anchor + "%"
		parts = append(parts, `(LOWER(name) LIKE ? OR LOWER(english_name) LIKE ?)`)
		args = append(args, like, like)
	}
	query += ` AND (` + strings.Join(parts, " OR ") + `)`

	for _, sym := range excludeSymbols {
		sym = strings.TrimSpace(strings.ToUpper(sym))
		if sym == "" {
			continue
		}
		query += ` AND UPPER(symbol) != ?`
		args = append(args, sym)
	}

	query += ` LIMIT ?`
	args = append(args, limit)

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

// SearchRelatedByUnderlyingSymbol finds leveraged/inverse ETFs referencing an underlying ticker.
func (r *StockRepo) SearchRelatedByUnderlyingSymbol(
	ctx context.Context,
	symbol string,
	excludeSymbols []string,
	limit int,
) ([]model.Stock, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" || len(symbol) < 2 || limit <= 0 {
		return nil, nil
	}

	like := "%" + strings.ToLower(symbol) + "%"
	query := `
		SELECT symbol, name, english_name, isin_code, market, security_type,
		       currency, asset_type, status, shares_outstanding, list_date, delist_date, updated_at
		FROM stocks
		WHERE status = 'NORMAL'
		  AND asset_type = 'etf'
		  AND UPPER(symbol) != ?
		  AND (LOWER(english_name) LIKE ? OR LOWER(name) LIKE ?)
	`
	args := []any{symbol, like, like}

	for _, sym := range excludeSymbols {
		sym = strings.TrimSpace(strings.ToUpper(sym))
		if sym == "" || sym == symbol {
			continue
		}
		query += ` AND UPPER(symbol) != ?`
		args = append(args, sym)
	}

	query += ` LIMIT ?`
	args = append(args, limit)

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

func normalizeAnchors(anchors []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(anchors))
	for _, raw := range anchors {
		a := strings.ToLower(strings.TrimSpace(raw))
		if len([]rune(a)) < 2 {
			continue
		}
		if _, dup := seen[a]; dup {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}

// CompanyNameAnchors extracts searchable tokens from an English security name.
func CompanyNameAnchors(englishName string) []string {
	tokens := tokenizeEnglishName(englishName)
	out := make([]string, 0, 2)
	for _, tok := range tokens {
		if len(tok) < 3 {
			continue
		}
		if _, skip := englishNameStopWords[tok]; skip {
			continue
		}
		out = append(out, tok)
		if len(out) >= 2 {
			break
		}
	}
	return out
}

func relatedProductScore(anchors []string, st model.Stock) int {
	name := strings.ToLower(st.Name + " " + st.EnglishName)
	score := 0
	for _, a := range anchors {
		if strings.Contains(name, a) {
			score += 100
		}
	}
	if st.AssetType == model.AssetETF {
		score += 50
	}
	if strings.Contains(name, "2x") || strings.Contains(name, "3x") ||
		strings.Contains(name, "lever") || strings.Contains(name, "ultra") ||
		strings.Contains(name, "short") || strings.Contains(name, "인버스") ||
		strings.Contains(name, "레버") || strings.Contains(name, "2배") {
		score += 30
	}
	pop := searchPopularity(st)
	if pop > 0 {
		score += 1
	}
	return score
}

func RankRelatedProducts(anchors []string, items []model.Stock) []model.Stock {
	if len(items) == 0 {
		return items
	}
	anchors = normalizeAnchors(anchors)
	scored := make([]scoredStock, 0, len(items))
	for _, st := range items {
		rel := relatedProductScore(anchors, st)
		if rel <= 0 {
			continue
		}
		scored = append(scored, scoredStock{stock: st, score: rel, pop: searchPopularity(st)})
	}
	if len(scored) == 0 {
		return nil
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].pop != scored[j].pop {
			return scored[i].pop > scored[j].pop
		}
		return scored[i].stock.Symbol < scored[j].stock.Symbol
	})
	out := make([]model.Stock, len(scored))
	for i, s := range scored {
		out[i] = s.stock
	}
	return out
}
