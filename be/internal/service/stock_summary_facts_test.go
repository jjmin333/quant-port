package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestClassifyRSIBands(t *testing.T) {
	cases := map[float64]string{
		75: "overbought", 60: "strong", 50: "neutral", 37.7: "weak", 25: "oversold",
	}
	for rsi, want := range cases {
		if got := classifyRSI(rsi); got != want {
			t.Errorf("RSI %.1f → %q, want %q", rsi, got, want)
		}
	}
}

// A histogram that is negative but closing is "recovering", not "bearish" —
// the distinction the old payload left the model to infer from raw MACD values.
func TestClassifyMACDStates(t *testing.T) {
	cases := []struct {
		macd, signal float64
		want         string
	}{
		{1.5, 1.0, "bullish"},
		{1.05, 1.0, "recovering"},
		{0.95, 1.0, "weakening"},
		{0.5, 1.0, "bearish"},
		{0, 0, "flat"},
	}
	for _, tc := range cases {
		if got := classifyMACD(tc.macd, tc.signal); got != tc.want {
			t.Errorf("macd=%.2f signal=%.2f → %q, want %q", tc.macd, tc.signal, got, tc.want)
		}
	}
}

func TestClassifyTrend(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	bearish := &TechnicalIndicators{LastClose: f(100), SMA20: f(110), SMA50: f(120), SMA200: f(130)}
	trend, below := classifyTrend(bearish)
	if trend != "bearish" || below == nil || !*below {
		t.Errorf("모든 이동평균 아래면 bearish + belowMajorMA=true, got %q %v", trend, below)
	}

	mixed := &TechnicalIndicators{LastClose: f(115), SMA20: f(110), SMA50: f(120)}
	if trend, below := classifyTrend(mixed); trend != "mixed" || *below {
		t.Errorf("엇갈리면 mixed + belowMajorMA=false, got %q %v", trend, *below)
	}

	if trend, _ := classifyTrend(nil); trend != "" {
		t.Errorf("데이터 없으면 빈 문자열이어야 함, got %q", trend)
	}
}

func TestClassifyEarningsTrend(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	if got := classifyEarningsTrend(f(60.2), f(96.2), f(164.6)); got != "strong_improving" {
		t.Errorf("전 항목 큰 폭 개선 → strong_improving, got %q", got)
	}
	if got := classifyEarningsTrend(f(3), f(4), f(2)); got != "improving" {
		t.Errorf("전 항목 소폭 개선 → improving, got %q", got)
	}
	if got := classifyEarningsTrend(f(-5), f(-10), f(-2)); got != "deteriorating" {
		t.Errorf("전 항목 악화 → deteriorating, got %q", got)
	}
	if got := classifyEarningsTrend(f(10), f(-5), nil); got != "mixed" {
		t.Errorf("엇갈림 → mixed, got %q", got)
	}
	if got := classifyEarningsTrend(nil, nil, nil); got != "" {
		t.Errorf("데이터 없으면 빈 문자열, got %q", got)
	}
}

func TestInsiderDirection(t *testing.T) {
	cases := []struct {
		buy, sell int
		want      string
	}{
		{23, 7, "net_buying"}, {2, 9, "net_selling"},
		{6, 5, "slight_buying"}, {5, 6, "slight_selling"},
		{5, 5, "balanced"}, {0, 0, ""},
	}
	for _, tc := range cases {
		if got := insiderDirection(tc.buy, tc.sell); got != tc.want {
			t.Errorf("buy=%d sell=%d → %q, want %q", tc.buy, tc.sell, got, tc.want)
		}
	}
}

// The same wire story arriving from several outlets must be sent once: it costs
// tokens twice over and skews the sentiment aggregate toward whatever was
// syndicated most.
func TestDedupeHeadlines(t *testing.T) {
	articles := []model.NewsArticle{
		{ID: "1", HeadlineKo: "SK하이닉스, 54조원 규모 투자 확정", PublisherName: "연합뉴스"},
		{ID: "2", HeadlineKo: "SK하이닉스, 54조원 규모 투자 확정 - 매일경제", PublisherName: "매일경제"},
		{ID: "3", HeadlineKo: "메모리 업황 둔화 우려", PublisherName: "한국경제"},
	}
	got := dedupeHeadlines(articles, 12)
	if len(got) != 2 {
		t.Fatalf("중복 제거 후 2건이어야 함, got %d: %+v", len(got), got)
	}
	// Summaries must not survive into the payload — they were the bulk of the
	// old 10.7KB facts blob.
	for _, a := range got {
		if a.Headline == "" {
			t.Error("헤드라인이 비어 있으면 안 됨")
		}
	}
}

func TestDedupeHeadlinesRespectsLimit(t *testing.T) {
	articles := make([]model.NewsArticle, 0, 30)
	for i := 0; i < 30; i++ {
		articles = append(articles, model.NewsArticle{
			ID: string(rune('a' + i)), HeadlineKo: "서로 다른 기사 제목 " + string(rune('a'+i)),
		})
	}
	if got := dedupeHeadlines(articles, 12); len(got) != 12 {
		t.Errorf("상한 12건이어야 함, got %d", len(got))
	}
}

func TestAxisStrengthsWeaknesses(t *testing.T) {
	strengths, weaknesses := axisStrengthsWeaknesses(map[string]int{
		"technical": 29, "quality": 82, "growth": 89, "valuation": 3,
	})
	if len(strengths) != 2 || strengths[0] != "growth" || strengths[1] != "quality" {
		t.Errorf("상위 2축이 강점이어야 함, got %v", strengths)
	}
	if len(weaknesses) != 2 {
		t.Errorf("하위 2축이 약점이어야 함, got %v", weaknesses)
	}
}

// The compact payload must stay far smaller than the ~10.7KB it replaced,
// while still carrying the score that the old one omitted entirely.
func TestCompactFactsAreSmallAndCarryScore(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	stock := &model.Stock{Symbol: "000660", Name: "SK하이닉스", Currency: model.CurrencyKRW}
	technical := &TechnicalIndicators{
		LastClose: f(1466000), SMA20: f(1600000), SMA50: f(1700000), SMA200: f(1800000),
		RSI14: f(37.7), MACD: f(1.05), MACDSignal: f(1.0),
		PctFromHigh52Week: f(-51.2), Return5D: f(-3.2),
	}
	fundamentals := &model.FundamentalsView{PER: f(24.9), PBR: f(8.9)}
	fundamentals.ROE = f(44.1)
	fundamentals.OperatingMargin = f(48.6)

	score := 53
	top := 46
	scoreView := &model.StockScoreView{
		Symbol: "000660", TradeDate: "2026-08-07", Score: &score, Grade: "B",
		PeerPct: &top, PeerGroup: "domestic:KOSPI", Coverage: 0.9,
		Breakdown: model.ScoreBreakdown{
			Technical: f(29), Quality: f(82), Growth: f(89), Valuation: f(3),
		},
		Inputs: map[string]any{"percentile": map[string]any{"per": 3.0, "roe": 99.0}},
	}

	articles := make([]model.NewsArticle, 0, 12)
	for i := 0; i < 12; i++ {
		articles = append(articles, model.NewsArticle{
			ID:         string(rune('a' + i)),
			HeadlineKo: "SK하이닉스 관련 기사 제목 " + string(rune('a'+i)),
		})
	}

	facts := buildStockSummaryFacts(stock, fundamentals, nil, nil, technical, articles, scoreView)
	raw, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}

	if len(raw) > 2500 {
		t.Errorf("압축 페이로드가 %d바이트 — 2500 이하를 기대", len(raw))
	}
	if facts.Score == nil || facts.Score.Total != 53 {
		t.Fatal("점수가 페이로드에 포함되어야 함")
	}
	if len(facts.Score.Strengths) == 0 || len(facts.Score.Weaknesses) == 0 {
		t.Error("강점/약점 축이 서버에서 계산되어야 함")
	}
	// 배치는 백분위(높을수록 우수)로 저장하고, 페이로드는 화면과 같은 상위 N%
	// (낮을수록 우수)로 뒤집어 넘긴다. per 백분위 3 → 상위 97%, roe 99 → 상위 1%.
	// 두 방향이 한 페이로드에 섞여 있던 것이 「동종 상위 94%」의 원인이었다.
	if facts.Fundamental == nil || facts.Fundamental.PERTopPercent == nil || *facts.Fundamental.PERTopPercent != 97 {
		t.Errorf("PER 피어 순위가 상위 N%%로 담겨야 함, got %v", facts.Fundamental.PERTopPercent)
	}
	if facts.Fundamental.ROETopPercent == nil || *facts.Fundamental.ROETopPercent != 1 {
		t.Errorf("ROE 백분위 99는 상위 1%%여야 함, got %v", facts.Fundamental.ROETopPercent)
	}
	if facts.Technical == nil || facts.Technical.Trend != "bearish" || facts.Technical.MACD != "recovering" {
		t.Errorf("기술적 분류가 서버에서 이뤄져야 함, got %+v", facts.Technical)
	}
	// Raw indicator internals must not leak back in.
	if s := string(raw); contains(s, "sma200") || contains(s, "adx") || contains(s, "recentSwing") {
		t.Error("원지표 구조체가 페이로드에 새어 들어감")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

/*
개요는 **네 갈래를 다 거친다** (§8).

예전에는 화면이 `valuation`을 그대로 첫 줄로 썼다. 그래서 내부자가 대량 매도
중이어도 개요는 밸류에이션 이야기로 끝났다. 이 검사는 네 섹션이 모두 개요에
한 번씩 등장하는지, 그리고 "데이터 없음" 문장은 빠지는지를 본다.
*/
func TestComposeOverviewDrawsOnEverySection(t *testing.T) {
	view := &model.StockSummaryView{
		Valuation: "PER은 동종 대비 낮다. 두 번째 문장은 버려진다.",
		Technical: "주가는 20일선 위에 있다. 두 번째 문장.",
		Insider:   "임원 대량 매도가 이어졌다. 두 번째 문장.",
		News:      "실적 기대 기사가 다수다. 두 번째 문장.",
	}
	got := composeOverview(view)

	for _, want := range []string{"PER", "20일선", "대량 매도", "실적 기대"} {
		if !strings.Contains(got, want) {
			t.Fatalf("overview dropped %q: %s", want, got)
		}
	}
	if strings.Contains(got, "두 번째 문장") {
		t.Fatalf("overview should keep only each section's first sentence: %s", got)
	}
}

// 「없습니다」로만 끝나는 규칙 기반 문장은 개요 네 줄 중 한 줄을 쓸 값이 없다.
func TestComposeOverviewSkipsEmptySections(t *testing.T) {
	view := &model.StockSummaryView{
		Valuation: "PER은 동종 대비 낮다.",
		Technical: "기술적 지표를 계산할 캔들 데이터가 부족합니다.",
		Insider:   "최근 내부자 거래 내역이 없습니다.",
		News:      "최근 관련 뉴스가 없습니다.",
	}
	got := composeOverview(view)
	if got != "PER은 동종 대비 낮다." {
		t.Fatalf("expected only the informative sentence, got %q", got)
	}
	if composeOverview(&model.StockSummaryView{}) != "" {
		t.Fatal("an empty view should produce an empty overview, not filler")
	}
}
