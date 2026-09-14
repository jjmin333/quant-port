package service

import (
	"context"
	"log"
	"sort"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
놓친 배치를 뒤늦게 채운다 — 다만 **채워도 되는 것만.**

2026-09-08에 호스트가 22:07에 죽고 스택이 23:44까지 누워 있었다. 그 사이 시간표에
있던 슬롯들이 그냥 지나갔다. 감시 체계(`batch_expectation.go`)는 그것을 `late`로
**판정까지는** 하는데, 판정은 사람이 화면을 열어야 닿는다. 새벽에 죽으면 아침까지
아무도 안 채운다.

# 왜 전부 다시 돌리지 않는가

「늦었으면 다시 돌린다」는 틀렸다. 잡마다 늦게 도는 것의 뜻이 다르다.

  - **멱등하고 싼 잡** — 다시 돌리면 그냥 최신이 된다. 채운다.
  - **로테이션 잡** — 늦게라도 돌면 그 슬라이스가 채워진다. 채운다.
  - **슬롯이 지나면 의미가 없는 잡** — 두 시간 전 슬롯의 뉴스 브리핑을 지금 만들어야
    이미 다음 슬롯이 덮는다. **채우지 않는다.** 돈만 태운다.
  - **돈이 드는 잡** — LLM 요약은 하루 예산이 있고, 창 밖에서 돌면 다음 날 몫을
    먹는다. 이 저장소의 규칙은 「LLM 호출은 배치·관리자 수동만」이다.
    **자동으로 돌리지 않는다.** 화면에 `late`로 남기고 사람이 정한다.
  - **자기 catch-up이 있는 잡** — 종가는 `catchUpMissedSessions`가 기동 때 커버리지를
    보고 스스로 메운다. 여기서 또 큐에 넣으면 같은 일을 두 번 한다.

그래서 이 표는 **허용 목록**이다. 빠진 잡은 자동 재처리하지 않는다 — 새 잡이
조용히 재처리 대상이 되는 것보다, 조용히 대상에서 빠지는 쪽이 안전하다.
*/
var batchCatchUpAllowed = map[model.BatchJobID]string{
	// ── 멱등 · DB만 ────────────────────────────────────────────────────────
	model.BatchStockScores:     "DB만 읽고 쓴다. 하루 낡으면 첫 화면의 AI 랭킹이 낡는다",
	model.BatchDataRetention:   "만료 토큰 정리. 늦게 돌아도 하는 일이 같다",
	model.BatchResearchVerdict: "DB만. 열린 콜의 판정이 밀린다",

	// ── 멱등 · 외부 호출이지만 「그날 것」을 다시 받으면 되는 잡 ──────────
	model.BatchWarnings:    "전 종목 매일. 경고 배지가 낡으면 틀린 정보를 보여준다",
	model.BatchDisclosures: "시장 전체 매일(실측 2페이지). 실적 탭의 최근 공시가 빈다",

	// ── 등급제 로테이션 · 늦게라도 돌면 그 슬라이스가 채워진다 ────────────
	model.BatchInsiderOverseas:    "로테이션 1,200. 건너뛴 슬라이스는 다음 바퀴까지 낡은 채로 남는다",
	model.BatchInsiderDomestic:    "로테이션 1,500. 위와 같다",
	model.BatchFinancialsOverseas: "로테이션 600. 위와 같다",
	model.BatchFinancialsDomestic: "로테이션 400. 위와 같다",

	// ── 실적 일정 ─────────────────────────────────────────────────────────
	// 기동 catch-up이 이미 있지만 그것은 **프로세스가 다시 뜰 때만** 돈다.
	// 워커가 살아 있는데 슬롯만 놓친 경우는 그쪽이 못 잡는다. 호출이 적어 겹쳐도 싸다.
	model.BatchEarningsOverseas: "18호출. 기동 catch-up은 프로세스가 떠야 도는데 그 기동이 안 올 수 있다",
	model.BatchEarningsDomestic: "월당 1호출. 위와 같다",
}

/*
batchCatchUpPerTick — 한 번의 워치독 회차에서 큐에 넣을 최대 개수.

상한을 두는 이유: 열두 시간 죽어 있다 살아나면 여러 잡이 **한꺼번에** late가 된다.
그것을 전부 큐에 밀어 넣으면 워커가 외부 API를 동시에 두들겨 한도에 걸리고, 결국
채우려던 것마저 실패한다. 워치독은 15분마다 도니 남은 것은 다음 회차가 가져간다.
*/
const batchCatchUpPerTick = 2

/*
enqueueCatchUps — late인 잡 중 허용된 것을 큐에 넣고, 넣은 잡 이름을 돌려준다.

`failed`·`stuck`은 **일부러 뺀다.** 그 둘은 「안 돌았다」가 아니라 「돌았는데 깨졌다」
또는 「지금 붙잡혀 있다」이고, 원인을 모른 채 다시 넣으면 같은 실패를 반복하거나
붙잡힌 실행과 겹친다. 그쪽은 사람이 봐야 한다.
*/
// selectCatchUpCandidates — 보고서에서 재처리할 잡을 고른다. DB를 보지 않는
// 순수 함수라 판정 규칙만 따로 시험할 수 있다.
func selectCatchUpCandidates(report model.BatchHealthReport) []model.BatchJobID {
	late := make([]model.BatchJobID, 0, 4)
	for _, job := range report.Jobs {
		if job.Health != model.BatchHealthLate {
			continue
		}
		if _, ok := batchCatchUpAllowed[job.JobID]; !ok {
			continue
		}
		late = append(late, job.JobID)
	}
	// 잡 이름 순으로 고른다. 보고서의 순서는 판정 랭크에 따라 흔들리는데, 상한에
	// 걸려 잘리는 자리에서 그 흔들림은 「이번엔 이게, 다음엔 저게」를 만들어 재현이
	// 안 된다. 굶는 잡은 없다 — 큐에 들어간 것은 ok가 되고 남은 것이 다음 회차의
	// 앞자리를 차지한다.
	sort.Slice(late, func(i, j int) bool { return late[i] < late[j] })
	if len(late) > batchCatchUpPerTick {
		late = late[:batchCatchUpPerTick]
	}
	return late
}

func (s *BatchHealthService) enqueueCatchUps(ctx context.Context, report model.BatchHealthReport) []string {
	if s == nil || s.runs == nil {
		return nil
	}

	queued := make([]string, 0, batchCatchUpPerTick)
	for _, id := range selectCatchUpCandidates(report) {
		// 이미 큐에 있거나 도는 중이면 넣지 않는다. 워치독은 15분마다 도는데
		// 두 시간짜리 잡이 아직 돌고 있을 수 있다.
		busy, err := s.runs.HasPending(ctx, string(id))
		if err != nil {
			log.Printf("batch catch-up: %s 대기 여부 확인 실패: %v", id, err)
			continue
		}
		if busy {
			continue
		}
		if _, err := s.runs.EnqueueAs(ctx, string(id), repository.TriggerCatchUp, nil); err != nil {
			log.Printf("batch catch-up: %s 큐 등록 실패: %v", id, err)
			continue
		}
		log.Printf("batch catch-up: %s 를 큐에 넣었다 — %s", id, batchCatchUpAllowed[id])
		queued = append(queued, string(id))
	}
	return queued
}
