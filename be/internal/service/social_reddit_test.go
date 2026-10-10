package service

import (
	"strings"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/repository"
)

func intp(v int) *int { return &v }

func TestCommunityScore(t *testing.T) {
	cases := []struct {
		sent, users int
		want        int
		ok          bool
	}{
		{72, 87, 30, true},  // NVDA 09-29: 표본이 두터워 거의 그대로
		{100, 11, 35, true}, // TSLA: 11명 100%는 보통 쪽으로 크게 당긴다
		{56, 114, 2, true},  // MU: 보통(55) 근처
		{27, 17, -28, true}, // AAPL
		{0, 4, 0, false},    // 5명 미만은 없음
	}
	for _, c := range cases {
		got, ok := CommunityScore(intp(c.sent), intp(c.users))
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("sent=%d users=%d → %d,%v want %d,%v", c.sent, c.users, got, ok, c.want, c.ok)
		}
	}
	if CommunityStance(30) != "낙관" || CommunityStance(-8) != "관망" || CommunityStance(-25) != "비관" {
		t.Error("stance")
	}
}

func TestPickPostTargets(t *testing.T) {
	now := time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	fresh, old := now.Add(-2*time.Hour), now.Add(-30*time.Hour)
	top := []repository.SocialTop{
		{Symbol: "NVDA", Mentions: 100, LastPostsAt: &fresh}, // 방금 받음 → 건너뜀
		{Symbol: "AMD", Mentions: 50, LastPostsAt: &old},     // 상위 40, 20시간 지남 → 대상
		{Symbol: "ON", Mentions: 40},                         // 낱말 티커
		{Symbol: "MU", Mentions: 30},                         // 처음 → 대상
		{Symbol: "X", Mentions: 1},                           // 언급 1 → 제외
	}
	got := pickPostTargets(top, 10, now)
	if len(got) != 2 || got[0].Symbol != "AMD" || got[1].Symbol != "MU" {
		t.Fatalf("targets = %+v", got)
	}
}

func TestRuleAndValidSocialSummary(t *testing.T) {
	f := socialFacts{Symbol: "NVDA", Mentions24h: 126, MentionsPrev24h: 6, Users: 20, SentimentPct: 72,
		CommunityScore: 30, Stance: "낙관", Keywords: []string{"calls", "buyback"}}
	b := RuleSocialSummary(f)
	if !strings.Contains(b.Summary, "크게 늘었습니다") || !strings.Contains(b.Summary, "+30점(낙관)") ||
		!strings.Contains(b.Summary, "표본이 작아") || len(b.Tags) != 3 {
		t.Fatalf("rule = %+v", b)
	}
	if ValidSocialSummary(nil) || ValidSocialSummary(&llm.SocialSummaryLLMOutput{Summary: "짧음", Tags: []string{"x"}}) {
		t.Fatal("invalid accepted")
	}
	if !ValidSocialSummary(&llm.SocialSummaryLLMOutput{Summary: strings.Repeat("가", 40), Tags: []string{"NVDA"}}) {
		t.Fatal("valid rejected")
	}
}
