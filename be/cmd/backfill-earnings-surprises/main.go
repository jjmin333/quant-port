// Command backfill-earnings-surprises fills earnings_surprise_history with the
// last four quarters of EPS 컨센서스 vs 실적 for US symbols.
//
// # 왜 일회성인가
//
// Finnhub 무료 티어의 실적 캘린더(`/calendar/earnings`)는 **과거 30일까지만**
// 준다(2026-08-21 실측: 2026-07-22 이전은 HTTP 200에 빈 배열). 그래서 해외는
// 배치가 돌기 시작한 뒤의 한 시즌치밖에 없었고, 「꾸준히 기대를 넘는 기업」
// 카드는 최소 표본 4분기를 넘는 종목이 0개라 늘 비어 있었다.
//
// `/stock/earnings`는 종목당 4분기를 주므로 그 공백을 한 번에 메운다. 정기 잡으로
// 두지 않는 이유는 새 분기는 캘린더 배치가 **발표일과 함께** 이미 넣기 때문이다 —
// 매일 돌리면 3,000호출을 써서 같은 4분기를 다시 받는다.
//
// # 사용법
//
//	go run ./cmd/backfill-earnings-surprises -status          # 현황만 보고 종료
//	go run ./cmd/backfill-earnings-surprises -limit=20 -dry-run
//	go run ./cmd/backfill-earnings-surprises -limit=20        # 소량 실제 저장
//	go run ./cmd/backfill-earnings-surprises                  # 전체 (약 80분)
//
// 중단해도 안전하다. 기본이 `-resume`이라 이미 이력이 있는 종목은 건너뛴다.
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
	symbolsCSV := flag.String("symbols", "", "explicit symbol list (comma separated); overrides the default candidate set")
	resume := flag.Bool("resume", true, "skip symbols that already have stored history")
	dryRun := flag.Bool("dry-run", false, "call the API but write nothing")
	statusOnly := flag.Bool("status", false, "print coverage and exit (no API calls, no writes)")
	paceMs := flag.Int("pace-ms", 0, "delay between calls in ms (0 = the finnhub_backfill budget's MinInterval)")
	timeoutMin := flag.Int("timeout-min", 180, "job timeout in minutes")
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
		log.Fatalf("migrate earnings_surprise_history: %v", err)
	}

	historyRepo := repository.NewEarningsSurpriseHistoryRepo(mysql.DB())
	eventRepo := repository.NewEarningsEventRepo(mysql.DB())

	if *statusOnly {
		printCoverage(ctx, historyRepo, eventRepo)
		return
	}

	finnhubCfg := service.EarningsConfig{FinnhubAPIKey: cfg.Finnhub.APIKey}
	if !finnhubCfg.Enabled() {
		log.Fatal("FINNHUB_API_KEY is empty — nothing to call")
	}

	// Redis가 있어야 예산 카운터가 프로세스 밖에 남는다. 없으면 Manager가
	// 메모리 카운터로 돌지만, 그때는 이 잡의 호출량이 다른 프로세스에 안 보인다.
	redis, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redis.Close()
	budget := apibudget.NewManager(redis.Client())

	// 전용 버킷으로 옮긴다 — 캘린더 배치의 20초 간격을 쓰면 3,000종목에 17시간이
	// 걸리고 그동안 그 배치를 막는다(providers.go ProviderFinnhubBackfill).
	client := service.NewEarningsClient(finnhubCfg, budget).UseBackfillBudget()

	backfill := service.NewEarningsSurpriseBackfill(historyRepo, eventRepo, client, budget)

	opts := service.BackfillOptions{
		Symbols: service.NormalizeEarningsSymbols(*symbolsCSV, 0),
		Limit:   *limit,
		Resume:  *resume,
		DryRun:  *dryRun,
		Pace:    time.Duration(*paceMs) * time.Millisecond,
	}

	res, runErr := backfill.Run(ctx, opts)
	log.Printf("backfill 결과: 후보 %d · 시도 %d · 성공 %d · 빈응답 %d · 실패 %d · 저장 %d행 · 소요 %v",
		res.Candidates, res.Attempted, res.Fetched, res.Empty, res.Failed, res.RowsSaved,
		res.Duration.Truncate(time.Second))
	if runErr != nil {
		// 중단이어도 지금까지 저장한 것은 남아 있고, 다시 실행하면 -resume이
		// 이어받는다. 그래서 Fatal 전에 결과부터 찍는다.
		log.Fatalf("backfill: %v", runErr)
	}
	printCoverage(ctx, historyRepo, eventRepo)
}

func printCoverage(
	ctx context.Context,
	history *repository.EarningsSurpriseHistoryRepo,
	events *repository.EarningsEventRepo,
) {
	rows, symbols, oldest, newest, err := history.Coverage(ctx, string(model.AssetUS))
	if err != nil {
		log.Printf("coverage: %v", err)
		return
	}
	log.Printf("earnings_surprise_history (us): %d행 · %d종목 · 기간 %s ~ %s",
		rows, symbols, oldest, newest)

	confirmed, err := events.ConfirmedSymbols(ctx, string(model.AssetUS))
	if err != nil {
		return
	}
	log.Printf("대상 후보(확정 실적이 있는 해외 종목): %d종목", len(confirmed))
}
