package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestRuleResearchReview(t *testing.T) {
	score, target, ret := 35, 120000.0, -4.2
	f := &repository.ResearchReviewFacts{
		Direction: "buy", ScoreNow: &score, BasePrice: 100000, TargetPrice: &target, HorizonDays: 90,
		ReturnSincePub: &ret, HeadlinesSince: []string{"실적 쇼크"},
	}
	out := RuleResearchReview(f)
	if len(out.Checks) != 3 {
		t.Fatalf("checks %+v", out.Checks)
	}
	if out.Checks[0].Verdict != "반대" {
		t.Fatalf("매수 의견인데 점수 35면 반대: %+v", out.Checks[0])
	}
	if out.Checks[1].Verdict != "반대" || out.Checks[1].Data != "발행가 대비 목표 +20.0%, 기한 90일 · 발행 뒤 -4.2%" {
		t.Fatalf("목표가 줄: %+v", out.Checks[1])
	}
	for _, c := range out.Checks {
		if !llm.ResearchReviewVerdicts[c.Verdict] {
			t.Fatalf("닫힌 판정 집합 밖: %q", c.Verdict)
		}
	}
}

func TestNextResearchReviewSlot(t *testing.T) {
	loc := kstLoc()
	cases := map[string]string{
		"21:00:00": "21:05:00",
		"21:05:00": "21:35:00",
		"21:20:00": "21:35:00",
		"21:40:00": "22:05:00",
	}
	for in, want := range cases {
		now, _ := time.ParseInLocation("15:04:05", in, loc)
		now = time.Date(2026, 9, 27, now.Hour(), now.Minute(), now.Second(), 0, loc)
		if got := NextResearchReviewSlot(now).In(loc).Format("15:04:05"); got != want {
			t.Errorf("%s → %s, want %s", in, got, want)
		}
	}
}
