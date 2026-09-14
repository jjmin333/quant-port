package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
터미널 스냅샷의 「신선한가」 판정 (2026-09-05 사용자 지적 6).

이 한 줄이 두 곳의 동작을 가른다:

  - EnsureTerminalSnapshot — 신선하면 그대로 주고, 낡았으면 **그것도 주되** 뒤에서
    새로 만든다(그 전에는 상류를 기다렸고, 실측 200~300ms가 거기서 나왔다).
  - loadQuoteDetailFromTerminalCache — 상세 헤더 가격을 정하는 사다리의 한 칸이라
    낡은 값을 올리면 v1.11.0에서 고친 「목록과 상세가 다른 가격」이 되살아난다.

TTL을 60초로 늘리면서 「살아 있다」와 「신선하다」가 갈라졌으므로, 그 경계를 여기서 못
박아 둔다.
*/
func TestTerminalSnapshotFresh(t *testing.T) {
	now := time.Date(2026, 9, 5, 6, 0, 0, 0, time.UTC)
	fresh := time.Duration(model.MarketDataQuoteCacheSec) * time.Second

	cases := []struct {
		name     string
		cachedAt time.Time
		want     bool
	}{
		{"방금 만든 값은 신선하다", now, true},
		{"신선한 폭 바로 안쪽", now.Add(-fresh + time.Millisecond), true},
		{"신선한 폭에 정확히 닿으면 낡았다", now.Add(-fresh), false},
		{"한 박자 지났으면 낡았다", now.Add(-2 * time.Second), false},
		{"TTL 안이지만 낡았다 — 살아 있는 것과 신선한 것은 다르다", now.Add(-30 * time.Second), false},
		{"시각이 없는 예전 캐시는 낡은 것으로 본다", time.Time{}, false},
		{"시계가 뒤로 간 경우는 신선하다고 본다", now.Add(time.Second), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := terminalSnapshotFresh(tc.cachedAt, now); got != tc.want {
				t.Fatalf("terminalSnapshotFresh(%v) = %v, want %v", tc.cachedAt, got, tc.want)
			}
		})
	}
}

// 「사는 시간」이 「신선한 시간」보다 넉넉해야 stale-while-revalidate가 성립한다.
// 둘이 같아지면 만료된 요청이 다시 상류를 기다린다 — 고치려던 그 상태다.
func TestTerminalSnapshotTTLOutlivesFreshness(t *testing.T) {
	fresh := time.Duration(model.MarketDataQuoteCacheSec) * time.Second
	if terminalSnapshotTTL <= fresh {
		t.Fatalf("terminalSnapshotTTL(%v)이 신선한 폭(%v)보다 길어야 한다", terminalSnapshotTTL, fresh)
	}
}
