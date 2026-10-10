package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type RankingSnapshotEntry struct {
	MarketCountry string
	RankingType   string
	Duration      string
	RankedAt      time.Time
	Symbol        string
	Rank          int
	ChangeRate    *float64
}

type RankingHistoryPoint struct {
	RankedAt   time.Time
	Rank       int
	ChangeRate *float64
}

type RankingSnapshotRepo struct {
	db *sql.DB
}

func NewRankingSnapshotRepo(db *sql.DB) *RankingSnapshotRepo {
	return &RankingSnapshotRepo{db: db}
}

func (r *RankingSnapshotRepo) SymbolHistory(
	ctx context.Context,
	symbol, marketCountry, rankingType, duration string,
	since time.Time,
	limit int,
) ([]RankingHistoryPoint, error) {
	if limit <= 0 {
		limit = 200
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	rows, err := r.db.QueryContext(ctx, `
		SELECT ranked_at, rank_pos, change_rate
		FROM ranking_snapshot_entries
		WHERE symbol = ?
		  AND market_country = ?
		  AND ranking_type = ?
		  AND duration = ?
		  AND ranked_at >= ?
		ORDER BY ranked_at ASC
		LIMIT ?
	`, symbol, marketCountry, rankingType, duration, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RankingHistoryPoint
	for rows.Next() {
		var p RankingHistoryPoint
		var change sql.NullFloat64
		if err := rows.Scan(&p.RankedAt, &p.Rank, &change); err != nil {
			return nil, err
		}
		if change.Valid {
			v := change.Float64
			p.ChangeRate = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// rankingHistoryRetention bounds how far back ranking history is kept. The
// sparkline only reads a recent window, but nothing pruned this table, so it
// grew without limit — its indexes had already outgrown its data.
const rankingHistoryRetention = 90 * 24 * time.Hour

// rankingPruneBatch keeps each DELETE short so the poller never holds a long
// row-lock range on the table it is also inserting into.
const rankingPruneBatch = 5000

// PruneOlderThan deletes ranking history beyond the retention window and
// returns how many rows were removed.
func (r *RankingSnapshotRepo) PruneOlderThan(ctx context.Context, retention time.Duration) (int64, error) {
	if retention <= 0 {
		retention = rankingHistoryRetention
	}
	cutoff := time.Now().Add(-retention)

	var total int64
	for {
		res, err := r.db.ExecContext(ctx, `
			DELETE FROM ranking_snapshot_entries
			WHERE ranked_at < ?
			LIMIT ?
		`, cutoff, rankingPruneBatch)
		if err != nil {
			return total, fmt.Errorf("prune ranking snapshots: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += n
		if n < rankingPruneBatch {
			return total, nil
		}
		if ctx.Err() != nil {
			return total, ctx.Err()
		}
	}
}
