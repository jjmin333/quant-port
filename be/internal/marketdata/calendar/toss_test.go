package calendar

import (
	"testing"

	"github.com/enact/quant-port/be/internal/toss"
)

func TestShouldPollKRXFromToss(t *testing.T) {
	kr := &toss.KrMarketCalendarResponse{
		Today: toss.KrMarketDay{
			Date: "2026-07-24",
			Integrated: &toss.IntegratedHour{
				RegularMarket: &toss.RegularMarketSession{
					StartTime: "2026-07-24T09:00:00+09:00",
					EndTime:   "2026-07-24T15:30:00+09:00",
				},
			},
		},
	}
	preOpen := mustKST(t, "2006-01-02 15:04", "2026-07-24 08:56")
	if !ShouldPoll(ScheduleKRX, preOpen, kr, nil) {
		t.Fatal("expected polling before KRX open from Toss calendar")
	}
	mid := mustKST(t, "2006-01-02 15:04", "2026-07-24 11:00")
	if !ShouldPoll(ScheduleKRX, mid, kr, nil) {
		t.Fatal("expected polling during KRX session from Toss calendar")
	}
	after := mustKST(t, "2006-01-02 15:04", "2026-07-24 15:31")
	if ShouldPoll(ScheduleKRX, after, kr, nil) {
		t.Fatal("expected no polling after KRX close from Toss calendar")
	}
}

func TestShouldPollKRXHolidayFromToss(t *testing.T) {
	kr := &toss.KrMarketCalendarResponse{
		Today: toss.KrMarketDay{Date: "2026-01-01"},
	}
	holiday := mustKST(t, "2006-01-02 15:04", "2026-01-01 10:00")
	if ShouldPoll(ScheduleKRX, holiday, kr, nil) {
		t.Fatal("expected KRX closed on Toss holiday")
	}
}

func TestShouldPollUSFromToss(t *testing.T) {
	us := &toss.UsMarketCalendarResponse{
		Today: toss.UsMarketDay{
			Date: "2026-07-24",
			RegularMarket: &toss.UsRegularMarketSession{
				StartTime: "2026-07-24T09:30:00-04:00",
				EndTime:   "2026-07-24T16:00:00-04:00",
			},
		},
	}
	preOpen := mustNY(t, "2006-01-02 15:04", "2026-07-24 09:26")
	if !ShouldPoll(ScheduleUS, preOpen, nil, us) {
		t.Fatal("expected polling before US cash open from Toss calendar")
	}
}
