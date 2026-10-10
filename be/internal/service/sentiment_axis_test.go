package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func i64(v int64) *int64 { return &v }

// 재료가 없으면 **점수가 아니라 「없음」**이다. 0을 돌려주면 뉴스가 없다는 이유로
// 심리 최하위가 되고, 그 축의 가중치만큼 종합 점수가 통째로 깎인다.
func TestSentimentAxisAbsentWithoutMaterial(t *testing.T) {
	if _, _, ok := sentimentAxis(nil, nil); ok {
		t.Fatal("expected no axis without news or insider data")
	}
	if _, _, ok := sentimentAxis(&model.NewsSentimentSummary{}, nil); ok {
		t.Fatal("expected no axis from an empty sentiment tally")
	}
	// SharesChange가 없는 신고는 방향을 말하지 않으므로 재료가 아니다.
	blank := []model.InsiderTransaction{{Date: "2026-09-01"}, {Date: "2026-09-02"}}
	if _, _, ok := sentimentAxis(nil, blank); ok {
		t.Fatal("expected no axis from insider rows without share counts")
	}
}

func TestSentimentAxisNewsDirection(t *testing.T) {
	pos, _, ok := sentimentAxis(&model.NewsSentimentSummary{Total: 10, AverageScore: 80}, nil)
	if !ok {
		t.Fatal("expected an axis from positive news")
	}
	neg, _, ok := sentimentAxis(&model.NewsSentimentSummary{Total: 10, AverageScore: -80}, nil)
	if !ok {
		t.Fatal("expected an axis from negative news")
	}
	neutral, _, _ := sentimentAxis(&model.NewsSentimentSummary{Total: 10, AverageScore: 0}, nil)

	if pos <= neutral || neutral <= neg {
		t.Fatalf("expected positive(%.1f) > neutral(%.1f) > negative(%.1f)", pos, neutral, neg)
	}
	if neutral < 49 || neutral > 51 {
		t.Fatalf("all-neutral coverage should land on 50, got %.1f", neutral)
	}
}

// 표본이 얇으면 confidence가 낮아야 한다 — 그 값이 커버리지를 통해 점수를 중립
// 쪽으로 당기는 유일한 장치다.
func TestSentimentConfidenceGrowsWithSample(t *testing.T) {
	_, thin, _ := sentimentAxis(&model.NewsSentimentSummary{Total: 1, AverageScore: 100}, nil)
	_, thick, _ := sentimentAxis(&model.NewsSentimentSummary{Total: 20, AverageScore: 100}, nil)
	if thin >= thick {
		t.Fatalf("1 article (%.2f) should be less confident than 20 (%.2f)", thin, thick)
	}
	if thick != 1 {
		t.Fatalf("a full sample should reach confidence 1, got %.2f", thick)
	}
}

/*
내부자 거래는 **건수가 아니라 주식 수로** 잰다. 1주짜리 매수 세 건이 100만 주
매도 한 건을 이기면, 대주주 대량 매도가 소액 신고에 묻힌다.
*/
func TestInsiderSentimentWeighsShareVolumeNotRowCount(t *testing.T) {
	txns := []model.InsiderTransaction{
		{SharesChange: i64(1)},
		{SharesChange: i64(1)},
		{SharesChange: i64(1)},
		{SharesChange: i64(-1_000_000)},
	}
	score, _, ok := insiderSentimentComponent(txns)
	if !ok {
		t.Fatal("expected an insider component")
	}
	if score >= 50 {
		t.Fatalf("a large net sale should read bearish, got %.1f", score)
	}
}

// 한 축이 0이나 100으로 튀면 종합 점수가 신고 두 건에 몇 점씩 움직인다.
func TestInsiderSentimentStaysOffTheExtremes(t *testing.T) {
	allBuy, _, _ := insiderSentimentComponent([]model.InsiderTransaction{{SharesChange: i64(500)}})
	allSell, _, _ := insiderSentimentComponent([]model.InsiderTransaction{{SharesChange: i64(-500)}})
	if allBuy > 90 || allSell < 10 {
		t.Fatalf("extremes should be damped, got buy=%.1f sell=%.1f", allBuy, allSell)
	}
	if allBuy <= allSell {
		t.Fatalf("all-buy (%.1f) should read above all-sell (%.1f)", allBuy, allSell)
	}
}

// 배치 경로(표에서 합친 값)가 요약 경로(행 목록)와 같은 답을 내야 한다 — 같은
// 종목의 심리가 어느 길로 계산됐는지에 따라 달라지면 안 된다.
func TestSentimentTallyMatchesListPath(t *testing.T) {
	news := &model.NewsSentimentSummary{Total: 6, AverageScore: 40}
	a, ac, _ := newsSentimentComponent(news)
	b, bc, _ := newsComponentFromTally(6, 0.40)
	if a != b || ac != bc {
		t.Fatalf("news: list %.2f/%.2f vs tally %.2f/%.2f", a, ac, b, bc)
	}
	v, c, ok := sentimentAxisFromParts(b, bc, true, 0, 0, false)
	if !ok || v != b || c != bc {
		t.Fatalf("news-only axis should equal the news part, got %.2f/%.2f", v, c)
	}
}

// 수급 성분: 순매수면 50 위, 순매도면 아래, 표본이 짧으면 없음, 극단은 85/15 안.
func TestFlowComponent(t *testing.T) {
	if _, _, ok := flowComponent(1000, 10000, 3); ok {
		t.Fatal("3일 표본은 성분을 내면 안 된다")
	}
	if _, _, ok := flowComponent(1000, 0, 20); ok {
		t.Fatal("거래량 0이면 비율을 낼 수 없다")
	}
	up, conf, ok := flowComponent(1000, 10000, 20) // 10% → ≈77
	if !ok || up < 75 || up > 79 || conf != 1 {
		t.Fatalf("up = %.2f conf %.2f", up, conf)
	}
	down, _, _ := flowComponent(-1000, 10000, 20)
	if down > 25 || down < 21 {
		t.Fatalf("down = %.2f", down)
	}
	top, _, _ := flowComponent(10000, 10000, 20)
	if top > 85 {
		t.Fatalf("극단이 85를 넘었다: %.2f", top)
	}
}

// 수급이 없는 종목(미국 전부)은 이 변경으로 점수가 움직이지 않는다.
func TestFlowAbsentLeavesNewsInsiderMixUnchanged(t *testing.T) {
	a, ac, _ := sentimentAxisFromParts(70, 1, true, 40, 0.5, true)
	b, bc, _ := sentimentAxisWithFlow(70, 1, true, 40, 0.5, true, 0, 0, false)
	if a != b || ac != bc {
		t.Fatalf("%.4f/%.4f vs %.4f/%.4f", a, ac, b, bc)
	}
	with, _, _ := sentimentAxisWithFlow(70, 1, true, 40, 0.5, true, 90, 1, true)
	if with <= a {
		t.Fatalf("강한 순매수가 축을 올려야 한다: %.2f → %.2f", a, with)
	}
}
