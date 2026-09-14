// Command backfill-earnings-announced attaches real announcement dates to the
// quarters that backfill-earnings-surprises stored, then promotes them into the
// calendar table.
//
// # 왜 필요한가
//
// Finnhub `/stock/earnings`는 **발표일을 안 준다**. 주는 날짜는 달력 분기로
// 반올림한 기간말이라, 그대로 캘린더에 올리면 네 날짜에 3천 종목씩 뭉친다
// (2026-06-30에 3,007종목 등). 미국 상장사는 실적을 **8-K Item 2.02**로
// 공시하고, `data.sec.gov/submissions`가 그 제출일을 종목당 1호출로 준다.
//
// # 순서
//
//	1) go run ./cmd/backfill-earnings-surprises     (수치 — Finnhub, 약 1h40m)
//	2) go run ./cmd/backfill-earnings-announced     (날짜 — EDGAR, 약 15분)
//
// # 사용법
//
//	go run ./cmd/backfill-earnings-announced -status
//	go run ./cmd/backfill-earnings-announced -limit=20 -dry-run
//	go run ./cmd/backfill-earnings-announced                 # 전체 + 캘린더 승격
//	go run ./cmd/backfill-earnings-announced -promote-only   # 이미 붙은 날짜만 승격
//
// 중단해도 안전하다. 이미 8-K를 받아 본 종목은 건너뛴다(`-recheck`로 재수집).
package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
)

func main() {
	limit := flag.Int("limit", 0, "max symbols this run (0 = all)")
	symbolsCSV := flag.String("symbols", "", "explicit symbol list (comma separated)")
	recheck := flag.Bool("recheck", false, "re-fetch symbols already checked")
	dryRun := flag.Bool("dry-run", false, "call EDGAR but write nothing")
	statusOnly := flag.Bool("status", false, "print coverage and exit")
	promoteOnly := flag.Bool("promote-only", false, "skip EDGAR; just promote already-dated quarters")
	noPromote := flag.Bool("no-promote", false, "resolve dates but do not touch earnings_events")
	paceMs := flag.Int("pace-ms", 0, "delay between EDGAR calls in ms (0 = edgar_bulk MinInterval)")
	timeoutMin := flag.Int("timeout-min", 120, "job timeout in minutes")
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

	migrations := filepath.Join("migrations")
	if err := repository.MigrateEarningsSurpriseHistoryIfNeeded(ctx, mysql.DB(), migrations); err != nil {
		log.Fatalf("migrate 055: %v", err)
	}
	if err := repository.MigrateEarningsSurpriseAnnouncedIfNeeded(ctx, mysql.DB(), migrations); err != nil {
		log.Fatalf("migrate 056: %v", err)
	}

	historyRepo := repository.NewEarningsSurpriseHistoryRepo(mysql.DB())

	if *statusOnly {
		printAnnounceCoverage(ctx, historyRepo)
		return
	}

	if *promoteOnly {
		n, err := historyRepo.PromoteAnnouncedToCalendar(ctx, string(model.AssetUS))
		if err != nil {
			log.Fatalf("promote: %v", err)
		}
		log.Printf("캘린더 승격: %d행 신규 (기존 행은 건드리지 않음)", n)
		printAnnounceCoverage(ctx, historyRepo)
		return
	}

	if !cfg.Edgar.Enabled() {
		log.Fatal("SEC_EDGAR_USER_AGENT is empty — nothing to call")
	}
	redis, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redis.Close()
	budget := apibudget.NewManager(redis.Client())

	// 조회 경로의 edgar(8/min)가 아니라 배치 버킷을 쓴다 — 안 그러면 두 번째
	// 종목부터 전부 "min interval not elapsed"로 떨어진다.
	edgar := service.NewEdgarClient(
		service.EdgarConfig{UserAgent: cfg.Edgar.UserAgent}, budget,
	).UseBatchBudget()

	job := service.NewEarningsAnnounceBackfill(historyRepo, edgar, budget)

	res, runErr := job.Run(ctx, service.AnnounceBackfillOptions{
		Symbols: service.NormalizeEarningsSymbols(*symbolsCSV, 0),
		Limit:   *limit,
		Recheck: *recheck,
		DryRun:  *dryRun,
		Promote: !*noPromote,
		Pace:    time.Duration(*paceMs) * time.Millisecond,
	})
	log.Printf("결과: 후보 %d · 시도 %d · 매칭성공 %d종목/%d분기 · 8-K없음 %d · 규칙탈락 %d · 실패 %d · 캘린더승격 %d행 · 소요 %v",
		res.Candidates, res.Attempted, res.Matched, res.QuartersDated,
		res.NoFilings, res.Rejected, res.Failed, res.Promoted,
		res.Duration.Truncate(time.Second))
	if runErr != nil {
		log.Fatalf("run: %v", runErr)
	}
	printAnnounceCoverage(ctx, historyRepo)
}

func printAnnounceCoverage(ctx context.Context, repo *repository.EarningsSurpriseHistoryRepo) {
	checked, dated, quarters, err := repo.AnnounceCoverage(ctx, string(model.AssetUS))
	if err != nil {
		log.Printf("coverage: %v", err)
		return
	}
	log.Printf("발표일 커버리지 (us): 8-K 확인 %d종목 · 날짜 붙음 %d종목 · %d분기",
		checked, dated, quarters)
}
