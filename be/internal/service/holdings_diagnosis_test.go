package service

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
진단 시험 — 재는 것은 **무엇을 근거로 말하는가**다.

이 기능이 틀리면 사용자가 손해를 보는 자리는 넷이다: ① 없는 것을 있는 척하는가(매수
시점 점수) ② 수익률로 판정하는가(동어반복) ③ 신호 하나로 판정하는가 ④ 컨센서스 없는
실적을 「예상대로」로 세는가.
*/

func scoreRow(date string, score int) repository.ScoreLightRow {
	return repository.ScoreLightRow{
		Symbol:    "005930",
		TradeDate: date,
		Score:     sql.NullInt64{Int64: int64(score), Valid: true},
		Coverage:  1,
	}
}

func holdingFixture(avg float64) model.UserHolding {
	return model.UserHolding{Symbol: "005930", Name: "삼성전자", Quantity: 10, AvgPrice: &avg}
}

/*
# 매수 시점 점수는 대부분 **없다**

실측: 매수 2026-03-03, 점수 보관 시작 2026-08-07. 다섯 달 전 점수는 지워졌고 되살릴 수
없다. 빈칸으로 두면 화면이 「안 변했다」로 읽는다.
*/
func TestDiagnosisSaysWhyPurchaseScoreIsMissing(t *testing.T) {
	got, reason := scoreAtPurchase(holdingInputs{
		firstBought: "2026-03-03",
		retention:   "2026-08-07",
		scores:      []repository.ScoreLightRow{scoreRow("2026-09-17", 46)},
	})
	if got != nil {
		t.Errorf("보관 밖 매수일의 점수를 지어내면 안 된다: %v", *got)
	}
	if reason != "out_of_retention" {
		t.Errorf("왜 없는지 적어야 한다: %q", reason)
	}

	// 매매 이력 자체가 없는 보유(화면에서 직접 입력)는 다른 사유다.
	if _, r := scoreAtPurchase(holdingInputs{retention: "2026-08-07"}); r != "no_trade_history" {
		t.Errorf("이력 없음과 보관 밖은 다른 답이다: %q", r)
	}

	// 보관 창 안에서 산 종목은 실제로 답이 나온다.
	v, r := scoreAtPurchase(holdingInputs{
		firstBought: "2026-08-10",
		retention:   "2026-08-07",
		scores: []repository.ScoreLightRow{
			scoreRow("2026-09-17", 46), scoreRow("2026-08-11", 73), scoreRow("2026-08-07", 59),
		},
	})
	if v == nil || *v != 73 || r != "" {
		t.Errorf("매수일 이후 첫 회차(08-11의 73)를 써야 한다: %v %q", v, r)
	}
}

/*
# 수익률은 판정에 쓰이지 않는다

「올랐으니 이유가 강화됐다」는 동어반복이다 — 판단의 근거가 아니라 결과다. 아래는
**+80% 수익**인데 신호는 전부 나쁜 종목이고, 답은 「약화」여야 한다.
*/
func TestDiagnosisIgnoresReturnWhenJudging(t *testing.T) {
	out := buildHoldingDiagnosis(holdingInputs{
		holding:     holdingFixture(100),
		symbol:      "005930",
		firstBought: "2026-08-10",
		retention:   "2026-08-07",
		lastClose:   repository.DiagnosisCloseRow{Date: "2026-09-17", Close: 180}, // +80%
		scores: []repository.ScoreLightRow{
			scoreRow("2026-09-17", 40), scoreRow("2026-08-10", 70), // −30
		},
		insider: []repository.DiagnosisInsiderRow{
			{Date: "2026-09-01", Count: 2, SellShares: 5000},
		},
		settings: model.DefaultScoreSettings(),
	})

	if out.ReturnPct == nil || *out.ReturnPct != 80 {
		t.Fatalf("수익률은 화면에 나가야 한다: %v", out.ReturnPct)
	}
	if out.Verdict != model.DiagnosisWeaker {
		t.Errorf("신호가 전부 나쁘면 수익이 나도 약화다: %v", out.Verdict)
	}
}

/*
# 약한 신호 하나로는 판정하지 않는다 — 그래도 **셋을 전부 보여 준다**

하나로 판정하면 그것은 판단이 아니라 그 신호의 별명이다. 그 하나가 아주 클 때는 예외이고
(`TestDiagnosisStrongSingleSignalStillJudges`), 여기서는 문턱을 겨우 넘는 경우를 잰다.

그리고 **못 쓴 신호도 목록에 남는다** — 무엇을 보고 판단했는지보다 무엇을 **못 보고**
판단했는지가 더 중요할 때가 있다.
*/
func TestDiagnosisWithholdsVerdictOnThinEvidence(t *testing.T) {
	out := buildHoldingDiagnosis(holdingInputs{
		holding:     holdingFixture(100),
		symbol:      "005930",
		firstBought: "2026-08-10",
		retention:   "2026-08-07",
		scores: []repository.ScoreLightRow{
			// −6점: 문턱(5)은 넘지만 하나로 판정할 만큼(10)은 아니다.
			scoreRow("2026-09-17", 64), scoreRow("2026-08-10", 70),
		},
		// 실적도 내부자도 없다 — 쓸 수 있는 신호가 점수 하나뿐이다.
		settings: model.DefaultScoreSettings(),
	})

	if out.Verdict != model.DiagnosisUnknown {
		t.Errorf("신호 하나로는 판정하지 않는다: %v", out.Verdict)
	}
	available := 0
	for _, s := range out.Signals {
		if s.Available {
			available++
		}
	}
	if available != 1 {
		t.Fatalf("이 시험은 신호 하나짜리를 재야 한다: %d", available)
	}
	// **못 쓴 신호도 목록에 남는다** — 무엇을 못 보고 판단했는지가 더 중요할 때가 있다.
	if len(out.Signals) != 3 {
		t.Errorf("신호 셋이 언제나 목록에 있어야 한다: %d", len(out.Signals))
	}
}

// 세 신호가 모두 좋으면 강화. 합산 규칙을 잠근다.
func TestDiagnosisAddsSignalsUp(t *testing.T) {
	out := buildHoldingDiagnosis(holdingInputs{
		holding:     holdingFixture(100),
		symbol:      "005930",
		firstBought: "2026-08-10",
		retention:   "2026-08-07",
		scores: []repository.ScoreLightRow{
			scoreRow("2026-09-17", 78), scoreRow("2026-08-10", 60), // +18
		},
		earnings: []repository.DiagnosisEarningsRow{{
			Date:       "2026-09-01",
			FiscalYear: sql.NullInt64{Int64: 2026, Valid: true},
			Period:     sql.NullString{String: "2Q", Valid: true},
			EPSAct:     sql.NullFloat64{Float64: 1.30, Valid: true},
			EPSEst:     sql.NullFloat64{Float64: 1.00, Valid: true}, // +30%
		}},
		insider: []repository.DiagnosisInsiderRow{
			{Date: "2026-09-02", Count: 1, BuyShares: 8000},
		},
		settings: model.DefaultScoreSettings(),
	})

	if out.Verdict != model.DiagnosisStronger {
		t.Errorf("세 신호가 모두 좋으면 강화다: %v", out.Verdict)
	}
	if out.Earnings == nil || out.Earnings.Surprise == nil {
		t.Fatalf("서프라이즈가 계산돼야 한다: %+v", out.Earnings)
	}
	if *out.Earnings.Surprise != 30 {
		t.Errorf("1.00 → 1.30이면 +30%%: %v", *out.Earnings.Surprise)
	}
}

/*
# 컨센서스가 없으면 실적 신호를 쓰지 않는다

추정치 없이 0%로 세면 「예상대로였다」가 되는데, 사실은 「비교할 예상이 없었다」다.
*/
func TestDiagnosisEarningsWithoutEstimateIsNotASignal(t *testing.T) {
	sig := earningsSignal(&model.DiagnosisEarnings{Count: 1, Period: "2026 2Q"}, "2026-08-10")
	if sig.Available {
		t.Error("컨센서스가 없으면 신호로 쓰지 않는다")
	}
	if sig.Reason != "no_estimate" || sig.Detail == "" {
		t.Errorf("왜 못 쓰는지 적어야 한다: %+v", sig)
	}
}

// 보유 기간 밖의 사건은 세지 않는다 — 3월에 산 종목에 2월 실적을 붙이면 안 된다.
func TestDiagnosisCountsOnlyEventsInsideTheHoldingPeriod(t *testing.T) {
	in := holdingInputs{
		firstBought: "2026-08-10",
		insider: []repository.DiagnosisInsiderRow{
			{Date: "2026-07-01", Count: 5, BuyShares: 99999}, // 매수 전
			{Date: "2026-09-01", Count: 1, BuyShares: 100},
		},
	}
	got := diagnosisInsider(in)
	if got == nil || got.Count != 1 || got.BuyShares != 100 {
		t.Errorf("보유 기간 안의 한 건만 세야 한다: %+v", got)
	}
}

// 점수 잔변동으로 방향을 바꾸지 않는다 — 매일 1~2점은 움직인다.
func TestDiagnosisScoreSignalNeedsARealMove(t *testing.T) {
	small := &model.DiagnosisScore{
		Current: ptrInt(62), FromValue: ptrInt(60), Delta: ptrInt(2), FromDate: "2026-08-07",
	}
	if got := scoreTrendSignal(small, "2026-08-07"); got.Direction != 0 {
		t.Errorf("2점 변화는 방향이 아니다: %+v", got)
	}
	big := &model.DiagnosisScore{
		Current: ptrInt(46), FromValue: ptrInt(59), Delta: ptrInt(-13), FromDate: "2026-08-07",
	}
	got := scoreTrendSignal(big, "2026-08-07")
	if !got.Available || got.Direction != -1 {
		t.Errorf("−13점은 악화다: %+v", got)
	}
	if got.Detail == "" {
		t.Error("숫자로 지은 문장이 함께 나가야 한다")
	}
}

// 목록은 **약화부터**. 이 화면을 여는 이유가 「무엇이 나빠졌나」다.
func TestDiagnosisVerdictOrdering(t *testing.T) {
	ranks := []model.DiagnosisVerdict{
		model.DiagnosisWeaker, model.DiagnosisUnknown,
		model.DiagnosisSteady, model.DiagnosisStronger,
	}
	for i := 1; i < len(ranks); i++ {
		if verdictRank(ranks[i-1]) >= verdictRank(ranks[i]) {
			t.Fatalf("약화 → 보류 → 유지 → 강화 순이어야 한다: %v", ranks)
		}
	}
}

// 보관 한계는 **몇 종목이 그런지**와 함께 말한다 — 「비교할 수 없습니다」만 쓰면 고장으로 읽힌다.
func TestDiagnosisNoteCountsAffectedHoldings(t *testing.T) {
	items := []model.HoldingDiagnosis{
		{ScoreAtPurchaseReason: "out_of_retention"},
		{ScoreAtPurchaseReason: "out_of_retention"},
		{ScoreAtPurchaseReason: ""},
	}
	note := diagnosisNote("2026-08-07", items)
	if note == "" {
		t.Fatal("안내가 비어 있다")
	}
	if !strings.Contains(note, "2종목") || !strings.Contains(note, "08-07") {
		t.Errorf("몇 종목인지와 언제부터인지를 적어야 한다: %q", note)
	}
	// 해당 종목이 없으면 안내도 없다 — 언제나 뜨는 경고는 읽히지 않는다.
	if diagnosisNote("2026-08-07", []model.HoldingDiagnosis{{}}) != "" {
		t.Error("해당 종목이 없으면 안내를 띄우지 않는다")
	}
}

/*
# 점수 추세는 **보유 기간 안에서** 잰다

실측 결함(2026-09-18): 08-11에 산 종목의 추세가 08-07(매수 전)부터 잡혔다 — 살기 전의
점수 변화가 보유 판단에 들어갔다. 실적·내부자는 보유 기간으로 자르면서 점수만 안 잘랐다.
*/
func TestDiagnosisScoreTrendStartsAfterPurchase(t *testing.T) {
	sc := diagnosisScore(holdingInputs{
		firstBought: "2026-08-11",
		scores: []repository.ScoreLightRow{
			scoreRow("2026-09-17", 29),
			scoreRow("2026-08-11", 75),
			scoreRow("2026-08-07", 67), // 매수 **전** — 기준점이 되면 안 된다
		},
		settings: model.DefaultScoreSettings(),
	})
	if sc == nil || sc.FromDate != "2026-08-11" {
		t.Fatalf("기준점은 매수 뒤 첫 회차여야 한다: %+v", sc)
	}
	if sc.FromValue == nil || *sc.FromValue != 75 || sc.Delta == nil || *sc.Delta != -46 {
		t.Errorf("75 → 29 = −46이어야 한다: %+v", sc)
	}

	// 매수일이 보관 창보다 오래됐으면 창의 첫 회차가 기준이다 — 그게 가진 전부다.
	old := diagnosisScore(holdingInputs{
		firstBought: "2026-03-03",
		scores: []repository.ScoreLightRow{
			scoreRow("2026-09-17", 46), scoreRow("2026-08-07", 59),
		},
		settings: model.DefaultScoreSettings(),
	})
	if old == nil || old.FromDate != "2026-08-07" {
		t.Errorf("보관 창의 첫 회차를 써야 한다: %+v", old)
	}
}

/*
# 강한 신호 하나면 판정한다

실측 결함(2026-09-18): 카카오는 매수 시점 75점이 29점으로 **46점** 떨어졌는데 실적·내부자
기록이 없어 「판단 보류」가 됐다. 사용자가 가장 봐야 할 종목이 가장 흐린 답을 받았다.
약한 것은 증거의 **다양성**이지 강도가 아니다.
*/
func TestDiagnosisStrongSingleSignalStillJudges(t *testing.T) {
	out := buildHoldingDiagnosis(holdingInputs{
		holding:     holdingFixture(50000),
		symbol:      "035720",
		firstBought: "2026-08-11",
		retention:   "2026-08-07",
		scores: []repository.ScoreLightRow{
			scoreRow("2026-09-17", 29), scoreRow("2026-08-11", 75), // −46
		},
		settings: model.DefaultScoreSettings(),
	})
	if out.Verdict != model.DiagnosisWeaker {
		t.Errorf("−46점이면 근거가 하나여도 약화다: %v", out.Verdict)
	}
	if out.SignalCount != 1 {
		t.Errorf("근거가 몇 개인지 화면이 적을 수 있어야 한다: %d", out.SignalCount)
	}

	// 문턱을 겨우 넘는 정도(−6점)는 여전히 보류다 — 하나로 판정할 만큼 크지 않다.
	weak := buildHoldingDiagnosis(holdingInputs{
		holding:     holdingFixture(50000),
		symbol:      "035720",
		firstBought: "2026-08-11",
		retention:   "2026-08-07",
		scores: []repository.ScoreLightRow{
			scoreRow("2026-09-17", 64), scoreRow("2026-08-11", 70), // −6
		},
		settings: model.DefaultScoreSettings(),
	})
	if weak.Verdict != model.DiagnosisUnknown {
		t.Errorf("작은 변화 하나로는 판정하지 않는다: %v", weak.Verdict)
	}
}

// 신호가 하나도 없으면 언제나 보류다.
func TestDiagnosisNoSignalsIsAlwaysUnknown(t *testing.T) {
	v, n := verdictFrom([]model.DiagnosisSignal{
		{ID: model.SignalScoreTrend, Reason: "no_data"},
		{ID: model.SignalEarnings, Reason: "no_data"},
		{ID: model.SignalInsider, Reason: "no_data"},
	})
	if v != model.DiagnosisUnknown || n != 0 {
		t.Errorf("근거가 없으면 보류다: %v %d", v, n)
	}
}

/*
# 하루 거래량의 1%도 안 되는 내부자 거래는 **방향이 아니다**

실측(2026-09-18): 애플은 3건·4,316주 순매도였는데, 그것은 하루 거래량 4,084만 주의
0.011%다. 크기 개념이 없던 시절 이 값은 엔비디아의 563만 주와 똑같이 −1로 세어졌고,
점수 +22(강한 신호)를 상쇄해 「유지」를 만들었다.

문턱에 못 미치면 신호를 **버리는 게 아니라 방향을 0으로** 둔다 — 재 보고 방향이
없었다는 것과 재지 못했다는 것은 다른 말이다.
*/
func TestDiagnosisInsiderNeedsSizeNotJustDirection(t *testing.T) {
	small := &model.DiagnosisInsider{Count: 3, SellShares: 4316, NetShares: -4316}

	tiny := insiderSignal(small, "2026-08-20", 40_841_269) // 0.011%
	if !tiny.Available {
		t.Fatal("잰 신호는 남아야 한다 — 「작다」와 「못 쟀다」는 다른 말이다")
	}
	if tiny.Direction != 0 {
		t.Errorf("하루 거래량의 0.011%%는 방향이 아니다: %d", tiny.Direction)
	}
	if !strings.Contains(tiny.Detail, "하루 거래량의") {
		t.Errorf("왜 0인지 화면이 읽을 수 있어야 한다: %q", tiny.Detail)
	}

	// 같은 주식 수라도 거래가 한산한 종목에서는 방향이 된다.
	big := insiderSignal(small, "2026-08-20", 100_000)
	if big.Direction != -1 {
		t.Errorf("하루 거래량의 4.3%%는 방향이다: %d", big.Direction)
	}

	// 거래량을 모르면 문턱을 적용하지 않는다 — 못 잰 것을 「작다」고 부르지 않는다.
	blind := insiderSignal(small, "2026-08-20", 0)
	if blind.Direction != -1 {
		t.Errorf("잴 수 없을 때는 예전대로 방향을 낸다: %d", blind.Direction)
	}
	if strings.Contains(blind.Detail, "하루 거래량의") {
		t.Errorf("재지 못한 비중을 적으면 안 된다: %q", blind.Detail)
	}
}

/*
# **침묵하는 신호는 판정을 뒤집지 못한다**

실측에서 카카오는 강한 −46 하나로 「약화」였는데, 삼성전자는 강한 −13 옆에 방향 0짜리
신호가 하나 붙자 합이 문턱(2)에 못 미쳐 「유지」가 됐다. 아무 말도 안 하는 증거를
더했다고 답이 바뀌면 그것은 판정이 아니라 신호 개수 세기다.

그래서 강한 신호는 두 몫을 센다 (`model.DiagnosisStrongWeight`).
*/
func TestDiagnosisSilentSignalCannotOverturnAStrongOne(t *testing.T) {
	strong := model.DiagnosisSignal{
		ID: model.SignalScoreTrend, Available: true, Direction: -1, Strong: true,
	}
	silent := model.DiagnosisSignal{
		ID: model.SignalInsider, Available: true, Direction: 0,
	}

	alone, _ := verdictFrom([]model.DiagnosisSignal{strong})
	withSilent, n := verdictFrom([]model.DiagnosisSignal{strong, silent})
	if alone != model.DiagnosisWeaker || withSilent != model.DiagnosisWeaker {
		t.Errorf("침묵이 답을 바꿨다: 혼자 %v, 침묵 추가 %v", alone, withSilent)
	}
	if n != 2 {
		t.Errorf("방향이 0이어도 잰 신호는 근거 수에 든다: %d", n)
	}

	// 반대로 **말하는** 신호는 강한 신호를 되돌릴 수 있어야 한다.
	opposed, _ := verdictFrom([]model.DiagnosisSignal{
		strong,
		{ID: model.SignalInsider, Available: true, Direction: 1},
	})
	if opposed != model.DiagnosisSteady {
		t.Errorf("정면으로 엇갈리면 유지다: %v", opposed)
	}
}

// 천 단위 쉼표 — 일곱 자리는 쉼표 없이 읽히지 않는다.
func TestDiagnosisGroupsDigits(t *testing.T) {
	cases := map[float64]string{
		0: "0", 7: "7", 42: "42", 999: "999", 1000: "1,000",
		4316: "4,316", 25668: "25,668", 5634565: "5,634,565",
	}
	for in, want := range cases {
		if got := groupDigits(in); got != want {
			t.Errorf("groupDigits(%v) = %q, want %q", in, got, want)
		}
	}
	if got := signedShares(-5634565); got != "매도 5,634,565주" {
		t.Errorf("순매도 문장: %q", got)
	}
}
