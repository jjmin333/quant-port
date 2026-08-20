package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/safego"
)

// BatchWorkerHandler executes one claimed run and reports its outcome.
type BatchWorkerHandler func(ctx context.Context, params map[string]any) (result map[string]any, message string, err error)

// batchQueuePollInterval trades promptness for load: an admin click should
// start within a few seconds, but polling every job's queue on every tick
// across five worker processes adds up, so this stays modest rather than tight.
const batchQueuePollInterval = 4 * time.Second

// RunBatchQueueWorker polls one job's queue forever, running claimed work
// through `handle` and recording the outcome. Meant to run alongside a
// worker's own schedule-driven loop (if it has one) inside the same process —
// this is what lets an admin's manual "지금 재계산" click reach a job that
// otherwise only fires on a market-close trigger.
func RunBatchQueueWorker(ctx context.Context, runs *repository.BatchJobRunRepo, jobID model.BatchJobID, handle BatchWorkerHandler) {
	if runs == nil {
		return
	}
	ticker := time.NewTicker(batchQueuePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			safego.Call("batch-worker-"+string(jobID), func() {
				runOneQueuedJob(ctx, runs, jobID, handle)
			})
		}
	}
}

func runOneQueuedJob(ctx context.Context, runs *repository.BatchJobRunRepo, jobID model.BatchJobID, handle BatchWorkerHandler) {
	id, params, ok, err := runs.ClaimNext(ctx, string(jobID))
	if err != nil {
		log.Printf("batch worker %s: claim: %v", jobID, err)
		return
	}
	if !ok {
		return
	}

	// Bounded generously: the rate-limited fetch jobs are paced by the Toss
	// limit and a large -limit legitimately takes tens of minutes.
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()

	result, message, err := handle(runCtx, params)
	status := "done"
	if err != nil {
		status = "error"
		message = err.Error()
		log.Printf("batch worker %s: run %d failed: %v", jobID, id, err)
	} else {
		log.Printf("batch worker %s: run %d done — %s", jobID, id, message)
	}
	payload, _ := json.Marshal(result)
	if e := runs.Finish(context.Background(), id, status, message, string(payload)); e != nil {
		log.Printf("batch worker %s: record finish: %v", jobID, e)
	}
}

// RunScheduledJob is the schedule-driven counterpart to the queue poller: it
// records a "schedule"-triggered run the same way a manual one is recorded, so
// the admin console shows both kinds in one history without caring which
// process or trigger produced them.
func RunScheduledJob(
	ctx context.Context,
	runs *repository.BatchJobRunRepo,
	jobID model.BatchJobID,
	run func(ctx context.Context) (result map[string]any, message string, err error),
) {
	var runID int64
	if runs != nil {
		if id, err := runs.Begin(ctx, string(jobID), "schedule"); err == nil {
			runID = id
		}
	}
	result, message, err := run(ctx)
	if runs == nil || runID == 0 {
		if err != nil {
			log.Printf("batch %s: %v", jobID, err)
		}
		return
	}
	status := "done"
	if err != nil {
		status = "error"
		message = err.Error()
	}
	payload, _ := json.Marshal(result)
	if e := runs.Finish(context.Background(), runID, status, message, string(payload)); e != nil {
		log.Printf("batch %s: record finish: %v", jobID, e)
	}
}
