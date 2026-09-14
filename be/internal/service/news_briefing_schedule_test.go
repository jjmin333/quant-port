package service

import (
	"testing"
	"time"
)

func seoulAt(t *testing.T, s string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatalf("load Asia/Seoul: %v", err)
	}
	at, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return at
}

/*
국내 브리핑은 **개장 한 시간 전부터** 30분 주기로 돈다.

전에는 09:00 정각에야 30분 주기가 시작돼, 그 시각에 처음 화면을 연 사람이 보는 문장이
08:00 슬롯 것(=07:00 이후 기사만 반영)이었다. 08:00~09:00은 전일 미국 증시 마감과
개장 전 시황이 몰리는 시간대라 한 시간을 통째로 놓치는 자리였다.
*/
func TestBriefingSlotStartsThirtyMinuteCadenceAtPreOpen(t *testing.T) {
	// 2026-08-31은 월요일(거래일).
	cases := []struct {
		at   string
		want string
	}{
		{"2026-08-31 07:40", "2026-08-31 07:00"}, // 리드 밖 — 1시간 슬롯
		{"2026-08-31 08:00", "2026-08-31 08:00"},
		{"2026-08-31 08:40", "2026-08-31 08:30"}, // 리드 안 — 30분 슬롯
		{"2026-08-31 09:40", "2026-08-31 09:30"}, // 장중
		{"2026-08-31 16:40", "2026-08-31 16:00"}, // 마감 후 — 1시간 슬롯
	}
	for _, tc := range cases {
		got := briefingSlot("domestic", seoulAt(t, tc.at))
		if want := seoulAt(t, tc.want); !got.Equal(want) {
			t.Errorf("briefingSlot(domestic, %s) = %s, want %s",
				tc.at, got.Format("2006-01-02 15:04"), tc.want)
		}
	}
}

// 휴장일에는 「개장 직전」이 없다 — 리드가 붙지 않는다.
func TestBriefingSlotNoPreOpenLeadOnNonTradingDay(t *testing.T) {
	// 2026-08-30은 일요일.
	got := briefingSlot("domestic", seoulAt(t, "2026-08-30 08:40"))
	if want := seoulAt(t, "2026-08-30 08:00"); !got.Equal(want) {
		t.Fatalf("weekend slot = %s, want %s", got.Format("15:04"), want.Format("15:04"))
	}
}
