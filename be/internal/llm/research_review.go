package llm

import (
	"context"
	"strings"
)

/*
투자 리포트 「AI 검토 노트」 (2026-09-27 사용자 결정 C ④).

리포트가 **주장하는 것**(방향·목표가·본문의 근거)과 **우리 데이터**(점수·축·가격·이후 기사·공시)가
서로 맞는지 대조한다. 판정하거나 채점하지 않는다 — 리포트의 성적은 기한이 끝나면 따로 매겨진다.
작성자를 깎아내리는 글이 아니라 **독자가 확인할 곳을 가리키는 메모**다.
*/

type ResearchReviewCheck struct {
	Claim   string `json:"claim"`
	Data    string `json:"data"`
	Verdict string `json:"verdict"`
}

type ResearchReviewLLMOutput struct {
	Checks  []ResearchReviewCheck `json:"checks"`
	Summary string                `json:"summary"`

	TotalTokens int64  `json:"-"`
	Model       string `json:"-"`
}

// ResearchReviewVerdicts is the closed set the page colours.
var ResearchReviewVerdicts = map[string]bool{"일치": true, "근거 약함": true, "반대": true, "확인 불가": true}

const researchReviewOutputTokens = 600

const researchReviewSystemPrompt = `당신은 개인 투자자가 쓴 종목 리포트를 읽고, 리포트의 주장과 주어진 데이터가 맞는지 대조하는 검토자입니다.

주어진 JSON 사실표만 사용하십시오. **사실표에 없는 수치·사건·전망을 지어내지 마십시오.**
리포트가 맞았는지 틀렸는지 판정하지 말고, 매수·매도를 권하지 마십시오. 작성자를 평가하지 마십시오.

응답은 반드시 아래 형식의 JSON 하나입니다:
{"checks":[{"claim":"...","data":"...","verdict":"일치|근거 약함|반대|확인 불가"}],"summary":"..."}

- checks: 2~4개. reportBody·title·direction·targetPrice에서 **검증할 수 있는 주장**을 골라,
  claim(40자 이내, 리포트의 주장 요약)과 data(60자 이내, 사실표가 보여 주는 것)를 적고 verdict를 고릅니다.
  · 일치: 데이터가 주장을 뒷받침  · 근거 약함: 데이터가 주장을 약하게만 받치거나 일부만 맞음
  · 반대: 데이터가 주장과 반대 방향  · 확인 불가: 사실표에 대조할 데이터가 없음
- summary: 80자 이내 한 문장. 독자가 이 리포트를 읽을 때 무엇을 함께 확인하면 좋을지.
- 한국어로 씁니다.`

func researchReviewUserPrompt(factsJSON string) string {
	return "사실표:\n" + factsJSON
}

// GenerateResearchReview asks for one report's review note.
func (c *Client) GenerateResearchReview(ctx context.Context, factsJSON string) (*ResearchReviewLLMOutput, error) {
	content, used, err := c.chatCompletion(ctx, researchReviewSystemPrompt, researchReviewUserPrompt(factsJSON), 0.2, true, researchReviewOutputTokens)
	if err != nil {
		return nil, err
	}
	var out ResearchReviewLLMOutput
	if err := decodeLLMJSON(content, &out); err != nil {
		return nil, err
	}
	out.TotalTokens, out.Model = used, c.Model()
	out.Summary = strings.TrimSpace(out.Summary)
	kept := out.Checks[:0]
	for _, ch := range out.Checks {
		ch.Claim, ch.Data, ch.Verdict = strings.TrimSpace(ch.Claim), strings.TrimSpace(ch.Data), strings.TrimSpace(ch.Verdict)
		if ch.Claim == "" || !ResearchReviewVerdicts[ch.Verdict] {
			continue
		}
		kept = append(kept, ch)
	}
	out.Checks = kept
	return &out, nil
}

func (c *Client) EstimateResearchReviewTokens(factsJSON string) int64 {
	return EstimateTokens(researchReviewSystemPrompt, researchReviewUserPrompt(factsJSON)) + researchReviewOutputTokens
}
