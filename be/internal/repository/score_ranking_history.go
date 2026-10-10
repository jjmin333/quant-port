package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

/*
AI 랭킹의 날짜별 구성 — 백테스트가 읽는 유일한 과거 기록.

표를 왜 따로 두는지는 `migrations/075_score_ranking_history.sql` 머리글에 있다.
요약하면: `stock_scores`는 30거래일만 남고(연 7.5GB), 백테스트가 필요로 하는 것은
그 표의 0.5%인 「그날 상위 N은 누구였나」뿐이다.
*/

// ScoreRankingHistoryRow is one (date, market, axis, rank) slot.
type ScoreRankingHistoryRow struct {
	TradeDate string
	Market    string
	Axis      string
	Rank      int
	Symbol    string
	Score     int
}

// ScoreRankingHistoryRepo owns score_ranking_history.
type ScoreRankingHistoryRepo struct {
	db *sql.DB
}

func NewScoreRankingHistoryRepo(db *sql.DB) *ScoreRankingHistoryRepo {
	return &ScoreRankingHistoryRepo{db: db}
}

// ScoreRankingHistoryDepth is how deep each daily snapshot goes.
//
// 200은 백테스트가 고를 수 있는 가장 큰 바구니다. 그보다 깊으면 「AI가 추천한 종목」이
// 아니라 그냥 시장이고, 행 수는 깊이에 정비례한다.
const ScoreRankingHistoryDepth = 200

// historyUpsertChunk keeps one statement well under MySQL's packet limit.
const historyUpsertChunk = 500

/*
ReplaceDate — 한 (날짜 · 시장 · 축)의 구성을 통째로 갈아 끼운다.

**덮어쓰기가 아니라 교체다.** 상위 200이 150으로 줄어드는 날(점수가 보류된 종목이
많은 날)에 upsert만 하면 151~200위 자리에 **어제 행이 그대로 남는다.** 그 행은 오늘
날짜를 달고 있으므로 다시는 틀렸다는 것을 알 수 없다.

같은 이유로 지우기와 넣기를 한 트랜잭션에 묶는다 — 중간에 끊기면 그 날짜의 랭킹이
반쪽만 남고, 백테스트는 그것을 「그날 상위가 80종목뿐이었다」로 읽는다.
*/
func (r *ScoreRankingHistoryRepo) ReplaceDate(
	ctx context.Context, tradeDate, market, axis string, rows []ScoreRankingHistoryRow,
) error {
	if r == nil || r.db == nil {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ranking history tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM score_ranking_history
		  WHERE trade_date = ? AND market = ? AND axis = ?`,
		tradeDate, market, axis); err != nil {
		return fmt.Errorf("ranking history clear: %w", err)
	}

	for start := 0; start < len(rows); start += historyUpsertChunk {
		end := min(start+historyUpsertChunk, len(rows))
		chunk := rows[start:end]

		var b strings.Builder
		b.WriteString(`INSERT INTO score_ranking_history
			(trade_date, market, axis, rank_no, symbol, score) VALUES `)
		args := make([]any, 0, len(chunk)*6)
		for i, row := range chunk {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?,?,?)")
			args = append(args, row.TradeDate, row.Market, row.Axis, row.Rank, row.Symbol, row.Score)
		}
		if _, err := tx.ExecContext(ctx, b.String(), args...); err != nil {
			return fmt.Errorf("ranking history insert: %w", err)
		}
	}
	return tx.Commit()
}

/*
Constituents — 한 (시장 · 축)의 구성을 날짜 범위로 읽는다.

순위 순으로 정렬해 돌려주므로 호출자는 앞에서 N개를 자르면 된다 —
**저장 깊이(200)보다 얕은 백테스트를 위해 쿼리를 다시 짜지 않는다.**
*/
func (r *ScoreRankingHistoryRepo) Constituents(
	ctx context.Context, market, axis, fromDate, toDate string, depth int,
) (map[string][]ScoreRankingHistoryRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if depth <= 0 || depth > ScoreRankingHistoryDepth {
		depth = ScoreRankingHistoryDepth
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'), rank_no, symbol, score
		  FROM score_ranking_history
		 WHERE market = ? AND axis = ?
		   AND trade_date BETWEEN ? AND ?
		   AND rank_no <= ?
		 ORDER BY trade_date, rank_no`,
		market, axis, fromDate, toDate, depth)
	if err != nil {
		return nil, fmt.Errorf("ranking history read: %w", err)
	}
	defer rows.Close()

	out := map[string][]ScoreRankingHistoryRow{}
	for rows.Next() {
		row := ScoreRankingHistoryRow{Market: market, Axis: axis}
		if err := rows.Scan(&row.TradeDate, &row.Rank, &row.Symbol, &row.Score); err != nil {
			return nil, fmt.Errorf("ranking history scan: %w", err)
		}
		out[row.TradeDate] = append(out[row.TradeDate], row)
	}
	return out, rows.Err()
}

// CoveredDates lists the dates that already have a snapshot for one
// (market, axis). The backfill uses it to skip work it has already done.
func (r *ScoreRankingHistoryRepo) CoveredDates(
	ctx context.Context, market, axis string,
) (map[string]int, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'), COUNT(*)
		  FROM score_ranking_history
		 WHERE market = ? AND axis = ?
		 GROUP BY trade_date`, market, axis)
	if err != nil {
		return nil, fmt.Errorf("ranking history dates: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var date string
		var n int
		if err := rows.Scan(&date, &n); err != nil {
			return nil, err
		}
		out[date] = n
	}
	return out, rows.Err()
}

// Span reports what the table actually holds, so a screen can say「표본 N거래일」
// instead of pretending the window it was asked for is the window it has.
func (r *ScoreRankingHistoryRepo) Span(
	ctx context.Context, market, axis string,
) (minDate, maxDate string, dates int, err error) {
	if r == nil || r.db == nil {
		return "", "", 0, nil
	}
	var lo, hi sql.NullString
	err = r.db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(MIN(trade_date), '%Y-%m-%d'),
		       DATE_FORMAT(MAX(trade_date), '%Y-%m-%d'),
		       COUNT(DISTINCT trade_date)
		  FROM score_ranking_history
		 WHERE market = ? AND axis = ?`, market, axis).Scan(&lo, &hi, &dates)
	if err != nil {
		return "", "", 0, fmt.Errorf("ranking history span: %w", err)
	}
	return lo.String, hi.String, dates, nil
}

// MigrateScoreRankingHistoryIfNeeded creates the table on an existing database
// (a fresh one gets it from the full migration run).
func MigrateScoreRankingHistoryIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "score_ranking_history")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir,
		"075_score_ranking_history.sql", "AI 랭킹 날짜별 구성 (백테스트용)")
}
