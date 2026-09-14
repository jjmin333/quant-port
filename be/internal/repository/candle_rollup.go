package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// RollupStep — 캐스케이드 한 칸. `AgeDays`보다 오래된 `From` 봉을 `To`로 접는다.
type RollupStep struct {
	From    model.CandleInterval
	To      model.CandleInterval
	Minutes int // To 버킷 크기(분)
	AgeDays int
}

// RollupResult reports what one chunk did.
type RollupResult struct {
	Symbols  int
	Inserted int64
	Deleted  int64
	Cutoff   string
}

/*
버킷 산술은 **벽시계만** 쓴다.

	DATE(bar_time) + INTERVAL (FLOOR((HOUR*60 + MINUTE)/N)*N) MINUTE

`UNIX_TIMESTAMP()`를 쓰면 세션 타임존에 따라 버킷 시작 시각이 달라진다. bar_time은
KST 벽시계로 저장돼 있고 Go 쪽 truncateCandleTime도 같은 산술을 쓰므로, 이렇게
하면 서버 time_zone 설정과 무관하게 양쪽이 정확히 일치한다.
(candles_tiering_test.go의 TestTruncateCandleTime_MatchesSQLWallClockArithmetic이
이 등가를 고정한다.)
*/
func bucketExpr(minutes int) string {
	return fmt.Sprintf(
		"DATE(bar_time) + INTERVAL (FLOOR((HOUR(bar_time)*60 + MINUTE(bar_time)) / %d) * %d) MINUTE",
		minutes, minutes)
}

/*
RollupCutoff — 컷오프를 **버킷 경계로 스냅**해서 돌려준다.

스냅하지 않으면 컷오프가 버킷 한가운데를 갈라, 앞부분만으로 만든 봉을 남기고
뒷부분 1분봉은 그대로 둔다. 다음 실행에서 컷오프가 지나가면 **남은 조각만으로
재집계해 그 봉을 덮어쓴다** — 거래량이 조용히 깎여나가고, 되돌릴 원본은 이미
삭제된 뒤다.

2026-08-22 샌드박스 실측: NVDA 22:20 5분봉이 33,105 → 22,069주로 줄었다.
*/
func (r *CandleBarRepo) RollupCutoff(ctx context.Context, step RollupStep) (string, error) {
	var cutoff string
	err := r.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT DATE(c) + INTERVAL (FLOOR((HOUR(c)*60 + MINUTE(c)) / %d) * %d) MINUTE
		FROM (SELECT DATE_SUB(NOW(), INTERVAL ? DAY) AS c) t
	`, step.Minutes, step.Minutes), step.AgeDays).Scan(&cutoff)
	if err != nil {
		return "", fmt.Errorf("rollup cutoff %s->%s: %w", step.From, step.To, err)
	}
	return cutoff, nil
}

// RollupSymbols lists symbols holding `interval` bars older than cutoff.
// 파티션 프루닝 덕에 해당 계층만 훑는다.
func (r *CandleBarRepo) RollupSymbols(ctx context.Context, interval model.CandleInterval, cutoff string, limit int) ([]string, error) {
	q := `SELECT DISTINCT symbol FROM candle_bars WHERE bar_interval = ? AND bar_time < ? ORDER BY symbol`
	args := []any{string(interval), cutoff}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("rollup symbols %s: %w", interval, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

/*
RollupChunk — 심볼 묶음 하나를 한 트랜잭션으로 접는다.

INSERT와 DELETE가 **같은 트랜잭션**이어야 한다. 나뉘면 그 사이에 조회가 두
해상도를 동시에 보고 거래량을 두 번 더한다.

심볼로 쪼개는 이유는 두 가지다. PK가 (symbol, bar_interval, bar_time)이라
`symbol IN (...)` + 파티션 프루닝이 정확한 범위 스캔이 되고, 트랜잭션 하나가
잡는 행 수에 상한이 생긴다 — 최초 백로그는 1,390만 행이라 통째로 돌리면 redo가
터진다.
*/
func (r *CandleBarRepo) RollupChunk(
	ctx context.Context,
	step RollupStep,
	cutoff string,
	symbols []string,
	dryRun bool,
) (RollupResult, error) {
	res := RollupResult{Symbols: len(symbols), Cutoff: cutoff}
	if len(symbols) == 0 {
		return res, nil
	}

	placeholders := "?" + strings.Repeat(",?", len(symbols)-1)
	bucket := bucketExpr(step.Minutes)

	selectSQL := fmt.Sprintf(`
		SELECT symbol, '%s', bucket,
			MAX(CASE WHEN rn_a = 1 THEN open_price  END),
			MAX(high_price), MIN(low_price),
			MAX(CASE WHEN rn_d = 1 THEN close_price END),
			SUM(volume), LEAST(SUM(src_bars), 255)
		FROM (
			SELECT symbol, bar_time, open_price, high_price, low_price, close_price, volume, src_bars,
				%s AS bucket,
				ROW_NUMBER() OVER (PARTITION BY symbol, %s ORDER BY bar_time ASC)  AS rn_a,
				ROW_NUMBER() OVER (PARTITION BY symbol, %s ORDER BY bar_time DESC) AS rn_d
			FROM candle_bars
			WHERE bar_interval = ? AND bar_time < ? AND symbol IN (%s)
		) t
		GROUP BY symbol, bucket`,
		step.To, bucket, bucket, bucket, placeholders)

	args := make([]any, 0, len(symbols)+2)
	args = append(args, string(step.From), cutoff)
	for _, s := range symbols {
		args = append(args, s)
	}

	if dryRun {
		// 모의 실행은 별도 쿼리다. selectSQL의 컬럼에 이름이 없어 그대로 감싸면
		// 별칭을 붙일 수 없다 — MySQL에는 `FROM (...) t(a, b, c)` 문법이 없다.
		var buckets, srcRows sql.NullInt64
		countSQL := fmt.Sprintf(`
			SELECT COUNT(*), COALESCE(SUM(src_rows), 0) FROM (
				SELECT symbol, %s AS bucket, COUNT(*) AS src_rows
				FROM candle_bars
				WHERE bar_interval = ? AND bar_time < ? AND symbol IN (%s)
				GROUP BY symbol, bucket
			) x`, bucket, placeholders)
		if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&buckets, &srcRows); err != nil {
			return res, fmt.Errorf("rollup dry-run %s->%s: %w", step.From, step.To, err)
		}
		res.Inserted = buckets.Int64
		res.Deleted = srcRows.Int64
		return res, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()

	// src_bars가 줄어드는 덮어쓰기는 거부한다. Toss 백필(getCandlesPageUncached)이
	// 이미 접힌 구간에 1분봉을 되넣으면 다음 실행이 그 조각만으로 재집계하는데,
	// 기존 봉이 더 완전하면 그대로 둔다.
	insertSQL := fmt.Sprintf(`
		INSERT INTO candle_bars
			(symbol, bar_interval, bar_time, open_price, high_price, low_price, close_price, volume, src_bars)
		%s
		ON DUPLICATE KEY UPDATE
			open_price  = IF(VALUES(src_bars) >= src_bars, VALUES(open_price),  open_price),
			high_price  = IF(VALUES(src_bars) >= src_bars, VALUES(high_price),  high_price),
			low_price   = IF(VALUES(src_bars) >= src_bars, VALUES(low_price),   low_price),
			close_price = IF(VALUES(src_bars) >= src_bars, VALUES(close_price), close_price),
			volume      = IF(VALUES(src_bars) >= src_bars, VALUES(volume),      volume),
			src_bars    = GREATEST(VALUES(src_bars), src_bars)`, selectSQL)

	ins, err := tx.ExecContext(ctx, insertSQL, args...)
	if err != nil {
		return res, fmt.Errorf("rollup insert %s->%s: %w", step.From, step.To, err)
	}
	if n, err := ins.RowsAffected(); err == nil {
		res.Inserted = n
	}

	delArgs := make([]any, 0, len(symbols)+2)
	delArgs = append(delArgs, string(step.From), cutoff)
	for _, s := range symbols {
		delArgs = append(delArgs, s)
	}
	del, err := tx.ExecContext(ctx, fmt.Sprintf(
		`DELETE FROM candle_bars WHERE bar_interval = ? AND bar_time < ? AND symbol IN (%s)`,
		placeholders), delArgs...)
	if err != nil {
		return res, fmt.Errorf("rollup delete %s: %w", step.From, err)
	}
	if n, err := del.RowsAffected(); err == nil {
		res.Deleted = n
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("rollup commit %s->%s: %w", step.From, step.To, err)
	}
	return res, nil
}

/*
PruneIntraday — 마지막 계층(60분봉)에서 만료된 봉을 지운다.

**1일봉으로 접지 않는다.** 1d는 이미 독립적으로, 더 정확하게(원본 세션 거래량),
더 길게(2009년~) 존재한다. 인트라데이 커버리지는 부분이라 접은 1일봉은 거래량이
모자라고, 같은 PK를 덮어써 stock_daily_closes 투영과 stock_scores까지 오염시킨다
(실측: 가장 조밀한 005930조차 67,679,295 대 67,600,262로 79,033주 부족).
*/
func (r *CandleBarRepo) PruneIntraday(
	ctx context.Context,
	interval model.CandleInterval,
	cutoff string,
	symbols []string,
	dryRun bool,
) (int64, error) {
	if len(symbols) == 0 {
		return 0, nil
	}
	placeholders := "?" + strings.Repeat(",?", len(symbols)-1)
	args := make([]any, 0, len(symbols)+2)
	args = append(args, string(interval), cutoff)
	for _, s := range symbols {
		args = append(args, s)
	}

	if dryRun {
		var n int64
		err := r.db.QueryRowContext(ctx, fmt.Sprintf(
			`SELECT COUNT(*) FROM candle_bars WHERE bar_interval = ? AND bar_time < ? AND symbol IN (%s)`,
			placeholders), args...).Scan(&n)
		return n, err
	}

	res, err := r.db.ExecContext(ctx, fmt.Sprintf(
		`DELETE FROM candle_bars WHERE bar_interval = ? AND bar_time < ? AND symbol IN (%s)`,
		placeholders), args...)
	if err != nil {
		return 0, fmt.Errorf("prune %s: %w", interval, err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
