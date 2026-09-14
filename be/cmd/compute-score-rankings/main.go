// Command compute-score-rankings rebuilds AI 추천 랭킹의 축·기간 조합 48개.
//
// 평소에는 worker-scores가 점수 배치 직후에 같은 일을 한다. 이 커맨드는 **그 배치를
// 기다리지 않고** 채워야 할 때를 위한 것이다:
//
//   - 기능을 처음 배포한 직후 (표가 비어 있어 화면의 축·기간 버튼이 전부 빈 목록이다)
//   - 축이나 기간을 새로 추가한 뒤
//   - 랭킹만 다시 만들어 보고 싶을 때 (점수 재계산 17,915행을 끌고 올 이유가 없다)
//
// 업스트림 호출이 없다 — 재료는 전부 MySQL에 이미 있다. 몇 번을 돌려도 같은 48행을
// 덮어쓸 뿐이라 자유롭게 다시 돌려도 된다.
//
//	go run ./cmd/compute-score-rankings
//	go run ./cmd/compute-score-rankings -timeout-min=10
package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
)

func main() {
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

	migrationsDir := filepath.Join("migrations")
	if err := repository.MigrateStockScoresIfNeeded(ctx, mysql.DB(), migrationsDir); err != nil {
		log.Fatalf("migrate scores: %v", err)
	}
	if err := repository.MigrateStockScoreRankingsIfNeeded(ctx, mysql.DB(), migrationsDir); err != nil {
		log.Fatalf("migrate rankings: %v", err)
	}

	settings := service.NewScoreSettingsStore(repository.NewScoreSettingsRepo(mysql.DB()))
	scores := service.NewScoreService(mysql.DB(), repository.NewStockScoreRepo(mysql.DB()), settings)
	rankings := service.NewScoreRankingService(
		repository.NewStockScoreRankingRepo(mysql.DB()),
		scores,
	)

	started := time.Now()
	if err := rankings.ComputeAll(ctx); err != nil {
		log.Fatalf("compute rankings: %v", err)
	}
	log.Printf("score rankings rebuilt in %s", time.Since(started).Round(time.Millisecond))
}
