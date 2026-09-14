package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

func krDay(date string, preStart, preEnd, regStart, regEnd, afterStart, afterEnd string) toss.KrMarketDay {
	return toss.KrMarketDay{
		Date: date,
		Integrated: &toss.IntegratedHour{
			PreMarket:     &toss.PreMarketSession{StartTime: preStart, EndTime: preEnd},
			RegularMarket: &toss.RegularMarketSession{StartTime: regStart, EndTime: regEnd},
			AfterMarket:   &toss.AfterMarketSession{StartTime: afterStart, EndTime: afterEnd},
		},
	}
}

// 사용자 지적의 실제 상황: KST 09-04 00:40, 국내는 전날 20:00에 완전히 닫혔고
// 다음 창(프리마켓)은 그날 08:00에 열린다.
func krCalendarAroundMidnight() marketCalendars {
	return marketCalendars{
		kr: &toss.KrMarketCalendarResponse{
			PreviousBusinessDay: krDay("2026-09-03",
				"2026-09-03T08:00:00+09:00", "2026-09-03T08:50:00+09:00",
				"2026-09-03T09:00:00+09:00", "2026-09-03T15:30:00+09:00",
				"2026-09-03T15:40:00+09:00", "2026-09-03T20:00:00+09:00"),
			Today: krDay("2026-09-04",
				"2026-09-04T08:00:00+09:00", "2026-09-04T08:50:00+09:00",
				"2026-09-04T09:00:00+09:00", "2026-09-04T15:30:00+09:00",
				"2026-09-04T15:40:00+09:00", "2026-09-04T20:00:00+09:00"),
		},
	}
}

func TestMarketClockAt_closedOvernightPointsAtLastCloseAndNextOpen(t *testing.T) {
	cals := krCalendarAroundMidnight()
	now := time.Date(2026, 9, 4, 0, 40, 0, 0, kst)

	clock := marketClockAt("domestic", cals, now)
	if !clock.Closed() {
		t.Fatalf("session = %q, want closed", clock.Session)
	}
	closedAt, err := time.Parse(time.RFC3339, clock.ClosedAt)
	if err != nil {
		t.Fatalf("closedAt %q: %v", clock.ClosedAt, err)
	}
	// 사용자가 화면에 적히길 원한 값: 전날 오후 8시.
	if want := time.Date(2026, 9, 3, 20, 0, 0, 0, kst); !closedAt.Equal(want) {
		t.Fatalf("closedAt = %s, want %s", closedAt.In(kst), want)
	}
	nextOpen, err := time.Parse(time.RFC3339, clock.NextOpenAt)
	if err != nil {
		t.Fatalf("nextSessionAt %q: %v", clock.NextOpenAt, err)
	}
	// 정규장 09:00이 아니라 **프리마켓 08:00**이다 — 그때부터 값이 움직인다.
	if want := time.Date(2026, 9, 4, 8, 0, 0, 0, kst); !nextOpen.Equal(want) {
		t.Fatalf("nextSessionAt = %s, want %s", nextOpen.In(kst), want)
	}
}

func TestMarketClockAt_openLeavesBothInstantsEmpty(t *testing.T) {
	cals := krCalendarAroundMidnight()
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, kst)

	clock := marketClockAt("domestic", cals, now)
	if clock.Session != "regular" {
		t.Fatalf("session = %q, want regular", clock.Session)
	}
	if clock.ClosedAt != "" || clock.NextOpenAt != "" {
		t.Fatalf("open market carried instants: closedAt=%q nextSessionAt=%q",
			clock.ClosedAt, clock.NextOpenAt)
	}
}

func TestMarketClockAt_preMarketCountsAsOpen(t *testing.T) {
	cals := krCalendarAroundMidnight()
	now := time.Date(2026, 9, 4, 8, 10, 0, 0, kst)

	clock := marketClockAt("domestic", cals, now)
	if clock.Closed() {
		t.Fatalf("session = %q at 08:10, want a live pre-market session", clock.Session)
	}
}

// 정규장이 끝나고 시간외가 시작되기 전의 10분(15:30~15:40)은 어느 창에도 없다.
// 그 구간은 닫힌 것이 맞지만, **다음 창은 같은 날 시간외**여야 한다 — 여기서
// 내일 아침을 가리키면 화면이 시간외 내내 폴링을 멈춘다.
func TestMarketSessionEdges_gapBetweenSameDaySessions(t *testing.T) {
	cals := krCalendarAroundMidnight()
	now := time.Date(2026, 9, 4, 15, 35, 0, 0, kst)

	lastClose, nextOpen := marketSessionEdges("domestic", cals, now)
	if want := time.Date(2026, 9, 4, 15, 30, 0, 0, kst); !lastClose.Equal(want) {
		t.Fatalf("lastClose = %s, want %s", lastClose.In(kst), want)
	}
	if want := time.Date(2026, 9, 4, 15, 40, 0, 0, kst); !nextOpen.Equal(want) {
		t.Fatalf("nextOpen = %s, want %s", nextOpen.In(kst), want)
	}
}

// 미국 정규장은 KST 자정을 넘는다. 달력의 `today`가 다음 날로 넘어간 뒤에도
// 지금 도는 세션은 `previousBusinessDay` 쪽에 있다(detectUsSessionAcrossDays와
// 같은 이유) — 창을 세 날에서 모으므로 그 구간도 맞는다.
func TestMarketClockAt_usSessionSpanningKstMidnight(t *testing.T) {
	cals := marketCalendars{
		us: &toss.UsMarketCalendarResponse{
			PreviousBusinessDay: toss.UsMarketDay{
				Date: "2026-09-03",
				RegularMarket: &toss.UsRegularMarketSession{
					StartTime: "2026-09-03T22:30:00+09:00",
					EndTime:   "2026-09-04T05:00:00+09:00",
				},
			},
			Today: toss.UsMarketDay{
				Date: "2026-09-04",
				RegularMarket: &toss.UsRegularMarketSession{
					StartTime: "2026-09-04T22:30:00+09:00",
					EndTime:   "2026-09-05T05:00:00+09:00",
				},
			},
		},
	}

	inSession := time.Date(2026, 9, 4, 1, 31, 0, 0, kst)
	if clock := marketClockAt("us", cals, inSession); clock.Closed() {
		t.Fatalf("session = %q at KST 01:31, want the running US regular session", clock.Session)
	}

	afterClose := time.Date(2026, 9, 4, 6, 0, 0, 0, kst)
	clock := marketClockAt("us", cals, afterClose)
	if !clock.Closed() {
		t.Fatalf("session = %q at KST 06:00, want closed", clock.Session)
	}
	closedAt, err := time.Parse(time.RFC3339, clock.ClosedAt)
	if err != nil {
		t.Fatalf("closedAt %q: %v", clock.ClosedAt, err)
	}
	if want := time.Date(2026, 9, 4, 5, 0, 0, 0, kst); !closedAt.Equal(want) {
		t.Fatalf("closedAt = %s, want %s", closedAt.In(kst), want)
	}
	nextOpen, err := time.Parse(time.RFC3339, clock.NextOpenAt)
	if err != nil {
		t.Fatalf("nextSessionAt %q: %v", clock.NextOpenAt, err)
	}
	if want := time.Date(2026, 9, 4, 22, 30, 0, 0, kst); !nextOpen.Equal(want) {
		t.Fatalf("nextSessionAt = %s, want %s", nextOpen.In(kst), want)
	}
}

// 달력을 못 구했으면 시각도 없다. 부르는 쪽은 그때 예전처럼 폴링을 이어 간다 —
// 「모른다」를 「닫혔다」로 굳히면 장중에 화면이 멈춘다.
func TestMarketClockAt_noCalendarCarriesNoInstants(t *testing.T) {
	clock := marketClockAt("domestic", marketCalendars{}, time.Now())
	if clock.ClosedAt != "" || clock.NextOpenAt != "" {
		t.Fatalf("clock without a calendar carried instants: %+v", clock)
	}
}
