package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// InsiderNetShares is one symbol's reported buy/sell volume over a window.
type InsiderNetShares struct {
	Buy     float64
	Sell    float64
	Counted int
}

/*
InsiderNetSharesSince — 종목별 내부자 매수·매도 주식 수를 한 번에.

점수 배치가 전 종목 심리 축을 매길 때 쓴다(sentiment_axis.go의
`insiderComponentFromTotals`). 종목마다 행을 끌어오지 않고 DB가 합친다.
`shares_change`가 없거나 0인 행은 방향을 말하지 않으므로 세지 않는다 — 요약
파이프라인이 쓰던 `insiderSentimentComponent`와 같은 규칙이다.
*/
func (r *InsiderRepo) InsiderNetSharesSince(ctx context.Context, since time.Time) (map[string]InsiderNetShares, error) {
	out := map[string]InsiderNetShares{}
	if r == nil || r.db == nil {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol,
		       COALESCE(SUM(CASE WHEN shares_change > 0 THEN shares_change END), 0),
		       COALESCE(SUM(CASE WHEN shares_change < 0 THEN -shares_change END), 0),
		       COUNT(*)
		  FROM insider_transactions
		 WHERE txn_date >= ? AND shares_change IS NOT NULL AND shares_change <> 0
		 GROUP BY symbol`, since.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("insider net shares: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			sym string
			v   InsiderNetShares
		)
		if err := rows.Scan(&sym, &v.Buy, &v.Sell, &v.Counted); err != nil {
			return nil, err
		}
		out[strings.ToUpper(sym)] = v
	}
	return out, rows.Err()
}
