package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

const persistedTradeTickLimit = 50

type QuoteTerminalRepo struct {
	db    *sql.DB
	table *tableExistsCache
}

func NewQuoteTerminalRepo(db *sql.DB) *QuoteTerminalRepo {
	return &QuoteTerminalRepo{db: db, table: newTableExistsCache("quote_snapshots")}
}

func (r *QuoteTerminalRepo) HasTable(ctx context.Context) bool {
	return r.table.Has(ctx, r.db)
}

func (r *QuoteTerminalRepo) UpsertDetail(ctx context.Context, symbol string, detail model.QuoteDetail, source string) error {
	if !r.HasTable(ctx) {
		return nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	payload, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	if source == "" {
		source = "toss"
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO quote_snapshots (symbol, detail_json, source)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			detail_json = VALUES(detail_json),
			source = VALUES(source)
	`, symbol, payload, source)
	return err
}

func (r *QuoteTerminalRepo) GetDetail(ctx context.Context, symbol string) (model.QuoteDetail, bool) {
	if !r.HasTable(ctx) {
		return model.QuoteDetail{}, false
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT detail_json FROM quote_snapshots WHERE symbol = ?
	`, symbol).Scan(&raw)
	if err != nil || len(raw) == 0 {
		return model.QuoteDetail{}, false
	}
	var detail model.QuoteDetail
	if json.Unmarshal(raw, &detail) != nil || detail.Symbol == "" {
		return model.QuoteDetail{}, false
	}
	return detail, true
}

func (r *QuoteTerminalRepo) UpsertOrderBook(ctx context.Context, symbol string, book model.OrderBook) error {
	if !r.HasTable(ctx) {
		return nil
	}
	if len(book.Asks) == 0 && len(book.Bids) == 0 {
		return nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	book.Symbol = symbol
	payload, err := json.Marshal(book)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO orderbook_snapshots (symbol, book_json)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE book_json = VALUES(book_json)
	`, symbol, payload)
	return err
}

func (r *QuoteTerminalRepo) GetOrderBook(ctx context.Context, symbol string) (model.OrderBook, bool) {
	if !r.HasTable(ctx) {
		return model.OrderBook{}, false
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT book_json FROM orderbook_snapshots WHERE symbol = ?
	`, symbol).Scan(&raw)
	if err != nil || len(raw) == 0 {
		return model.OrderBook{}, false
	}
	var book model.OrderBook
	if json.Unmarshal(raw, &book) != nil {
		return model.OrderBook{}, false
	}
	if book.Symbol == "" {
		book.Symbol = symbol
	}
	return book, len(book.Asks) > 0 || len(book.Bids) > 0
}

func (r *QuoteTerminalRepo) ReplaceTrades(ctx context.Context, symbol string, trades []model.TradeTick) error {
	if !r.HasTable(ctx) || len(trades) == 0 {
		return nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if len(trades) > persistedTradeTickLimit {
		trades = trades[:persistedTradeTickLimit]
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM trade_ticks WHERE symbol = ?`, symbol); err != nil {
		return err
	}

	const stmt = `
		INSERT INTO trade_ticks (symbol, trade_id, trade_time, price, quantity, side)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	for _, t := range trades {
		id := strings.TrimSpace(t.ID)
		if id == "" {
			id = fmt.Sprintf("%s-%g-%v", t.Time, t.Price, t.Quantity)
		}
		side := strings.TrimSpace(t.Side)
		if side == "" {
			side = "unknown"
		}
		if _, err := tx.ExecContext(ctx, stmt, symbol, id, t.Time, t.Price, t.Quantity, side); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *QuoteTerminalRepo) ListTrades(ctx context.Context, symbol string, limit int) ([]model.TradeTick, error) {
	if !r.HasTable(ctx) {
		return nil, nil
	}
	if limit <= 0 {
		limit = persistedTradeTickLimit
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	rows, err := r.db.QueryContext(ctx, `
		SELECT trade_id, trade_time, price, quantity, side
		FROM trade_ticks
		WHERE symbol = ?
		ORDER BY trade_time DESC
		LIMIT ?
	`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.TradeTick
	for rows.Next() {
		var t model.TradeTick
		t.Symbol = symbol
		if err := rows.Scan(&t.ID, &t.Time, &t.Price, &t.Quantity, &t.Side); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *QuoteTerminalRepo) UpsertLastPrice(ctx context.Context, symbol string, lastPrice float64, currency model.QuoteCurrency) error {
	if !r.HasTable(ctx) || lastPrice <= 0 {
		return nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	cur := string(currency)
	if cur == "" {
		cur = string(model.CurrencyKRW)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO symbol_last_prices (symbol, last_price, currency)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			last_price = VALUES(last_price),
			currency = VALUES(currency)
	`, symbol, lastPrice, cur)
	return err
}

func (r *QuoteTerminalRepo) GetLastPrice(ctx context.Context, symbol string) (toss.PriceResponse, bool) {
	prices := r.GetLastPrices(ctx, []string{symbol})
	p, ok := prices[strings.ToUpper(strings.TrimSpace(symbol))]
	return p, ok
}

func (r *QuoteTerminalRepo) GetLastPrices(ctx context.Context, symbols []string) map[string]toss.PriceResponse {
	out := make(map[string]toss.PriceResponse)
	if !r.HasTable(ctx) || len(symbols) == 0 {
		return out
	}

	seen := map[string]struct{}{}
	var args []any
	var placeholders []string
	for _, sym := range symbols {
		up := strings.ToUpper(strings.TrimSpace(sym))
		if up == "" {
			continue
		}
		if _, ok := seen[up]; ok {
			continue
		}
		seen[up] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, up)
	}
	if len(args) == 0 {
		return out
	}

	query := fmt.Sprintf(`
		SELECT symbol, last_price, currency
		FROM symbol_last_prices
		WHERE symbol IN (%s)
	`, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var sym, cur string
		var price float64
		if err := rows.Scan(&sym, &price, &cur); err != nil {
			continue
		}
		if price <= 0 {
			continue
		}
		out[sym] = toss.PriceResponse{
			Symbol:    sym,
			LastPrice: strconv.FormatFloat(price, 'f', -1, 64),
			Currency:  toss.Currency(cur),
		}
	}
	return out
}
