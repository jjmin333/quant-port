// Package slots runs a role's daily KST slots and per-job manual queues — the loop
// worker-filings 가 써 온 것을 다른 역할이 같이 쓰도록 꺼낸 것(2026-10-01).
package slots

import (
	"context"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
)

// Slot is one scheduled job: when it fires (KST, daily).
type Slot struct {
	Job          model.BatchJobID
	Hour, Minute int
}

// Dispatch maps a job id to its run. limit은 관리자 수동 실행의 숫자 칸(예약 실행은 0).
type Dispatch func(ctx context.Context, job model.BatchJobID, limit int) (map[string]any, string, error)

var kst = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}()

// next is the next daily HH:MM KST strictly after now.
func next(now time.Time, hour, minute int) time.Time {
	local := now.In(kst)
	at := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, kst)
	if !at.After(local) {
		at = at.AddDate(0, 0, 1)
	}
	return at
}

// Run closes this role's interrupted runs, starts every slot loop and queue, and blocks until ctx ends.
func Run(ctx context.Context, runs *repository.BatchJobRunRepo, name string, list []Slot, dispatch Dispatch) {
	jobs := make([]model.BatchJobID, 0, len(list))
	for _, s := range list {
		jobs = append(jobs, s.Job)
	}
	service.CloseInterruptedRuns(ctx, runs, jobs...)
	for _, s := range list {
		go schedule(ctx, runs, s, dispatch)
		job := s.Job
		go service.RunBatchQueueWorker(ctx, runs, job,
			func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
				return dispatch(ctx, job, LimitFromParams(params))
			})
	}
	log.Printf("%s: running (%d jobs + manual queue)", name, len(list))
	<-ctx.Done()
}

func schedule(ctx context.Context, runs *repository.BatchJobRunRepo, s Slot, dispatch Dispatch) {
	// 한 슬롯은 한 번만 — time.After가 몇 초 일찍 깨면 같은 슬롯을 다시 계산해 두 번 돌았다(service.SlotGate 머리글).
	gate := service.NewSlotGate()
	for {
		at := next(time.Now(), s.Hour, s.Minute)
		if !service.WaitForSlot(ctx, at) {
			return
		}
		if !gate.Allow(at) {
			continue
		}
		service.RunScheduledJob(ctx, runs, s.Job, func(ctx context.Context) (map[string]any, string, error) {
			return dispatch(ctx, s.Job, 0)
		})
	}
}

// LimitFromParams reads the admin console's optional 숫자 knob.
func LimitFromParams(params map[string]any) int {
	switch v := params["limit"].(type) {
	case float64: // JSON round-trip
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}
