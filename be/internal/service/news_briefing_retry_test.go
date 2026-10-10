package service

import (
	"errors"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
슬롯이 AI 문장 없이 끝나면 **같은 슬롯 안에서 5분마다** 다시 묻는다.

예전에는 장 밖 한 시간 슬롯에서 503 한 번이 곧 한 시간짜리 규칙·승계 문장이었다.
09-24 밤에는 22:00~00:00 슬롯이 모두 503이었고, 재시도는 4초 뒤 한 번뿐이었다.
503은 우리 예산에 잡히지 않아 자주 물어도 되지만, Google이 세는지는 문서에 없어
하루 상한을 둔다.
*/
func TestBriefingRunStateRetriesInsideTheSlot(t *testing.T) {
	st := newBriefingRunState()
	unavailable := briefingRun{narrativeErr: errors.New("llm api 503: high demand")}

	// 2026-09-01(화) 22:00 KST — 국내 장 밖, 한 시간 슬롯.
	t0 := seoulAt(t, "2026-09-01 22:00")
	if a, _ := st.due("domestic", t0); a != briefingSlotRun {
		t.Fatalf("first tick = %v, want slot run", a)
	}
	st.settle("domestic", t0, unavailable)

	if a, _ := st.due("domestic", t0.Add(4*time.Minute)); a != briefingIdle {
		t.Fatalf("4분 뒤 = %v, want idle", a)
	}
	if a, _ := st.due("domestic", t0.Add(5*time.Minute)); a != briefingRetry {
		t.Fatalf("5분 뒤 = %v, want retry", a)
	}
	// 재시도도 실패하면 다시 5분 뒤.
	st.settle("domestic", t0.Add(5*time.Minute), unavailable)
	if a, _ := st.due("domestic", t0.Add(9*time.Minute)); a != briefingIdle {
		t.Fatalf("9분 뒤 = %v, want idle", a)
	}
	if a, _ := st.due("domestic", t0.Add(10*time.Minute)); a != briefingRetry {
		t.Fatalf("10분 뒤 = %v, want retry", a)
	}
	// 재시도가 성공하면 슬롯이 끝날 때까지 더 묻지 않는다.
	st.settle("domestic", t0.Add(10*time.Minute), briefingRun{generated: true})
	if a, _ := st.due("domestic", t0.Add(50*time.Minute)); a != briefingIdle {
		t.Fatalf("성공 뒤 = %v, want idle", a)
	}
	// 다음 슬롯은 평소대로.
	if a, _ := st.due("domestic", t0.Add(time.Hour)); a != briefingSlotRun {
		t.Fatalf("다음 슬롯 = %v, want slot run", a)
	}
}

// 새 슬롯이 오면 지난 슬롯의 재시도 예약은 버린다 — 새 슬롯의 첫 실행이 곧 재시도다.
func TestBriefingRunStateNewSlotClearsPendingRetry(t *testing.T) {
	st := newBriefingRunState()
	t0 := seoulAt(t, "2026-09-01 22:58")
	st.due("domestic", t0)
	st.settle("domestic", t0, briefingRun{narrativeErr: errors.New("llm api 503")})

	if a, _ := st.due("domestic", seoulAt(t, "2026-09-01 23:00")); a != briefingSlotRun {
		t.Fatalf("23:00 = %v, want slot run", a)
	}
	if _, pending := st.retryAt["domestic"]; pending {
		t.Fatal("new slot must clear the old slot's retry")
	}
}

// 하루 상한을 다 쓰면 그 브리핑 날은 더 묻지 않는다. 날은 **서울 자정이 아니라
// 개장 준비 시각(국내 08:00)**에 바뀐다 — 자정은 503 구간 한가운데라 거기서 새로
// 세면 한 밤에 상한을 두 번 쓴다. 시장마다 따로 센다.
func TestBriefingRunStateDailyRetryCap(t *testing.T) {
	st := newBriefingRunState()
	unavailable := briefingRun{narrativeErr: errors.New("llm api 503")}

	// 22:00부터 1분마다 틱 — 슬롯 첫 실행(매시 정각)과 재시도가 섞여도 재시도만 센다.
	now := seoulAt(t, "2026-09-01 22:00")
	retries := 0
	for ; now.Before(seoulAt(t, "2026-09-02 07:59")); now = now.Add(time.Minute) {
		action, _ := st.due("domestic", now)
		if action == briefingIdle {
			continue
		}
		if action == briefingRetry {
			retries++
		}
		st.settle("domestic", now, unavailable)
	}
	if retries != briefingRetryDailyCap {
		t.Fatalf("retries across the night = %d, want the cap %d (midnight must not reset it)",
			retries, briefingRetryDailyCap)
	}
	if got := st.retriesToday("us", now); got != 0 {
		t.Fatalf("us retries = %d, want 0 (counted per market)", got)
	}

	// 개장 준비(08:00)부터는 새 브리핑 날 — 다시 센다.
	next := seoulAt(t, "2026-09-02 08:00")
	st.due("domestic", next)
	st.settle("domestic", next, unavailable)
	if _, pending := st.retryAt["domestic"]; !pending {
		t.Fatal("the next briefing day must reset the cap")
	}
}

// 한도 초과는 다시 묻지 않는다 — 같은 대답이고, 통과하면 쓰지 않기로 한 예산이다.
// 문장을 얻은 실행이나 LLM을 아예 부르지 않은 실행(꺼짐 · 조용한 슬롯)도 마찬가지.
func TestBriefingRunNeedsRetry(t *testing.T) {
	cases := []struct {
		name string
		run  briefingRun
		want bool
	}{
		{"503", briefingRun{narrativeErr: errors.New("llm api 503")}, true},
		{"빈 summary", briefingRun{narrativeErr: errBriefingEmptySummary}, true},
		{"한도 초과", briefingRun{narrativeErr: apibudget.Exhaustedf("llm api 429: quota")}, false},
		{"문장 얻음", briefingRun{generated: true}, false},
		{"LLM 안 부름", briefingRun{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.run.needsRetry(); got != tc.want {
				t.Fatalf("needsRetry = %v, want %v", got, tc.want)
			}
		})
	}
}
