// Command worker-scores is the standalone process for the daily 종합 점수
// batch (stocks + indices).
//
// DB-only — no upstream API calls — so it does not compete with the fetch
// worker's Toss budget. Runs once daily at a fixed KST slot, reading whatever
// the two close workers most recently wrote; kept separate from them so a slow
// or stuck score recompute cannot delay either market's close ingestion.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

// nextScheduledRefresh is the next 00:00:00 KST strictly after now. Scoring
// runs a bit after that (see scoreDelay) so the close workers have had time to
// land the newest session first.
func nextScheduledRefresh(now time.Time) time.Time {
	loc := kstLocation()
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	if !midnight.After(local) {
		midnight = midnight.AddDate(0, 0, 1)
	}
	return midnight
}

func kstLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}

const scoreDelay = 25 * time.Minute

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps, err := service.BootstrapWorker(ctx)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer deps.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-stop
		log.Printf("shutdown: received %s", sig)
		cancel()
	}()

	go runSchedule(ctx, deps)

	log.Println("worker-scores: running (daily 00:25 KST + manual queue)")
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchStockScores,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return recompute(ctx, deps)
		})
}

func runSchedule(ctx context.Context, deps *service.WorkerDeps) {
	for {
		wait := time.Until(nextScheduledRefresh(time.Now())) + scoreDelay
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		service.RunScheduledJob(ctx, deps.Runs, model.BatchStockScores,
			func(ctx context.Context) (map[string]any, string, error) {
				return recompute(ctx, deps)
			})
	}
}

func recompute(ctx context.Context, deps *service.WorkerDeps) (map[string]any, string, error) {
	tradeDate, err := deps.Scores.LatestTradeDate(ctx)
	if err != nil {
		return nil, "", err
	}
	dist, err := deps.Scores.Recompute(ctx, tradeDate)
	if err != nil {
		return nil, "", err
	}
	// Indices ride the same daily slot: they read the closes the close
	// workers already persisted, so this costs no upstream call either.
	indexCount, err := deps.Scores.RecomputeIndices(ctx)
	if err != nil {
		log.Printf("worker-scores: index recompute: %v", err)
	}

	msg := fmt.Sprintf("%s 기준 %d종목 + 지수 %d개 산출 (평균 %.1f, 표준편차 %.1f)",
		tradeDate, dist.Count, indexCount, dist.Mean, dist.StdDev)
	if !dist.Healthy {
		msg += fmt.Sprintf(" — 분포 경고: %v", dist.Warnings)
	}
	return map[string]any{"tradeDate": tradeDate, "distribution": dist, "indexScores": indexCount}, msg, nil
}
