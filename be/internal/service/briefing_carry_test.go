package service

import (
	"testing"
	"time"
)

// 승계는 「직전 AI 문장을 그대로 쓰되 시각·숫자는 이 슬롯 것」이다. 그래서 문장이
// 얼마나 낡아도 되는지가 이 기능의 유일한 판단이고, 여기서 못 박는다.
func TestBriefingCarryAllowed(t *testing.T) {
	seoul := seoulLocation()
	at := func(h, m int) time.Time {
		return time.Date(2026, 9, 1, h, m, 0, 0, seoul)
	}

	cases := []struct {
		name   string
		origin time.Time
		now    time.Time
		want   bool
	}{
		{"직전 슬롯", at(17, 0), at(18, 0), true},
		{"창 경계 정각 3시간", at(15, 0), at(18, 0), true},
		{"창을 1분 넘김", at(14, 59), at(18, 0), false},
		{"같은 슬롯", at(18, 0), at(18, 0), true},
		{
			// 밤새 브리핑이 멈춘 뒤 개장 전 첫 슬롯. 3시간 안이지만 어제 장 문장이다.
			name:   "날이 바뀌면 3시간 이내여도 안 된다",
			origin: time.Date(2026, 9, 1, 23, 30, 0, 0, seoul),
			now:    time.Date(2026, 9, 2, 1, 0, 0, 0, seoul),
			want:   false,
		},
		{
			// 판정은 UTC가 아니라 서울 날짜로 한다. 아래 둘은 UTC로는 날이 갈리지만
			// 서울에서는 같은 9월 1일 오후다.
			name:   "UTC 자정을 건너도 서울 날짜가 같으면 된다",
			origin: time.Date(2026, 8, 31, 23, 0, 0, 0, time.UTC), // 서울 09-01 08:00
			now:    time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC),   // 서울 09-01 10:00
			want:   true,
		},
		{"origin이 비어 있으면 승계 없음", time.Time{}, at(18, 0), false},
		{"시계가 거꾸로 간 행은 무시", at(19, 0), at(18, 0), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := briefingCarryAllowed(tc.origin, tc.now); got != tc.want {
				t.Fatalf("briefingCarryAllowed(%s, %s) = %v, want %v",
					tc.origin.Format(time.RFC3339), tc.now.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}
