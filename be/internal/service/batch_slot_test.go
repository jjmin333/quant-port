package service

import (
	"context"
	"testing"
	"time"
)

/*
예약 잡이 하루 두 번 돌던 버그의 회귀선 (2026-09-11 실측).

원장에서 본 것: data_retention·candle_rollup·disclosures_daily 가 매일 두 번 돌았고
(04:29:56 + 04:30:00 꼴), 이른 쪽이 날마다 더 일찍 깨어났다(−0.4초 → −4.0초).
disclosures_daily 는 DART 잡이라 그 두 번이 곧 DART 호출 두 배였다.
*/

func TestSlotGate_runsOneSlotOnce(t *testing.T) {
	gate := NewSlotGate()
	slot := mustTime(t, "2026-09-11T04:30:00Z")

	if !gate.Allow(slot) {
		t.Fatal("첫 슬롯은 통과해야 한다")
	}
	if gate.Allow(slot) {
		t.Fatal("같은 슬롯이 두 번 통과했다 — 이것이 하루 두 번 돌던 버그다")
	}
}

func TestSlotGate_allowsTheNextSlot(t *testing.T) {
	gate := NewSlotGate()
	today := mustTime(t, "2026-09-11T04:30:00Z")
	tomorrow := mustTime(t, "2026-09-12T04:30:00Z")

	gate.Allow(today)
	if !gate.Allow(tomorrow) {
		t.Fatal("다음 슬롯이 막혔다 — 잡이 하루 만에 멈춘다")
	}
}

func TestSlotGate_rejectsSlotsThatWentBackwards(t *testing.T) {
	// 시계가 뒤로 밀려 이미 지난 슬롯이 다시 계산되는 경우.
	gate := NewSlotGate()
	gate.Allow(mustTime(t, "2026-09-11T04:30:00Z"))
	if gate.Allow(mustTime(t, "2026-09-10T04:30:00Z")) {
		t.Fatal("과거 슬롯이 통과했다 — 지난 일을 다시 한다")
	}
}

func TestSlotGate_nilIsPermissive(t *testing.T) {
	// 배선을 잊었을 때 잡이 아예 안 도는 쪽으로 실패하면 안 된다.
	var gate *SlotGate
	if !gate.Allow(time.Now()) {
		t.Fatal("nil gate 가 슬롯을 막았다")
	}
}

func TestWaitForSlot_returnsImmediatelyForPastSlots(t *testing.T) {
	// 기동 직후 「놓친 슬롯을 지금」은 호출부의 판단이다.
	start := time.Now()
	if !WaitForSlot(context.Background(), time.Now().Add(-time.Hour)) {
		t.Fatal("지난 슬롯에서 false 가 나왔다")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("지난 슬롯인데 기다렸다")
	}
}

func TestWaitForSlot_waitsUntilTheSlot(t *testing.T) {
	at := time.Now().Add(120 * time.Millisecond)
	if !WaitForSlot(context.Background(), at) {
		t.Fatal("false 가 나왔다")
	}
	// **핵심**: 목표보다 이르게 돌아오지 않는다. 이르게 돌아오던 것이 버그였다.
	if time.Now().Before(at) {
		t.Fatalf("슬롯(%s)보다 이르게 돌아왔다: %s", at, time.Now())
	}
}

func TestWaitForSlot_stopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if WaitForSlot(ctx, time.Now().Add(time.Hour)) {
		t.Fatal("ctx가 끝났는데 true 를 줬다 — 루프가 종료하지 않는다")
	}
}
