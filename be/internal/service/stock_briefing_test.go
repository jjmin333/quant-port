package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func f64(v float64) *float64 { return &v }

func briefingTechnicals() *TechnicalIndicators {
	return &TechnicalIndicators{
		LastClose:       f64(1000),
		SMA20:           f64(1100),
		SMA50:           f64(1250),
		SMA200:          f64(1500),
		RSI14:           f64(37.7),
		MACD:            f64(1.05),
		MACDSignal:      f64(1.0),
		High52Week:      f64(2050),
		Low52Week:       f64(900),
		RecentSwingHigh: f64(1180),
		RecentSwingLow:  f64(950),
		PctFromHigh52Week: f64(-51.2),
	}
}

func TestBriefingLevelsSplitAroundPrice(t *testing.T) {
	up, down := briefingScenarios(1000, briefingTechnicals())
	if up == nil || down == nil {
		t.Fatal("양방향 시나리오가 모두 나와야 함")
	}
	for _, l := range up.Levels {
		if l.Price <= 1000 {
			t.Errorf("상승 시나리오에 현재가 이하 레벨: %+v", l)
		}
	}
	for _, l := range down.Levels {
		if l.Price >= 1000 {
			t.Errorf("하락 시나리오에 현재가 이상 레벨: %+v", l)
		}
	}
	// Nearest first — the ladder is only useful in order.
	for i := 1; i < len(up.Levels); i++ {
		if up.Levels[i].DistancePct < up.Levels[i-1].DistancePct {
			t.Errorf("상승 레벨이 가까운 순이 아님: %+v", up.Levels)
		}
	}
	if len(up.Levels) > maxScenarioLevels || len(down.Levels) > maxScenarioLevels {
		t.Errorf("레벨 개수 상한 초과: up=%d down=%d", len(up.Levels), len(down.Levels))
	}
}

// Every level must be traceable to a real indicator — no invented targets.
func TestBriefingLevelsAreRealIndicatorValues(t *testing.T) {
	ti := briefingTechnicals()
	real := map[float64]bool{
		*ti.SMA20: true, *ti.SMA50: true, *ti.SMA200: true,
		*ti.RecentSwingHigh: true, *ti.RecentSwingLow: true,
		*ti.High52Week: true, *ti.Low52Week: true,
	}
	up, down := briefingScenarios(1000, ti)
	for _, side := range []*model.BriefingScenario{up, down} {
		for _, l := range side.Levels {
			if !real[l.Price] {
				t.Errorf("실제 지표에서 유래하지 않은 가격대: %+v", l)
			}
		}
	}
}

func TestBriefingSkipsMissingIndicators(t *testing.T) {
	// Only a price and a 20-day average: the ladder must simply be shorter,
	// not padded with guesses.
	ti := &TechnicalIndicators{LastClose: f64(1000), SMA20: f64(1100)}
	up, down := briefingScenarios(1000, ti)
	if up == nil || len(up.Levels) != 1 {
		t.Fatalf("가용 지표 1개면 레벨 1개여야 함, got %+v", up)
	}
	if down != nil {
		t.Errorf("아래쪽 지표가 없으면 하락 시나리오는 nil이어야 함, got %+v", down)
	}
}

func TestBriefingNilWithoutPrice(t *testing.T) {
	stock := &model.Stock{Symbol: "X", Name: "테스트"}
	if b := buildMarketBriefing(stock, nil, nil, nil, nil, nil); b != nil {
		t.Error("가격이 없으면 브리핑을 만들지 않아야 함")
	}
	if b := buildMarketBriefing(stock, nil, nil, nil, &TechnicalIndicators{}, nil); b != nil {
		t.Error("LastClose가 없으면 브리핑을 만들지 않아야 함")
	}
}

func TestBriefingKeyPointsRankedAndBounded(t *testing.T) {
	stock := &model.Stock{Symbol: "000660", Name: "SK하이닉스", Currency: model.CurrencyKRW}
	fundamentals := &model.FundamentalsView{PER: f64(24.9), PBR: f64(8.9)}
	fundamentals.ROE = f64(44.1)
	fundamentals.OperatingMargin = f64(48.6)

	sc := 53
	top := 46
	score := &model.StockScoreView{
		Score: &sc, Grade: "B", PeerPct: &top, TradeDate: "2026-08-07",
		Breakdown: model.ScoreBreakdown{Quality: f64(82), Valuation: f64(3), Technical: f64(29)},
	}
	history := []model.FinancialHistoryPeriod{{
		RevenueGrowthQoQ: f64(60.2), OperatingIncomeGrowthQoQ: f64(96.2), NetIncomeGrowthQoQ: f64(164.6),
	}}
	insider := []model.InsiderTransaction{}
	for i := 0; i < 23; i++ {
		v := int64(100)
		insider = append(insider, model.InsiderTransaction{SharesChange: &v})
	}

	b := buildMarketBriefing(stock, fundamentals, history, insider, briefingTechnicals(), score)
	if b == nil {
		t.Fatal("브리핑이 생성되어야 함")
	}
	if len(b.KeyPoints) == 0 || len(b.KeyPoints) > maxBriefingPoints {
		t.Fatalf("투자 포인트 1~%d개여야 함, got %d", maxBriefingPoints, len(b.KeyPoints))
	}
	if b.KeyPoints[0].Label != "종합" {
		t.Errorf("첫 포인트는 종합 점수여야 함, got %q", b.KeyPoints[0].Label)
	}
	// A valuation axis of 3 must read as expensive, not be silently neutral.
	for _, p := range b.KeyPoints {
		if p.Label == "밸류에이션" && p.Tone != model.ToneNegative {
			t.Errorf("가치 축 3점이면 부정 톤이어야 함, got %q (%s)", p.Tone, p.Text)
		}
	}
	if b.Price != 1000 {
		t.Errorf("현재가가 반영되어야 함, got %v", b.Price)
	}
}
