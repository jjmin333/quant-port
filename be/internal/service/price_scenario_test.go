package service

import (
	"math"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
시나리오 시험 — 재는 것은 **미래를 안 보고 만들었는가**다.

이 기능이 틀리면 사용자가 손해를 보는 자리는 셋이다: ① 변동성을 잴 때 미래 종가가
섞이는가(적중률이 부풀려진다) ② 없는 관측을 0으로 세는가 ③ 이력이 모자란 종목에
밴드를 지어내는가.
*/

func scenPoints(start string, closes ...float64) []repository.DailyClosePoint {
	d, err := time.Parse("2006-01-02", start)
	if err != nil {
		panic(err)
	}
	out := make([]repository.DailyClosePoint, 0, len(closes))
	for i, c := range closes {
		out = append(out, repository.DailyClosePoint{
			Date:  d.AddDate(0, 0, i).Format("2006-01-02"),
			Close: c,
		})
	}
	return out
}

/*
# 변동성은 **로그수익률**로 잰다

단순 차분으로 재면 고가 종목의 변동성이 과대평가되고, 밴드 하단이 0 아래로 내려갈
수 있다. 같은 비율로 오르내리는 두 종목은 가격이 달라도 같은 변동성이어야 한다.
*/
func TestScenarioVolatilityIsScaleFree(t *testing.T) {
	cheap := scenPoints("2026-01-01", 100, 110, 99, 108.9, 98.01, 107.8)
	dear := scenPoints("2026-01-01", 100000, 110000, 99000, 108900, 98010, 107800)

	a, _ := dailyVolatility(cheap)
	b, _ := dailyVolatility(dear)
	if math.Abs(a-b) > 1e-9 {
		t.Errorf("가격 수준이 변동성을 바꿨다: %v vs %v", a, b)
	}
	if a <= 0 {
		t.Errorf("움직이는 종목의 변동성이 0이다: %v", a)
	}

	// 0 이하 종가는 로그가 정의되지 않으므로 건너뛴다 — 세지도 않는다.
	bad := scenPoints("2026-01-01", 100, 0, 110, 121)
	_, n := dailyVolatility(bad)
	if n != 1 {
		t.Errorf("0 종가가 낀 구간에서 쓸 수 있는 수익률은 1개다: %d", n)
	}
}

/*
# 밴드는 현재가를 **곱셈으로** 감싼다

`exp(±zσ√T)`라서 하단은 0 아래로 못 가고, 상단의 거리가 하단보다 멀다. 그것이
가격의 성질이고, 화면이 그 비대칭을 그대로 보여 준다.
*/
func TestScenarioBandIsMultiplicative(t *testing.T) {
	bear, bull, vol := scenarioBand(1000, 0.02)
	if bear <= 0 {
		t.Errorf("하단이 0 이하다: %v", bear)
	}
	if bear >= 1000 || bull <= 1000 {
		t.Errorf("밴드가 현재가를 감싸지 않는다: %v ~ %v", bear, bull)
	}
	if 1000-bear >= bull-1000 {
		t.Errorf("상단이 하단보다 가깝다 — 곱셈 밴드가 아니다: %v %v", bear, bull)
	}
	want := 0.02 * math.Sqrt(float64(model.ScenarioTradingDays))
	if math.Abs(vol-want) > 1e-12 {
		t.Errorf("지평 변동성이 거래일 수로 안 자랐다: %v want %v", vol, want)
	}

	// 변동성이 0이면 세 점이 한 자리에 모인다 — 서비스가 그 전에 걸러야 하는 상태다.
	b0, u0, _ := scenarioBand(1000, 0)
	if b0 != 1000 || u0 != 1000 {
		t.Errorf("변동성 0에서 밴드가 벌어졌다: %v %v", b0, u0)
	}
}

/*
# **부등호 하나가 정직성을 정한다**

각 시점의 변동성은 그 시점 **이전**으로만 잰다. 시점 이후 종가가 한 줄이라도 섞이면
그 밴드는 미래를 알고 그린 것이고, 적중률은 그만큼 부풀려진다.

아래는 시점 전까지 **완전히 잠잠하다가** 시점 직후 폭등하는 종목이다. 미래를 보면
변동성이 크게 잡혀 밴드가 넓어지고 적중한다. 안 보면 밴드가 좁아 빗나가야 한다.
*/
func TestScenarioAccuracyDoesNotPeekAhead(t *testing.T) {
	// 240일 잠잠(±0.1%), 그 뒤 폭등.
	closes := make([]float64, 0, 400)
	px := 1000.0
	for i := 0; i < 300; i++ {
		if i%2 == 0 {
			px *= 1.001
		} else {
			px *= 0.999
		}
		closes = append(closes, px)
	}
	for i := 0; i < 100; i++ {
		px *= 1.03
		closes = append(closes, px)
	}
	points := scenPoints("2025-01-01", closes...)
	series := map[string][]repository.DailyClosePoint{"TEST": points}

	// 시점을 「잠잠한 구간의 끝」에 둔다. 그 뒤 90일이 폭등 구간이다.
	forecast, err := time.Parse("2006-01-02", points[299].Date)
	if err != nil {
		t.Fatal(err)
	}
	acc := scenarioAccuracyFrom(series, []time.Time{forecast})
	if acc.Samples != 1 {
		t.Fatalf("표본 하나여야 한다: %d", acc.Samples)
	}
	if acc.HitRate != 0 || acc.MissAbovePct != 100 {
		t.Errorf("미래를 안 봤다면 위로 뚫려야 한다 — 적중 %.0f%% · 위이탈 %.0f%%",
			acc.HitRate, acc.MissAbovePct)
	}
}

// 이력이 모자란 표본은 **세지 않는다**. 0으로 채우면 없는 관측이 적중률을 끌어내린다.
func TestScenarioAccuracySkipsThinHistory(t *testing.T) {
	short := scenPoints("2026-06-01", 100, 101, 102)
	acc := scenarioAccuracyFrom(
		map[string][]repository.DailyClosePoint{"TEST": short},
		[]time.Time{time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)},
	)
	if acc.Samples != 0 {
		t.Errorf("이력이 모자라면 표본이 아니다: %d", acc.Samples)
	}
	if acc.HitRate != 0 || acc.BaseErrorPct != 0 {
		t.Errorf("표본 0에서 숫자를 지어내면 안 된다: %+v", acc)
	}
}

/*
# 내재 PER은 가격에 **비례**한다

이익 전망을 지어내지 않는다. 가격이 x배가 되면 배수도 x배 — 「이 가격이면 지금 이익
기준 몇 배를 받는 것인가」라는 뜻이고, 앱이 이미 쓰는 PER에서 출발하므로 종목 화면과
어긋나지 않는다.
*/
func TestScenarioImpliedPERIsProportional(t *testing.T) {
	per := 38.4
	p := scenarioPoint(model.ScenarioBull, 512.0, 256.0, &per)
	if p.ImpliedPER == nil {
		t.Fatal("PER이 있으면 내재 PER도 나와야 한다")
	}
	if math.Abs(*p.ImpliedPER-76.8) > 0.01 {
		t.Errorf("두 배 가격이면 두 배 배수다: %v", *p.ImpliedPER)
	}
	if math.Abs(p.ChangePct-100) > 0.01 {
		t.Errorf("등락률: %v", p.ChangePct)
	}

	// PER이 없는 종목(국내 자격 기준 40%)에서는 줄 자체가 없어야 한다.
	none := scenarioPoint(model.ScenarioBull, 512.0, 256.0, nil)
	if none.ImpliedPER != nil {
		t.Errorf("PER이 없는데 내재 PER을 지어냈다: %v", *none.ImpliedPER)
	}

	// 중립은 언제나 0%이고 현재 배수 그대로다.
	base := scenarioPoint(model.ScenarioBase, 256.0, 256.0, &per)
	if base.ChangePct != 0 || base.ImpliedPER == nil || math.Abs(*base.ImpliedPER-38.4) > 0.01 {
		t.Errorf("중립이 제자리가 아니다: %+v", base)
	}
}

// 구간 자르기는 양끝을 포함한다 — 하루가 새면 변동성 창이 조용히 달라진다.
func TestScenarioSliceIsInclusive(t *testing.T) {
	points := scenPoints("2026-01-01", 1, 2, 3, 4, 5)
	got := scenarioSlice(points, "2026-01-02", "2026-01-04")
	if len(got) != 3 || got[0].Close != 2 || got[2].Close != 4 {
		t.Errorf("양끝 포함이어야 한다: %+v", got)
	}
	if s := scenarioSlice(points, "2026-02-01", "2026-02-05"); s != nil {
		t.Errorf("구간 밖이면 비어야 한다: %+v", s)
	}
}
