package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// fakeQueueStore — queued 행을 잡별 개수로만 들고, 불린 횟수를 센다.
type fakeQueueStore struct {
	mu       sync.Mutex
	queued   map[string]int
	polls    int
	claims   map[string]int
	finished map[string]int
	nextID   int64
}

func newFakeQueueStore() *fakeQueueStore {
	return &fakeQueueStore{queued: map[string]int{}, claims: map[string]int{}, finished: map[string]int{}}
}

func (f *fakeQueueStore) enqueue(job string, n int) {
	f.mu.Lock()
	f.queued[job] += n
	f.mu.Unlock()
}

func (f *fakeQueueStore) QueuedJobIDs(_ context.Context, jobIDs []string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polls++
	var out []string
	for _, id := range jobIDs {
		if f.queued[id] > 0 {
			out = append(out, id)
		}
	}
	return out, nil
}

func (f *fakeQueueStore) ClaimNext(ctx context.Context, jobID string) (int64, map[string]any, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, nil, false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.queued[jobID] == 0 {
		return 0, nil, false, nil
	}
	f.queued[jobID]--
	f.claims[jobID]++
	f.nextID++
	return f.nextID, nil, true, nil
}

func (f *fakeQueueStore) Finish(_ context.Context, _ int64, _, _, _ string) error {
	return nil
}

func (f *fakeQueueStore) snapshot() (polls int, claims map[string]int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := map[string]int{}
	for k, v := range f.claims {
		c[k] = v
	}
	return f.polls, c
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("시간 안에 안 됨: %s", what)
}

// 잡이 여럿이어도 틱마다 확인 쿼리는 한 번이고, 대기 행이 있는 잡만 집는다.
func TestBatchQueueHub_onePollPerTickClaimsOnlyQueuedJobs(t *testing.T) {
	store := newFakeQueueStore()
	hubs := &batchQueueHubs{interval: 5 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ran sync.Map
	handler := func(job string) BatchWorkerHandler {
		return func(context.Context, map[string]any) (map[string]any, string, error) {
			ran.Store(job, true)
			return nil, "ok", nil
		}
	}
	jobs := []model.BatchJobID{"job_a", "job_b", "job_c", "job_d"}
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runBatchQueue(ctx, hubs, store, j, handler(string(j)))
		}()
	}
	store.enqueue("job_b", 1)

	waitFor(t, "job_b 실행", func() bool { _, ok := ran.Load("job_b"); return ok })
	polls, claims := store.snapshot()
	if claims["job_b"] != 1 || len(claims) != 1 {
		t.Fatalf("job_b만 한 번 집혀야 한다: %v", claims)
	}
	// 잡마다 따로 묻던 예전 방식이면 틱당 4번이다. 몇 틱이 지났든 틱당 1번이어야 한다.
	time.Sleep(30 * time.Millisecond)
	polls2, _ := store.snapshot()
	ticks := (polls2 - polls)
	if ticks <= 0 || ticks > 12 { // 30ms / 5ms ≈ 6틱, 여유를 두되 4배(24)는 넘지 않게
		t.Fatalf("30ms 동안 확인 쿼리 %d회 — 틱당 한 번이 아니다", ticks)
	}
	cancel()
	wg.Wait()
}

// 같은 잡은 한 프로세스에서 한 번에 하나만 돈다. 다른 잡은 그동안에도 돈다.
func TestBatchQueueHub_sameJobNeverOverlapsOtherJobsProceed(t *testing.T) {
	store := newFakeQueueStore()
	hubs := &batchQueueHubs{interval: 2 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	release := make(chan struct{})
	var mu sync.Mutex
	inFlight, maxInFlight, slowRuns := 0, 0, 0
	slow := func(context.Context, map[string]any) (map[string]any, string, error) {
		mu.Lock()
		inFlight++
		slowRuns++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()
		<-release
		mu.Lock()
		inFlight--
		mu.Unlock()
		return nil, "", nil
	}
	fastDone := make(chan struct{}, 4)
	fast := func(context.Context, map[string]any) (map[string]any, string, error) {
		fastDone <- struct{}{}
		return nil, "", nil
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); runBatchQueue(ctx, hubs, store, "slow", slow) }()
	go func() { defer wg.Done(); runBatchQueue(ctx, hubs, store, "fast", fast) }()

	store.enqueue("slow", 3)
	waitFor(t, "slow 첫 실행", func() bool { mu.Lock(); defer mu.Unlock(); return slowRuns == 1 })

	store.enqueue("fast", 1)
	select {
	case <-fastDone:
	case <-time.After(2 * time.Second):
		t.Fatal("slow가 도는 동안 fast가 못 돌았다")
	}

	time.Sleep(20 * time.Millisecond) // 여러 틱 — slow를 또 집으면 안 된다
	mu.Lock()
	if slowRuns != 1 {
		mu.Unlock()
		t.Fatalf("도는 중인 잡을 또 집었다: %d", slowRuns)
	}
	mu.Unlock()

	close(release)
	waitFor(t, "남은 slow 두 건", func() bool { mu.Lock(); defer mu.Unlock(); return slowRuns == 3 })
	if maxInFlight != 1 {
		t.Fatalf("같은 잡이 겹쳐 돌았다: 최대 %d", maxInFlight)
	}
	cancel()
	wg.Wait()
}

// ctx가 끝나면 도는 실행이 끝난 뒤에야 돌아온다 — main이 그 뒤에 DB를 닫는다.
func TestBatchQueueHub_returnsOnlyAfterInFlightRunEnds(t *testing.T) {
	store := newFakeQueueStore()
	hubs := &batchQueueHubs{interval: 2 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	var finished bool
	var mu sync.Mutex
	h := func(runCtx context.Context, _ map[string]any) (map[string]any, string, error) {
		close(started)
		<-runCtx.Done()
		time.Sleep(10 * time.Millisecond) // 정리하는 척
		mu.Lock()
		finished = true
		mu.Unlock()
		return nil, "", runCtx.Err()
	}
	returned := make(chan struct{})
	go func() { runBatchQueue(ctx, hubs, store, "job", h); close(returned) }()
	store.enqueue("job", 1)
	<-started
	cancel()
	<-returned
	mu.Lock()
	defer mu.Unlock()
	if !finished {
		t.Fatal("실행이 끝나기 전에 돌아왔다")
	}
}
