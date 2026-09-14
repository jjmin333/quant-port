// Command rollup-candles folds aged intraday bars into coarser tiers.
//
// 1m ──3일──▶ 5m ──7일──▶ 10m ──14일──▶ 30m ──30일──▶ 60m ──60일──▶ 삭제
//
// 계층표(internal/service/candles_tiers.go)가 유일한 진실이고 이 커맨드는 그것을
// 실행할 뿐이다. 일상 운영은 worker-scores의 예약 슬롯이 맡는다 — 이 커맨드는
// **최초 백로그**를 나눠 돌리거나 결과를 미리 세어 보기 위한 것이다.
//
// 되돌릴 수 없다. 접힌 1분봉은 삭제되고, 원본은 백업 표에만 남는다.
//
//	go run ./cmd/rollup-candles -dry-run                  # 아무것도 쓰지 않고 집계량만
//	go run ./cmd/rollup-candles -symbols=005930,NVDA      # 카나리아: 지정 종목만
//	go run ./cmd/rollup-candles -max-symbols=200          # 백로그를 200종목씩
//	go run ./cmd/rollup-candles                           # 전량
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "만들어질 봉 수만 세고 아무것도 쓰지 않는다")
	chunk := flag.Int("chunk", 0, "트랜잭션 하나가 다루는 심볼 수 (0 = 기본 100)")
	maxSymbols := flag.Int("max-symbols", 0, "단계당 처리할 최대 심볼 수 (0 = 무제한)")
	symbols := flag.String("symbols", "", "쉼표로 구분한 대상 종목 (지정 시 그 종목만 — 카나리아용)")
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

	// 파티션과 src_bars가 없으면 롤업이 성립하지 않는다.
	if err := repository.MigrateCandleBarsTieringIfNeeded(ctx, mysql.DB()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	svc := service.NewCandleRollupService(repository.NewCandleBarRepo(mysql.DB()))
	opts := service.RollupOptions{
		DryRun:            *dryRun,
		ChunkSize:         *chunk,
		MaxSymbolsPerStep: *maxSymbols,
		OnlySymbols:       splitSymbols(*symbols),
	}

	if *dryRun {
		log.Println("rollup-candles: 모의 실행 — 아무것도 쓰지 않는다")
	} else {
		log.Println("rollup-candles: 실제 실행 — 접힌 1분봉은 삭제된다")
	}
	if len(opts.OnlySymbols) > 0 {
		log.Printf("rollup-candles: 대상 종목 %d개 (%s)", len(opts.OnlySymbols), strings.Join(opts.OnlySymbols, ", "))
	}

	summary, err := svc.Run(ctx, opts)
	if err != nil {
		log.Printf("rollup-candles: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(summary)
	fmt.Println(summary.Message())

	if err != nil {
		os.Exit(1)
	}
}

func splitSymbols(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if s := strings.ToUpper(strings.TrimSpace(part)); s != "" {
			out = append(out, s)
		}
	}
	return out
}
