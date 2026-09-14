package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

/*
ListSymbolsTrades — 여러 심볼의 체결을 **한 왕복으로** 읽는다.

일괄 등록(스크린샷 임포트)이 끝나면 영향받은 심볼마다 파생 보유를 다시 계산해야
하는데, 예전에는 심볼마다 `ListSymbol`을 불렀다. 80종목이면 80회 순차 왕복이고
`[HW]` DB가 1 ECPU라 그 시간 동안 한 요청이 커넥션을 붙들고 있었다.

정렬은 `ListSymbol`과 같게 유지한다 — 호출자가 `DerivePositions`에 그대로 넘기고,
그 함수는 최신순 입력을 전제한다.
*/
func (r *HoldingTradesRepo) ListSymbolsTrades(
	ctx context.Context, userID int64, symbols []string,
) (map[string][]model.HoldingTrade, error) {
	out := map[string][]model.HoldingTrade{}
	if r == nil || r.db == nil || len(symbols) == 0 {
		return out, nil
	}

	seen := map[string]struct{}{}
	args := []any{userID}
	var placeholders []string
	for _, sym := range symbols {
		norm := normalizeSymbol(sym)
		if norm == "" {
			continue
		}
		if _, dup := seen[norm]; dup {
			continue
		}
		seen[norm] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, norm)
	}
	if len(placeholders) == 0 {
		return out, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT `+holdingTradeCols+`
		  FROM holding_trades t
		  LEFT JOIN stocks s ON s.symbol = t.symbol
		 WHERE t.user_id = ? AND t.symbol IN (`+strings.Join(placeholders, ",")+`)
		 ORDER BY t.symbol ASC, t.trade_date DESC, t.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	all, err := r.scan(rows)
	if err != nil {
		return nil, err
	}
	for _, t := range all {
		sym := normalizeSymbol(t.Symbol)
		out[sym] = append(out[sym], t)
	}
	// 체결이 하나도 안 남은 심볼도 키를 만들어 둔다 — 호출자가 「전량 매도」를
	// 판단하려면 「조회했는데 없었다」와 「조회 자체를 안 했다」를 구분해야 한다.
	for sym := range seen {
		if _, ok := out[sym]; !ok {
			out[sym] = nil
		}
	}
	return out, nil
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
// FindOversell reports the first trade that sells more than the running
// quantity holds, applying trades in the canonical order (date, then id).
// It returns the offending trade and how much was actually held at that point.
//
// # 읽기는 눈감고 쓰기는 막는 이유
//
// `DerivePositions`는 같은 상황에서 매도 수량을 보유분까지로 **깎는다** — 조회가
// 테이블에 뭐가 들었든 음수 포지션을 만들어서는 안 되기 때문이고, 그건 읽기에서는
// 옳다. 하지만 쓰기에서까지 눈감으면 **없던 매도가 이력에 남는다**: 10주를 갖고
// 50주를 팔면 내역은 「50주 매도」라고 적히는데 실현손익·평가금액은 전부 10주로
// 계산된다. 사용자가 증권사와 대조하면 어디에도 없는 40주를 찾게 된다.
//
// 그래서 들어올 때 막고, 깎는 쪽은 **이미 들어와 있는 옛 행의 안전망**으로 남긴다.
func FindOversell(trades []model.HoldingTrade) (model.HoldingTrade, float64, bool) {
	ordered := make([]model.HoldingTrade, len(trades))
	copy(ordered, trades)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].TradeDate != ordered[j].TradeDate {
			return ordered[i].TradeDate < ordered[j].TradeDate
		}
		return ordered[i].ID < ordered[j].ID
	})

	held := map[string]float64{}
	for _, t := range ordered {
		sym := normalizeSymbol(t.Symbol)
		if strings.ToLower(t.Side) == "sell" {
			// 부동소수 여유는 DerivePositions의 소진 판정과 같은 값을 쓴다.
			if t.Quantity > held[sym]+0.00005 {
				return t, held[sym], true
			}
			held[sym] -= t.Quantity
			continue
		}
		held[sym] += t.Quantity
	}
	return model.HoldingTrade{}, 0, false
}

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

/*
BulkRegisterAll writes every 일괄 등록 항목 in **one transaction**, and returns
the symbols it actually touched.

# 왜 트랜잭션인가 (이게 이 함수의 존재 이유다)

예전에는 서비스가 항목마다 조회 두 번과 쓰기 한 번을 따로 냈다. 중간에서
실패하면 그대로 반환했고, **앞쪽 종목만 등록된 상태가 남았다.** 스크린샷
임포트가 이 경로를 쓰며 한 번에 수십 종목을 넣으므로, 사용자는 "일부만 들어온"
포트폴리오를 보게 되고 그것이 정상인지 알 방법이 없었다
(docs/PERFORMANCE_AUDIT.md P1-8).

# 왜 배치 조회인가

중복 판정과 기존 일괄등록 행 조회가 항목마다 한 번씩이었다. 항목 50개면
조회만 100회다. 여기서는 **대상 심볼의 기존 매매를 한 번에 읽어** 메모리에서
판정한다. 판정 규칙(수량·단가 오차 1e-5, 같은 날짜·방향)은 FindDuplicateTrade와
동일하게 유지했다 — 성능을 위해 판정을 느슨하게 하지 않는다.

# 남겨 둔 것

동시에 들어온 두 일괄 등록이 서로의 쓰기를 못 보고 둘 다 INSERT하는 경합은
그대로다. 단건 `Create` 경로에도 똑같이 있는 오래된 성질이라 여기서만 행 락을
걸면 일관성이 없고, 락 도입은 이 수정의 위험을 크게 키운다. 별도 항목으로 남긴다.
*/
func (r *HoldingTradesRepo) BulkRegisterAll(
	ctx context.Context, userID int64, items []model.HoldingTrade,
) ([]string, error) {
	if r == nil || r.db == nil || len(items) == 0 {
		return nil, nil
	}

	symbols := make([]string, 0, len(items))
	seenSymbol := map[string]struct{}{}
	for _, it := range items {
		sym := normalizeSymbol(it.Symbol)
		if sym == "" {
			continue
		}
		if _, dup := seenSymbol[sym]; dup {
			continue
		}
		seenSymbol[sym] = struct{}{}
		symbols = append(symbols, sym)
	}
	if len(symbols) == 0 {
		return nil, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 대상 심볼의 기존 매매를 한 번에 읽는다.
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(symbols)), ",")
	args := make([]any, 0, len(symbols)+1)
	args = append(args, userID)
	for _, sym := range symbols {
		args = append(args, sym)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, symbol, side, quantity, price, trade_date, COALESCE(note, '')
		  FROM holding_trades
		 WHERE user_id = ? AND symbol IN (`+placeholders+`)
		 ORDER BY id ASC`, args...)
	if err != nil {
		return nil, err
	}

	type existingTrade struct {
		id        int64
		symbol    string
		side      string
		quantity  float64
		price     float64
		tradeDate string
		note      string
	}
	var existing []existingTrade
	for rows.Next() {
		var e existingTrade
		var tradeDate time.Time
		if err := rows.Scan(&e.id, &e.symbol, &e.side, &e.quantity, &e.price, &tradeDate, &e.note); err != nil {
			rows.Close()
			return nil, err
		}
		e.tradeDate = tradeDate.Format("2006-01-02")
		existing = append(existing, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// 심볼별 첫 일괄등록 행(id 오름차순이라 가장 앞의 것).
	bulkRowBySymbol := map[string]int64{}
	for _, e := range existing {
		if e.note != bulkRegisterNote {
			continue
		}
		if _, ok := bulkRowBySymbol[e.symbol]; !ok {
			bulkRowBySymbol[e.symbol] = e.id
		}
	}

	const eps = 0.00001
	isDuplicate := func(t model.HoldingTrade) bool {
		sym := normalizeSymbol(t.Symbol)
		side := strings.ToLower(t.Side)
		for _, e := range existing {
			if e.symbol != sym || strings.ToLower(e.side) != side {
				continue
			}
			if e.tradeDate != t.TradeDate {
				continue
			}
			if math.Abs(e.quantity-t.Quantity) < eps && math.Abs(e.price-t.Price) < eps {
				return true
			}
		}
		return false
	}

	touched := map[string]struct{}{}
	var order []string
	markTouched := func(sym string) {
		if _, ok := touched[sym]; ok {
			return
		}
		touched[sym] = struct{}{}
		order = append(order, sym)
	}

	for _, t := range items {
		sym := normalizeSymbol(t.Symbol)
		if sym == "" {
			continue
		}
		t.Symbol = sym

		if isDuplicate(t) {
			markTouched(sym)
			continue
		}
		if id, ok := bulkRowBySymbol[sym]; ok {
			if _, err := tx.ExecContext(ctx, `
				UPDATE holding_trades
				   SET quantity = ?, price = ?, currency = ?, trade_date = ?, note = ?
				 WHERE id = ? AND user_id = ?`,
				t.Quantity, t.Price, strings.ToUpper(t.Currency), t.TradeDate, t.Note, id, userID); err != nil {
				return nil, err
			}
			markTouched(sym)
			continue
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO holding_trades
				(user_id, symbol, side, quantity, price, currency, fx_rate, trade_date, fee, note)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			userID, sym, t.Side, t.Quantity, t.Price,
			strings.ToUpper(strings.TrimSpace(t.Currency)), nullFloat(t.FxRate),
			t.TradeDate, t.Fee, t.Note)
		if err != nil {
			return nil, err
		}
		// 같은 요청 안에서 두 항목이 같은 심볼을 가리키면, 뒤엣것이 방금 넣은
		// 행을 갱신하도록 기록해 둔다 — 예전 흐름과 같은 결과다.
		if t.Note == bulkRegisterNote {
			if id, err := res.LastInsertId(); err == nil {
				if _, ok := bulkRowBySymbol[sym]; !ok {
					bulkRowBySymbol[sym] = id
				}
			}
		}
		markTouched(sym)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return order, nil
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
