package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/enact/quant-port/be/internal/model"
)

var ErrStampNotFound = errors.New("stamp not found")

type StampRepo struct {
	db *sql.DB
}

func NewStampRepo(db *sql.DB) *StampRepo {
	return &StampRepo{db: db}
}

const stampCols = `id, symbol, symbol_name, asset_type, currency, stamped_at, trade_date,
	market_session, benchmark_id, price, prev_close, change_rate, volume, score, grade,
	stance, target_price, conviction, horizon_kind, due_at, note, tags_json,
	snapshot, context, chart, opened_at, created_at, updated_at`

// StampListFilter narrows a user's stamp list. Zero values mean "no filter".
type StampListFilter struct {
	Symbol string
	Stance string
	From   string // YYYY-MM-DD, inclusive (trade_date)
	To     string // YYYY-MM-DD, inclusive (trade_date)
	Limit  int
	Offset int
}

func scanStamp(row interface{ Scan(dest ...any) error }) (model.Stamp, error) {
	var (
		s          model.Stamp
		assetType  string
		currency   string
		stampedAt  time.Time
		tradeDate  time.Time
		prevClose  sql.NullFloat64
		changeRate sql.NullFloat64
		score      sql.NullInt64
		grade      sql.NullString
		stance      string
		horizonKind string
		dueAt       time.Time
		tagsJSON    []byte
		snapJSON    []byte
		ctxJSON     []byte
		chartJSON   []byte
		openedAt    sql.NullTime
		createdAt   time.Time
		updatedAt   time.Time
	)
	if err := row.Scan(
		&s.ID, &s.Symbol, &s.SymbolName, &assetType, &currency, &stampedAt, &tradeDate,
		&s.MarketSession, &s.BenchmarkID, &s.Price, &prevClose, &changeRate, &s.Volume,
		&score, &grade, &stance, &s.TargetPrice, &s.Conviction, &horizonKind, &dueAt,
		&s.Note, &tagsJSON, &snapJSON, &ctxJSON, &chartJSON, &openedAt, &createdAt, &updatedAt,
	); err != nil {
		return model.Stamp{}, err
	}

	s.AssetType = model.QuoteAssetType(assetType)
	s.Currency = model.QuoteCurrency(currency)
	s.StampedAt = stampedAt.UTC()
	s.TradeDate = tradeDate.Format("2006-01-02")
	s.Stance = model.StampStance(stance)
	s.HorizonKind = model.StampHorizonKind(horizonKind)
	s.DueAt = dueAt.UTC()
	s.CreatedAt = createdAt.UTC()
	s.UpdatedAt = updatedAt.UTC()
	if prevClose.Valid {
		v := prevClose.Float64
		s.PrevClose = &v
	}
	if changeRate.Valid {
		v := changeRate.Float64
		s.ChangeRate = &v
	}
	if score.Valid {
		v := int(score.Int64)
		s.Score = &v
	}
	s.Grade = grade.String
	if openedAt.Valid {
		t := openedAt.Time.UTC()
		s.OpenedAt = &t
	}

	// A malformed JSON blob must not sink the whole list — the stamp still has
	// its scalar columns, which is what the list view renders.
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &s.Tags)
	}
	if len(snapJSON) > 0 {
		_ = json.Unmarshal(snapJSON, &s.Snapshot)
	}
	if len(ctxJSON) > 0 {
		_ = json.Unmarshal(ctxJSON, &s.Context)
	}
	if len(chartJSON) > 0 {
		_ = json.Unmarshal(chartJSON, &s.Chart)
	}
	if s.Context.News == nil {
		s.Context.News = []model.StampNews{}
	}
	if s.Context.Disclosures == nil {
		s.Context.Disclosures = []model.BriefingEvent{}
	}
	if s.Context.Indices == nil {
		s.Context.Indices = []model.StampIndex{}
	}
	return s, nil
}

func (r *StampRepo) Create(ctx context.Context, userID int64, s model.Stamp) (model.Stamp, error) {
	tags, err := json.Marshal(s.Tags)
	if err != nil {
		return model.Stamp{}, err
	}
	snap, err := json.Marshal(s.Snapshot)
	if err != nil {
		return model.Stamp{}, err
	}
	situation, err := json.Marshal(s.Context)
	if err != nil {
		return model.Stamp{}, err
	}
	chart, err := json.Marshal(s.Chart)
	if err != nil {
		return model.Stamp{}, err
	}

	res, err := r.db.ExecContext(ctx, `
		INSERT INTO stamps (
			user_id, symbol, symbol_name, asset_type, currency, stamped_at, trade_date,
			market_session, benchmark_id, price, prev_close, change_rate, volume, score, grade,
			stance, target_price, conviction, horizon_kind, due_at, note, tags_json,
			snapshot, context, chart
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		userID, s.Symbol, s.SymbolName, string(s.AssetType), string(s.Currency),
		s.StampedAt.UTC(), s.TradeDate, s.MarketSession, s.BenchmarkID,
		// nullFloat64 (not nullFloat) — nullFloat drops non-positive values, which
		// would silently erase every negative change rate, the half of the data a
		// stamp most needs to keep.
		s.Price, nullFloat64(s.PrevClose), nullFloat64(s.ChangeRate), s.Volume,
		nullIntArg(s.Score), nullString(s.Grade), string(s.Stance), s.TargetPrice,
		s.Conviction, string(s.HorizonKind), s.DueAt.UTC(),
		s.Note, string(tags), string(snap), string(situation), string(chart),
	)
	if err != nil {
		return model.Stamp{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Stamp{}, err
	}
	return r.Get(ctx, userID, id)
}

func (r *StampRepo) Get(ctx context.Context, userID, id int64) (model.Stamp, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+stampCols+` FROM stamps WHERE id = ? AND user_id = ?`, id, userID)
	s, err := scanStamp(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Stamp{}, ErrStampNotFound
	}
	return s, err
}

func (r *StampRepo) List(ctx context.Context, userID int64, f StampListFilter) ([]model.Stamp, error) {
	query := `SELECT ` + stampCols + ` FROM stamps WHERE user_id = ?`
	args := []any{userID}

	if f.Symbol != "" {
		query += ` AND symbol = ?`
		args = append(args, strings.ToUpper(f.Symbol))
	}
	if f.Stance != "" {
		query += ` AND stance = ?`
		args = append(args, f.Stance)
	}
	if f.From != "" {
		query += ` AND trade_date >= ?`
		args = append(args, f.From)
	}
	if f.To != "" {
		query += ` AND trade_date <= ?`
		args = append(args, f.To)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	query += ` ORDER BY stamped_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, max(f.Offset, 0))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Stamp, 0)
	for rows.Next() {
		s, err := scanStamp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateAnnotation writes the fields that stay editable for the life of a
// stamp. Target price, stance and deadline are absent on purpose: they are the
// prediction being scored, and a prediction you can revise after watching the
// price is not a prediction. Typos are handled by UpdatePrediction, which the
// service only permits during the grace window right after stamping.
func (r *StampRepo) UpdateAnnotation(
	ctx context.Context, userID, id int64, conviction int, note string, tags []string,
) (model.Stamp, error) {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return model.Stamp{}, err
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE stamps
		   SET conviction = ?, note = ?, tags_json = ?
		 WHERE id = ? AND user_id = ?
	`, conviction, note, string(tagsJSON), id, userID)
	if err != nil {
		return model.Stamp{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Stamp{}, ErrStampNotFound
	}
	return r.Get(ctx, userID, id)
}

// UpdatePrediction rewrites the scored fields. Guarded by the service — see
// ErrStampPredictionLocked.
func (r *StampRepo) UpdatePrediction(
	ctx context.Context, userID, id int64,
	stance string, targetPrice float64, horizonKind string, dueAt time.Time,
) (model.Stamp, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE stamps
		   SET stance = ?, target_price = ?, horizon_kind = ?, due_at = ?
		 WHERE id = ? AND user_id = ?
	`, stance, targetPrice, horizonKind, dueAt.UTC(), id, userID)
	if err != nil {
		return model.Stamp{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Stamp{}, ErrStampNotFound
	}
	return r.Get(ctx, userID, id)
}

// MarkOpened records that the user has seen the result after the horizon
// elapsed, so it stops counting as "개봉 대기".
func (r *StampRepo) MarkOpened(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE stamps SET opened_at = CURRENT_TIMESTAMP(3) WHERE id = ? AND user_id = ? AND opened_at IS NULL`,
		id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Already opened, or not ours — Get resolves which.
		if _, err := r.Get(ctx, userID, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *StampRepo) Delete(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM stamps WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStampNotFound
	}
	return nil
}

// LastStampedAt powers both the 5-minute cooldown and the button state.
func (r *StampRepo) LastStampedAt(ctx context.Context, userID int64, symbol string) (time.Time, bool, error) {
	var at time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT stamped_at FROM stamps
		 WHERE user_id = ? AND symbol = ?
		 ORDER BY stamped_at DESC LIMIT 1`,
		userID, strings.ToUpper(strings.TrimSpace(symbol)),
	).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return at.UTC(), true, nil
}

func (r *StampRepo) CountForSymbol(ctx context.Context, userID int64, symbol string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM stamps WHERE user_id = ? AND symbol = ?`,
		userID, strings.ToUpper(strings.TrimSpace(symbol)),
	).Scan(&n)
	return n, err
}

func (r *StampRepo) CountForUser(ctx context.Context, userID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stamps WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}

// SymbolCounts returns stamp counts per symbol for the current user, newest
// first — the 종목별 묶음 view and the button badge both read it.
func (r *StampRepo) SymbolCounts(ctx context.Context, userID int64) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT symbol, COUNT(*) FROM stamps WHERE user_id = ? GROUP BY symbol`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var sym string
		var n int
		if err := rows.Scan(&sym, &n); err != nil {
			return nil, err
		}
		out[sym] = n
	}
	return out, rows.Err()
}

// MigrateStampsIfNeeded applies 035_stamps.sql.
func MigrateStampsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stamps")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "035_stamps.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 035_stamps.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("035_stamps.sql: %w", err)
		}
	}
	return nil
}

// schemaRaceErrors are the MySQL codes that mean "another process already did
// this": duplicate column, duplicate key name, and nothing to drop.
//
// The API and six workers all bootstrap the schema at once, so exactly one of
// them wins each DDL statement and the rest arrive to find the work done. That
// is a normal outcome of the deployment shape, not a failure — treating it as
// an error made six containers log a scary line on every restart.
var schemaRaceErrors = map[uint16]struct{}{
	1060: {}, // duplicate column name
	1061: {}, // duplicate key name
	1091: {}, // can't drop, does not exist
}

func isSchemaRaceError(err error) bool {
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}
	_, ok := schemaRaceErrors[mysqlErr.Number]
	return ok
}

// MigrateStampTargetPriceIfNeeded applies 046_stamps_target_price.sql.
//
// The file starts with TRUNCATE: pre-v2 rows carry no target price and there is
// no honest way to invent one, so they go. Presence of the target_price column
// is the marker, which also makes the migration a no-op on a table 035 just
// created empty.
func MigrateStampTargetPriceIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stamps")
	if err != nil || !exists {
		return err
	}
	has, err := columnExists(ctx, db, "stamps", "target_price")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	path := filepath.Join(migrationsDir, "046_stamps_target_price.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 046_stamps_target_price.sql: %w", err)
	}
	applied := false
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("046_stamps_target_price.sql: %w", err)
		}
		applied = true
	}
	if applied {
		log.Println("mysql: stamps migrated to v2 (target_price, horizon_kind, due_at) — pre-v2 stamps removed")
	}
	return nil
}
