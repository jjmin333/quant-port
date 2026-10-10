package service

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"sync"
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

// batchQueueStore is the slice of BatchJobRunRepo the queue poller needs.
type batchQueueStore interface {
	QueuedJobIDs(ctx context.Context, jobIDs []string) ([]string, error)
	ClaimNext(ctx context.Context, jobID string) (id int64, params map[string]any, ok bool, err error)
	Finish(ctx context.Context, id int64, status, message, result string) error
}

/*
RunBatchQueueWorker registers one job's queue with the process-wide poller and
blocks until ctx ends. Claimed work runs through `handle` and its outcome is
recorded. Meant to run alongside a worker's own schedule-driven loop (if it has
one) inside the same process — this is what lets an admin's manual "지금 재계산"
click reach a job that otherwise only fires on a market-close trigger.

# 잡마다 폴러를 두지 않는 이유 (2026-09-26 실측)

예전에는 잡마다 4초 틱 폴러가 따로 돌며 틱마다 `BEGIN → SELECT … FOR UPDATE SKIP
LOCKED → ROLLBACK`을 냈다. 폴러가 프로세스 9개에 31개라 **아무 일이 없어도 초당
트랜잭션 7.8개** — 유휴 DB 문장의 절반 이상이었다. 게다가 한 프로세스의 폴러들이
같은 순간에 깨어나 동시에 커넥션을 잡는데 워커 풀의 유휴 상한이 2라, 넘친 커넥션을
매 틱 닫고 다시 열었다(유휴인데 **초당 새 연결 1.27개**, 연결마다 인증 + SET 두 번).

이제 프로세스당 폴러 하나가 틱마다 `QueuedJobIDs` 한 번(잠금 없음)으로 등록된 잡
전부를 확인하고, 대기 행이 있는 잡만 예전과 같은 `ClaimNext`로 집는다. 지켜야 할
성질은 그대로다: 같은 잡은 한 프로세스에서 한 번에 하나만 돌고, 다른 잡끼리는
동시에 돈다. 반응 시간도 그대로 한 틱(4초) 안이다.
*/
func RunBatchQueueWorker(ctx context.Context, runs *repository.BatchJobRunRepo, jobID model.BatchJobID, handle BatchWorkerHandler) {
	if runs == nil {
		return
	}
	runBatchQueue(ctx, defaultBatchQueueHubs, runs, jobID, handle)
}

func runBatchQueue(ctx context.Context, hubs *batchQueueHubs, store batchQueueStore, jobID model.BatchJobID, handle BatchWorkerHandler) {
	hub := hubs.get(ctx, store)
	hub.register(jobID, handle)
	<-ctx.Done()
	// 예전 폴러는 도는 실행이 끝나야 돌아왔다 — main이 이 호출 뒤에 DB를 닫으므로
	// 그 성질을 지킨다(실행은 ctx에서 파생돼 곧 끝나고, Finish는 자기 ctx로 기록한다).
	hub.unregister(jobID)
	hub.waitIdle(jobID)
}

// batchQueueHubs holds one poller per store — in practice one per process,
// since every caller passes the process's single BatchJobRunRepo.
type batchQueueHubs struct {
	mu       sync.Mutex
	byStore  map[batchQueueStore]*batchQueueHub
	interval time.Duration
}

var defaultBatchQueueHubs = &batchQueueHubs{interval: batchQueuePollInterval}

/*
get returns the store's poller, starting it on first use with the caller's ctx.

모든 호출자가 같은 프로세스 ctx를 넘긴다(각 main의 signal ctx, API는 배경 ctx).
그래서 처음 부른 쪽의 ctx가 폴러의 수명이 된다. 그 ctx가 끝나면 폴러를 지워
다음 호출이 새로 시작하게 한다 — 시험처럼 ctx를 여러 번 만드는 경우를 위해서다.
*/
func (h *batchQueueHubs) get(ctx context.Context, store batchQueueStore) *batchQueueHub {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.byStore == nil {
		h.byStore = map[batchQueueStore]*batchQueueHub{}
	}
	if hub, ok := h.byStore[store]; ok {
		return hub
	}
	hub := &batchQueueHub{
		store:    store,
		handlers: map[model.BatchJobID]BatchWorkerHandler{},
		busy:     map[model.BatchJobID]chan struct{}{},
	}
	h.byStore[store] = hub
	go func() {
		hub.run(ctx, h.interval)
		h.mu.Lock()
		if h.byStore[store] == hub {
			delete(h.byStore, store)
		}
		h.mu.Unlock()
	}()
	return hub
}

type batchQueueHub struct {
	store batchQueueStore

	mu       sync.Mutex
	handlers map[model.BatchJobID]BatchWorkerHandler
	busy     map[model.BatchJobID]chan struct{} // 지금 도는 잡 → 끝나면 닫힌다. 같은 잡을 겹쳐 집지 않는다
}

func (b *batchQueueHub) register(jobID model.BatchJobID, handle BatchWorkerHandler) {
	b.mu.Lock()
	b.handlers[jobID] = handle
	b.mu.Unlock()
}

func (b *batchQueueHub) unregister(jobID model.BatchJobID) {
	b.mu.Lock()
	delete(b.handlers, jobID)
	b.mu.Unlock()
}

// waitIdle blocks until jobID has no run in flight in this process.
func (b *batchQueueHub) waitIdle(jobID model.BatchJobID) {
	b.mu.Lock()
	done := b.busy[jobID]
	b.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (b *batchQueueHub) run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			safego.Call("batch-queue-poll", func() { b.tick(ctx) })
		}
	}
}

// tick asks once which idle jobs have queued rows, then claims each in its own
// goroutine so a long run never delays another job's start.
func (b *batchQueueHub) tick(ctx context.Context) {
	b.mu.Lock()
	idle := make([]string, 0, len(b.handlers))
	for id := range b.handlers {
		if b.busy[id] == nil {
			idle = append(idle, string(id))
		}
	}
	b.mu.Unlock()
	if len(idle) == 0 {
		return
	}
	sort.Strings(idle) // 같은 문장 모양 — 로그·시험이 읽기 쉽다

	queued, err := b.store.QueuedJobIDs(ctx, idle)
	if err != nil {
		log.Printf("batch queue poll: %v", err)
		return
	}
	for _, raw := range queued {
		jobID := model.BatchJobID(raw)
		b.mu.Lock()
		handle, ok := b.handlers[jobID]
		if !ok || b.busy[jobID] != nil {
			b.mu.Unlock()
			continue
		}
		done := make(chan struct{})
		b.busy[jobID] = done
		b.mu.Unlock()

		go func() {
			defer func() {
				b.mu.Lock()
				delete(b.busy, jobID)
				b.mu.Unlock()
				close(done)
			}()
			safego.Call("batch-worker-"+string(jobID), func() {
				runOneQueuedJob(ctx, b.store, jobID, handle)
			})
		}()
	}
}

func runOneQueuedJob(ctx context.Context, runs batchQueueStore, jobID model.BatchJobID, handle BatchWorkerHandler) {
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

/*
RunScheduledJobResult records a scheduled run that has **already finished**.

`RunScheduledJob`은 실행을 감싸므로 Begin이 실행 앞에 온다. 그런데 어떤 예약 실행은
「돌았다」는 사실이 실행 **뒤에야** 정해진다 — 뉴스 브리핑 슬롯이 그렇다: 슬롯이 지나야
그 슬롯을 기록할 수 있고, 그 전에는 이번 틱이 슬롯 경계인지도 모른다. 그럴 때 한 줄을
사후에 남긴다.

시작 시각이 실제보다 늦게 찍히는 것이 유일한 손해인데, 초 단위라 감시에는 영향이 없다.
*/
func RunScheduledJobResult(
	ctx context.Context,
	runs *repository.BatchJobRunRepo,
	jobID model.BatchJobID,
	result map[string]any,
	message string,
	runErr error,
) {
	if runs == nil {
		return
	}
	id, err := runs.Begin(ctx, string(jobID), "schedule")
	if err != nil || id == 0 {
		if err != nil {
			log.Printf("batch %s: ledger begin: %v", jobID, err)
		}
		return
	}
	status := "done"
	if runErr != nil {
		status = "error"
		message = runErr.Error()
	}
	payload, _ := json.Marshal(result)
	if e := runs.Finish(context.Background(), id, status, message, string(payload)); e != nil {
		log.Printf("batch %s: record finish: %v", jobID, e)
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
		// 같은 잡이 이미 돌거나(수동 일괄 적재가 예약 시각을 넘김) 큐에 있으면 이번 예약은 건너뛴다 —
		// 겹쳐 돌면 외부 호출을 두 번 쓴다(2026-10-02, re_trades 전국 첫 적재가 03:30 예약과 겹침).
		if busy, err := runs.HasPending(ctx, string(jobID)); err == nil && busy {
			log.Printf("batch %s: already queued/running — scheduled run skipped", jobID)
			return
		}
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

/*
CloseInterruptedRuns — 워커 기동 직후 한 번. 이 워커가 맡은 잡의 running 행 중 기동
전에 시작된 것을 닫는다(repository.MarkInterruptedRuns 헤더). 원장이 없거나 실패해도
기동은 계속한다 — 최악이 예전 동작(6시간 뒤 API가 닫음)이다.
*/
func CloseInterruptedRuns(ctx context.Context, runs *repository.BatchJobRunRepo, jobs ...model.BatchJobID) {
	if runs == nil || len(jobs) == 0 {
		return
	}
	ids := make([]string, 0, len(jobs))
	for _, j := range jobs {
		ids = append(ids, string(j))
	}
	n, err := runs.MarkInterruptedRuns(ctx, ids, time.Now())
	if err != nil {
		log.Printf("batch: close interrupted runs: %v", err)
		return
	}
	if n > 0 {
		log.Printf("batch: 재기동 전 끊긴 실행 %d건을 닫았다(interrupted) — 만회가 다시 큐에 넣는다", n)
	}
}
