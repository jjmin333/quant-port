// Package index is the worker-index role (예전 단독 프로세스, 지금은 cmd/worker가 역할로 돌린다) is the standalone process for index close history
// backfill (Toss + FRED).
//
// Manual-only (no fixed schedule) — an admin requests a backfill window from
// /admin/batch, this process claims it from the queue. Split from the two
// close workers because it calls FRED as well as Toss, and from worker-fetch
// because indices and stocks have entirely different failure modes (FRED rate
// limits vs Toss 400s on delisted tickers) that should not share a queue.
package index

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

// Run starts this role's schedules and manual queues and blocks until ctx is done.
// 부트스트랩·종료 신호는 cmd/worker가 한 번만 한다(여러 역할이 한 프로세스를 나눠 쓴다).
func Run(ctx context.Context, deps *service.WorkerDeps) {
	// 이 워커의 잡은 이 프로세스만 돌린다 — 재기동 전에 끊긴 실행을 지금 닫아 만회가 다시 넣게 한다.
	service.CloseInterruptedRuns(ctx, deps.Runs, model.BatchIndexCloses)

	if deps.IndexFill == nil {
		log.Println("worker-index: Toss not configured — queue will accumulate until credentials are set")
	}

	log.Println("worker-index: running (manual queue only)")
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchIndexCloses,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			if deps.IndexFill == nil {
				return nil, "", fmt.Errorf("Toss 자격증명이 없어 실행할 수 없습니다")
			}
			since, _ := params["since"].(string)
			if since == "" {
				since = "2026-07-01"
			}
			sinceDate, err := time.Parse("2006-01-02", since)
			if err != nil {
				return nil, "", fmt.Errorf("잘못된 기준일 %q", since)
			}

			res, err := deps.IndexFill.Run(ctx, sinceDate)
			if err != nil {
				return nil, "", err
			}
			scored, err := deps.Indices.RecomputeAll(ctx)
			if err != nil {
				return nil, "", err
			}
			return map[string]any{"rows": res.Rows, "indices": res.Indices, "scored": scored, "report": res.Report},
				fmt.Sprintf("%d개 지수 %d행 적재, %d개 채점", res.Indices, res.Rows, scored), nil
		})
}
