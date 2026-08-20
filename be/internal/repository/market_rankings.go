package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type MarketRankingRow struct {
	MarketCountry string
	RankingType   string
	Duration      string
	RankedAt      *time.Time
	Payload       json.RawMessage
	UpdatedAt     time.Time
}

type MarketRankingsRepo struct {
	db *sql.DB
}

func NewMarketRankingsRepo(db *sql.DB) *MarketRankingsRepo {
	return &MarketRankingsRepo{db: db}
}

func (r *MarketRankingsRepo) Upsert(
	ctx context.Context,
	marketCountry, rankingType, duration string,
	rankedAt *time.Time,
	payload json.RawMessage,
) error {
	marketCountry = strings.ToUpper(strings.TrimSpace(marketCountry))
	rankingType = strings.TrimSpace(rankingType)
	duration = strings.TrimSpace(duration)
	if marketCountry == "" || rankingType == "" || duration == "" || len(payload) == 0 {
		return fmt.Errorf("invalid market rankings upsert args")
	}

	var rankedArg any
	if rankedAt != nil {
		rankedArg = rankedAt.UTC()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO market_rankings (market_country, ranking_type, duration, ranked_at, payload)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  ranked_at = VALUES(ranked_at),
		  payload = VALUES(payload),
		  updated_at = CURRENT_TIMESTAMP(3)
	`, marketCountry, rankingType, duration, rankedArg, payload)
	if err != nil {
		return fmt.Errorf("upsert market rankings: %w", err)
	}
	return nil
}

func (r *MarketRankingsRepo) Get(
	ctx context.Context,
	marketCountry, rankingType, duration string,
) (*MarketRankingRow, error) {
	marketCountry = strings.ToUpper(strings.TrimSpace(marketCountry))
	rankingType = strings.TrimSpace(rankingType)
	duration = strings.TrimSpace(duration)

	var row MarketRankingRow
	var rankedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT market_country, ranking_type, duration, ranked_at, payload, updated_at
		FROM market_rankings
		WHERE market_country = ? AND ranking_type = ? AND duration = ?
	`, marketCountry, rankingType, duration).Scan(
		&row.MarketCountry,
		&row.RankingType,
		&row.Duration,
		&rankedAt,
		&row.Payload,
		&row.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if rankedAt.Valid {
		t := rankedAt.Time
		row.RankedAt = &t
	}
	return &row, nil
}
