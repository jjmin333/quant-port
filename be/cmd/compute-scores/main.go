// Command compute-scores runs the 종합 점수 batch for one trade date.
//
// Costs no upstream API calls — every input (fundamentals, daily closes, daily
// candles) is already in MySQL, so it is safe to re-run freely while tuning
// weights. Writes are idempotent UPSERTs keyed on (symbol, trade_date).
//
//	go run ./cmd/compute-scores                      # newest stored close date
//	go run ./cmd/compute-scores -date=2026-08-07
//	go run ./cmd/compute-scores -dry-run             # report distribution only
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
)

func main() {
	date := flag.String("date", "", "trade date to score (YYYY-MM-DD; default = newest stored close)")
	timeoutMin := flag.Int("timeout-min", 20, "job timeout in minutes")
	flag.Parse()

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

	if err := repository.MigrateStockScoresIfNeeded(ctx, mysql.DB(), filepath.Join("migrations")); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	settings := service.NewScoreSettingsStore(repository.NewScoreSettingsRepo(mysql.DB()))
	svc := service.NewScoreService(mysql.DB(), repository.NewStockScoreRepo(mysql.DB()), settings)

	tradeDate := *date
	if tradeDate == "" {
		tradeDate, err = svc.LatestTradeDate(ctx)
		if err != nil {
			log.Fatalf("resolve trade date: %v", err)
		}
	}

	current := settings.Get(ctx)
	log.Printf("weights: 추세 %d / 수익성 %d / 성장 %d / 가치 %d / 심리 %d (floor %.2f, shrinkFull %.2f)",
		current.Weights.Technical, current.Weights.Quality, current.Weights.Growth,
		current.Weights.Valuation, current.Weights.Sentiment,
		current.CoverageFloor, current.ShrinkFull)

	dist, err := svc.Recompute(ctx, tradeDate)
	if err != nil {
		log.Fatalf("recompute: %v", err)
	}

	out, _ := json.MarshalIndent(dist, "", "  ")
	fmt.Printf("\n=== %s 분포 ===\n%s\n", tradeDate, out)
	if !dist.Healthy {
		// Not a hard failure: the rows are stored and auditable. But the shape
		// is wrong, so surface it loudly rather than let it reach the UI quietly.
		log.Printf("WARNING: 분포 검증 실패 — %v", dist.Warnings)
	}
}
