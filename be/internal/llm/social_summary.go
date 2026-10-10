package llm

import (
	"context"
	"strings"
)

/*
종목 커뮤니티 탭의 「AI 핵심 요약」 (2026-09-29 사용자 요청 — Reddit 데이터로 AI 요약).

서버가 커뮤니티 점수(규칙)·언급 수·감성·서브레딧 비중·이번 주 인기 글 제목과 앞부분을 사실표로
넘기고, 모델은 **레딧 이용자들이 무엇을 두고 어느 쪽으로 기울었는지**를 쓴다. 글은 사실이 아니라
의견이므로 「레딧에서는 …라는 의견」으로 옮기게 하고, 점수는 서버 값에 맞추게 한다.
*/

// SocialSummaryLLMOutput is the model's JSON.
type SocialSummaryLLMOutput struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
	Bull    []string `json:"bull"`
	Bear    []string `json:"bear"`

	TotalTokens int64  `json:"-"`
	Model       string `json:"-"`
}

// 요약 3문장 + 태그 5 + 근거 넷. 한국어 400음절 남짓에 여유.
const socialSummaryOutputTokens = 700

const socialSummarySystemPrompt = `당신은 한국 개인 투자자를 위해 미국 종목에 대한 Reddit(주식 커뮤니티) 여론을 정리하는 애널리스트입니다.

주어진 JSON 사실표만 사용하십시오. 사실표에 없는 수치·가격·사건을 지어내지 마십시오.
"posts"는 Reddit 이용자의 **의견**입니다. 사실로 단정하지 말고 「레딧에서는 …라는 의견이 많다」처럼 옮기십시오.
매수·매도 권유나 주가 예측을 하지 마십시오.

"communityScore"(-100..100)와 "stance"는 서버가 감성 비율과 표본 크기로 계산한 값입니다. 글은 이 점수와
같은 방향이어야 하며, 점수를 다시 매기거나 다른 숫자를 만들지 마십시오. "users"가 30명 미만이면 표본이
작다는 점을 한 번 짚으십시오.

응답은 반드시 아래 형식의 JSON 하나입니다:
{"summary":"...","tags":["...","..."],"bull":["..."],"bear":["..."]}

- summary: 2~3문장, 200자 이내. 레딧 이용자들이 이 종목을 두고 무엇을 이야기하는지(글 제목·앞부분·keywords가
  가리키는 쟁점), 분위기가 어느 쪽인지(점수와 같은 방향), 언급이 늘었는지 줄었는지를 엮습니다.
- tags: 3~5개. 각 2~8자 한국어 또는 티커, "#" 없이(예: "NVDA", "AI반도체", "옵션수급", "실적기대").
- bull: 0~2개. 각 50자 이내. 글에 나타난 낙관 쪽 근거.
- bear: 0~2개. 각 50자 이내. 글에 나타난 비관·주의 쪽 근거.
- 한국어로 씁니다. 종목명은 사실표의 name을 씁니다.`

func socialSummaryUserPrompt(factsJSON string) string {
	return "사실표:\n" + factsJSON
}

// GenerateSocialSummary asks for one symbol's community summary.
func (c *Client) GenerateSocialSummary(ctx context.Context, factsJSON string) (*SocialSummaryLLMOutput, error) {
	content, used, err := c.chatCompletion(ctx, socialSummarySystemPrompt, socialSummaryUserPrompt(factsJSON), 0.3, true, socialSummaryOutputTokens)
	if err != nil {
		return nil, err
	}
	var out SocialSummaryLLMOutput
	if err := decodeLLMJSON(content, &out); err != nil {
		return nil, err
	}
	out.TotalTokens = used
	out.Model = c.Model()
	out.Summary = strings.TrimSpace(out.Summary)
	out.Tags = trimList(out.Tags, 5, func(s string) string { return strings.TrimPrefix(s, "#") })
	out.Bull = trimList(out.Bull, 2, nil)
	out.Bear = trimList(out.Bear, 2, nil)
	return &out, nil
}

func trimList(in []string, max int, fix func(string) string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if fix != nil {
			s = fix(s)
		}
		if s = strings.TrimSpace(s); s != "" && len(out) < max {
			out = append(out, s)
		}
	}
	return out
}

// EstimateSocialSummaryTokens is the pre-flight cost of one call.
func (c *Client) EstimateSocialSummaryTokens(factsJSON string) int64 {
	return EstimateTokens(socialSummarySystemPrompt, socialSummaryUserPrompt(factsJSON)) + socialSummaryOutputTokens
}
