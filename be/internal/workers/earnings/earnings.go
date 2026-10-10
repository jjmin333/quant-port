// Package earnings is the worker-earnings role (예전 단독 프로세스, 지금은 cmd/worker가 역할로 돌린다) is the standalone process for 실적 발표 적재.
//
// # 왜 별 프로세스인가
//
// 두 소스 모두 우리 통제 밖이다. FnGuide는 공개 API가 아닌 내부 JSON이라 언제든
// 느려지거나 모양이 바뀔 수 있고, Finnhub은 429를 던진다. 그 둘을 사용자 요청을
// 처리하는 프로세스에서 부르면, 남의 사이트가 느린 동안 우리 화면이 함께 느려진다.
// **적재는 여기, 조회는 API 프로세스** — 표를 통해서만 만난다.
//
// # 시각
//
//	국내 시즌(1–2·4–5·7–8·10–11월) 10:00–20:00 /2h KST
//	                 — FnGuide가 이미 지난 날짜에 개별·발표치를 나중에 붙인다.
//	                   당월만 다시 받고, 20시(하루의 마지막)만 전월을 겹친다.
//	     비시즌 18:30 — 정정용 전월을 포함해 하루 한 번.
//	해외 06:00 KST  — 일정 정정·먼 미래 (−49/+76, 7일 조각 18회).
//	     5분        — KST 어제~오늘 1회. 실적치가 세션 직후 들어오게.
//	                   06:00 전체 창이 도는 동안은 건너뛴다.
//	                   어제보다 오래된 「예정」(실적치 없는 잔류)은
//	                   여기서 지우지 않는다. 화면 조회
//	                   DropStaleOverseasScheduled가 KST 어제 미만만 숨긴다
//	                   (2026-08-21: 날짜를 다음날로 밀거나 배지를 바꾸지 않기로 함).
//
// 기동 시 놓친 슬롯을 한 번 메운다. next는 미래만 보므로, 재시작이 슬롯을
// 지나면 그날 적재가 통째로 사라진다 — 일별 종가 워커가 같은 이유로 catch-up을
// 갖고 있다.
//
// 국내(FnGuide)와 해외(Finnhub)는 호스트·키·버킷이 달라 동시에 돈다. 같은 잡의
// catch-up과 정규 슬롯만 직렬이다 — FnGuide 30초·Finnhub 20초 간격은 자기
// 자신과만 싸운다.
package earnings

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
	service.CloseInterruptedRuns(ctx, deps.Runs, model.BatchEarningsDomestic, model.BatchEarningsOverseas)

	// 시장마다 고루틴 하나: catch-up 다음 정규 스케줄. 두 시장은 동시에 시작한다.
	go runDomesticLoop(ctx, deps)
	go runOverseasLoop(ctx, deps)

	// 수동 실행 큐는 국내 쪽을 이 goroutine에서, 해외 쪽을 하나 더 띄워 받는다.
	// 두 큐를 한 폴러로 합치지 않는 이유는 RunBatchQueueWorker가 잡 하나를
	// 맡는 계약이고, 둘을 섞으면 한쪽의 긴 백필이 다른 쪽 클릭을 막는다.
	go service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchEarningsOverseas,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return runOverseas(ctx, deps)
		})

	log.Printf("worker-earnings: running (국내 시즌 10–20/2h · 비시즌 18:30 · 해외 06:00 전체창 + 5분 당일 + manual queue)")
	service.RunBatchQueueWorker(ctx, deps.Runs, model.BatchEarningsDomestic,
		func(ctx context.Context, params map[string]any) (map[string]any, string, error) {
			return runDomestic(ctx, deps, monthsFromParams(params))
		})
}

func runDomesticLoop(ctx context.Context, deps *service.WorkerDeps) {
	catchUpDomestic(ctx, deps)
	runDomesticSchedule(ctx, deps)
}

func runOverseasLoop(ctx context.Context, deps *service.WorkerDeps) {
	catchUpOverseas(ctx, deps)
	// 기동 catch-up이 전체 창이면 그 다음에 5분 폴링을 연다. 같이 띄우면
	// 첫 틱이 18조각과 같은 키를 쓴다.
	go runOverseasLiveLoop(ctx, deps)
	runOverseasSchedule(ctx, deps)
}

func runOverseasLiveLoop(ctx context.Context, deps *service.WorkerDeps) {
	runOverseasLiveOnce(ctx, deps)
	// 한 슬롯은 한 번만 (service.SlotGate 머리글 — 실측으로 예약 잡이 두 번 돌았다).
	gate := service.NewSlotGate()
	for {
		slot := service.NextOverseasLiveSlot(time.Now())
		if !service.WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		runOverseasLiveOnce(ctx, deps)
	}
}

func runOverseasLiveOnce(ctx context.Context, deps *service.WorkerDeps) {
	pending := false
	if deps.Runs != nil {
		p, err := deps.Runs.HasPending(ctx, string(model.BatchEarningsOverseas))
		if err != nil {
			log.Printf("worker-earnings: live HasPending: %v", err)
		}
		pending = p
	}
	if service.ShouldSkipOverseasLive(time.Now(), service.OverseasFullInFlight() || pending) {
		return
	}
	n, err := deps.Earnings.IngestOverseasLive(ctx)
	if err != nil {
		log.Printf("worker-earnings: live ingest: %v", err)
		return
	}
	log.Printf("worker-earnings: live ingest %d행 (어제~오늘)", n)
}

func runDomesticSchedule(ctx context.Context, deps *service.WorkerDeps) {
	gate := service.NewSlotGate()
	for {
		slot := service.NextDomesticSlot(time.Now())
		if !service.WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		months := service.DomesticMonthsForSlot(slot)
		service.RunScheduledJob(ctx, deps.Runs, model.BatchEarningsDomestic,
			func(ctx context.Context) (map[string]any, string, error) {
				return runDomestic(ctx, deps, months)
			})
	}
}

func runOverseasSchedule(ctx context.Context, deps *service.WorkerDeps) {
	gate := service.NewSlotGate()
	for {
		slot := service.NextOverseasSlot(time.Now())
		if !service.WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		service.RunScheduledJob(ctx, deps.Runs, model.BatchEarningsOverseas,
			func(ctx context.Context) (map[string]any, string, error) {
				return runOverseas(ctx, deps)
			})
	}
}

func catchUpDomestic(ctx context.Context, deps *service.WorkerDeps) {
	elapsed := service.LastElapsedDomesticSlot(time.Now())
	if !shouldCatchUp(ctx, deps, model.BatchEarningsDomestic, elapsed) {
		return
	}
	log.Printf("worker-earnings: catch-up 국내 (놓친 슬롯 %s)", elapsed.Format("15:04 MST"))
	service.RunScheduledJob(ctx, deps.Runs, model.BatchEarningsDomestic,
		func(ctx context.Context) (map[string]any, string, error) {
			n, err := deps.Earnings.IngestDomestic(ctx, 2)
			if err != nil {
				return nil, "", err
			}
			return map[string]any{"rows": n, "months": 2, "catchUp": true},
				fmt.Sprintf("국내 실적 발표 %d행 재적재(catch-up)", n), nil
		})
}

func catchUpOverseas(ctx context.Context, deps *service.WorkerDeps) {
	elapsed := service.LastElapsedOverseasSlot(time.Now())
	if !shouldCatchUp(ctx, deps, model.BatchEarningsOverseas, elapsed) {
		return
	}
	log.Printf("worker-earnings: catch-up 해외 (놓친 슬롯 %s)", elapsed.Format("15:04 MST"))
	service.RunScheduledJob(ctx, deps.Runs, model.BatchEarningsOverseas,
		func(ctx context.Context) (map[string]any, string, error) {
			return runOverseas(ctx, deps)
		})
}

func shouldCatchUp(ctx context.Context, deps *service.WorkerDeps, job model.BatchJobID, elapsed time.Time) bool {
	if deps.Runs != nil {
		pending, err := deps.Runs.HasPending(ctx, string(job))
		if err == nil && pending {
			log.Printf("worker-earnings: catch-up %s skipped (queued/running)", job)
			return false
		}
	}
	last, err := deps.Runs.LastSuccess(ctx, string(job))
	if err != nil {
		log.Printf("worker-earnings: last success %s: %v", job, err)
		return true
	}
	return service.ShouldCatchUp(successAt(last), elapsed)
}

func successAt(run *model.BatchJobRun) time.Time {
	if run == nil {
		return time.Time{}
	}
	if run.FinishedAt != nil {
		return *run.FinishedAt
	}
	return run.StartedAt
}

func runDomestic(ctx context.Context, deps *service.WorkerDeps, months int) (map[string]any, string, error) {
	n, err := deps.Earnings.IngestDomestic(ctx, months)
	if err != nil {
		return nil, "", err
	}
	return map[string]any{"rows": n, "months": months},
		fmt.Sprintf("국내 실적 발표 %d행 적재", n), nil
}

func runOverseas(ctx context.Context, deps *service.WorkerDeps) (map[string]any, string, error) {
	service.BeginOverseasFullIngest()
	defer service.EndOverseasFullIngest()
	n, err := deps.Earnings.IngestOverseas(ctx, 0, 0)
	if err != nil {
		return nil, "", err
	}
	// 컨센서스가 덮어써지기 전에 바뀐 것만 이력으로 남긴다(EPS 리비전 재료). 실패해도
	// 일정 적재는 성공이다 — 하루 빠진 이력은 다음 날의 값이 이어 준다.
	revisions, rerr := deps.Earnings.SnapshotEstimates(ctx)
	if rerr != nil {
		log.Printf("earnings overseas: estimate snapshot: %v", rerr)
	}
	return map[string]any{
			"rows":              n,
			"backDays":          service.OverseasFullBackDays,
			"forwardDays":       service.OverseasFullForwardDays,
			"estimateRevisions": revisions,
		},
		fmt.Sprintf("해외 실적 일정 %d행 적재 (−%d/+%d) · 추정치 변화 %d건", n, service.OverseasFullBackDays, service.OverseasFullForwardDays, revisions), nil
}

// monthsFromParams turns the console's `since` date into a month count.
//
// 관리자 화면의 컨트롤은 「기준일」 하나뿐이므로(BatchJobSpec.AcceptsSince),
// 개월 수를 따로 받을 칸이 없다. 기준일에서 지금까지의 달 수를 세어 쓴다 —
// 「2024-09-01부터」가 곧 「23개월치」다.
func monthsFromParams(params map[string]any) int {
	raw, _ := params["since"].(string)
	if raw == "" {
		return 0
	}
	since, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return 0
	}
	now := time.Now().In(model.KST)
	months := (now.Year()-since.Year())*12 + int(now.Month()) - int(since.Month()) + 1
	if months < 1 {
		return 0
	}
	// FnGuide 이력이 약 2년이라 그 이상을 요청해도 빈 응답만 늘어난다. 상한을
	// 두는 것은 호출 한도를 헛되게 쓰지 않기 위한 것이다.
	if months > 30 {
		months = 30
	}
	return months
}
