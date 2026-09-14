package service

import (
	"context"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

func TestDetectKrSession_afterWindow(t *testing.T) {
	integrated := toss.IntegratedHour{
		RegularMarket: &toss.RegularMarketSession{
			StartTime: "2026-07-16T09:00:00+09:00",
			EndTime:   "2026-07-16T15:30:00+09:00",
		},
		AfterMarket: &toss.AfterMarketSession{
			StartTime: "2026-07-16T15:30:00+09:00",
			EndTime:   "2026-07-16T18:00:00+09:00",
		},
	}
	now := time.Date(2026, 7, 16, 16, 30, 0, 0, kst)
	if got := detectKrSession(integrated, now); got != "after" {
		t.Fatalf("session = %q want after", got)
	}
	closed := time.Date(2026, 7, 16, 19, 0, 0, 0, kst)
	if got := detectKrSession(integrated, closed); got != "closed" {
		t.Fatalf("session = %q want closed after after-market", got)
	}
}

func TestDetectMarketSession_domesticHolidayClosed(t *testing.T) {
	cals := marketCalendars{
		kr: &toss.KrMarketCalendarResponse{
			Today: toss.KrMarketDay{
				Date:       "2026-07-16",
				Integrated: nil,
			},
		},
	}
	now := time.Date(2026, 7, 16, 17, 0, 0, 0, kst)
	if got := detectMarketSession("domestic", cals, now); got != "closed" {
		t.Fatalf("session = %q want closed on holiday", got)
	}
}

func TestShouldLivePollSymbol_followsTossEnabled(t *testing.T) {
	s := &QuotesService{}
	if s.shouldLivePollSymbol(context.Background(), "005930") {
		t.Fatal("expected false when Toss disabled")
	}
}

func TestFilterLiveSessionSymbols(t *testing.T) {
	cases := []struct {
		session string
		keep    bool
	}{
		{"regular", true},
		{"pre", true},
		{"after", true},
		{"day", true},
		{"closed", false},
		{"", false},
	}
	for _, c := range cases {
		if got := quoteSessionHasLiveMicrostructure(c.session); got != c.keep {
			t.Errorf("session %q: live=%v want %v", c.session, got, c.keep)
		}
	}
}

func TestFilterLiveSessionSymbolsEmpty(t *testing.T) {
	s := &QuotesService{}
	if got := s.filterLiveSessionSymbols(context.Background(), nil); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

/*
미국 정규장은 KST 자정을 넘는다. 달력의 `today`가 새 KST 날짜로 넘어간 뒤에도
진행 중인 세션은 `previousBusinessDay`에 남아 있으므로, 그때 "closed"로
판정되면 폴러가 1분봉 적재를 멈추고 차트가 얼어붙는다(detectUsSessionAcrossDays 참조).

실측 재현: 2026-08-25 01:31 KST = ET 12:31로 정규장 한복판인데 closed였다.
*/
func TestDetectMarketSession_usSessionSpanningKSTMidnight(t *testing.T) {
	cals := marketCalendars{
		us: &toss.UsMarketCalendarResponse{
			// KST 자정을 넘겼으므로 today는 이미 08-25 — 오늘 "밤"에 열릴 세션이다.
			Today: toss.UsMarketDay{
				Date: "2026-08-25",
				RegularMarket: &toss.UsRegularMarketSession{
					StartTime: "2026-08-25T22:30:00+09:00",
					EndTime:   "2026-08-26T05:00:00+09:00",
				},
			},
			// 지금 실제로 돌아가고 있는 세션은 이쪽에 들어 있다.
			PreviousBusinessDay: toss.UsMarketDay{
				Date: "2026-08-24",
				RegularMarket: &toss.UsRegularMarketSession{
					StartTime: "2026-08-24T22:30:00+09:00",
					EndTime:   "2026-08-25T05:00:00+09:00",
				},
				AfterMarket: &toss.UsAfterMarketSession{
					StartTime: "2026-08-25T05:00:00+09:00",
					EndTime:   "2026-08-25T08:50:00+09:00",
				},
			},
		},
	}

	inSession := time.Date(2026, 8, 25, 1, 31, 0, 0, kst)
	if got := detectMarketSession("us", cals, inSession); got != "regular" {
		t.Fatalf("session = %q want regular (ET 12:31, 정규장 한복판)", got)
	}
	if !quoteSessionHasLiveMicrostructure(detectMarketSession("us", cals, inSession)) {
		t.Fatal("정규장인데 라이브가 아니라고 판정했다 — 폴러가 캔들 적재를 멈춘다")
	}

	// previousBusinessDay의 애프터마켓도 같은 KST 날짜에 걸쳐 있다.
	afterHours := time.Date(2026, 8, 25, 6, 0, 0, 0, kst)
	if got := detectMarketSession("us", cals, afterHours); got != "after" {
		t.Fatalf("session = %q want after", got)
	}

	// 양쪽 어느 창에도 안 들어가면 여전히 closed여야 한다 — 오탐이 없어야 한다.
	trulyClosed := time.Date(2026, 8, 25, 12, 0, 0, 0, kst)
	if got := detectMarketSession("us", cals, trulyClosed); got != "closed" {
		t.Fatalf("session = %q want closed (KST 정오는 어느 세션도 아니다)", got)
	}

	// 자정 전, today 쪽 창이 맞는 경우도 그대로 동작해야 한다.
	tonight := time.Date(2026, 8, 25, 23, 0, 0, 0, kst)
	if got := detectMarketSession("us", cals, tonight); got != "regular" {
		t.Fatalf("session = %q want regular (today 창)", got)
	}
}
