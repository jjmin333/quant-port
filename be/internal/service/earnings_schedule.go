package service

import (
	"sync/atomic"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// 국내 실적 시즌 — 잠정실적이 몰리는 달. 1–2 결산, 4–5 1Q, 7–8 2Q, 10–11 3Q.
// 이 달에만 당월을 2시간마다 다시 받는다. FnGuide가 이미 지난 날짜에 개별·발표치를
// 나중에 붙이므로, 시즌이 아닌 달에 같은 페이스를 유지할 이유가 없다.
func IsDomesticEarningsSeason(month time.Month) bool {
	switch month {
	case time.January, time.February,
		time.April, time.May,
		time.July, time.August,
		time.October, time.November:
		return true
	default:
		return false
	}
}

type clockHM struct{ hour, min int }

// 시즌: 10:00부터 2시간 간격으로 20:00까지. 비시즌: 18:30 한 번.
//
// 20:00를 마지막에 둔 것은 창이 10–21시이기 때문이다 — 22시는 창 밖이고,
// 21시는 2시간 격자가 아니다. 전월(정정)은 이 마지막 슬롯에서만 받는다.
func domesticClocks(month time.Month) []clockHM {
	if IsDomesticEarningsSeason(month) {
		return []clockHM{
			{10, 0}, {12, 0}, {14, 0}, {16, 0}, {18, 0}, {20, 0},
		}
	}
	return []clockHM{{18, 30}}
}

func clockOn(day time.Time, hour, min int) time.Time {
	local := day.In(model.KST)
	return time.Date(local.Year(), local.Month(), local.Day(), hour, min, 0, 0, model.KST)
}

// DomesticSlotsOn is every ingest moment on that KST calendar day.
func DomesticSlotsOn(day time.Time) []time.Time {
	clocks := domesticClocks(day.In(model.KST).Month())
	out := make([]time.Time, len(clocks))
	for i, c := range clocks {
		out[i] = clockOn(day, c.hour, c.min)
	}
	return out
}

// NextDomesticSlot is the next ingest moment strictly after now.
func NextDomesticSlot(now time.Time) time.Time {
	local := now.In(model.KST)
	for d := 0; d <= 2; d++ {
		for _, slot := range DomesticSlotsOn(local.AddDate(0, 0, d)) {
			if slot.After(local) {
				return slot
			}
		}
	}
	return clockOn(local.AddDate(0, 0, 1), 18, 30)
}

// LastElapsedDomesticSlot is the most recent ingest moment that is not after now.
//
// 워커가 재시작되면 next는 미래만 보므로, 이미 지난 슬롯은 이 값으로 메운다.
func LastElapsedDomesticSlot(now time.Time) time.Time {
	local := now.In(model.KST)
	for d := 0; d <= 3; d++ {
		slots := DomesticSlotsOn(local.AddDate(0, 0, -d))
		for i := len(slots) - 1; i >= 0; i-- {
			if !slots[i].After(local) {
				return slots[i]
			}
		}
	}
	return clockOn(local.AddDate(0, 0, -1), 18, 30)
}

// DomesticMonthsForSlot is 1 for mid-day season polls (당월만) and 2 for the
// day's last slot (정정용 전월). Catch-up always uses 2 — 놓친 것이 마지막
// 슬롯이었는지 알 수 없어서다.
func DomesticMonthsForSlot(slot time.Time) int {
	slots := DomesticSlotsOn(slot)
	last := slots[len(slots)-1]
	if slot.Hour() == last.Hour() && slot.Minute() == last.Minute() {
		return 2
	}
	return 1
}

const (
	overseasHourKST = 6
	overseasMinKST  = 0

	// 06:00 전체 창. 7일 조각 18회를 유지하려고 앞을 49일(7주)로 맞추고, 뒤를
	// 77이 아니라 76으로 둔다 — 양끝 포함 127일이면 조각이 19가 된다.
	OverseasFullBackDays    = 49
	OverseasFullForwardDays = 76

	overseasLiveInterval     = 5 * time.Minute
	overseasFullSkipUntilMin = 10 // 06:00–06:09 틱은 전체 적재와 타이머가 겹친다
)

// overseasFullInFlight is set by the worker around the 06:00 / catch-up /
// 수동 전체 창 실행. 5분 당일 폴링이 그 사이에 같은 키로 나가지 않게 한다.
var overseasFullInFlight atomic.Bool

func BeginOverseasFullIngest() { overseasFullInFlight.Store(true) }
func EndOverseasFullIngest()   { overseasFullInFlight.Store(false) }
func OverseasFullInFlight() bool {
	return overseasFullInFlight.Load()
}

// NextOverseasSlot is the next 06:00 KST strictly after now.
func NextOverseasSlot(now time.Time) time.Time {
	local := now.In(model.KST)
	slot := clockOn(local, overseasHourKST, overseasMinKST)
	if !slot.After(local) {
		slot = clockOn(local.AddDate(0, 0, 1), overseasHourKST, overseasMinKST)
	}
	return slot
}

// LastElapsedOverseasSlot is today's 06:00 if it has already passed, else yesterday's.
func LastElapsedOverseasSlot(now time.Time) time.Time {
	local := now.In(model.KST)
	slot := clockOn(local, overseasHourKST, overseasMinKST)
	if slot.After(local) {
		return clockOn(local.AddDate(0, 0, -1), overseasHourKST, overseasMinKST)
	}
	return slot
}

// NextOverseasLiveSlot is the next KST clock mark on the 5-minute grid,
// strictly after now. 00:00부터 맞춰 실적치가 나온 직후 틱에서 받는다.
func NextOverseasLiveSlot(now time.Time) time.Time {
	local := now.In(model.KST)
	min := (local.Minute() / 5) * 5
	slot := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), min, 0, 0, model.KST)
	if !slot.After(local) {
		slot = slot.Add(overseasLiveInterval)
	}
	return slot
}

// OverseasLiveWindow is KST yesterday..today.
//
// 장후(amc) 실적치는 Finnhub 날짜가 미국 세션일이다. 05시 KST는 아직 미국
// "어제"라, 오늘만 받으면 막 나온 장후가 빠진다. 이틀은 7일 조각 1회다.
//
// 이 창 밖의 「예정」 잔류(8/19 이전 등)를 여기서 지우지 않는다. 라이브 잡은
// 실적치를 따라가는 용도이고, 지난 예정을 화면에서 빼는 규칙은 조회
// DropStaleOverseasScheduled(KST 어제 미만)다. DB를 지우면 나중에 들어오는
// 확정을 같은 키로 갱신할 행이 없어진다.
func OverseasLiveWindow(now time.Time) (from, to time.Time) {
	kst := now.In(model.KST)
	today := time.Date(kst.Year(), kst.Month(), kst.Day(), 0, 0, 0, 0, model.KST)
	return today.AddDate(0, 0, -1), today
}

// ShouldSkipOverseasLive is true while the 06:00 full ingest is running or
// about to start. 5분 폴링이 전체 창 18조각과 같은 키를 동시에 쓰지 않는다.
func ShouldSkipOverseasLive(now time.Time, fullInFlight bool) bool {
	if fullInFlight {
		return true
	}
	local := now.In(model.KST)
	return local.Hour() == overseasHourKST && local.Minute() < overseasFullSkipUntilMin
}

// ShouldCatchUp reports whether a restart should re-run the last elapsed slot.
//
// lastSuccess가 zero면 한 번도 성공한 적이 없는 것이고, 그때는 반드시 받는다.
func ShouldCatchUp(lastSuccess, lastElapsed time.Time) bool {
	if lastElapsed.IsZero() {
		return false
	}
	if lastSuccess.IsZero() {
		return true
	}
	return lastSuccess.Before(lastElapsed)
}
