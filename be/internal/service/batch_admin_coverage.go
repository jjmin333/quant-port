package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
배치 화면의 「커버리지」는 **기다려 주지 않는다** (2026-09-27 사용자 제보 1).

커버리지는 잡마다 큰 표를 세는 질의(일봉 수백만 행 · 공시 · 실적 · 동기화 상태)라
화면 한 번에 열 개 남짓이 돌았고, 응답이 20초였다. 그런데 화면은 잡이 도는 동안
4초마다 같은 응답을 다시 불렀다 — 앞 요청이 끝나기도 전에 다음 요청이 쌓여 DB가
더 느려지고, 관리자에게는 「화면이 안 뜬다」로 보였다.

커버리지는 배치가 끝나야 바뀌는 값이라 몇 분 묵어도 판단이 달라지지 않는다. 그래서
  - 목록 응답은 **들고 있는 커버리지를 그대로** 싣는다(몇 분 묵어도).
  - 묵었으면 **뒤에서 한 번만** 다시 센다(동시에 둘 돌지 않는다).
  - 아직 한 번도 못 셌으면 빈 채로 보내고 `pending`을 세운다 — 화면이 잠시 뒤 다시 묻는다.
잡의 상태·최근 실행·판정은 여전히 매 요청 새로 읽는다. 늦으면 안 되는 쪽은 그쪽이다.
*/

// batchCoverageTTL — 이보다 오래된 커버리지는 다음 요청 때 뒤에서 다시 센다.
const batchCoverageTTL = 3 * time.Minute

// batchCoverageTimeout — 뒤에서 세는 한 회차의 상한. 요청 컨텍스트와 무관하게 돈다.
const batchCoverageTimeout = 2 * time.Minute

type batchCoverageCache struct {
	mu         sync.Mutex
	data       map[model.BatchJobID]map[string]any
	at         time.Time
	refreshing bool
}

// coverageCacheAction decides what a read does with the cache — pure so the
// three cases (cold · fresh · stale) are pinned by tests.
//
//   - pending: nothing to serve yet (cold); the caller should say so.
//   - refresh: start a background recount now.
func coverageCacheAction(now, at time.Time, has, refreshing bool) (pending, refresh bool) {
	if !has {
		return true, !refreshing
	}
	if now.Sub(at) >= batchCoverageTTL {
		return false, !refreshing
	}
	return false, false
}

// CoverageSnapshot returns the cached coverage per job without waiting on the
// database, kicking off a single background recount when it is cold or stale.
func (s *BatchAdminService) CoverageSnapshot() (map[model.BatchJobID]map[string]any, bool) {
	c := &s.coverage
	c.mu.Lock()
	pending, refresh := coverageCacheAction(time.Now(), c.at, c.data != nil, c.refreshing)
	data := c.data
	if refresh {
		c.refreshing = true
	}
	c.mu.Unlock()

	if refresh {
		go s.refreshCoverage()
	}
	return data, pending
}

func (s *BatchAdminService) refreshCoverage() {
	ctx, cancel := context.WithTimeout(context.Background(), batchCoverageTimeout)
	defer cancel()
	started := time.Now()

	out := make(map[model.BatchJobID]map[string]any, len(batchJobCatalog))
	// 같은 질문을 한 회차에 두 번 묻지 않는다 — 「전 종목 일봉」은 잡 세 개가 같은 값을 쓴다.
	memo := map[string]map[string]any{}
	for _, spec := range batchJobCatalog {
		if cov := s.coverageForMemo(ctx, spec.ID, memo); cov != nil {
			out[spec.ID] = cov
		}
	}

	c := &s.coverage
	c.mu.Lock()
	c.refreshing = false
	// 실패로 텅 빈 회차가 멀쩡한 이전 값을 지우지 않게 한다.
	if len(out) > 0 || c.data == nil {
		c.data = out
		c.at = time.Now()
	}
	c.mu.Unlock()

	if d := time.Since(started); d > 5*time.Second {
		log.Printf("batch admin: coverage recount took %s", d.Round(time.Millisecond))
	}
}

// coverageForMemo dedupes jobs that ask the identical coverage question.
func (s *BatchAdminService) coverageForMemo(ctx context.Context, id model.BatchJobID, memo map[string]map[string]any) map[string]any {
	key := coverageMemoKey(id)
	if key != "" {
		if v, ok := memo[key]; ok {
			return v
		}
	}
	v := s.coverageFor(ctx, id)
	if key != "" {
		memo[key] = v
	}
	return v
}

// coverageMemoKey names the shared question for jobs that ask the same one.
func coverageMemoKey(id model.BatchJobID) string {
	switch id {
	case model.BatchDailyCloseFetchStale, model.BatchDailyCloseFetchMissing, model.BatchDailyCloseFetchHistory:
		return "daily_close_all"
	}
	return ""
}
