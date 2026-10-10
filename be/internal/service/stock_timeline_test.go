package service

import (
	"database/sql"
	"math"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
타임라인 시험 — 재는 것은 **무엇을 보여 주기로 골랐는가**다.

조인은 SQL이 하고 그건 실측으로 확인했다. 여기서 틀리면 사용자가 손해를 보는 자리는
셋이다: ① 조용한 종목과 사나운 종목에 같은 잣대를 대는가 ② 뉴스만 있는 평범한 날이
목록을 채우는가 ③ 「수집 전」을 「아무 일도 없었다」로 그리는가.
*/

func point(date string, close, rate float64) model.TimelinePoint {
	r := rate
	return model.TimelinePoint{Date: date, Close: close, ChangeRate: &r}
}

/*
# 문턱은 종목마다 다르다

하루 8%가 예사인 종목과 1%도 드문 종목에 같은 잣대를 대면 한쪽은 전부 급등이고
다른 쪽은 아무 일도 없다. 그래서 그 창의 표준편차에서 뽑되, 사람이 「큰 움직임」이라
부르는 범위(2~10%)에 가둔다.
*/
func TestTimelineThresholdAdaptsToVolatility(t *testing.T) {
	quiet := []model.TimelinePoint{}
	wild := []model.TimelinePoint{}
	for i := 0; i < 20; i++ {
		sign := 1.0
		if i%2 == 1 {
			sign = -1
		}
		quiet = append(quiet, point("2026-03-02", 100, sign*0.2))
		wild = append(wild, point("2026-03-02", 100, sign*20))
	}

	_, quietT := timelineThreshold(quiet)
	if quietT != model.TimelineMoveFloor {
		t.Errorf("아주 조용한 종목은 바닥(%.1f%%)에 걸려야 한다: %.2f", model.TimelineMoveFloor, quietT)
	}
	_, wildT := timelineThreshold(wild)
	if wildT != model.TimelineMoveCeil {
		t.Errorf("아주 사나운 종목은 천장(%.1f%%)에 걸려야 한다: %.2f", model.TimelineMoveCeil, wildT)
	}

	// 그 사이에서는 실제로 변동성을 따라간다.
	mid := []model.TimelinePoint{}
	for i := 0; i < 20; i++ {
		sign := 1.0
		if i%2 == 1 {
			sign = -1
		}
		mid = append(mid, point("2026-03-02", 100, sign*3))
	}
	sd, midT := timelineThreshold(mid)
	if sd <= 0 {
		t.Fatalf("표준편차가 0이면 안 된다: %v", sd)
	}
	if math.Abs(midT-sd*model.TimelineMoveSigma) > 0.01 {
		t.Errorf("문턱은 표준편차 × %.1f다: sd=%.2f threshold=%.2f",
			model.TimelineMoveSigma, sd, midT)
	}
}

/*
# 뉴스만 있는 날은 목록에 세우지 않는다

삼성전자는 하루 평균 180건의 기사가 붙는다(실측). 뉴스 건수로 고르면 **모든 날**이
남고, 그러면 타임라인이 아니라 달력이 된다.
*/
func TestTimelineSkipsOrdinaryNewsOnlyDays(t *testing.T) {
	b := timelineBuild{
		closes: []model.TimelinePoint{
			point("2026-03-02", 100, 0.3),
			point("2026-03-03", 100, 0.1),
		},
		scores: map[string]model.TimelineScore{},
		news: []repository.TimelineDateCount{
			{Date: "2026-03-02", Count: 180},
			{Date: "2026-03-03", Count: 210},
		},
	}
	days := buildTimelineDays(b, 3.0)
	if len(days) != 0 {
		t.Errorf("조용한 날은 기사가 많아도 세우지 않는다: %+v", days)
	}
}

/*
# 분명한 사건은 등락과 무관하게 남긴다

「실적을 발표했는데 주가가 안 움직였다」도 답이다. 큰 등락만 고르면 그 사실이 사라진다.
*/
func TestTimelineKeepsHardEventsEvenWhenPriceIsFlat(t *testing.T) {
	b := timelineBuild{
		closes: []model.TimelinePoint{point("2026-03-02", 100, 0.1)},
		scores: map[string]model.TimelineScore{},
		earnings: []repository.TimelineEarningsRow{{
			Date:       "2026-03-02",
			FiscalYear: sql.NullInt64{Int64: 2026, Valid: true},
			Period:     sql.NullString{String: "1Q", Valid: true},
			EPSAct:     sql.NullFloat64{Float64: 1.20, Valid: true},
			EPSEst:     sql.NullFloat64{Float64: 1.00, Valid: true},
		}},
	}
	days := buildTimelineDays(b, 3.0)
	if len(days) != 1 {
		t.Fatalf("실적 발표일은 남아야 한다: %+v", days)
	}
	d := days[0]
	if d.Earnings == nil || d.Earnings.Surprise == nil {
		t.Fatalf("서프라이즈가 계산돼야 한다: %+v", d.Earnings)
	}
	if math.Abs(*d.Earnings.Surprise-20) > 0.01 {
		t.Errorf("1.00 → 1.20이면 +20%%: %.2f", *d.Earnings.Surprise)
	}
	if d.Earnings.Status != "actual" {
		t.Errorf("발표치가 있으면 actual이다: %q", d.Earnings.Status)
	}
	if len(d.Reasons) == 0 || d.Reasons[0] != "실적 발표" {
		t.Errorf("이 날이 왜 목록에 있는지 적어야 한다: %+v", d.Reasons)
	}
}

/*
# 컨센서스가 없으면 서프라이즈를 만들지 않는다

추정치가 없는데 0%로 적으면 「예상대로였다」가 되는데, 사실은 「비교할 예상이 없었다」다.
*/
func TestTimelineEarningsWithoutEstimateHasNoSurprise(t *testing.T) {
	out := timelineEarnings(repository.TimelineEarningsRow{
		Date:   "2026-03-02",
		EPSAct: sql.NullFloat64{Float64: 1.20, Valid: true},
	})
	if out.Surprise != nil {
		t.Errorf("추정이 없으면 서프라이즈도 없다: %v", *out.Surprise)
	}
	if out.Status != "actual" {
		t.Errorf("발표치는 있으므로 actual이다: %q", out.Status)
	}
}

// 큰 등락은 그 자체로 이유가 되고, 방향이 문장에 드러나야 한다.
func TestTimelineBigMoveBecomesAReason(t *testing.T) {
	b := timelineBuild{
		closes: []model.TimelinePoint{
			point("2026-03-02", 100, 6.4),
			point("2026-03-03", 94, -5.1),
			point("2026-03-04", 94, 0.2),
		},
		scores: map[string]model.TimelineScore{},
	}
	days := buildTimelineDays(b, 3.0)
	if len(days) != 2 {
		t.Fatalf("문턱을 넘은 이틀만: %+v", days)
	}
	// 최신순이다.
	if days[0].Date != "2026-03-03" || days[1].Date != "2026-03-02" {
		t.Errorf("최신순으로 나가야 한다: %s, %s", days[0].Date, days[1].Date)
	}
	if len(days[1].Reasons) == 0 || days[1].Reasons[0] != "급등" {
		t.Errorf("+6.4%%는 급등이다: %+v", days[1].Reasons)
	}
	if len(days[0].Reasons) == 0 || days[0].Reasons[0] != "급락" {
		t.Errorf("−5.1%%는 급락이다: %+v", days[0].Reasons)
	}
}

// 내부자 거래는 순증감까지 낸다 — 1만 주 사고 1만 주 판 날이 조용한 날로 보이면 안 된다.
func TestTimelineInsiderReportsBothSides(t *testing.T) {
	b := timelineBuild{
		closes: []model.TimelinePoint{point("2026-03-02", 100, 0.1)},
		scores: map[string]model.TimelineScore{},
		insider: []repository.TimelineInsiderDay{
			{Date: "2026-03-02", Count: 2, BuyShares: 10000, SellShares: 10000},
		},
	}
	days := buildTimelineDays(b, 3.0)
	if len(days) != 1 || days[0].Insider == nil {
		t.Fatalf("내부자 거래일은 남아야 한다: %+v", days)
	}
	in := days[0].Insider
	if in.BuyShares != 10000 || in.SellShares != 10000 || in.NetShares != 0 {
		t.Errorf("매수·매도·순증감을 모두 내야 한다: %+v", in)
	}
}

/*
# 「수집 전」과 「아무 일도 없었다」를 가른다

이 기능에서 가장 중요한 한 줄이다. 실측(005930): 종가는 2024-12-16부터인데 공시는
2026-08-20부터다. 1년을 그리면 공시 레인이 열 달 비는데 그것은 수집 전이다 (요구 8).
*/
func TestTimelineLanesReportWhenEachSourceBegan(t *testing.T) {
	starts := map[model.TimelineLaneID]string{
		model.TimelineLaneClose:      "2024-12-16",
		model.TimelineLaneDisclosure: "2026-08-20",
		// 내부자는 이 종목에 자료가 아예 없다.
	}
	lanes := buildTimelineLanes(starts, "2026-06-20", timelineBuild{
		closes:      []model.TimelinePoint{point("2026-06-21", 100, 0.1)},
		disclosures: []repository.TimelineDisclosureRow{{Date: "2026-08-20", Title: "t"}},
	})

	byID := map[model.TimelineLaneID]model.TimelineLane{}
	for _, l := range lanes {
		byID[l.ID] = l
	}
	if len(lanes) != len(model.TimelineLaneOrder) {
		t.Fatalf("레인은 언제나 전부 나간다(비어 있어도): %d", len(lanes))
	}
	if !byID[model.TimelineLaneClose].CoversWindow {
		t.Error("종가는 창보다 먼저 시작했으므로 창 전체를 덮는다")
	}
	if byID[model.TimelineLaneDisclosure].CoversWindow {
		t.Error("공시는 창 중간부터라 창 전체를 덮지 못한다 — 그 앞은 「수집 전」이다")
	}
	if byID[model.TimelineLaneDisclosure].AvailableFrom != "2026-08-20" {
		t.Errorf("언제부터인지 적어야 한다: %q", byID[model.TimelineLaneDisclosure].AvailableFrom)
	}
	insider := byID[model.TimelineLaneInsider]
	if insider.AvailableFrom != "" || insider.CoversWindow {
		t.Errorf("자료가 한 건도 없으면 시작일도 없다: %+v", insider)
	}
}

// 큰 점수 변화는 그 자체로 표시할 만한 날이다 — 주가가 조용해도 판단의 근거가 바뀐다.
func TestTimelineScoreJumpBecomesAReason(t *testing.T) {
	delta := 7
	b := timelineBuild{
		closes: []model.TimelinePoint{point("2026-03-02", 100, 0.2)},
		scores: map[string]model.TimelineScore{
			"2026-03-02": {Value: ptrInt(62), Delta: &delta, Driver: "추세 40.0 → 70.0"},
		},
	}
	days := buildTimelineDays(b, 3.0)
	if len(days) != 1 {
		t.Fatalf("점수가 크게 움직인 날은 남는다: %+v", days)
	}
	if days[0].Score == nil || days[0].Score.Driver == "" {
		t.Errorf("무엇이 점수를 바꿨는지 함께 나가야 한다: %+v", days[0].Score)
	}
	found := false
	for _, r := range days[0].Reasons {
		if r == "AI 점수 +7" {
			found = true
		}
	}
	if !found {
		t.Errorf("점수 변화가 이유로 적혀야 한다: %+v", days[0].Reasons)
	}

	// 작은 변화는 이유가 되지 않는다 — 매일 1~2점은 움직인다.
	small := 1
	b.scores = map[string]model.TimelineScore{
		"2026-03-02": {Value: ptrInt(62), Delta: &small},
	}
	if got := buildTimelineDays(b, 3.0); len(got) != 0 {
		t.Errorf("1점 변화로 날을 세우면 목록이 매일로 찬다: %+v", got)
	}
}

func ptrInt(v int) *int { return &v }
