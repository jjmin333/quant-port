package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

// synthETF builds n days where NAV follows beta×index plus noise, price = NAV×(1+prem).
func synthETF(n int, beta, noise, prem float64, value, assets int64, lag int) []repository.ETFPoint {
	pts := make([]repository.ETFPoint, 0, n)
	nav := 10000.0
	idx := make([]float64, n)
	for i := range idx {
		idx[i] = 0.01 * math.Sin(float64(i)*1.7) // 결정적 지수 등락
	}
	for i := 0; i < n; i++ {
		if i > 0 {
			src := i - lag
			if src < 0 {
				src = 0
			}
			nav *= 1 + beta*idx[src] + noise*math.Cos(float64(i)*3.1)
		}
		pts = append(pts, repository.ETFPoint{
			Date:            fmt.Sprintf("2026-07-%02d", i+1),
			Close:           nav * (1 + prem),
			NAV:             sql.NullFloat64{Float64: nav, Valid: true},
			NetAssets:       sql.NullInt64{Int64: assets, Valid: true},
			TradeValue:      sql.NullInt64{Int64: value, Valid: true},
			IndexName:       "코스피 200",
			IndexChangeRate: sql.NullFloat64{Float64: idx[i] * 100, Valid: true},
		})
	}
	return pts
}

func TestETFScoresRankTightTrackersFirst(t *testing.T) {
	window := map[string][]repository.ETFPoint{}
	last := ""
	for i := 0; i < 25; i++ {
		// i가 클수록 괴리·잡음이 크고 거래·규모가 작다 — 점수가 떨어져야 한다.
		pts := synthETF(30, 1, 0.0002*float64(i), 0.0005*float64(i), int64(1e10)/int64(i+1), int64(1e12)/int64(i+1), 0)
		window[fmt.Sprintf("E%02d", i)] = pts
		last = pts[len(pts)-1].Date
	}
	rows := computeETFScores(window, last)
	if len(rows) != 25 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Symbol != "E00" || rows[len(rows)-1].Symbol != "E24" {
		t.Fatalf("order: first %s last %s", rows[0].Symbol, rows[len(rows)-1].Symbol)
	}
	var parts map[string]map[string]any
	_ = json.Unmarshal(rows[0].Parts, &parts)
	for _, k := range []string{"premium", "tracking", "liquidity", "size"} {
		if _, ok := parts[k]; !ok {
			t.Errorf("part %s missing", k)
		}
	}
}

// 레버리지(2배)는 베타로 갈려 따로 비교되고, 추적오차는 배율을 뺀 잔차로 잰다.
func TestETFLeveragedIsItsOwnPeerGroupAndTracksAfterBeta(t *testing.T) {
	pts := synthETF(40, 2, 0, 0, 1e9, 1e11, 0)
	m := measureETF("LEV", pts)
	if m.beta == nil || math.Abs(*m.beta-2) > 0.01 {
		t.Fatalf("beta = %v", m.beta)
	}
	if m.tracking == nil || *m.tracking > 0.01 {
		t.Fatalf("정확히 2배를 따르면 추적오차가 0이어야 한다: %v", m.tracking)
	}
	if etfPeerGroup(m.beta, m.indexVol) != ETFPeerLeveraged {
		t.Fatal("2배는 레버리지 집단이다")
	}
}

// 해외 지수처럼 하루 늦게 따라가도 멀쩡한 추적으로 읽어야 한다.
func TestETFTrackingPicksTheBetterLag(t *testing.T) {
	m := measureETF("US", synthETF(40, 1, 0, 0, 1e9, 1e11, 1))
	if m.tracking == nil || *m.tracking > 0.01 {
		t.Fatalf("하루 시차를 못 맞췄다: %v", m.tracking)
	}
}

func TestETFFetchDaysSkipsWeekendsAndStopsYesterday(t *testing.T) {
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC) // 목요일
	got := etfFetchDays("2026-09-18", now, 90)          // 금요일까지 있음
	want := []string{"2026-09-21", "2026-09-22", "2026-09-23"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// 지수가 거의 안 움직이는 상품(MMF·CD)은 주식형과 따로 비교한다.
func TestETFLowVolIndexIsItsOwnPeerGroup(t *testing.T) {
	calm := measureETF("MMF", synthETF(40, 1, 0, 0, 1e9, 1e11, 0))
	if calm.indexVol == nil {
		t.Fatal("index vol missing")
	}
	// synthETF의 지수는 일 1% 진폭이라 연 10%를 넘는다 — 일반.
	if g := etfPeerGroup(calm.beta, calm.indexVol); g != ETFPeerPlain {
		t.Fatalf("group = %s", g)
	}
	low := 1.2
	if g := etfPeerGroup(calm.beta, &low); g != ETFPeerLowVol {
		t.Fatalf("group = %s", g)
	}
}
