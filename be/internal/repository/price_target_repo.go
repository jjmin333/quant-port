package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

var ErrPriceTargetNotFound = errors.New("price target not found")

type PriceTargetRepo struct {
	db *sql.DB
}

func NewPriceTargetRepo(db *sql.DB) *PriceTargetRepo {
	return &PriceTargetRepo{db: db}
}

const priceTargetCols = `id, symbol, target_price, target_price_2, stop_price, ` +
	`base_price, base_date, currency, note, created_at, updated_at`

func scanPriceTarget(row interface {
	Scan(dest ...any) error
}) (model.PriceTarget, error) {
	var (
		t         model.PriceTarget
		target2   sql.NullFloat64
		stop      sql.NullFloat64
		base      sql.NullFloat64
		baseDate  sql.NullTime
		createdAt time.Time
		updatedAt time.Time
	)
	if err := row.Scan(
		&t.ID, &t.Symbol, &t.TargetPrice, &target2, &stop, &base, &baseDate,
		&t.Currency, &t.Note, &createdAt, &updatedAt,
	); err != nil {
		return t, err
	}
	t.TargetPrice2 = positiveOrNil(target2)
	t.StopPrice = positiveOrNil(stop)
	t.BasePrice = positiveOrNil(base)
	if baseDate.Valid {
		t.BaseDate = baseDate.Time.Format("2006-01-02")
	}
	t.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	t.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return t, nil
}

// positiveOrNil keeps a stored 0 out of the response. A stop price of 0 would
// render as "손절 0원" and, worse, read as "already stopped out".
func positiveOrNil(v sql.NullFloat64) *float64 {
	if !v.Valid || v.Float64 <= 0 {
		return nil
	}
	out := v.Float64
	return &out
}

// PriceTargetWrite is the settable part of a target. Base price is on it
// because Create measures it; Update leaves it untouched.
type PriceTargetWrite struct {
	Symbol       string
	TargetPrice  float64
	TargetPrice2 float64
	StopPrice    float64
	Currency     string
	Note         string

	BasePrice float64
	BaseDate  string
}

func (r *PriceTargetRepo) List(ctx context.Context, userID int64) ([]model.PriceTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+priceTargetCols+`
		  FROM user_price_targets
		 WHERE user_id = ?
		 ORDER BY updated_at DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.PriceTarget, 0)
	for rows.Next() {
		t, err := scanPriceTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Create upserts on (user_id, symbol).
//
// base_price is written with COALESCE so that re-registering an existing symbol
// keeps the first registration's price. Overwriting it would quietly reset the
// "등록 후 +14.9%" reading every time the user nudged the target upward — the
// number would then only ever say "roughly flat".
func (r *PriceTargetRepo) Create(ctx context.Context, userID int64, w PriceTargetWrite) (model.PriceTarget, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO user_price_targets
			(user_id, symbol, target_price, target_price_2, stop_price,
			 base_price, base_date, currency, note)
		VALUES (?, ?, ?, NULLIF(?, 0), NULLIF(?, 0), NULLIF(?, 0), NULLIF(?, ''), ?, ?)
		ON DUPLICATE KEY UPDATE
			target_price = VALUES(target_price),
			target_price_2 = VALUES(target_price_2),
			stop_price = VALUES(stop_price),
			base_price = COALESCE(base_price, VALUES(base_price)),
			base_date = COALESCE(base_date, VALUES(base_date)),
			currency = VALUES(currency),
			note = VALUES(note),
			updated_at = CURRENT_TIMESTAMP(3)`,
		userID, normalizeSymbol(w.Symbol), w.TargetPrice, w.TargetPrice2, w.StopPrice,
		w.BasePrice, w.BaseDate, strings.ToUpper(w.Currency), w.Note)
	if err != nil {
		return model.PriceTarget{}, err
	}
	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		return r.getBySymbol(ctx, userID, w.Symbol)
	}
	return r.get(ctx, userID, id)
}

// Update never touches base_price/base_date — see Create.
func (r *PriceTargetRepo) Update(ctx context.Context, userID, id int64, w PriceTargetWrite) (model.PriceTarget, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE user_price_targets
		   SET target_price = ?, target_price_2 = NULLIF(?, 0), stop_price = NULLIF(?, 0),
		       currency = ?, note = ?
		 WHERE id = ? AND user_id = ?`,
		w.TargetPrice, w.TargetPrice2, w.StopPrice,
		strings.ToUpper(w.Currency), w.Note, id, userID)
	if err != nil {
		return model.PriceTarget{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.PriceTarget{}, ErrPriceTargetNotFound
	}
	return r.get(ctx, userID, id)
}

func (r *PriceTargetRepo) Delete(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM user_price_targets WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrPriceTargetNotFound
	}
	return nil
}

func (r *PriceTargetRepo) get(ctx context.Context, userID, id int64) (model.PriceTarget, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+priceTargetCols+` FROM user_price_targets WHERE id = ? AND user_id = ?`,
		id, userID)
	t, err := scanPriceTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PriceTarget{}, ErrPriceTargetNotFound
	}
	return t, err
}

func (r *PriceTargetRepo) getBySymbol(ctx context.Context, userID int64, symbol string) (model.PriceTarget, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+priceTargetCols+` FROM user_price_targets WHERE user_id = ? AND symbol = ?`,
		userID, normalizeSymbol(symbol))
	t, err := scanPriceTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PriceTarget{}, ErrPriceTargetNotFound
	}
	return t, err
}

// MigrateUserPriceTargetsIfNeeded applies 043_user_price_targets.sql.
func MigrateUserPriceTargetsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "user_price_targets")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "043_user_price_targets.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 043_user_price_targets.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("043_user_price_targets.sql: %w", err)
		}
	}
	log.Println("mysql: applied 043_user_price_targets.sql")
	return nil
}

// MigratePriceTargetLevelsIfNeeded applies 048_price_target_levels.sql.
//
// Existing rows keep a NULL base_price. There is no honest way to recover the
// price a target was set at — the table never stored it — and inserting today's
// price would claim every old target was registered this morning.
func MigratePriceTargetLevelsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "user_price_targets")
	if err != nil || !exists {
		return err
	}
	has, err := columnExists(ctx, db, "user_price_targets", "base_price")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	path := filepath.Join(migrationsDir, "048_price_target_levels.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 048_price_target_levels.sql: %w", err)
	}
	applied := false
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("048_price_target_levels.sql: %w", err)
		}
		applied = true
	}
	if applied {
		log.Println("mysql: user_price_targets migrated to v2 (base_price, stop_price, target_price_2)")
	}
	return nil
}

// MigrateHoldingTradesDedupIfNeeded removes duplicate holding_trades rows once.
func MigrateHoldingTradesDedupIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "holding_trades")
	if err != nil || !exists {
		return err
	}
	var dupCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM holding_trades t1
		INNER JOIN holding_trades t2
		  ON t1.user_id = t2.user_id AND t1.symbol = t2.symbol AND t1.side = t2.side
		 AND ABS(t1.quantity - t2.quantity) < 0.00001 AND ABS(t1.price - t2.price) < 0.00001
		 AND t1.trade_date = t2.trade_date AND t1.id > t2.id`).Scan(&dupCount); err != nil {
		return err
	}
	if dupCount == 0 {
		return nil
	}
	path := filepath.Join(migrationsDir, "044_holding_trades_dedup.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 044_holding_trades_dedup.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("044_holding_trades_dedup.sql: %w", err)
		}
	}
	log.Printf("mysql: holding_trades dedup removed %d duplicate rows", dupCount)
	return nil
}
