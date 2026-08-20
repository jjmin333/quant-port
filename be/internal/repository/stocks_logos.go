package repository

import (
	"context"

	"github.com/enact/quant-port/be/internal/model"
)

// ListLogoStocks returns active catalog rows for logo migration (symbol, currency, asset_type).
func (r *StockRepo) ListLogoStocks(ctx context.Context) ([]model.Stock, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, currency, asset_type
		FROM stocks
		WHERE status = 'NORMAL'
		ORDER BY asset_type ASC, symbol ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Stock
	for rows.Next() {
		var s model.Stock
		var currency, assetType string
		if err := rows.Scan(&s.Symbol, &currency, &assetType); err != nil {
			return nil, err
		}
		s.Currency = model.QuoteCurrency(currency)
		s.AssetType = model.QuoteAssetType(assetType)
		out = append(out, s)
	}
	return out, rows.Err()
}
