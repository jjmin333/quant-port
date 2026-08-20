package service

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

// The editor prefill is guidance, not content: it has to stay short enough to
// read at a glance, because the author's first click deletes it.
func TestDraftPromptStaysTwoLines(t *testing.T) {
	for _, direction := range []model.ResearchDirection{
		model.DirectionBuy, model.DirectionSell, model.DirectionHold,
	} {
		draft := buildResearchDraft(model.ResearchCall{}, direction)
		lines := strings.Split(strings.TrimSpace(draft), "\n")
		if len(lines) > 2 {
			t.Fatalf("%s: prompt must be at most 2 lines, got %d:\n%s", direction, len(lines), draft)
		}
		if strings.Contains(draft, "#") {
			t.Fatalf("%s: the prompt must not carry headings — that is the data block's job", direction)
		}
		if !strings.Contains(draft, "클릭하면") {
			t.Fatalf("%s: the prompt should say it disappears on click", direction)
		}
	}
	if buildResearchDraft(model.ResearchCall{}, model.DirectionBuy) ==
		buildResearchDraft(model.ResearchCall{}, model.DirectionSell) {
		t.Fatal("the prompt should ask the question that fits the direction")
	}
}

// The auto-generated section still exists — it just moved behind a button.
// Everything in it must come from the frozen snapshot, never from an invention.
func TestDataBlockCarriesFrozenNumbers(t *testing.T) {
	score := 62
	call := model.ResearchCall{
		Currency:   model.CurrencyKRW,
		ScoreAtPub: &score,
		GradeAtPub: "B+",
		Evidence: model.ResearchEvidence{
			Coverage: 0.9,
			Briefing: &model.MarketBriefing{
				KeyPoints: []model.BriefingPoint{{Label: "수익성", Text: "ROE 10.8%"}},
				Upside: &model.BriefingScenario{
					Summary: "추세 전환 확인 구간",
					Levels:  []model.PriceLevel{{Label: "50일선", Price: 258260, DistancePct: 9.9}},
				},
			},
		},
		Context: model.ResearchContext{
			News: []model.ResearchNews{{Headline: "메모리 업황 반등", URL: "https://example.com/a"}},
		},
	}

	block := buildResearchDataBlock(call)
	for _, want := range []string{
		"## 발행 시점 데이터",
		"**종합 62점(B+)**",
		"커버리지 90%",
		"### 투자 포인트",
		"ROE 10.8%",
		"### 상승 시나리오",
		"50일선 258,260원 (+9.9%)",
		"[메모리 업황 반등](https://example.com/a)",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("data block missing %q:\n%s", want, block)
		}
	}
}

// A symbol with no score must say so rather than print a zero.
func TestDataBlockNamesMissingScore(t *testing.T) {
	block := buildResearchDataBlock(model.ResearchCall{Currency: model.CurrencyKRW})
	if !strings.Contains(block, "종합 점수 없음") {
		t.Fatalf("missing score must be stated, not implied:\n%s", block)
	}
	if strings.Contains(block, "0점") {
		t.Fatalf("absent must not render as zero:\n%s", block)
	}
}
