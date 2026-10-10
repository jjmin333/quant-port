package service

import (
	"testing"
	"time"
)

// 승계는 「직전 AI 문장을 그대로 쓰되 시각·숫자는 이 슬롯 것」이다. 그래서 문장이
// 얼마나 낡아도 되는지가 이 기능의 유일한 판단이고, 여기서 못 박는다.
//
// 2026-09-01은 화요일(두 시장 모두 거래일). 9월의 뉴욕은 서머타임이라 KST = ET + 13시간
// — 미국 개장 준비(08:30 ET)는 21:30 KST다.
func TestBriefingCarryAllowed(t *testing.T) {
	seoul := seoulLocation()
	at := func(d, h, m int) time.Time {
		return time.Date(2026, 9, d, h, m, 0, 0, seoul)
	}

	cases := []struct {
		name   string
		market string
		origin time.Time
		now    time.Time
		want   bool
	}{
		{"직전 슬롯", "domestic", at(1, 17, 0), at(1, 18, 0), true},
		{"같은 슬롯", "domestic", at(1, 18, 0), at(1, 18, 0), true},
		{"장중 창 경계 정각 3시간", "domestic", at(1, 10, 0), at(1, 13, 0), true},
		{"장중 창을 1분 넘김", "domestic", at(1, 10, 0), at(1, 13, 1), false},
		{
			// 장중에 쓴 문장은 장 밖으로 넘어가도 3시간까지다 — 마감 전 숫자로 쓴 문장이다.
			name: "장중 문장은 마감 뒤에도 3시간", market: "domestic",
			origin: at(1, 15, 0), now: at(1, 19, 0), want: false,
		},
		{
			// 09-24 밤 실측: 21:00 문장이 00:00에 끊겨 규칙으로 떨어졌다. 국내는 이미
			// 마감한 뒤라 자정은 경계가 아니다.
			name: "국내 장 밖끼리는 자정을 넘어도 이어진다", market: "domestic",
			origin: at(1, 21, 0), now: at(2, 3, 0), want: true,
		},
		{"국내 개장 준비 직전까지", "domestic", at(1, 21, 0), at(2, 7, 59), true},
		{
			// 막으려던 본래 경우 — 어제 장 마감 문장 옆에 오늘 개장 전 숫자.
			name: "국내 개장 준비(08:00)부터는 어제 문장이 아니다", market: "domestic",
			origin: at(1, 21, 0), now: at(2, 8, 0), want: false,
		},
		{
			// 판정은 UTC가 아니라 시장 시각으로 한다. UTC로는 날이 갈리지만 서울에서는
			// 같은 9월 1일 장중이다.
			name:   "UTC 자정을 건너도 국내 브리핑 날이 같으면 된다",
			market: "domestic",
			origin: time.Date(2026, 8, 31, 23, 30, 0, 0, time.UTC), // 서울 09-01 08:30
			now:    time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC),    // 서울 09-01 10:00
			want:   true,
		},
		{
			// 09-24 밤 실측: 해외 21:30 문장이 3시간이 안 됐는데도 00:00에 끊겼다.
			// 자정 KST는 뉴욕 장중이다.
			name: "해외는 자정 KST를 넘어도 3시간 안이면 이어진다", market: "us",
			origin: at(1, 21, 30), now: at(2, 0, 0), want: true,
		},
		{"해외 장은 밤새 움직여 3시간 그대로", "us", at(1, 21, 30), at(2, 0, 31), false},
		{
			// 21:00 KST = 08:00 ET(개장 준비 전) → 21:40 KST = 08:40 ET(준비 뒤).
			name: "해외 개장 준비(08:30 ET)를 건너면 3시간 안이어도 안 된다", market: "us",
			origin: at(1, 21, 0), now: at(1, 21, 40), want: false,
		},
		{"origin이 비어 있으면 승계 없음", "domestic", time.Time{}, at(1, 18, 0), false},
		{"시계가 거꾸로 간 행은 무시", "domestic", at(1, 19, 0), at(1, 18, 0), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := briefingCarryAllowed(tc.market, tc.origin, tc.now); got != tc.want {
				t.Fatalf("briefingCarryAllowed(%s, %s, %s) = %v, want %v", tc.market,
					tc.origin.Format(time.RFC3339), tc.now.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}
