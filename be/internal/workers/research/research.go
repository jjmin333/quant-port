// Package research is the worker-research role (예전 단독 프로세스, 지금은 cmd/worker가 역할로 돌린다) is the standalone process for the 리서치 콜 verdict
// batch.
//
// It only reads stored closes, so it costs no upstream API budget and cannot
// be rate-limited. What it does need is ordering: the daily-close workers must
// have landed the session first, or a call maturing today would be judged
// against yesterday's price. Hence the schedule below sits well after both
// market close workers, and any call it cannot resolve simply stays open for
// the next run rather than being resolved with stale data.
package research

import (
	"context"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

// verdictHourKST / verdictMinuteKST place the daily slot after both close
// workers have written the newest session:
//
//	KRX  close 15:30 KST + 20m  → ~15:50 KST (previous KST day)
//	US   close 16:00 ET + 20m   → ~05:20–06:20 KST (this morning, DST-dependent)
//
// 07:00 KST clears the later of the two with room to spare. Running earlier
// would judge a call maturing today against yesterday's US close.
const verdictHourKST = 7

const verdictMinuteKST = 0

func kstLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}

// nextRun is the next verdict slot strictly after now.
func nextRun(now time.Time) time.Time {
	loc := kstLocation()
	local := now.In(loc)
	slot := time.Date(local.Year(), local.Month(), local.Day(), verdictHourKST, verdictMinuteKST, 0, 0, loc)
	if !slot.After(local) {
		slot = slot.AddDate(0, 0, 1)
	}
	return slot
}

// Run starts this role's schedules and manual queues and blocks until ctx is done.
// 부트스트랩·종료 신호는 cmd/worker가 한 번만 한다(여러 역할이 한 프로세스를 나눠 쓴다).
func Run(ctx context.Context, deps *service.WorkerDeps) {
	// 이 워커의 잡은 이 프로세스만 돌린다 — 재기동 전에 끊긴 실행을 지금 닫아 만회가 다시 넣게 한다.
	service.CloseInterruptedRuns(ctx, deps.Runs, model.BatchResearchVerdict)
	service.CloseInterruptedRuns(ctx, deps.Runs, model.BatchBotDrafts)

	go runSchedule(ctx, deps)

	// 봇 초안(2026-10-16) — 같은 「글」 갈래라 이 역할에 둔다. DB만 읽는다.
	bots := service.NewBotDraftService(deps.MySQL.DB()).WithLLM(deps.BotLLM, deps.Budget)
	go runBotSchedule(ctx, deps, bots)
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchBotDrafts,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return bots.Run(ctx)
		})

	log.Println("worker-research: running (daily 07:00 KST · bot drafts every 3h KST + manual queues)")
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchResearchVerdict,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return deps.Research.RunVerdicts(ctx)
		})
}

func runSchedule(ctx context.Context, deps *service.WorkerDeps) {
	for {
		wait := time.Until(nextRun(time.Now()))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		service.RunScheduledJob(ctx, deps.Runs, model.BatchResearchVerdict,
			func(ctx context.Context) (map[string]any, string, error) {
				return deps.Research.RunVerdicts(ctx)
			})
	}
}

func runBotSchedule(ctx context.Context, deps *service.WorkerDeps, bots *service.BotDraftService) {
	// 한 슬롯은 한 번만(service.SlotGate 머리글).
	gate := service.NewSlotGate()
	for {
		slot := service.BotNextSlot(time.Now())
		if !service.WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		service.RunScheduledJob(ctx, deps.Runs, model.BatchBotDrafts,
			func(ctx context.Context) (map[string]any, string, error) {
				return bots.Run(ctx)
			})
	}
}
