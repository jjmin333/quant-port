package service

import (
	"context"
	"log"
	"sync"
	"time"
)

/*
예약 슬롯을 **한 번만** 돌게 하는 두 부품.

# 무엇이 터졌나 (2026-09-11 실측)

원장을 세어 보면 빠른 예약 잡들이 매일 **두 번** 돌고 있었다:

	data_retention    09-09  04:29:59.623 · 04:30:00.002
	data_retention    09-10  04:29:57.488 · 04:30:00.013
	data_retention    09-11  04:29:56.031 · 04:30:00.001
	candle_rollup     09-10  05:29:57.322 · 05:30:00.004
	disclosures_daily 09-10  19:29:56.241 · 19:30:00.015   ← DART 호출이 두 배가 된다

느린 잡(stock_scores·earnings_domestic)만 한 번인데, 그건 고쳐져 있었던 게 아니라
**claim 락이 두 번째를 막았을 뿐**이다 — 락은 동시 실행을 막고 반복은 막지 않는다.

# 원인

루프가 이렇게 생겼다: `time.After(time.Until(다음슬롯))`. 그런데 깨어난 시각이
목표보다 **이르면** 아무도 그것을 확인하지 않았고, 루프는 곧바로 다음 슬롯을 다시
계산했다 — 여전히 같은 04:30:00이므로 4초를 더 자고 한 번 더 돌았다.

이르게 깨는 이유는 벽시계가 뒤로 밀리기 때문이다. `time.After`는 단조 시계로 자고
슬롯은 벽시계로 계산하는데, 그 둘이 어긋나면 이 창이 열린다. 매일 조금씩 더 일찍
깨어난 것(−0.4초 → −2.5초 → −4.0초)이 그 증거다 — 호스트 시계가 빠르게 가고 NTP가
주기적으로 되돌리고 있다(이 호스트에는 09-08 커널 크래시 이력이 있다).

# 고친 방식

두 겹으로 막는다.

  - `WaitForSlot` — 이르게 깨면 **남은 만큼 더 잔다.** 관측된 버그는 이것만으로 사라진다.
  - `SlotGate` — 같은 슬롯은 두 번 돌지 않는다. 잡이 끝난 **뒤에** 시계가 뒤로 밀려
    같은 슬롯이 다시 계산되는 경우를 받는다.

원장 쪽에서 막지 않은 이유: `Begin`은 무조건 INSERT이고 그게 맞다 — 원장은 「무엇이
실행됐나」를 있는 그대로 적어야 한다. 두 번 돌지 않게 하는 것은 부르는 쪽의 일이다.
*/

/*
WaitForSlot — `at`이 될 때까지 잔다. 이르게 깨면 다시 잔다.

돌아온 값이 false면 ctx가 끝난 것이다(호출부는 루프를 나가야 한다).

이미 지난 시각을 주면 곧바로 true다 — 기동 직후 「놓친 슬롯을 지금 돌린다」는
호출부의 판단이고, 이 함수가 대신 정하지 않는다.
*/
func WaitForSlot(ctx context.Context, at time.Time) bool {
	for {
		wait := time.Until(at)
		if wait <= 0 {
			return true
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
			// 벽시계가 뒤로 밀려 이르게 깼을 수 있다 — 루프가 다시 재운다.
			if remaining := time.Until(at); remaining > 0 {
				log.Printf("batch slot: %s 보다 %s 이르게 깨어났다 — 다시 기다린다",
					at.Format(time.RFC3339), remaining)
			}
		}
	}
}

// SlotGate remembers which slot a scheduling loop last ran, so one slot runs
// once even if the clock moves backwards after the run. Safe for concurrent use.
type SlotGate struct {
	mu   sync.Mutex
	last time.Time
}

func NewSlotGate() *SlotGate { return &SlotGate{} }

/*
Allow — 이 슬롯을 지금 돌려도 되나.

같은 슬롯을 두 번째로 물으면 false다. **시각이 같은가**로만 판단한다 — 슬롯은
벽시계로 계산된 값이라 같은 슬롯이면 문자 그대로 같은 순간이다.

뒤로 간 슬롯도 막는다: 마지막으로 돌린 슬롯보다 **이전** 슬롯이 오면 그건 시계가
뒤로 밀린 것이고, 이미 지난 일을 다시 하는 것이다.
*/
func (g *SlotGate) Allow(slot time.Time) bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.last.IsZero() && !slot.After(g.last) {
		return false
	}
	g.last = slot
	return true
}
