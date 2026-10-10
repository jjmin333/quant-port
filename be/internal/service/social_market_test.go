package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/repository"
)

// 「미국 개미 분위기」(2026-10-01 요청 3): 표본이 작으면 점수를 내지 않고, ETF 표본이 작으면 전체만 본다.
func TestMarketMoodScore(t *testing.T) {
	// 10-01 실측: ETF(SPY 65·143명, QQQ 78·49명, IWM 57·10명) · 전체 61.8% · 1,536명.
	etfSU, etfU := int64(65*143+78*49+57*10), int64(202)
	allU := int64(1536)
	allSU := int64(94925) // 61.8% × 1,536명
	s, pct, ok := MarketMoodScore(etfSU, etfU, allSU, allU)
	if !ok || s != 23 || pct < 65 || pct > 66 {
		t.Fatalf("score=%d pct=%v ok=%v", s, pct, ok)
	}
	if MarketMoodStance(s) != "약한 낙관" {
		t.Fatalf("stance %q", MarketMoodStance(s))
	}
	if _, _, ok := MarketMoodScore(0, 0, 55*40, 40); ok {
		t.Fatal("tiny sample must not score")
	}
	if s, _, _ := MarketMoodScore(99*10, 10, 55*100, 100); s != 0 {
		t.Fatalf("ETF sample under %d must be ignored, got %d", marketMoodETFMin, s)
	}
	if s, _, _ := MarketMoodScore(0, 0, 100*100, 100); s != 99 {
		t.Fatalf("clamped scale: got %d", s)
	}
}

func TestMarketSurges(t *testing.T) {
	top := []repository.SocialTop{
		{Symbol: "SPY", Mentions: 239, MentionsPrev: 261},
		{Symbol: "MU", Name: "마이크론", Mentions: 376, MentionsPrev: 173},
		{Symbol: "HOOD", Mentions: 33, MentionsPrev: 6},
		{Symbol: "TSLA", Mentions: 81, MentionsPrev: 70}, // 급증 아님
		{Symbol: "XYZ", Mentions: 10, MentionsPrev: 1},   // 20회 미만
	}
	etfs, surges := marketETFsAndSurges(top, map[string]repository.SocialInsight{})
	if len(etfs) != 1 || etfs[0].Symbol != "SPY" {
		t.Fatalf("etfs %+v", etfs)
	}
	if len(surges) != 2 || surges[0].Symbol != "HOOD" || surges[1].Symbol != "MU" {
		t.Fatalf("surges %+v", surges)
	}
}
