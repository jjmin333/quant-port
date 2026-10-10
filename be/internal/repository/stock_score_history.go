package repository

import (
	"context"
	"database/sql"
	"fmt"
)

/*
종목별 축 점수 이력 — `stock_scores`가 30거래일에서 잘린 뒤에도 남는 얇은 사본.

표를 왜 따로 두는지는 `migrations/077_stock_score_history.sql` 머리글에 있다.
랭킹 이력(`score_ranking_history`)과 같은 저장소 타입에 두는 이유: 둘 다 점수 배치의
꼬리에서 **정리(prune)보다 먼저** 같은 날짜로 쓰이고, 하나만 채워진 날이 생기면
백테스트와 보유 점검이 같은 날에 다른 과거를 본다.
*/

/*
ArchiveScoreAxes — 한 날짜의 `stock_scores` 행을 이력 표로 복사한다.

**DB 안에서 끝낸다**(`INSERT … SELECT`). 17,906행을 Go로 끌어왔다 다시 보내면 그
왕복이 이 작업의 대부분이 된다. 이미 있는 날짜는 덮어쓴다 — 관리자 재계산이 그날
점수를 바꿨는데 이력만 옛 값으로 남으면, 30거래일 안에서는 화면과 이력이 갈린다.

없는 축은 `breakdown`에 키가 없으므로(`omitempty`) `->>`가 NULL을 준다 — 0이 아니다.
*/
func (r *ScoreRankingHistoryRepo) ArchiveScoreAxes(ctx context.Context, tradeDate string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx, scoreHistoryCopySQL("s.trade_date = ?"), tradeDate)
	if err != nil {
		return 0, fmt.Errorf("score history archive %s: %w", tradeDate, err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

/*
scoreHistoryCopySQL — `stock_scores` 행을 이력 표로 옮기는 **유일한** 문장.

두 곳이 쓴다: 날짜 단위 복사(점수 배치 꼬리 · 백필)와 종목 단위 복사(`UpdateAxes`).
심리 축은 00:25 점수 배치가 아니라 **08:00 AI 요약 배치가 나중에 얹는다**
(`stock_score_sentiment.go`). 날짜 단위 복사만 있으면 이력의 심리 축은 거의 언제나 비고,
그 날짜는 다시 복사되지 않는다(백필은 이미 채운 날짜를 건너뛴다). 문장이 둘로 갈리면
두 경로가 서로 다른 열을 채우게 되므로 하나로 둔다.
*/
func scoreHistoryCopySQL(where string) string {
	return `
		INSERT INTO stock_score_history
		  (symbol, trade_date, score, technical, quality, growth, valuation, sentiment, coverage)
		SELECT s.symbol, s.trade_date, s.score,
		       CAST(s.breakdown->>'$.technical' AS DECIMAL(5,2)),
		       CAST(s.breakdown->>'$.quality'   AS DECIMAL(5,2)),
		       CAST(s.breakdown->>'$.growth'    AS DECIMAL(5,2)),
		       CAST(s.breakdown->>'$.valuation' AS DECIMAL(5,2)),
		       CAST(s.breakdown->>'$.sentiment' AS DECIMAL(5,2)),
		       s.coverage
		  FROM stock_scores s
		 WHERE ` + where + `
		ON DUPLICATE KEY UPDATE
		  score     = VALUES(score),
		  technical = VALUES(technical),
		  quality   = VALUES(quality),
		  growth    = VALUES(growth),
		  valuation = VALUES(valuation),
		  sentiment = VALUES(sentiment),
		  coverage  = VALUES(coverage)`
}

/*
ArchivedScoreDates — `since` 이후 이력 표에 들어 있는 날짜.

백필이 「이미 복사한 날짜」를 건너뛰는 데 쓴다. `idx_stock_score_history_date`만
훑으므로 표가 몇 년치로 자라도 창(30거래일) 크기에 비례한다.
*/
func (r *ScoreRankingHistoryRepo) ArchivedScoreDates(ctx context.Context, since string) (map[string]bool, error) {
	out := map[string]bool{}
	if r == nil || r.db == nil {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT DATE_FORMAT(trade_date, '%Y-%m-%d')
		  FROM stock_score_history
		 WHERE trade_date >= ?`, since)
	if err != nil {
		return nil, fmt.Errorf("score history dates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d] = true
	}
	return out, rows.Err()
}

// MigrateStockScoreHistoryIfNeeded creates the table on an existing database
// (a fresh one gets it from the full migration run).
func MigrateStockScoreHistoryIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_score_history")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir,
		"077_stock_score_history.sql", "종목별 축 점수 이력 (30거래일 이후 보관)")
}
