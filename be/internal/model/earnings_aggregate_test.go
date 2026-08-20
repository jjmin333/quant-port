package model

import "testing"

func ev(symbol, basis string, year int, period string, pct *float64) EarningsEvent {
	e := EarningsEvent{
		Symbol:       symbol,
		Market:       string(AssetDomestic),
		Date:         "2026-08-14",
		DateStatus:   EarningsDateConfirmed,
		FiscalYear:   year,
		FiscalPeriod: period,
		Basis:        basis,
	}
	if pct != nil {
		e.Surprise = &EarningsSurprise{Metric: EarningsMetricOpIncome, Pct: pct}
	}
	return e
}

// 실측 상황: 미래에셋생명 2026 2Q가 연결 +54.6%와 개별 −60.8% 두 줄로 내려온다
// (같은 모양이 338건). 한 화면에서 한 회사가 상회와 하회를 동시에 말하면 둘 다
// 못 믿게 되므로, 연결을 대표로 세우고 개별은 접어 둔다.
func TestCollapseDomesticBasisPrefersConsolidated(t *testing.T) {
	rows := []EarningsEvent{
		ev("085620", "개별", 2026, "2Q", f(-60.8)),
		ev("085620", "연결", 2026, "2Q", f(54.6)),
	}
	out := CollapseDomesticBasis(rows)

	if len(out) != 1 {
		t.Fatalf("rows = %d, want 1 (개별은 접혀야 한다)", len(out))
	}
	if out[0].Basis != "연결" {
		t.Errorf("primary basis = %q, want 연결 — 국내 실적 보도의 기준", out[0].Basis)
	}
	if len(out[0].Alternates) != 1 || out[0].Alternates[0].Basis != "개별" {
		t.Fatalf("alternates = %+v, want the 개별 row kept (지주회사는 개별이 의미 있다)", out[0].Alternates)
	}
	// 접힌 행이 자기 대안을 또 들고 있으면 트리가 되어 화면이 무한히 펼친다.
	if len(out[0].Alternates[0].Alternates) != 0 {
		t.Error("alternates must be flat, not nested")
	}
}

// 순서에 의존해서 결과가 달라지면 안 된다 — DB 정렬은 basis를 보장하지 않는다.
func TestCollapseDomesticBasisOrderIndependent(t *testing.T) {
	for _, rows := range [][]EarningsEvent{
		{ev("A", "연결", 2026, "2Q", f(10)), ev("A", "개별", 2026, "2Q", f(-5))},
		{ev("A", "개별", 2026, "2Q", f(-5)), ev("A", "연결", 2026, "2Q", f(10))},
	} {
		out := CollapseDomesticBasis(rows)
		if len(out) != 1 || out[0].Basis != "연결" {
			t.Fatalf("got %d rows with primary %q, want 1 row with 연결", len(out), out[0].Basis)
		}
	}
}

// 연결 실적을 아예 안 내는 종목이 화면에서 사라지면 안 된다.
func TestCollapseDomesticBasisKeepsSeparateWhenNoConsolidated(t *testing.T) {
	out := CollapseDomesticBasis([]EarningsEvent{ev("B", "개별", 2026, "1Q", f(3))})
	if len(out) != 1 || out[0].Basis != "개별" {
		t.Fatalf("got %+v, want the 개별 row to stand alone", out)
	}
}

// 서로 다른 분기·서로 다른 종목은 합치지 않는다.
func TestCollapseDomesticBasisDoesNotMergeAcrossPeriods(t *testing.T) {
	out := CollapseDomesticBasis([]EarningsEvent{
		ev("A", "연결", 2026, "2Q", f(1)),
		ev("A", "연결", 2026, "1Q", f(2)),
		ev("B", "연결", 2026, "2Q", f(3)),
	})
	if len(out) != 3 {
		t.Fatalf("rows = %d, want 3", len(out))
	}
}

// 추정 행은 병합 대상이 아니다 — 근거(EstimateBasis)를 잃으면 화면이 「추정」이라고
// 말할 수 없게 된다.
func TestCollapseDomesticBasisLeavesEstimatesAlone(t *testing.T) {
	est := ev("C", "연결", 2026, "3Q", nil)
	est.DateStatus = EarningsDateEstimated
	est.EstimateBasis = "작년 3Q는 10/31 발표"
	est2 := est
	est2.Basis = "개별"

	out := CollapseDomesticBasis([]EarningsEvent{est, est2})
	if len(out) != 2 {
		t.Fatalf("rows = %d, want 2 (추정은 접지 않는다)", len(out))
	}
	for _, o := range out {
		if o.EstimateBasis == "" {
			t.Error("estimateBasis lost")
		}
	}
}

func TestComputeHitHistory(t *testing.T) {
	rows := []EarningsEvent{
		ev("A", "연결", 2026, "2Q", f(5)),
		ev("A", "연결", 2026, "1Q", f(40)),
		ev("A", "연결", 2025, "FY", f(-10)),
		ev("A", "연결", 2025, "3Q", f(20)),
		ev("A", "연결", 2025, "2Q", nil), // 비교 불가 — 세지 않는다
	}
	h := ComputeHitHistory(rows, 8)
	if h == nil {
		t.Fatal("hit history = nil")
	}
	if h.Quarters != 4 || h.Beats != 3 || h.Misses != 1 {
		t.Errorf("got %d분기 %d상회 %d하회, want 4/3/1 (컨센서스 없는 분기는 제외)",
			h.Quarters, h.Beats, h.Misses)
	}
	// 평균 13.75 · 중위 12.5 — 둘을 함께 주는 이유는 한 분기의 극단이 평균만
	// 끌어올리는 경우를 화면이 드러낼 수 있어야 하기 때문이다.
	if h.AvgPct == nil || h.MedianPct == nil {
		t.Fatal("avg/median must both be present")
	}
	if *h.AvgPct < 13.7 || *h.AvgPct > 13.8 {
		t.Errorf("avg = %v, want ~13.75", *h.AvgPct)
	}
	if *h.MedianPct != 12.5 {
		t.Errorf("median = %v, want 12.5", *h.MedianPct)
	}

	if ComputeHitHistory([]EarningsEvent{ev("A", "연결", 2026, "2Q", nil)}, 8) != nil {
		t.Error("no comparable quarter must yield nil, not a zero-filled history")
	}
}

func TestBuildEarningsStats(t *testing.T) {
	small := ev("TINY", "연결", 2026, "2Q", f(5000))
	small.Surprise.Estimate = 1e8 // 1억원 — 유의성 하한 아래
	small.Name = "초소형주"

	big := ev("BIG", "연결", 2026, "2Q", f(30))
	big.Surprise.Estimate = 1e11
	big.Name = "대형주"

	miss := ev("MID", "연결", 2026, "2Q", f(-12))
	miss.Surprise.Estimate = 1e11
	miss.Name = "중형주"

	sched := ev("FUT", "", 2026, "3Q", nil)
	sched.DateStatus = EarningsDateScheduled

	material := func(s EarningsSurprise) bool { return s.Estimate >= 5e9 }
	stats := BuildEarningsStats([]EarningsEvent{small, big, miss, sched}, material)

	if stats.Total != 4 || stats.Confirmed != 3 || stats.Scheduled != 1 {
		t.Errorf("counts = %d/%d/%d, want 4/3/1", stats.Total, stats.Confirmed, stats.Scheduled)
	}
	// 하한 아래 종목은 **통계에서 빠지지만 건수에는 남는다** — 발표가 있었던 것은
	// 사실이고, 평균만 오염시키지 않으면 된다.
	if stats.Compared != 2 || stats.Beats != 1 || stats.Misses != 1 {
		t.Errorf("compared/beats/misses = %d/%d/%d, want 2/1/1",
			stats.Compared, stats.Beats, stats.Misses)
	}
	if stats.Best == nil || stats.Best.Symbol != "BIG" {
		t.Errorf("best = %+v, want BIG (초소형주 +5000%%가 하한에 걸려야 한다)", stats.Best)
	}
	if stats.Worst == nil || stats.Worst.Symbol != "MID" {
		t.Errorf("worst = %+v, want MID", stats.Worst)
	}
	if stats.BeatRate == nil || *stats.BeatRate != 50 {
		t.Errorf("beatRate = %v, want 50", stats.BeatRate)
	}
}
