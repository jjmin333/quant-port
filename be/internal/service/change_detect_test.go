package service

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/repository"
)

// 문턱은 종목 자신의 변동성이 정한다 — 같은 +6%가 조용한 종목에서는 급등이고
// 늘 크게 움직이는 종목에서는 아니다.
func TestSessionEventsUseOwnVolatility(t *testing.T) {
	calm := repository.PriceSessionRow{Symbol: "A", Date: "2026-09-22", ChangeRate: 6, StdDev: 1.5}
	wild := repository.PriceSessionRow{Symbol: "B", Date: "2026-09-22", ChangeRate: 6, StdDev: 4}
	if ev := sessionEvents(calm, false, 0); len(ev) != 1 || ev[0].Kind != "price_move" {
		t.Fatalf("calm +6%% should be a move, got %+v", ev)
	}
	if ev := sessionEvents(wild, false, 0); len(ev) != 0 {
		t.Fatalf("wild +6%% (σ 4 → 10%%) should not be a move, got %+v", ev)
	}
}

// 설명이 없으면 그대로 「설명 불가」로 남는다.
func TestSessionEventsCarryExplained(t *testing.T) {
	p := repository.PriceSessionRow{Symbol: "A", Date: "2026-09-22", ChangeRate: -9, StdDev: 2}
	ev := sessionEvents(p, false, 1)
	if len(ev) != 1 || ev[0].Explained == nil || *ev[0].Explained {
		t.Fatalf("want unexplained move, got %+v", ev)
	}
	if ev[0].Direction != -1 {
		t.Fatalf("direction = %d", ev[0].Direction)
	}
}

func TestSessionEventsVolumeSurge(t *testing.T) {
	p := repository.PriceSessionRow{Symbol: "A", Date: "2026-09-22", ChangeRate: 1, StdDev: 2, Volume: 600, AvgVolume: 100}
	ev := sessionEvents(p, true, 5)
	if len(ev) != 1 || ev[0].Kind != "volume_surge" {
		t.Fatalf("want one volume surge, got %+v", ev)
	}
}

// 예상이 없으면 서프라이즈를 만들지 않는다.
func TestEarningsEventWithoutEstimate(t *testing.T) {
	e := earningsEvent(repository.EarningsRow{Symbol: "A", Date: "2026-09-22",
		OpAct: sql.NullFloat64{Float64: 100, Valid: true}})
	if e.Direction != 0 || e.Subtype != "" || e.Magnitude != nil {
		t.Fatalf("no estimate must not become a surprise: %+v", e)
	}
}

func TestEarningsEventSurprise(t *testing.T) {
	e := earningsEvent(repository.EarningsRow{Symbol: "A", Date: "2026-09-22",
		EPSEst: sql.NullFloat64{Float64: 1.0, Valid: true}, EPSAct: sql.NullFloat64{Float64: 1.2, Valid: true}})
	if e.Direction != 1 || e.Subtype != "surprise" || e.Magnitude == nil || *e.Magnitude < 19 {
		t.Fatalf("want +20%% EPS surprise, got %+v", e)
	}
}

// 목표가 하향 기사는 감성과 무관하게 아래 방향이다(「하향에도 매수 유지」 같은 제목이
// 모델에게는 긍정으로 읽힌다).
func TestGroupNewsEventsTargetDirection(t *testing.T) {
	ev := groupNewsEvents([]repository.NewsEventRow{
		{Symbol: "A", EventType: "target_down", Score: 0.6, Headline: "목표가 하향에도 매수 유지", Date: "2026-09-22"},
	})
	if len(ev) != 1 || ev[0].Direction != -1 {
		t.Fatalf("target_down should point down, got %+v", ev)
	}
}

func TestInsiderEventThreshold(t *testing.T) {
	if _, ok := insiderEvent(repository.InsiderNetRow{Symbol: "A", Net: 10}, 1000); ok {
		t.Fatal("1% of volume should not count")
	}
	e, ok := insiderEvent(repository.InsiderNetRow{Symbol: "A", Net: -500}, 1000)
	if !ok || e.Direction != -1 {
		t.Fatalf("50%% net sell should count, got %+v %v", e, ok)
	}
}

// 남의 목표가 변경이 태그만 걸린 종목의 「최근 변화」가 되지 않는다(2026-09-26 실측 기사들).
func TestNewsRowsAboutTheirSymbol(t *testing.T) {
	universe := map[string]repository.ChangeUniverseRow{
		"NVDA":   {Symbol: "NVDA", Name: "엔비디아", EnglishName: "NVIDIA CORP"},
		"CRWV":   {Symbol: "CRWV", Name: "코어위브", EnglishName: "COREWEAVE INC-CL A"},
		"181710": {Symbol: "181710", Name: "NHN", EnglishName: "NHN CORP"},
		"005930": {Symbol: "005930", Name: "삼성전자", EnglishName: "SAMSUNG ELECTRONICS CO LTD"},
		"TGTX":   {Symbol: "TGTX", Name: "TG 테라퓨틱스", EnglishName: "TG THERAPEUTICS INC"},
		"AI":     {Symbol: "AI", Name: "C3.ai", EnglishName: "C3.AI INC-A"},
	}
	rows := []repository.NewsEventRow{
		{Symbol: "CRWV", Original: "CoreWeave Stock Upgraded On 'Favorable' AI Cloud Pricing Power", Headline: "'유리한' AI 클라우드 가격력으로 코어위브 주식 상향 조정"},
		{Symbol: "NVDA", Original: "CoreWeave Stock Upgraded On 'Favorable' AI Cloud Pricing Power", Headline: "'유리한' AI 클라우드 가격력으로 코어위브 주식 상향 조정"},
		{Symbol: "NVDA", Original: "3 High-Yield Stocks Near Their 52-Week Lows", Headline: "52주 저점에 근접한 고수익 주식 3개"},
		{Symbol: "NVDA", Original: "Nvidia's Rubin ramp lifts targets", Headline: "엔비디아 루빈 증산에 목표가 상향"},
		{Symbol: "181710", Original: "\"GPU 빌려주니 1000억 매출\"… NHN, AI 데이터센터로 기업가치 재평가", Headline: "\"GPU 빌려주니 1000억 매출\"… NHN, AI 데이터센터로 기업가치 재평가"},
		{Symbol: "NVDA", Original: "\"GPU 빌려주니 1000억 매출\"… NHN, AI 데이터센터로 기업가치 재평가", Headline: "\"GPU 빌려주니 1000억 매출\"… NHN, AI 데이터센터로 기업가치 재평가"},
		{Symbol: "005930", Original: "삼성전자, 30조 배당 예고", Headline: "삼성전자, 30조 배당 예고"},
		{Symbol: "TGTX", Original: "TG Therapeutics raises guidance", Headline: "TG테라퓨틱스, 가이던스 상향"},
		{Symbol: "AI", Original: "Analyst said the AI trade is overdone", Headline: "AI 거래가 과하다는 분석"},
		{Symbol: "ZZZ", Original: "ZZZ news", Headline: "ZZZ"},
	}
	got := newsRowsAboutTheirSymbol(rows, universe)
	var keep []string
	for _, r := range got {
		keep = append(keep, r.Symbol)
	}
	// 마지막 둘: 「AI」 티커는 낱말 「AI」에 걸린다 — 이건 막지 못한다(태그가 AI일 때만 해당).
	want := []string{"CRWV", "NVDA", "181710", "005930", "TGTX", "AI"}
	if strings.Join(keep, ",") != strings.Join(want, ",") {
		t.Fatalf("kept %v, want %v", keep, want)
	}
}

func TestEnglishCoreName(t *testing.T) {
	for in, want := range map[string]string{
		"NVIDIA CORP":          "nvidia",
		"TG THERAPEUTICS INC":  "tg therapeutics",
		"COREWEAVE INC-CL A":   "coreweave",
		"ALPHABET INC-CL A":    "alphabet",
		"THE TRADE DESK INC-A": "trade",
		"":                     "",
	} {
		if got := englishCoreName(in); got != want {
			t.Errorf("englishCoreName(%q) = %q, want %q", in, got, want)
		}
	}
}
