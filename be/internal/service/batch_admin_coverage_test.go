package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestCoverageCacheAction(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		at          time.Time
		has         bool
		refreshing  bool
		wantPending bool
		wantRefresh bool
	}{
		{"처음 — 비어 있으면 기다리지 않고 pending, 뒤에서 센다", time.Time{}, false, false, true, true},
		{"처음인데 이미 세는 중 — 두 번째 회차를 띄우지 않는다", time.Time{}, false, true, true, false},
		{"신선 — 그대로 준다", now.Add(-time.Minute), true, false, false, false},
		{"묵음 — 묵은 값을 주고 뒤에서 한 번 센다", now.Add(-batchCoverageTTL), true, false, false, true},
		{"묵음인데 세는 중 — 겹치지 않는다", now.Add(-time.Hour), true, true, false, false},
	}
	for _, tc := range cases {
		pending, refresh := coverageCacheAction(now, tc.at, tc.has, tc.refreshing)
		if pending != tc.wantPending || refresh != tc.wantRefresh {
			t.Errorf("%s: got pending=%v refresh=%v, want %v %v", tc.name, pending, refresh, tc.wantPending, tc.wantRefresh)
		}
	}
}

func TestCoverageMemoKeySharesAllMarketCloses(t *testing.T) {
	a := coverageMemoKey(model.BatchDailyCloseFetchStale)
	b := coverageMemoKey(model.BatchDailyCloseFetchMissing)
	c := coverageMemoKey(model.BatchDailyCloseFetchHistory)
	if a == "" || a != b || b != c {
		t.Fatalf("전 종목 일봉 잡 셋은 같은 질문을 공유해야 한다: %q %q %q", a, b, c)
	}
	if coverageMemoKey(model.BatchDailyCloseDomestic) != "" {
		t.Fatalf("시장별 일봉은 따로 센다")
	}
}

// 커버리지 소스가 하나도 없는 서비스(시험 배선)에서도 스냅샷이 막히지 않고,
// 뒤에서 한 번 센 뒤에는 pending이 풀린다.
func TestCoverageSnapshotWarmsInBackground(t *testing.T) {
	s := &BatchAdminService{}
	if _, pending := s.CoverageSnapshot(); !pending {
		t.Fatal("차가운 캐시는 pending이어야 한다")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.coverage.mu.Lock()
		warmed := s.coverage.data != nil && !s.coverage.refreshing
		s.coverage.mu.Unlock()
		if warmed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, pending := s.CoverageSnapshot(); pending {
		t.Fatal("한 번 센 뒤에는 pending이 풀려야 한다")
	}
}
