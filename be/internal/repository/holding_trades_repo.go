package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

type HoldingTradesRepo struct {
	db *sql.DB
}

func NewHoldingTradesRepo(db *sql.DB) *HoldingTradesRepo {
	return &HoldingTradesRepo{db: db}
}

const holdingTradeCols = `t.id, t.symbol, COALESCE(s.name, ''), COALESCE(s.english_name, ''),
	t.side, t.quantity, t.price, t.currency, t.fx_rate,
	DATE_FORMAT(t.trade_date, '%Y-%m-%d'), t.fee, t.note, t.created_at`

func (r *HoldingTradesRepo) scan(rows *sql.Rows) ([]model.HoldingTrade, error) {
	out := make([]model.HoldingTrade, 0)
	for rows.Next() {
		var (
			t             model.HoldingTrade
			name, english string
			fx            sql.NullFloat64
		)
		if err := rows.Scan(
			&t.ID, &t.Symbol, &name, &english, &t.Side, &t.Quantity, &t.Price,
			&t.Currency, &fx, &t.TradeDate, &t.Fee, &t.Note, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		if fx.Valid && fx.Float64 > 0 {
			v := fx.Float64
			t.FxRate = &v
		}
		if name != "" {
			t.Name = name
		} else if english != "" {
			t.Name = english
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// List returns every trade for a user, oldest first. The valuation series walks
// them in order, so date ascending is the useful order — not newest-first.
func (r *HoldingTradesRepo) List(ctx context.Context, userID int64) ([]model.HoldingTrade, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+holdingTradeCols+`
		  FROM holding_trades t
		  LEFT JOIN stocks s ON s.symbol = t.symbol
		 WHERE t.user_id = ?
		 ORDER BY t.trade_date ASC, t.id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scan(rows)
}

// ListSymbol returns one symbol's trades, newest first (the detail modal reads
// them as a log).
func (r *HoldingTradesRepo) ListSymbol(ctx context.Context, userID int64, symbol string) ([]model.HoldingTrade, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+holdingTradeCols+`
		  FROM holding_trades t
		  LEFT JOIN stocks s ON s.symbol = t.symbol
		 WHERE t.user_id = ? AND t.symbol = ?
		 ORDER BY t.trade_date DESC, t.id DESC`,
		userID, normalizeSymbol(symbol))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scan(rows)
}

func (r *HoldingTradesRepo) Insert(ctx context.Context, userID int64, t model.HoldingTrade) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO holding_trades
			(user_id, symbol, side, quantity, price, currency, fx_rate, trade_date, fee, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, normalizeSymbol(t.Symbol), t.Side, t.Quantity, t.Price,
		strings.ToUpper(strings.TrimSpace(t.Currency)), nullFloat(t.FxRate),
		t.TradeDate, t.Fee, t.Note)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *HoldingTradesRepo) Delete(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM holding_trades WHERE user_id = ? AND id = ?`, userID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("trade not found")
	}
	return nil
}

// DeleteSymbol drops a symbol's whole history — used when the holding itself is
// deleted, so a re-added symbol does not inherit a stale cost basis.
func (r *HoldingTradesRepo) DeleteSymbol(ctx context.Context, userID int64, symbol string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM holding_trades WHERE user_id = ? AND symbol = ?`,
		userID, normalizeSymbol(symbol))
	return err
}

// TradedSymbols lists the symbols this user has any history for, so callers can
// tell derived holdings from screenshot-only ones without loading every trade.
func (r *HoldingTradesRepo) TradedSymbols(ctx context.Context, userID int64) (map[string]bool, error) {
	if r == nil || r.db == nil {
		return map[string]bool{}, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT symbol FROM holding_trades WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
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

func normalizeSymbol(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// DerivePositions folds a trade list into per-symbol quantity and average cost.
//
// Average cost is the weighted mean of buys, and a sell reduces quantity at that
// average rather than re-pricing it (moving-average method, what Korean brokers
// show). Selling everything and buying again therefore starts a fresh average
// instead of carrying the old one forward.
func DerivePositions(trades []model.HoldingTrade) map[string]model.HoldingPosition {
	type acc struct {
		qty      float64
		cost     float64 // total cost of the remaining quantity, in trade currency
		realized float64
		currency string
		count    int
		first    string
	}
	byS := map[string]*acc{}

	ordered := make([]model.HoldingTrade, len(trades))
	copy(ordered, trades)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].TradeDate != ordered[j].TradeDate {
			return ordered[i].TradeDate < ordered[j].TradeDate
		}
		return ordered[i].ID < ordered[j].ID
	})

	for _, t := range ordered {
		sym := normalizeSymbol(t.Symbol)
		a := byS[sym]
		if a == nil {
			a = &acc{currency: strings.ToUpper(t.Currency), first: t.TradeDate}
			byS[sym] = a
		}
		a.count++
		if a.currency == "" {
			a.currency = strings.ToUpper(t.Currency)
		}
		switch strings.ToLower(t.Side) {
		case "sell":
			sellQty := t.Quantity
			if sellQty > a.qty {
				// Selling more than the recorded history holds. Trust the entry
				// and floor at zero rather than carrying a negative position.
				sellQty = a.qty
			}
			if a.qty > 0 {
				unit := a.cost / a.qty
				a.realized += (t.Price-unit)*sellQty - t.Fee
				a.cost -= unit * sellQty
			}
			a.qty -= sellQty
			if a.qty <= 0.00005 {
				a.qty = 0
				a.cost = 0
			}
		default:
			a.qty += t.Quantity
			a.cost += t.Price*t.Quantity + t.Fee
		}
	}

	out := map[string]model.HoldingPosition{}
	for sym, a := range byS {
		pos := model.HoldingPosition{
			Symbol:      sym,
			Quantity:    a.qty,
			Currency:    a.currency,
			RealizedPnL: a.realized,
			TradeCount:  a.count,
			FirstDate:   a.first,
		}
		if a.qty > 0 {
			pos.AvgPrice = a.cost / a.qty
		}
		out[sym] = pos
	}
	return out
}

// MigrateHoldingTradesIfNeeded applies 039_holding_trades.sql.
func MigrateHoldingTradesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "holding_trades")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "039_holding_trades.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 039_holding_trades.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("039_holding_trades.sql: %w", err)
		}
	}
	log.Println("mysql: applied 039_holding_trades.sql")
	return nil
}

// priceMismatchTolerance is the minimum |trade price − daily close| treated as a mismatch.
const priceMismatchTolerance = 0.01

// tradeDateCloseJoin matches each trade to the latest stock_daily_closes row on or
// before trade_date (exact day when available, else prior session — e.g. same-day
// before EOD backfill).
const tradeDateCloseJoin = `
	INNER JOIN stock_daily_closes dc
	  ON dc.symbol = t.symbol
	 AND dc.trade_date = (
	   SELECT MAX(trade_date) FROM stock_daily_closes
	    WHERE symbol = t.symbol AND trade_date <= t.trade_date
	 )`

const tradeDateCloseJoinT2 = `
	INNER JOIN stock_daily_closes dc
	  ON dc.symbol = t2.symbol
	 AND dc.trade_date = (
	   SELECT MAX(trade_date) FROM stock_daily_closes
	    WHERE symbol = t2.symbol AND trade_date <= t2.trade_date
	 )`

// HoldingTradesPriceMismatchCleanup removes trade history for holdings where the
// recorded buy price (평단) differs from the market close on the trade date.
// Those rows skew the valuation chart (cost uses avg, value uses close).
type HoldingTradesPriceMismatchCleanup struct {
	AffectedPairs int
	DeletedTrades int
	ResetHoldings int
	DryRun        bool
}

// countHoldingTradesPriceMismatches counts distinct user/symbol pairs with at
// least one trade whose price does not match the market close on trade_date.
func countHoldingTradesPriceMismatches(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT DISTINCT t.user_id, t.symbol
			  FROM holding_trades t`+tradeDateCloseJoin+`
			 WHERE ABS(t.price - dc.close_price) >= ?) x`, priceMismatchTolerance).Scan(&n)
	return n, err
}

// CleanupHoldingTradesPriceMismatch deletes all trades for user/symbol pairs
// where any trade price mismatches the daily close on its trade_date, then
// reverts those holdings from source='trades' to 'manual' so quantity/avg remain.
func CleanupHoldingTradesPriceMismatch(ctx context.Context, db *sql.DB, dryRun bool) (HoldingTradesPriceMismatchCleanup, error) {
	out := HoldingTradesPriceMismatchCleanup{DryRun: dryRun}
	tradesExists, err := tableExists(ctx, db, "holding_trades")
	if err != nil {
		return out, err
	}
	closesExists, err := tableExists(ctx, db, "stock_daily_closes")
	if err != nil {
		return out, err
	}
	if !tradesExists || !closesExists {
		return out, nil
	}

	out.AffectedPairs, err = countHoldingTradesPriceMismatches(ctx, db)
	if err != nil {
		return out, err
	}
	if out.AffectedPairs == 0 {
		return out, nil
	}

	if dryRun {
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			  FROM holding_trades t
			  INNER JOIN (
				SELECT DISTINCT t2.user_id, t2.symbol
				  FROM holding_trades t2`+tradeDateCloseJoinT2+`
				 WHERE ABS(t2.price - dc.close_price) >= ?
			  ) bad ON bad.user_id = t.user_id AND bad.symbol = t.symbol`,
			priceMismatchTolerance).Scan(&out.DeletedTrades); err != nil {
			return out, err
		}
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			  FROM user_holdings h
			  INNER JOIN (
				SELECT DISTINCT t2.user_id, t2.symbol
				  FROM holding_trades t2`+tradeDateCloseJoinT2+`
				 WHERE ABS(t2.price - dc.close_price) >= ?
			  ) bad ON bad.user_id = h.user_id AND bad.symbol = h.symbol
			 WHERE h.source = 'trades'`, priceMismatchTolerance).Scan(&out.ResetHoldings); err != nil {
			return out, err
		}
		return out, nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		DELETE t FROM holding_trades t
		INNER JOIN (
			SELECT DISTINCT t2.user_id, t2.symbol
			  FROM holding_trades t2`+tradeDateCloseJoinT2+`
			 WHERE ABS(t2.price - dc.close_price) >= ?
		) bad ON bad.user_id = t.user_id AND bad.symbol = t.symbol`,
		priceMismatchTolerance)
	if err != nil {
		return out, err
	}
	out.DeletedTrades = int(rowsAffected(res))

	res, err = tx.ExecContext(ctx, `
		UPDATE user_holdings h
		   SET h.source = 'manual'
		 WHERE h.source = 'trades'
		   AND NOT EXISTS (
			SELECT 1 FROM holding_trades ht
			 WHERE ht.user_id = h.user_id AND ht.symbol = h.symbol
		   )`)
	if err != nil {
		return out, err
	}
	out.ResetHoldings = int(rowsAffected(res))

	if err := tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}

func rowsAffected(res sql.Result) int64 {
	if res == nil {
		return 0
	}
	n, _ := res.RowsAffected()
	return n
}

// MigrateHoldingTradesPriceMismatchCleanupIfNeeded removes valuation-skewing trades once.
func MigrateHoldingTradesPriceMismatchCleanupIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	_ = migrationsDir
	n, err := countHoldingTradesPriceMismatches(ctx, db)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	result, err := CleanupHoldingTradesPriceMismatch(ctx, db, false)
	if err != nil {
		return err
	}
	log.Printf("mysql: holding_trades price-mismatch cleanup pairs=%d deleted=%d holdings_reset=%d",
		result.AffectedPairs, result.DeletedTrades, result.ResetHoldings)
	return nil
}

const bulkRegisterNote = "보유 종목 일괄 등록"

func (r *HoldingTradesRepo) scanOneTrade(row *sql.Row) (model.HoldingTrade, error) {
	var (
		t             model.HoldingTrade
		name, english string
		fx            sql.NullFloat64
	)
	if err := row.Scan(
		&t.ID, &t.Symbol, &name, &english, &t.Side, &t.Quantity, &t.Price,
		&t.Currency, &fx, &t.TradeDate, &t.Fee, &t.Note, &t.CreatedAt,
	); err != nil {
		return t, err
	}
	if fx.Valid && fx.Float64 > 0 {
		v := fx.Float64
		t.FxRate = &v
	}
	if name != "" {
		t.Name = name
	} else if english != "" {
		t.Name = english
	}
	return t, nil
}

// FindDuplicateTrade returns an existing trade matching the same key fields.
func (r *HoldingTradesRepo) FindDuplicateTrade(ctx context.Context, userID int64, t model.HoldingTrade) (*model.HoldingTrade, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+holdingTradeCols+`
		  FROM holding_trades t
		  LEFT JOIN stocks s ON s.symbol = t.symbol
		 WHERE t.user_id = ? AND t.symbol = ? AND t.side = ?
		   AND ABS(t.quantity - ?) < 0.00001 AND ABS(t.price - ?) < 0.00001
		   AND t.trade_date = ?
		 LIMIT 1`,
		userID, normalizeSymbol(t.Symbol), strings.ToLower(t.Side),
		t.Quantity, t.Price, t.TradeDate)
	tr, err := r.scanOneTrade(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tr, nil
}

// FindBulkRegisterTrade returns the bulk-register trade for a symbol, if any.
func (r *HoldingTradesRepo) FindBulkRegisterTrade(ctx context.Context, userID int64, symbol string) (*model.HoldingTrade, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+holdingTradeCols+`
		  FROM holding_trades t
		  LEFT JOIN stocks s ON s.symbol = t.symbol
		 WHERE t.user_id = ? AND t.symbol = ? AND t.note = ?
		 ORDER BY t.id ASC
		 LIMIT 1`,
		userID, normalizeSymbol(symbol), bulkRegisterNote)
	tr, err := r.scanOneTrade(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tr, nil
}

func (r *HoldingTradesRepo) UpdateTrade(ctx context.Context, userID, id int64, t model.HoldingTrade) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE holding_trades
		   SET quantity = ?, price = ?, currency = ?, trade_date = ?, note = ?
		 WHERE id = ? AND user_id = ?`,
		t.Quantity, t.Price, strings.ToUpper(t.Currency), t.TradeDate, t.Note, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("trade not found")
	}
	return nil
}
