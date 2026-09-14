package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

/*
달력 재조회 게이트 (2026-09-12 실측 회귀).

무엇이 터졌나: 같은 날 넣은 세션 판정 두 곳(환율 주기·순위 마감 스킵)이 매 틱
`EnsureCalendars`를 부르자 **휴장일에 분당 7회**의 상류 달력 호출이 나갔다
(하루 약 10,000회 — 그날 없앤 환율 5,760회보다 크다).

원인은 `marketCalendarsDateStale`이 「달력의 Today.Date가 오늘과 다르면 낡았다」로
판정하는데 **휴장일에는 그것이 영원히 참**이라는 점이었다. 토요일의 KR 달력은
오늘 날짜를 담지 않는다.
*/

func calStore(t *testing.T, at time.Time, attempted time.Time, krToday string) *MarketDataStore {
	t.Helper()
	s := &MarketDataStore{}
	s.calendarsAt = at
	s.calAttemptedAt = attempted
	if krToday != "" {
		s.calendars = marketCalendars{kr: &toss.KrMarketCalendarResponse{}}
		s.calendars.kr.Today.Date = krToday
	}
	return s
}

func TestCalendarGate_staleOnAHolidayDoesNotRefetchEveryCall(t *testing.T) {
	now := time.Now()
	// 달력은 방금 받았지만 Today.Date 가 오늘이 아니다(= 휴장일).
	s := calStore(t, now.Add(-time.Minute), now.Add(-time.Minute), "1999-01-01")
	if s.calendarsNeedRefresh(now) {
		t.Fatal("낡아 보이는데 방금 시도했다 — 다시 받으면 휴장일마다 분당 호출이 된다")
	}
	// 재시도 간격이 지나면 한 번은 다시 묻는다.
	if !s.calendarsNeedRefresh(now.Add(marketCalendarRetryAfterStale + time.Second)) {
		t.Fatal("재시도 간격이 지났는데도 안 묻는다 — 날짜가 바뀌어도 못 따라간다")
	}
}

func TestCalendarGate_freshCalendarNeedsNothing(t *testing.T) {
	now := time.Now()
	today := now.In(time.FixedZone("KST", 9*3600)).Format("2006-01-02")
	s := calStore(t, now.Add(-time.Minute), now.Add(-time.Minute), today)
	if s.calendarsNeedRefresh(now) {
		t.Fatal("오늘 날짜를 담은 달력을 다시 받으려 한다")
	}
}

func TestCalendarGate_neverFetchedStillPacesRetries(t *testing.T) {
	now := time.Now()
	// 한 번도 못 받았고 방금 시도했다 — 상류가 죽어 있는 경우.
	s := calStore(t, time.Time{}, now.Add(-time.Second), "")
	if s.calendarsNeedRefresh(now) {
		t.Fatal("실패 직후 재시도했다 — 상류 장애 중에 호출마다 재시도가 된다")
	}
	// 처음(시도 기록 없음)에는 받아야 한다.
	first := calStore(t, time.Time{}, time.Time{}, "")
	if !first.calendarsNeedRefresh(now) {
		t.Fatal("첫 조회를 막았다 — 달력 없이 시작한다")
	}
}

func TestCalendarGate_dailyPollStillWins(t *testing.T) {
	now := time.Now()
	today := now.In(time.FixedZone("KST", 9*3600)).Format("2006-01-02")
	// 하루가 지났으면 낡음 판정과 무관하게 받는다.
	s := calStore(t, now.Add(-marketCalendarPoll-time.Minute), now.Add(-time.Minute), today)
	if !s.calendarsNeedRefresh(now) {
		t.Fatal("24시간이 지났는데 안 받는다")
	}
}
