package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// StockScoreRow is one stored score. Breakdown/Inputs are pre-marshalled JSON
// so the batch can build thousands of rows without re-encoding per write.
type StockScoreRow struct {
	Symbol    string
	TradeDate string
	Score     sql.NullInt64
	ScoreKind string
	Grade     string
	Coverage  float64
	PeerGroup string
	PeerPct   sql.NullInt64
	Breakdown string
	Inputs    string

	// WeightSource is transient — which weight set produced this row. It is not
	// persisted because weights can be edited after a batch, and the API
	// reports the weights in force at read time.
	WeightSource string
}

type StockScoreRepo struct {
	db *sql.DB
}

func NewStockScoreRepo(db *sql.DB) *StockScoreRepo {
	return &StockScoreRepo{db: db}
}

// scoreUpsertChunk keeps each multi-row statement well under MySQL's packet
// limit while still cutting round trips by ~500x versus row-at-a-time.
const scoreUpsertChunk = 500

func (r *StockScoreRepo) UpsertBatch(ctx context.Context, rows []StockScoreRow) error {
	if r == nil || r.db == nil || len(rows) == 0 {
		return nil
	}
	for start := 0; start < len(rows); start += scoreUpsertChunk {
		end := start + scoreUpsertChunk
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]

		var b strings.Builder
		b.WriteString(`INSERT INTO stock_scores
			(symbol, trade_date, score, score_kind, grade, coverage, peer_group, peer_pct, breakdown, inputs)
			VALUES `)
		args := make([]any, 0, len(chunk)*10)
		for i, row := range chunk {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?,?,?,?,?,?,?)")
			var grade any
			if row.Grade != "" {
				grade = row.Grade
			}
			args = append(args,
				row.Symbol, row.TradeDate, row.Score, row.ScoreKind, grade,
				row.Coverage, row.PeerGroup, row.PeerPct, row.Breakdown, row.Inputs,
			)
		}
		b.WriteString(` ON DUPLICATE KEY UPDATE
			score = VALUES(score),
			score_kind = VALUES(score_kind),
			grade = VALUES(grade),
			coverage = VALUES(coverage),
			peer_group = VALUES(peer_group),
			peer_pct = VALUES(peer_pct),
			breakdown = VALUES(breakdown),
			inputs = VALUES(inputs),
			computed_at = CURRENT_TIMESTAMP(3)`)

		if _, err := r.db.ExecContext(ctx, b.String(), args...); err != nil {
			return fmt.Errorf("upsert stock_scores: %w", err)
		}
	}
	return nil
}

const scoreSelectCols = `sc.symbol, s.name, DATE_FORMAT(sc.trade_date, '%Y-%m-%d'), sc.score,
	sc.score_kind, sc.grade, sc.coverage, sc.peer_group, sc.peer_pct,
	sc.breakdown, sc.inputs, sc.computed_at`

func (r *StockScoreRepo) scan(rows *sql.Rows) (*model.StockScoreView, error) {
	var (
		v                  model.StockScoreView
		name, grade, peer  sql.NullString
		score, peerPct     sql.NullInt64
		breakdown, inputs  sql.NullString
	)
	if err := rows.Scan(
		&v.Symbol, &name, &v.TradeDate, &score, &v.ScoreKind, &grade,
		&v.Coverage, &peer, &peerPct, &breakdown, &inputs, &v.ComputedAt,
	); err != nil {
		return nil, err
	}
	v.Name = name.String
	v.Grade = grade.String
	v.PeerGroup = peer.String
	if score.Valid {
		s := int(score.Int64)
		v.Score = &s
	}
	if peerPct.Valid {
		p := int(peerPct.Int64)
		v.PeerPct = &p
	}
	if breakdown.Valid && breakdown.String != "" {
		_ = json.Unmarshal([]byte(breakdown.String), &v.Breakdown)
	}
	if inputs.Valid && inputs.String != "" {
		_ = json.Unmarshal([]byte(inputs.String), &v.Inputs)
	}
	return &v, nil
}

// Latest returns the newest stored score for one symbol.
func (r *StockScoreRepo) Latest(ctx context.Context, symbol string) (*model.StockScoreView, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+scoreSelectCols+`
		  FROM stock_scores sc
		  LEFT JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.symbol = ?
		 ORDER BY sc.trade_date DESC
		 LIMIT 1`, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	return r.scan(rows)
}

// History returns a symbol's recent scores, newest first.
func (r *StockScoreRepo) History(ctx context.Context, symbol string, limit int) ([]model.StockScoreView, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 120 {
		limit = 60
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+scoreSelectCols+`
		  FROM stock_scores sc
		  LEFT JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.symbol = ?
		 ORDER BY sc.trade_date DESC
		 LIMIT ?`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.StockScoreView
	for rows.Next() {
		v, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// LatestBatch returns the newest score for each of several symbols in one
// query. The holdings screen needs a score per held symbol, and looping Latest()
// there would mean one round trip per row — twenty holdings, twenty queries.
//
// The join picks each symbol's max trade_date rather than a single global date:
// a symbol that stopped being scored (delisted, missing fundamentals) still
// shows its last known score instead of vanishing from the list.
func (r *StockScoreRepo) LatestBatch(ctx context.Context, symbols []string) ([]model.StockScoreView, error) {
	if r == nil || r.db == nil || len(symbols) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(symbols))
	placeholders := make([]string, 0, len(symbols))
	seen := map[string]struct{}{}
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, sym)
	}
	if len(args) == 0 {
		return nil, nil
	}
	in := strings.Join(placeholders, ",")
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+scoreSelectCols+`
		  FROM stock_scores sc
		  JOIN (
		        SELECT symbol, MAX(trade_date) AS trade_date
		          FROM stock_scores
		         WHERE symbol IN (`+in+`)
		         GROUP BY symbol
		       ) latest
		    ON latest.symbol = sc.symbol AND latest.trade_date = sc.trade_date
		  LEFT JOIN stocks s ON s.symbol = sc.symbol`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.StockScoreView, 0, len(args))
	for rows.Next() {
		v, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// SyncDailyCloseScores copies the batch's scores onto that date's 일별 종가
// rows, so the existing 일별 시세 list shows a score without a second lookup.
//
// One-way and idempotent, matching the projection rule the close pipeline
// already follows: stock_scores is the source, stock_daily_closes only mirrors.
func (r *StockScoreRepo) SyncDailyCloseScores(ctx context.Context, tradeDate string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE stock_daily_closes d
		  JOIN stock_scores sc
		    ON sc.symbol = d.symbol AND sc.trade_date = d.trade_date
		   SET d.score = sc.score
		 WHERE d.trade_date = ?`, tradeDate)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// TopScores returns the highest-scored symbols for one trade date, restricted
// to the given asset types (domestic vs overseas), for the 종목 랭킹 list.
func (r *StockScoreRepo) TopScores(ctx context.Context, tradeDate string, assetTypes []string, limit int) ([]model.StockScoreListItem, error) {
	if r == nil || r.db == nil || tradeDate == "" || len(assetTypes) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	placeholders := strings.Repeat("?,", len(assetTypes))
	placeholders = placeholders[:len(placeholders)-1]

	// STRAIGHT_JOIN으로 stock_scores를 구동 테이블로 못박는다.
	//
	// 옵티마이저는 자유롭게 두면 stocks를 먼저 잡는다 — asset_type 조건이 거기 있고
	// 카디널리티가 낮아 보이기 때문이다. 그러면 17,070행을 전부 읽어 매 행마다
	// stock_scores를 찔러보고, 모은 결과를 filesort 한 뒤에야 스무 줄을 자른다
	// (실측: 해외 탭 2.10초).
	//
	// 반대로 sc부터 읽으면 idx_stock_scores_rank(trade_date, score DESC, peer_pct DESC)가
	// ORDER BY와 순서가 같아 정렬이 공짜가 되고, LIMIT을 채우는 순간 스캔이 끝난다.
	// 해외는 전체의 75%라 스무 줄을 채우는 데 서른 행 남짓만 읽는다 (실측 0.036초).
	query := `
		SELECT sc.symbol, s.name, s.asset_type, sc.score, sc.grade, sc.peer_pct,
		       d.change_rate, d.close_price, d.currency
		  FROM stock_scores sc
		  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		  LEFT JOIN stock_daily_closes d ON d.symbol = sc.symbol AND d.trade_date = sc.trade_date
		 WHERE sc.trade_date = ? AND sc.score IS NOT NULL AND s.asset_type IN (` + placeholders + `)
		 ORDER BY sc.score DESC, sc.peer_pct DESC
		 LIMIT ?`

	args := make([]any, 0, len(assetTypes)+2)
	args = append(args, tradeDate)
	for _, a := range assetTypes {
		args = append(args, a)
	}
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.StockScoreListItem, 0, limit)
	for rows.Next() {
		var (
			it                        model.StockScoreListItem
			name, grade, currency     sql.NullString
			score                     sql.NullInt64
			peerPct                   sql.NullInt64
			changeRate, closePrice    sql.NullFloat64
		)
		if err := rows.Scan(&it.Symbol, &name, &it.AssetType, &score, &grade, &peerPct,
			&changeRate, &closePrice, &currency); err != nil {
			return nil, err
		}
		it.Name = name.String
		it.Grade = grade.String
		it.Currency = currency.String
		it.TradeDate = tradeDate
		if score.Valid {
			it.Score = int(score.Int64)
		}
		if peerPct.Valid {
			p := int(peerPct.Int64)
			it.PeerPct = &p
		}
		if changeRate.Valid {
			it.ChangeRate = &changeRate.Float64
		}
		if closePrice.Valid {
			it.ClosePrice = &closePrice.Float64
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Rank = i + 1
	}
	return out, nil
}

// Stats reports what a given date's batch produced, for the admin page.
func (r *StockScoreRepo) Stats(ctx context.Context, tradeDate string) (total, scored int, err error) {
	if r == nil || r.db == nil {
		return 0, 0, nil
	}
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(score IS NOT NULL), 0)
		  FROM stock_scores WHERE trade_date = ?`, tradeDate).Scan(&total, &scored)
	return total, scored, err
}

// ---- settings -------------------------------------------------------------

type ScoreSettingsRepo struct {
	db *sql.DB
}

func NewScoreSettingsRepo(db *sql.DB) *ScoreSettingsRepo {
	return &ScoreSettingsRepo{db: db}
}

func (r *ScoreSettingsRepo) Get(ctx context.Context) (model.ScoreSettings, error) {
	if r == nil || r.db == nil {
		return model.DefaultScoreSettings(), nil
	}
	var s model.ScoreSettings
	err := r.db.QueryRowContext(ctx, `
		SELECT w_technical, w_quality, w_growth, w_valuation, w_sentiment,
		       coverage_floor, shrink_full, updated_at
		  FROM score_settings WHERE id = 1`).Scan(
		&s.Weights.Technical, &s.Weights.Quality, &s.Weights.Growth,
		&s.Weights.Valuation, &s.Weights.Sentiment,
		&s.CoverageFloor, &s.ShrinkFull, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.DefaultScoreSettings(), nil
		}
		return model.ScoreSettings{}, err
	}
	s.Weights = s.Weights.Clamped()
	return s, nil
}

func (r *ScoreSettingsRepo) Update(ctx context.Context, s model.ScoreSettings) (model.ScoreSettings, error) {
	if r == nil || r.db == nil {
		return model.DefaultScoreSettings(), nil
	}
	w := s.Weights.Clamped()
	floor := clampRatio(s.CoverageFloor, 0, 0.9, 0.20)
	full := clampRatio(s.ShrinkFull, 0.1, 1.0, 0.60)
	_, err := r.db.ExecContext(ctx, `
		UPDATE score_settings
		   SET w_technical = ?, w_quality = ?, w_growth = ?, w_valuation = ?, w_sentiment = ?,
		       coverage_floor = ?, shrink_full = ?
		 WHERE id = 1`,
		w.Technical, w.Quality, w.Growth, w.Valuation, w.Sentiment, floor, full)
	if err != nil {
		return model.ScoreSettings{}, err
	}
	return r.Get(ctx)
}

func clampRatio(v, lo, hi, fallback float64) float64 {
	if v <= 0 {
		return fallback
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ListSymbolWeights returns every per-symbol override, joined to the stock name
// so the admin table can show 종목명 rather than bare codes.
func (r *ScoreSettingsRepo) ListSymbolWeights(ctx context.Context) ([]model.SymbolScoreWeights, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT w.symbol, COALESCE(s.name, ''), w.w_technical, w.w_quality, w.w_growth,
		       w.w_valuation, w.w_sentiment, w.note, w.updated_at
		  FROM score_symbol_weights w
		  LEFT JOIN stocks s ON s.symbol = w.symbol
		 ORDER BY w.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.SymbolScoreWeights, 0)
	for rows.Next() {
		var it model.SymbolScoreWeights
		if err := rows.Scan(&it.Symbol, &it.Name,
			&it.Weights.Technical, &it.Weights.Quality, &it.Weights.Growth,
			&it.Weights.Valuation, &it.Weights.Sentiment, &it.Note, &it.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *ScoreSettingsRepo) UpsertSymbolWeights(ctx context.Context, it model.SymbolScoreWeights) error {
	if r == nil || r.db == nil {
		return nil
	}
	symbol := strings.ToUpper(strings.TrimSpace(it.Symbol))
	if symbol == "" {
		return fmt.Errorf("symbol required")
	}
	w := it.Weights.Clamped()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO score_symbol_weights
			(symbol, w_technical, w_quality, w_growth, w_valuation, w_sentiment, note)
		VALUES (?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE
			w_technical = VALUES(w_technical),
			w_quality   = VALUES(w_quality),
			w_growth    = VALUES(w_growth),
			w_valuation = VALUES(w_valuation),
			w_sentiment = VALUES(w_sentiment),
			note        = VALUES(note)`,
		symbol, w.Technical, w.Quality, w.Growth, w.Valuation, w.Sentiment, it.Note)
	return err
}

func (r *ScoreSettingsRepo) DeleteSymbolWeights(ctx context.Context, symbol string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM score_symbol_weights WHERE symbol = ?`,
		strings.ToUpper(strings.TrimSpace(symbol)))
	return err
}

// MigrateStockScoresIfNeeded applies 030_stock_scores.sql.
func MigrateStockScoresIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_scores")
	if err != nil {
		return err
	}
	settingsExists, err := tableExists(ctx, db, "score_settings")
	if err != nil {
		return err
	}
	if exists && settingsExists {
		return nil
	}
	path := filepath.Join(migrationsDir, "030_stock_scores.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 030_stock_scores.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("030_stock_scores.sql: %w", err)
		}
	}
	return nil
}

// MigrateIndexScoresIfNeeded applies 031_index_scores.sql.
func MigrateIndexScoresIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'index_daily_closes'
		   AND COLUMN_NAME = 'score'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		path := filepath.Join(migrationsDir, "031_index_scores.sql")
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read 031_index_scores.sql: %w", err)
		}
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				return fmt.Errorf("031_index_scores.sql: %w", err)
			}
		}
		return nil
	}

	// Columns already exist — but an earlier revision of this migration created
	// ai_summary without a default and left source at VARCHAR(16), which made
	// every index close write fail. MODIFY is idempotent, so repair in place
	// rather than requiring a manual fix.
	repairs := []string{
		`ALTER TABLE index_daily_closes MODIFY COLUMN ai_summary TEXT NOT NULL DEFAULT ('')`,
		`ALTER TABLE index_daily_closes MODIFY COLUMN source VARCHAR(32) NOT NULL DEFAULT ''`,
	}
	for _, stmt := range repairs {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("031 repair: %w", err)
		}
	}
	return nil
}

