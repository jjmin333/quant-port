package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
사용자별 축 가중치와 「내 기준」 랭킹의 재료. 표는 `migrations/082_user_score_weights.sql`.
*/

// UserScoreWeightsRepo owns user_score_weights and the custom-ranking source query.
type UserScoreWeightsRepo struct {
	db *sql.DB
}

func NewUserScoreWeightsRepo(db *sql.DB) *UserScoreWeightsRepo {
	return &UserScoreWeightsRepo{db: db}
}

// Get returns the user's saved weights (ok=false when none).
func (r *UserScoreWeightsRepo) Get(ctx context.Context, userID int64) (model.ScoreWeights, bool, error) {
	var w model.ScoreWeights
	err := r.db.QueryRowContext(ctx, `
		SELECT w_technical, w_quality, w_growth, w_valuation, w_sentiment
		  FROM user_score_weights WHERE user_id = ?`, userID).
		Scan(&w.Technical, &w.Quality, &w.Growth, &w.Valuation, &w.Sentiment)
	if errors.Is(err, sql.ErrNoRows) {
		return w, false, nil
	}
	if err != nil {
		return w, false, fmt.Errorf("user score weights: %w", err)
	}
	return w, true, nil
}

// Put saves the user's weights.
func (r *UserScoreWeightsRepo) Put(ctx context.Context, userID int64, w model.ScoreWeights) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_score_weights (user_id, w_technical, w_quality, w_growth, w_valuation, w_sentiment)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE w_technical = VALUES(w_technical), w_quality = VALUES(w_quality),
		  w_growth = VALUES(w_growth), w_valuation = VALUES(w_valuation), w_sentiment = VALUES(w_sentiment)`,
		userID, w.Technical, w.Quality, w.Growth, w.Valuation, w.Sentiment)
	if err != nil {
		return fmt.Errorf("user score weights put: %w", err)
	}
	return nil
}

// CustomRankingRow is one rankable symbol's stored composite material.
type CustomRankingRow struct {
	Symbol    string
	Name      string
	Score     sql.NullInt64
	Coverage  float64
	Breakdown model.ScoreBreakdown
}

/*
CustomRankingRows — 최신 점수일에 랭킹 자격을 갖춘 종목들의 (점수 · 커버리지 · 축 점수).

AI 랭킹과 **같은 우주**다(축 3개 이상 · 지수 추종 상품 제외). `inputs` 블롭은 읽지 않는다 —
다시 합산에 필요한 것은 축 점수와 커버리지뿐이고 그 블롭이 행당 825B다.
*/
func (r *UserScoreWeightsRepo) CustomRankingRows(ctx context.Context, assetTypes []string) (string, []CustomRankingRow, error) {
	var tradeDate sql.NullString
	if err := r.db.QueryRowContext(ctx,
		`SELECT DATE_FORMAT(MAX(trade_date), '%Y-%m-%d') FROM stock_scores`).Scan(&tradeDate); err != nil {
		return "", nil, fmt.Errorf("custom ranking date: %w", err)
	}
	if !tradeDate.Valid {
		return "", nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(assetTypes)), ",")
	productClause, productArgs := scoreRankableProductClause("s")
	sizeClause, sizeArgs := scoreRecommendableClause("sc", "s")
	args := []any{tradeDate.String}
	for _, a := range assetTypes {
		args = append(args, a)
	}
	args = append(args, productArgs...)
	args = append(args, sizeArgs...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT sc.symbol, COALESCE(s.name, ''), sc.score, sc.coverage, sc.breakdown
		  FROM stock_scores sc
		  JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.trade_date = ? AND s.asset_type IN (`+placeholders+`)`+productClause+
		scoreMinAxesClause("sc")+sizeClause, args...)
	if err != nil {
		return "", nil, fmt.Errorf("custom ranking rows: %w", err)
	}
	defer rows.Close()
	var out []CustomRankingRow
	for rows.Next() {
		var (
			c  CustomRankingRow
			bd sql.NullString
		)
		if err := rows.Scan(&c.Symbol, &c.Name, &c.Score, &c.Coverage, &bd); err != nil {
			return "", nil, err
		}
		if bd.Valid {
			_ = json.Unmarshal([]byte(bd.String), &c.Breakdown)
		}
		out = append(out, c)
	}
	return tradeDate.String, out, rows.Err()
}

// MigrateUserScoreWeightsIfNeeded creates the table on an existing database.
func MigrateUserScoreWeightsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "user_score_weights")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "082_user_score_weights.sql", "사용자 축 가중치")
}
