// Command repair-candle-tz fixes daily bars whose bar_time was stored in the
// process's local timezone instead of KST.
//
// Background: CandleBarRepo handed the driver a KST time.Time while the DSN
// says loc=Local. In the Docker container (UTC) that wrote KST midnight as the
// PREVIOUS day at 15:00. The Go read path multiplied by .In(KST) and so looked
// correct, but anything doing DATE(bar_time) in SQL — the 일별 종가 projection —
// read the trade date one day early. Host-run tools (Windows, KST) wrote the
// same bar at 00:00, so both spellings now coexist for the same session.
//
// The write path is fixed separately (daily bars are now stored as a KST
// wall-clock literal). This repairs the rows already on disk.
//
// Scoped to bar_interval = '1d' deliberately. MySQL's default collation is
// case-insensitive, so a predicate of IN ('1d','1w','1M') also matches the
// '1m' minute bars — 2.8M rows that must not be touched.
//
//	go run ./cmd/repair-candle-tz            # report only
//	go run ./cmd/repair-candle-tz -apply
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
)

func main() {
	apply := flag.Bool("apply", false, "actually rewrite rows (default: report only)")
	timeoutMin := flag.Int("timeout-min", 30, "job timeout in minutes")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutMin)*time.Minute)
	defer cancel()

	mysql, err := repository.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()
	db := mysql.DB()

	fmt.Println("== 일봉 bar_time 시각 분포 (정상값은 00:00:00) ==")
	if err := report(ctx, db, `
		SELECT TIME(bar_time) AS t, COUNT(*) AS rows_count,
		       MIN(DATE(bar_time)) AS first_date, MAX(DATE(bar_time)) AS last_date
		  FROM candle_bars
		 WHERE bar_interval = '1d'
		 GROUP BY TIME(bar_time) ORDER BY rows_count DESC LIMIT 12`); err != nil {
		log.Fatalf("report: %v", err)
	}

	var shifted int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM candle_bars
		 WHERE bar_interval = '1d' AND TIME(bar_time) = '15:00:00'`).Scan(&shifted); err != nil {
		log.Fatalf("count shifted: %v", err)
	}

	var collide int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM candle_bars bad
		  JOIN candle_bars ok
		    ON ok.symbol = bad.symbol AND ok.bar_interval = bad.bar_interval
		   AND ok.bar_time = bad.bar_time + INTERVAL 9 HOUR
		 WHERE bad.bar_interval = '1d' AND TIME(bad.bar_time) = '15:00:00'`).Scan(&collide); err != nil {
		log.Fatalf("count collisions: %v", err)
	}

	fmt.Printf("\nUTC로 기록된 일봉(15:00): %d행\n", shifted)
	fmt.Printf("  → 올바른 KST 행이 이미 있어 삭제 대상: %d행\n", collide)
	fmt.Printf("  → +9시간 이동 대상: %d행\n", shifted-collide)

	// Any other non-midnight time is not the known UTC pattern; surface it
	// instead of guessing at a correction.
	var other int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM candle_bars
		 WHERE bar_interval = '1d'
		   AND TIME(bar_time) NOT IN ('00:00:00','15:00:00')`).Scan(&other); err != nil {
		log.Fatalf("count other: %v", err)
	}
	if other > 0 {
		fmt.Printf("  ⚠ 00:00/15:00 이 아닌 일봉 %d행 — 자동 보정하지 않음 (수동 확인 필요)\n", other)
	}

	// Safety check before deleting anything.
	//
	// Values do not have to match exactly. Many UTC rows are partial captures:
	// the poller wrote the bar mid-session, then a later daily fetch stored the
	// completed session under its KST timestamp. Those pairs differ in close
	// AND carry a much smaller volume on the UTC side (measured: CSCO 3.4M vs
	// 16.0M for the same session), so dropping the UTC row keeps the better
	// bar.
	//
	// The condition that actually matters is that we never delete a row whose
	// KST counterpart is LESS complete than it is.
	var regressions int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM candle_bars bad
		  JOIN candle_bars ok
		    ON ok.symbol = bad.symbol AND ok.bar_interval = bad.bar_interval
		   AND ok.bar_time = bad.bar_time + INTERVAL 9 HOUR
		 WHERE bad.bar_interval = '1d' AND TIME(bad.bar_time) = '15:00:00'
		   AND ok.volume < bad.volume`).Scan(&regressions); err != nil {
		log.Fatalf("verify duplicates: %v", err)
	}
	// Neither side is uniformly better — some UTC rows are partial captures the
	// poller took mid-session, and some KST rows are partial bars the daily
	// fetch stored for a session still in progress. So the repair merges rather
	// than picking a winner by timestamp: whichever row holds the fuller
	// session survives, at the canonical KST slot.
	fmt.Printf("  · UTC 행이 더 완성된 봉이라 KST 행으로 승격할 대상: %d행\n", regressions)

	if !*apply {
		fmt.Println("\n(보고 전용 — 실제 반영하려면 -apply)")
		return
	}

	start := time.Now()

	// Copy every UTC row to its correct KST slot in one statement, then drop
	// the originals.
	//
	// An in-place UPDATE of bar_time cannot work here: MySQL applies it row by
	// row against a live unique index, so a row landing on a slot that another
	// pending row still occupies aborts the whole statement — which is exactly
	// what happened (Duplicate entry 'GEV-1d-2026-08-05'). INSERT ... ON
	// DUPLICATE KEY resolves each collision explicitly instead, keeping the
	// fuller bar. Volume is assigned last because the price columns compare
	// against the pre-update value.
	res, err := db.ExecContext(ctx, `
		INSERT INTO candle_bars
			(symbol, bar_interval, bar_time, open_price, high_price, low_price, close_price, volume)
		SELECT src.symbol, src.bar_interval, src.bar_time + INTERVAL 9 HOUR,
		       src.open_price, src.high_price, src.low_price, src.close_price, src.volume
		  FROM candle_bars AS src
		 WHERE src.bar_interval = '1d' AND TIME(src.bar_time) = '15:00:00'
		ON DUPLICATE KEY UPDATE
			open_price  = IF(VALUES(volume) > candle_bars.volume, VALUES(open_price),  candle_bars.open_price),
			high_price  = IF(VALUES(volume) > candle_bars.volume, VALUES(high_price),  candle_bars.high_price),
			low_price   = IF(VALUES(volume) > candle_bars.volume, VALUES(low_price),   candle_bars.low_price),
			close_price = IF(VALUES(volume) > candle_bars.volume, VALUES(close_price), candle_bars.close_price),
			volume      = GREATEST(VALUES(volume), candle_bars.volume)`)
	if err != nil {
		log.Fatalf("move to KST slot: %v", err)
	}
	moved, _ := res.RowsAffected()

	res, err = db.ExecContext(ctx, `
		DELETE FROM candle_bars WHERE bar_interval = '1d' AND TIME(bar_time) = '15:00:00'`)
	if err != nil {
		log.Fatalf("drop UTC rows: %v", err)
	}
	deleted, _ := res.RowsAffected()

	fmt.Printf("\n적용 완료: KST 슬롯 반영 %d건(신규+병합), UTC 원본 삭제 %d행 (%s)\n",
		moved, deleted, time.Since(start).Round(time.Millisecond))

	fmt.Println("\n== 보정 후 분포 ==")
	if err := report(ctx, db, `
		SELECT TIME(bar_time) AS t, COUNT(*) AS rows_count
		  FROM candle_bars
		 WHERE bar_interval = '1d'
		 GROUP BY TIME(bar_time) ORDER BY rows_count DESC LIMIT 6`); err != nil {
		log.Fatalf("report: %v", err)
	}
	fmt.Println("\n다음 단계: go run ./cmd/backfill-daily-closes -since=... (투영 재실행) → go run ./cmd/compute-scores")
}

func report(ctx context.Context, db *sql.DB, query string) error {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	fmt.Println("  " + join(cols, " | "))
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		parts := make([]string, len(cols))
		for i := range cols {
			parts[i] = render(vals[i])
		}
		fmt.Println("  " + join(parts, " | "))
	}
	return rows.Err()
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func render(v any) string {
	switch t := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(t)
	case time.Time:
		return t.Format("2006-01-02")
	default:
		return fmt.Sprint(t)
	}
}
