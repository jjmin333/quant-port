package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
)

// seoulClock renders a UTC instant in Seoul time — deadlines are stored in UTC
// but every assertion below is about what the user sees on a KRX clock.
func seoulClock(t *testing.T, at time.Time) string {
	t.Helper()
	return at.In(calendar.LocationFor(calendar.ScheduleKRX)).Format("2006-01-02 15:04")
}

func nyClock(t *testing.T, at time.Time) string {
	t.Helper()
	return at.In(calendar.LocationFor(calendar.ScheduleUS)).Format("2006-01-02 15:04")
}

// 2026-08-13 is a Thursday, 08-14 a Friday, 08-15 a Saturday (and 광복절).
func TestResolveDueAtSessionClose(t *testing.T) {
	// 10:00 KST Thursday — the bell has not rung yet, so "이번 장 종료" is today.
	during := time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC)
	got := resolveDueAt(model.HorizonSessionClose, model.AssetDomestic, during, "")
	if want := "2026-08-13 15:30"; seoulClock(t, got) != want {
		t.Fatalf("session close during the session = %s, want %s", seoulClock(t, got), want)
	}

	// 18:00 KST Thursday — today's close is behind us. A deadline in the past
	// would settle the stamp before it existed, so it rolls to the next session.
	after := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	got = resolveDueAt(model.HorizonSessionClose, model.AssetDomestic, after, "")
	if want := "2026-08-14 15:30"; seoulClock(t, got) != want {
		t.Fatalf("session close after the bell = %s, want %s", seoulClock(t, got), want)
	}
}

// Friday evening: the weekend and 광복절 are not sessions, so "다음 장 종료"
// has to walk past them rather than land on Saturday.
func TestResolveDueAtNextSessionSkipsWeekend(t *testing.T) {
	fridayMorning := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC) // 10:00 KST Fri
	got := resolveDueAt(model.HorizonNextSessionClose, model.AssetDomestic, fridayMorning, "")
	if want := "2026-08-17 15:30"; seoulClock(t, got) != want {
		t.Fatalf("next session close = %s, want %s (Monday)", seoulClock(t, got), want)
	}
}

func TestResolveDueAtFixedHorizons(t *testing.T) {
	at := time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC) // Thu 10:00 KST
	cases := []struct {
		kind model.StampHorizonKind
		want string
	}{
		{model.Horizon1W, "2026-08-20 15:30"},
		{model.Horizon1M, "2026-09-14 15:30"},
		{model.Horizon3M, "2026-11-13 15:30"},
	}
	for _, tc := range cases {
		got := resolveDueAt(tc.kind, model.AssetDomestic, at, "")
		if seoulClock(t, got) != tc.want {
			t.Errorf("%s = %s, want %s", tc.kind, seoulClock(t, got), tc.want)
		}
	}
}

// A horizon landing on a holiday settles at the next close that actually
// exists — otherwise the verdict has no settlement price behind it.
func TestResolveDueAtRollsOffNonTradingDay(t *testing.T) {
	// +1 week from Thu 2026-08-08 is Sat 2026-08-15.
	at := time.Date(2026, 8, 8, 1, 0, 0, 0, time.UTC)
	got := resolveDueAt(model.Horizon1W, model.AssetDomestic, at, "")
	if want := "2026-08-17 15:30"; seoulClock(t, got) != want {
		t.Fatalf("1W landing on a Saturday = %s, want %s", seoulClock(t, got), want)
	}
}

func TestResolveDueAtCustom(t *testing.T) {
	at := time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC)

	got := resolveDueAt(model.HorizonCustom, model.AssetDomestic, at, "2026-08-27")
	if want := "2026-08-27 15:30"; seoulClock(t, got) != want {
		t.Fatalf("custom date = %s, want %s", seoulClock(t, got), want)
	}

	// A date already behind us cannot be a deadline.
	got = resolveDueAt(model.HorizonCustom, model.AssetDomestic, at, "2026-08-01")
	if !got.After(at) {
		t.Fatalf("past custom date = %s, want a future close", seoulClock(t, got))
	}

	// Garbage from the client falls back to the default horizon rather than
	// dropping the stamp.
	got = resolveDueAt(model.HorizonCustom, model.AssetDomestic, at, "nonsense")
	if want := "2026-09-14 15:30"; seoulClock(t, got) != want {
		t.Fatalf("unparseable custom date = %s, want the 1M default %s", seoulClock(t, got), want)
	}
}

// A US stamp settles on the New York bell, not the Seoul one.
func TestResolveDueAtUsesTheMarketsOwnCalendar(t *testing.T) {
	// 2026-08-13 01:00 UTC = 21:00 ET on 08-12, after that day's close.
	at := time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC)
	got := resolveDueAt(model.HorizonSessionClose, model.AssetUS, at, "")
	if want := "2026-08-13 16:00"; nyClock(t, got) != want {
		t.Fatalf("US session close = %s, want %s", nyClock(t, got), want)
	}
}

func TestNormalizeHorizonKind(t *testing.T) {
	cases := map[string]model.StampHorizonKind{
		"session_close":      model.HorizonSessionClose,
		"NEXT_SESSION_CLOSE": model.HorizonNextSessionClose,
		" 1w ":               model.Horizon1W,
		"3m":                 model.Horizon3M,
		"custom":             model.HorizonCustom,
		"":                   model.Horizon1M,
		"nonsense":           model.Horizon1M,
	}
	for in, want := range cases {
		if got := normalizeHorizonKind(in); got != want {
			t.Errorf("normalizeHorizonKind(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every kind needs a label, or the client renders an empty chip.
func TestHorizonLabelsCoverEveryKind(t *testing.T) {
	kinds := []model.StampHorizonKind{
		model.HorizonSessionClose, model.HorizonNextSessionClose,
		model.Horizon1W, model.Horizon1M, model.Horizon3M, model.HorizonCustom,
	}
	for _, k := range kinds {
		if HorizonLabels[k] == "" {
			t.Errorf("no label for %q", k)
		}
	}
}
