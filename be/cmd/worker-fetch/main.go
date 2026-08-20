// Command worker-fetch is the standalone process for the rate-limited
// stock daily-close fetch jobs: missing symbols, stale sessions, and history gaps.
//
// All three share Toss's per-second candle budget with the live app, and running
// them as a single-process queue (rather than inside the API or alongside the
// close workers) guarantees they never run concurrently with each other and
// never compete with a live request for CPU or memory in the same process.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

// fetchDelay paces requests under Toss's shared candle limit (5/sec app-wide).
const fetchDelay = 250 * time.Millisecond

type fetchKind int

const (
	fetchStale fetchKind = iota
	fetchMissing
	fetchHistory
)

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

	if deps.Toss == nil {
		log.Println("worker-fetch: Toss not configured — queue will accumulate until credentials are set")
	}

	log.Println("worker-fetch: running (manual queue only, fetch-stale + fetch-missing + fetch-history)")

	// Three job ids share one Toss budget, so a mutex serializes the actual work
	// even though each has its own queue-poll loop.
	var tossMu sync.Mutex
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDailyCloseFetchStale,
		handler(deps, fetchStale, &tossMu))
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDailyCloseFetchMissing,
		handler(deps, fetchMissing, &tossMu))
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchDailyCloseFetchHistory,
		handler(deps, fetchHistory, &tossMu))
}

func handler(deps *service.WorkerDeps, kind fetchKind, tossMu *sync.Mutex) service.BatchWorkerHandler {
	return func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
		tossMu.Lock()
		defer tossMu.Unlock()
		if deps.Toss == nil {
			return nil, "", fmt.Errorf("Toss 자격증명이 없어 실행할 수 없습니다")
		}
		since, _ := params["since"].(string)
		if since == "" {
			since = service.DailyCloseBackfillStart
		}
		limit := 500
		if v, ok := params["limit"].(float64); ok && v > 0 {
			limit = int(v)
		}
		market, _ := params["market"].(string)
		assetTypes, err := service.ParseDailyCloseMarket(market)
		if err != nil {
			return nil, "", err
		}

		var (
			res service.FetchMissingResult
		)
		switch kind {
		case fetchStale:
			asOf, e := deps.Closes.LatestStoredTradeDate(ctx)
			if e != nil || asOf == "" {
				return nil, "", fmt.Errorf("최신 거래일을 확인할 수 없습니다")
			}
			res, err = deps.Closes.FetchStaleCloses(ctx, deps.Toss, deps.Bars, asOf, since, limit, fetchDelay, 0, assetTypes...)
		case fetchMissing:
			res, err = deps.Closes.FetchMissingCloses(ctx, deps.Toss, deps.Bars, since, limit, fetchDelay, 0, assetTypes...)
		case fetchHistory:
			res, err = deps.Closes.FetchHistoryCloses(ctx, deps.Toss, deps.Bars, since, limit, fetchDelay, 0, assetTypes...)
		}
		if err != nil {
			return nil, "", err
		}
		// Project what was just fetched so the store reflects it immediately.
		if _, e := deps.Closes.BackfillStocksFromCandleBars(ctx, since, assetTypes...); e != nil {
			log.Printf("worker-fetch: project after fetch: %v", e)
		}
		return map[string]any{
				"attempted": res.Attempted, "filled": res.Filled,
				"noData": res.Empty, "failed": res.Failed, "remainingBefore": res.Remaining,
			},
			fmt.Sprintf("시도 %d · 채움 %d · 데이터없음 %d · 실패 %d (실행 전 대상 %d)",
				res.Attempted, res.Filled, res.Empty, res.Failed, res.Remaining), nil
	}
}
