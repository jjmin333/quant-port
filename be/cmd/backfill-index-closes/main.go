// Command backfill-index-closes loads index_daily_closes with real dated
// history, then scores every index.
//
// Sources differ per index and each one is dated at the source, so no date is
// ever inferred:
//   - Toss indicator candles   → kospi, kosdaq
//   - FRED series observations → nasdaq, nasdaq100, sp500, dow, vix, us10y and
//     usd-krw (DEXKOUS — Toss returns 400 for the USD/KRW candle symbol, so the
//     live source and the history source differ for that one index)
//   - Upbit daily candles      → btc
//   - stock_daily_closes       → nq100 (QQQ), sox (SOXX), which are ETFs we
//     already store daily closes for, so this costs no API call
//
// The run ends by deleting poller rows that duplicate the previous close — FRED
// publishes a day late, so the daily batch used to write yesterday's value under
// today's date and leave a 0% change rate behind.
//
// Indices without a dated history source are skipped and reported; their rows
// still accumulate going forward through the daily close batch.
//
//	go run ./cmd/backfill-index-closes -since=2026-07-01
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/toss"
)

func main() {
	since := flag.String("since", "2026-07-01", "first trade date to load (YYYY-MM-DD)")
	timeoutMin := flag.Int("timeout-min", 20, "job timeout in minutes")
	flag.Parse()

	sinceDate, err := time.Parse("2006-01-02", *since)
	if err != nil {
		log.Fatalf("invalid -since %q: %v", *since, err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutMin)*time.Minute)
	defer cancel()

	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	migrations := filepath.Join("migrations")
	if err := repository.MigrateDailyClosesIfNeeded(ctx, mysql.DB(), migrations); err != nil {
		log.Fatalf("migrate daily closes: %v", err)
	}
	if err := repository.MigrateIndexScoresIfNeeded(ctx, mysql.DB(), migrations); err != nil {
		log.Fatalf("migrate index scores: %v", err)
	}

	indexRepo := repository.NewIndexDailyCloseRepo(mysql.DB())

	var tossClient *toss.Client
	if cfg.Toss.Enabled() {
		redis, err := repository.NewRedis(cfg.RedisURL)
		if err != nil {
			log.Fatalf("redis: %v", err)
		}
		defer redis.Close()
		if tossClient, err = toss.NewClient(cfg.Toss, toss.NewTokenStore(redis.Client())); err != nil {
			log.Fatalf("toss: %v", err)
		}
	}

	backfill := service.NewIndexBackfillService(mysql.DB(), indexRepo, tossClient, cfg)
	res, err := backfill.Run(ctx, sinceDate)
	if err != nil {
		log.Fatalf("backfill: %v", err)
	}
	for _, line := range res.Report {
		log.Println("  " + line)
	}
	log.Printf("index backfill: %d rows across %d indices", res.Rows, res.Indices)

	scores := service.NewIndexScoreService(mysql.DB(), indexRepo)
	updated, err := scores.RecomputeAll(ctx)
	if err != nil {
		log.Fatalf("score: %v", err)
	}
	log.Printf("index scores updated: %d", updated)

	var total int
	_ = mysql.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM index_daily_closes WHERE trade_date >= ?`, *since).Scan(&total)
	log.Printf("index_daily_closes since %s: %d rows", *since, total)
	_ = sql.ErrNoRows
}
