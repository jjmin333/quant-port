package service

import (
	"math"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
백테스트 시험 — 재는 것은 속도가 아니라 **정직성**이다.

숫자가 맞는지보다 먼저 물어야 할 것이 있다: 이 엔진이 미래를 보지 않는가, 없는 값을
0으로 때우지 않는가, 표본이 짧을 때 입을 다무는가. 아래 시험들은 그 셋을 각각 재고,
수익률 계산은 손으로 검산할 수 있는 작은 숫자로 고정한다.
*/

func constituentsOf(byDate map[string][]string) map[string][]repository.ScoreRankingHistoryRow {
	out := map[string][]repository.ScoreRankingHistoryRow{}
	for date, syms := range byDate {
		for i, sym := range syms {
			out[date] = append(out[date], repository.ScoreRankingHistoryRow{
				TradeDate: date, Rank: i + 1, Symbol: sym, Score: 90 - i,
			})
		}
	}
	return out
}

func baseRequest() model.BacktestRequest {
	return model.BacktestRequest{
		Market:    "domestic",
		Axis:      model.RankAxisComposite,
		TopN:      2,
		Rebalance: model.RebalanceDaily,
		FromDate:  "2026-01-01",
		ToDate:    "2026-12-31",
		// `runBacktest`는 정규화를 거치지 않으므로 시험이 직접 정한다 — 실제 경로에서는
		// `normalizeBacktestRequest`가 시장별 기본값을 채운다.
		Benchmarks: []string{"kospi"},
	}
}

/*
# 미래를 보지 않는다

이 시험이 이 파일에서 가장 중요하다.

D일 랭킹은 D일 **종가로** 계산된다. 그러니 그 랭킹으로 살 수 있는 가장 이른 순간은
D일 종가이고, 수익은 D+1일부터 난다. 그 부등호를 `<=`로 잘못 두면 「그날 폭등한
종목을 그날 아침에 고른」 것이 되어 **어떤 랭킹이든 좋아 보인다.**

그래서 3일에 +100% 폭등하는 종목을 **3일 랭킹에만** 넣는다. 정직한 엔진은 그 폭등을
한 푼도 먹지 못한다 — 3일 종가를 보고서야 그 종목을 알았기 때문이다.
*/
func TestBacktestDoesNotSeeTheFuture(t *testing.T) {
	constituents := constituentsOf(map[string][]string{
		"2026-03-02": {"FLAT"},
		"2026-03-03": {"MOON"}, // 3일에 폭등한 뒤에야 랭킹에 든다
		"2026-03-04": {"FLAT"},
	})
	closes := map[string]map[string]float64{
		"FLAT": {"2026-03-02": 100, "2026-03-03": 100, "2026-03-04": 100},
		"MOON": {"2026-03-02": 100, "2026-03-03": 200, "2026-03-04": 200},
	}
	req := baseRequest()
	req.TopN = 1

	out := runBacktest(req, constituents,
		pickRebalanceDates(btSortedDates(constituents), model.RebalanceDaily),
		closes, nil)

	if out == nil {
		t.Fatal("nil 결과")
	}
	if math.Abs(out.Portfolio.TotalReturn) > 0.001 {
		t.Errorf("미래를 봤다 — 폭등일을 먹으면 안 된다: 총수익 %.2f%%", out.Portfolio.TotalReturn)
	}
}

/*
# 수익률은 손으로 검산된다

2종목 동일가중. 3일에 A +10% · B 0% → 포트 +5%. 4일에 A 0% · B +20% → +10%.
누적 = 1.05 × 1.10 − 1 = **15.5%**.
*/
func TestBacktestEqualWeightedCompounding(t *testing.T) {
	constituents := constituentsOf(map[string][]string{
		"2026-03-02": {"AAA", "BBB"},
		"2026-03-03": {"AAA", "BBB"},
	})
	closes := map[string]map[string]float64{
		"AAA": {"2026-03-02": 100, "2026-03-03": 110, "2026-03-04": 110},
		"BBB": {"2026-03-02": 100, "2026-03-03": 100, "2026-03-04": 120},
	}
	req := baseRequest()

	out := runBacktest(req, constituents,
		pickRebalanceDates(btSortedDates(constituents), model.RebalanceDaily),
		closes, nil)

	if math.Abs(out.Portfolio.TotalReturn-15.5) > 0.01 {
		t.Errorf("누적 15.5%%를 기대했다: %.2f%%", out.Portfolio.TotalReturn)
	}
	if out.Coverage.TradingDays != 2 {
		t.Errorf("수익률이 난 거래일은 2일이다: %d", out.Coverage.TradingDays)
	}
}

/*
# 없는 종가를 0%로 세지 않는다

거래정지된 종목을 「보합」으로 세면 나머지 종목의 수익률이 **희석된다.** 아래에서
AAA는 +10%이고 HALT는 이틀째 종가가 없다. 정직한 답은 +10%(AAA 하나로 계산)이지
+5%(둘 평균)가 아니다.
*/
func TestBacktestSkipsMissingCloses(t *testing.T) {
	constituents := constituentsOf(map[string][]string{"2026-03-02": {"AAA", "HALT"}})
	closes := map[string]map[string]float64{
		"AAA":  {"2026-03-02": 100, "2026-03-03": 110},
		"HALT": {"2026-03-02": 100}, // 03-03 거래정지
	}
	req := baseRequest()

	out := runBacktest(req, constituents,
		pickRebalanceDates(btSortedDates(constituents), model.RebalanceDaily),
		closes, nil)

	if math.Abs(out.Portfolio.TotalReturn-10) > 0.01 {
		t.Errorf("+10%%를 기대했다(희석 금지): %.2f%%", out.Portfolio.TotalReturn)
	}
	if out.Coverage.MissingPrices != 1 {
		t.Errorf("빠진 종가를 세어 보고해야 한다: %d", out.Coverage.MissingPrices)
	}
	if out.Curve[len(out.Curve)-1].Holdings != 1 {
		t.Errorf("그날 실제로 수익을 낸 종목 수를 적어야 한다: %+v", out.Curve)
	}
}

/*
# 표본이 짧으면 입을 다문다

30거래일치 수익을 연율화하면 세 자리 수가 된다. 그건 측정이 아니라 외삽이다.
*/
func TestBacktestWithholdsAnnualizedOnShortSample(t *testing.T) {
	dates := map[string][]string{}
	closes := map[string]map[string]float64{"AAA": {}}
	price := 100.0
	for i := 0; i < 10; i++ {
		date := shiftDate("2026-03-02", i)
		dates[date] = []string{"AAA"}
		closes["AAA"][date] = price
		price *= 1.01
	}
	req := baseRequest()

	out := runBacktest(req, constituentsOf(dates),
		pickRebalanceDates(btSortedDates(constituentsOf(dates)), model.RebalanceDaily),
		closes, nil)

	if out.Portfolio.AnnualizedReturn != nil {
		t.Errorf("9거래일을 연율화하면 안 된다: %v", *out.Portfolio.AnnualizedReturn)
	}
	if out.Portfolio.Sharpe != nil {
		t.Errorf("9거래일로 Sharpe를 내면 안 된다: %v", *out.Portfolio.Sharpe)
	}
	if out.Coverage.Sufficient {
		t.Error("표본이 충분하다고 말하면 안 된다")
	}
	if out.Coverage.Note == "" {
		t.Error("짧다는 사실을 문장으로 적어야 한다")
	}
	// 누적 수익률과 곡선은 **그린다** — 못 쓰는 것은 외삽이지 측정이 아니다.
	if out.Portfolio.TotalReturn <= 0 || len(out.Curve) < 2 {
		t.Errorf("측정값 자체는 나와야 한다: %+v", out.Portfolio)
	}
}

// 벤치마크 대비 초과수익은 단순 차이다 — 화면이 그 정의를 다시 만들지 않도록 잠근다.
func TestBacktestAlphaIsExcessOverBenchmark(t *testing.T) {
	constituents := constituentsOf(map[string][]string{"2026-03-02": {"AAA"}})
	closes := map[string]map[string]float64{
		"AAA": {"2026-03-02": 100, "2026-03-03": 110},
	}
	benchmark := map[string]float64{"2026-03-02": 1000, "2026-03-03": 1040}

	out := runBacktest(baseRequest(), constituents,
		pickRebalanceDates(btSortedDates(constituents), model.RebalanceDaily),
		closes, map[string]map[string]float64{"kospi": benchmark})

	if len(out.Benchmarks) != 1 || out.Alpha == nil {
		t.Fatalf("벤치마크와 초과수익이 있어야 한다: %+v", out)
	}
	bench := out.Benchmarks[0]
	if !bench.Primary || bench.ID != "kospi" || bench.Label != "KOSPI" {
		t.Errorf("기준 벤치마크가 이름과 함께 나가야 한다: %+v", bench)
	}
	if math.Abs(bench.Metrics.TotalReturn-4) > 0.01 {
		t.Errorf("벤치마크 +4%%: %.2f%%", bench.Metrics.TotalReturn)
	}
	if math.Abs(*out.Alpha-6) > 0.01 {
		t.Errorf("초과수익 +6%%p: %.2f", *out.Alpha)
	}
}

// MDD는 고점 대비 최대 낙폭이고 음수다. 100 → 120 → 90이면 −25%.
func TestBacktestMaxDrawdown(t *testing.T) {
	constituents := constituentsOf(map[string][]string{
		"2026-03-02": {"AAA"}, "2026-03-03": {"AAA"}, "2026-03-04": {"AAA"},
	})
	closes := map[string]map[string]float64{
		"AAA": {"2026-03-02": 100, "2026-03-03": 120, "2026-03-04": 90, "2026-03-05": 90},
	}
	out := runBacktest(baseRequest(), constituents,
		pickRebalanceDates(btSortedDates(constituents), model.RebalanceDaily),
		closes, nil)

	if math.Abs(out.Portfolio.MDD-(-25)) > 0.01 {
		t.Errorf("MDD −25%%를 기대했다: %.2f%%", out.Portfolio.MDD)
	}
}

/*
# 리밸런싱 주기는 「그 주에 처음 랭킹이 나온 날」이다

요일로 못박으면 점수 배치가 쉰 주가 통째로 건너뛰어진다(실측 27일 중 16일만 점수가
있었다).
*/
func TestPickRebalanceDatesUsesFirstOfEachBucket(t *testing.T) {
	dates := []string{
		"2026-03-02", "2026-03-03", "2026-03-05", // 같은 주
		"2026-03-10", "2026-03-12", // 다음 주
		"2026-04-01", // 다음 달
	}
	weekly := pickRebalanceDates(dates, model.RebalanceWeekly)
	if len(weekly) != 3 || weekly[0] != "2026-03-02" || weekly[1] != "2026-03-10" {
		t.Errorf("주 단위: 주마다 첫 날 하나씩 — %v", weekly)
	}
	monthly := pickRebalanceDates(dates, model.RebalanceMonthly)
	if len(monthly) != 2 || monthly[0] != "2026-03-02" || monthly[1] != "2026-04-01" {
		t.Errorf("월 단위: 달마다 첫 날 하나씩 — %v", monthly)
	}
	daily := pickRebalanceDates(dates, model.RebalanceDaily)
	if len(daily) != len(dates) {
		t.Errorf("일 단위는 전부 — %v", daily)
	}
}

// 회전율은 바뀐 비중이다. 둘 중 하나가 바뀌면 50%.
func TestTurnoverBetween(t *testing.T) {
	if got := turnoverBetween([]string{"A", "B"}, []string{"A", "C"}); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("50%%를 기대했다: %v", got)
	}
	if got := turnoverBetween([]string{"A", "B"}, []string{"A", "B"}); got != 0 {
		t.Errorf("그대로면 0: %v", got)
	}
	if got := turnoverBetween([]string{"A"}, []string{"X", "Y"}); math.Abs(got-1) > 1e-9 {
		t.Errorf("전부 바뀌면 100%%: %v", got)
	}
}

// 이력이 없는 창은 오류가 아니라 「아직 없음」이다.
func TestBacktestEmptyWindowIsNotAnError(t *testing.T) {
	constituents := constituentsOf(map[string][]string{"2026-03-02": {"AAA"}})
	// 종가가 하루뿐이면 수익률을 낼 날이 없다.
	closes := map[string]map[string]float64{"AAA": {"2026-03-02": 100}}
	out := runBacktest(baseRequest(), constituents,
		pickRebalanceDates(btSortedDates(constituents), model.RebalanceDaily),
		closes, nil)
	if out == nil {
		t.Fatal("빈 창도 결과를 돌려줘야 한다 — nil은 오류와 구분이 안 된다")
	}
	if out.Coverage.TradingDays != 0 || out.Coverage.Note == "" {
		t.Errorf("거래일 0과 그 사유를 적어야 한다: %+v", out.Coverage)
	}
}

// 거래 비용은 바뀐 비중만큼만 문다 — 안 바뀌면 0이다.
func TestBacktestCostAppliesOnlyToTurnover(t *testing.T) {
	constituents := constituentsOf(map[string][]string{
		"2026-03-02": {"AAA"}, "2026-03-03": {"AAA"},
	})
	closes := map[string]map[string]float64{
		"AAA": {"2026-03-02": 100, "2026-03-03": 100, "2026-03-04": 100},
	}
	req := baseRequest()
	req.CostBps = 100 // 1%

	out := runBacktest(req, constituents,
		pickRebalanceDates(btSortedDates(constituents), model.RebalanceDaily),
		closes, nil)

	if math.Abs(out.Portfolio.TotalReturn) > 0.001 {
		t.Errorf("종목이 그대로면 비용이 붙지 않는다: %.4f%%", out.Portfolio.TotalReturn)
	}
}

// shiftDate walks calendar days — the fixtures do not care about weekends.
func shiftDate(base string, days int) string {
	t, err := time.Parse("2006-01-02", base)
	if err != nil {
		return base
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

/*
# 짧은 표본에서는 Sharpe도 내지 않는다

28거래일로 잰 Sharpe가 4.65였다(실측 2026-09-18, 국내 · 성장 · 상위20). 짧은 창은
표준편차를 작게 만들고 작은 분모는 Sharpe를 크게 만든다 — 운을 실력으로 읽은 숫자다.

변동성은 **남긴다**. 그건 성과 주장이 아니라 「얼마나 흔들렸나」라는 서술이고, 짧은
창에서도 사실이다.
*/
func TestBacktestWithholdsSharpeButKeepsVolatility(t *testing.T) {
	dates := map[string][]string{}
	closes := map[string]map[string]float64{"AAA": {}}
	price := 100.0
	// 25거래일 — 변동성 문턱(20)은 넘고 성과 문턱(40)은 못 넘는다.
	for i := 0; i < 26; i++ {
		date := shiftDate("2026-03-02", i)
		dates[date] = []string{"AAA"}
		closes["AAA"][date] = price
		if i%2 == 0 {
			price *= 1.02
		} else {
			price *= 0.99
		}
	}
	c := constituentsOf(dates)
	out := runBacktest(baseRequest(), c,
		pickRebalanceDates(btSortedDates(c), model.RebalanceDaily), closes, nil)

	if out.Coverage.TradingDays < model.BacktestMinSharpeDays ||
		out.Coverage.TradingDays >= model.BacktestMinDays {
		t.Fatalf("이 시험은 20~39거래일 구간을 재야 한다: %d", out.Coverage.TradingDays)
	}
	if out.Portfolio.Sharpe != nil {
		t.Errorf("%d거래일로 Sharpe를 내면 안 된다: %v", out.Coverage.TradingDays, *out.Portfolio.Sharpe)
	}
	if out.Portfolio.Volatility == nil {
		t.Error("변동성은 서술 통계다 — 이 구간에서도 나와야 한다")
	}
}

// 초과수익에 부동소수 꼬리가 남으면 화면이 「+6.7700000000000005%p」를 그린다.
func TestBacktestAlphaIsRounded(t *testing.T) {
	c := constituentsOf(map[string][]string{"2026-03-02": {"AAA"}})
	closes := map[string]map[string]float64{"AAA": {"2026-03-02": 100, "2026-03-03": 114.07}}
	benchmark := map[string]float64{"2026-03-02": 1000, "2026-03-03": 1073}

	out := runBacktest(baseRequest(), c,
		pickRebalanceDates(btSortedDates(c), model.RebalanceDaily), closes, map[string]map[string]float64{"kospi": benchmark})

	if out.Alpha == nil {
		t.Fatal("초과수익이 없다")
	}
	if got := *out.Alpha; got != btRound2(got) {
		t.Errorf("반올림되지 않았다: %v", got)
	}
}

/*
# 벤치마크는 화이트리스트를 지난 것만 남는다

임의의 index_id를 받으면 「VIX 대비 초과수익」을 요청 하나로 만들 수 있다. 숫자는
나오지만 뜻이 없고, 화면에 한 번 나가면 사람은 그것을 성적으로 읽는다.
*/
func TestNormalizeRejectsNonBenchmarkIndices(t *testing.T) {
	req := normalizeBacktestRequest(model.BacktestRequest{
		Market:     "domestic",
		Benchmarks: []string{"vix", "us10y", "btc", "kosdaq", "kosdaq"},
	})
	if len(req.Benchmarks) != 1 || req.Benchmarks[0] != "kosdaq" {
		t.Errorf("주식 시장 지수만, 중복 없이 남아야 한다: %v", req.Benchmarks)
	}

	// 하나도 안 남으면 시장 기본값으로 되돌아간다 — 빈 손으로 돌리지 않는다.
	fallback := normalizeBacktestRequest(model.BacktestRequest{
		Market:     "overseas",
		Benchmarks: []string{"vix", "kospi"}, // kospi는 해외 시장의 지수가 아니다
	})
	if len(fallback.Benchmarks) == 0 ||
		fallback.Benchmarks[0] != model.DefaultBacktestBenchmarks["overseas"][0] {
		t.Errorf("기본값으로 되돌아가야 한다: %v", fallback.Benchmarks)
	}
}

/*
# 지수가 쉰 날은 건너뛴다 — 0%로 세지 않는다

해외 지수는 국내 달력과 어긋나고 FRED 계열은 미국 공휴일에 빈다. 그 날을 보합으로
세면 지수가 실제보다 덜 움직인 것이 되고, 그만큼 포트폴리오의 초과수익이 **부풀려진다.**

아래에서 KOSPI는 03-03 종가가 없다. 포트폴리오는 사흘을 재지만 지수는 **하루만** 잰다
(03-04 → 03-05). 그 사실이 `Days`로 나가야 화면이 「같은 기간을 잰 숫자가 아니다」를
말할 수 있다.
*/
func TestBenchmarkSkipsItsOwnHolidays(t *testing.T) {
	c := constituentsOf(map[string][]string{"2026-03-02": {"AAA"}})
	closes := map[string]map[string]float64{
		"AAA": {"2026-03-02": 100, "2026-03-03": 110, "2026-03-04": 110, "2026-03-05": 110},
	}
	benchmarks := map[string]map[string]float64{
		// 03-03이 비어 있다 → 02→03, 03→04 두 구간을 건너뛰고 04→05만 잰다.
		"kospi": {"2026-03-02": 1000, "2026-03-04": 1000, "2026-03-05": 1020},
	}
	out := runBacktest(baseRequest(), c,
		pickRebalanceDates(btSortedDates(c), model.RebalanceDaily), closes, benchmarks)

	if out.Coverage.TradingDays != 3 {
		t.Fatalf("포트폴리오는 사흘을 잰다: %d", out.Coverage.TradingDays)
	}
	if len(out.Benchmarks) != 1 {
		t.Fatalf("벤치마크 한 줄: %+v", out.Benchmarks)
	}
	b := out.Benchmarks[0]
	if b.Days != 1 {
		t.Errorf("지수는 하루만 잰다(건너뛴 구간을 0%%로 세지 않는다): %d일", b.Days)
	}
	if math.Abs(b.Metrics.TotalReturn-2) > 0.01 {
		t.Errorf("잰 구간만 반영해 +2%%: %.2f%%", b.Metrics.TotalReturn)
	}
}

// 여러 벤치마크가 각자의 초과수익을 들고 나가고, 머리줄은 첫 번째(primary)의 것이다.
func TestBacktestReportsEveryBenchmarkSeparately(t *testing.T) {
	c := constituentsOf(map[string][]string{"2026-03-02": {"AAA"}})
	closes := map[string]map[string]float64{
		"AAA": {"2026-03-02": 100, "2026-03-03": 110},
	}
	benchmarks := map[string]map[string]float64{
		"kospi":  {"2026-03-02": 1000, "2026-03-03": 1040}, // +4%
		"kosdaq": {"2026-03-02": 800, "2026-03-03": 816},   // +2%
	}
	req := baseRequest()
	req.Benchmarks = []string{"kospi", "kosdaq"}

	out := runBacktest(req, c,
		pickRebalanceDates(btSortedDates(c), model.RebalanceDaily), closes, benchmarks)

	if len(out.Benchmarks) != 2 {
		t.Fatalf("둘 다 나와야 한다: %+v", out.Benchmarks)
	}
	byID := map[string]model.BacktestBenchmark{}
	for _, b := range out.Benchmarks {
		byID[b.ID] = b
	}
	if math.Abs(byID["kospi"].Alpha-6) > 0.01 {
		t.Errorf("KOSPI 대비 +6%%p: %v", byID["kospi"].Alpha)
	}
	if math.Abs(byID["kosdaq"].Alpha-8) > 0.01 {
		t.Errorf("KOSDAQ 대비 +8%%p: %v", byID["kosdaq"].Alpha)
	}
	if !byID["kospi"].Primary || byID["kosdaq"].Primary {
		t.Errorf("첫 번째가 기준이어야 한다: %+v", out.Benchmarks)
	}
	if out.Alpha == nil || math.Abs(*out.Alpha-6) > 0.01 {
		t.Errorf("머리줄 초과수익은 기준 벤치마크의 것이다: %v", out.Alpha)
	}
	// 곡선의 각 점이 지수별 값을 들고 있어야 선이 어긋나지 않는다.
	last := out.Curve[len(out.Curve)-1]
	if last.Benchmarks["kospi"] != 104 || last.Benchmarks["kosdaq"] != 102 {
		t.Errorf("곡선에 지수별 값이 실려야 한다: %+v", last.Benchmarks)
	}
}

// 이력이 없는 지수는 **행을 만들지 않는다** — 0%짜리 벤치마크가 서 있으면
// 포트폴리오가 시장을 이긴 것처럼 읽힌다.
func TestBacktestOmitsBenchmarkWithoutHistory(t *testing.T) {
	c := constituentsOf(map[string][]string{"2026-03-02": {"AAA"}})
	closes := map[string]map[string]float64{
		"AAA": {"2026-03-02": 100, "2026-03-03": 110},
	}
	req := baseRequest()
	req.Benchmarks = []string{"kospi", "kosdaq"}
	benchmarks := map[string]map[string]float64{
		"kosdaq": {"2026-03-02": 800, "2026-03-03": 816},
	}
	out := runBacktest(req, c,
		pickRebalanceDates(btSortedDates(c), model.RebalanceDaily), closes, benchmarks)

	if len(out.Benchmarks) != 1 || out.Benchmarks[0].ID != "kosdaq" {
		t.Fatalf("이력 있는 지수만: %+v", out.Benchmarks)
	}
	// 기준이 비었으면 실제로 잰 첫 번째로 옮긴다 — 머리줄이 비어 있으면 안 된다.
	if !out.Benchmarks[0].Primary || out.BenchmarkID != "kosdaq" || out.Alpha == nil {
		t.Errorf("기준을 실제로 잰 지수로 옮겨야 한다: %+v", out)
	}
}
