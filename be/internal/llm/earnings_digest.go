package llm

import (
	"context"
	"strings"
)

/*
실적 화면 맨 위 「AI 실적 요약」 (2026-10-09 사용자 요청 1).

종목 하나의 「관전 포인트」(earnings_preview.go)와 달리 **시장 전체의 한 주**를 요약한다:
최근 7일 발표의 서프라이즈와 다음 발표 대형주. 숫자는 규칙이 단위까지 붙여 넘기고(「10.74조원」),
모델은 그것을 읽는 순서만 정한다 — 계산을 시키지 않는다(사실표에 없는 숫자는 서비스가 거른다).
*/

// EarningsDigestLLMOutput is the model's JSON.
type EarningsDigestLLMOutput struct {
	Headline string   `json:"headline"`
	Points   []string `json:"points"`
	Watch    string   `json:"watch"`

	TotalTokens int64  `json:"-"`
	Model       string `json:"-"`
}

// 한 줄 + 세 줄 + 한 줄 — 관전 포인트보다 짧게 잡는다(입력이 더 길다).
const earningsDigestOutputTokens = 350

const earningsDigestSystemPrompt = `당신은 한국 개인 투자자를 위해 이번 주 실적 발표 흐름을 요약하는 애널리스트입니다.

주어진 JSON 사실표만 사용하십시오. **사실표에 없는 수치·비율·전망을 만들거나 계산하지 마십시오** — 숫자는 사실표에 적힌 그대로 옮깁니다.
매수·매도 권유나 주가 예측을 하지 마십시오.

응답은 반드시 아래 형식의 JSON 하나입니다:
{"headline":"...","points":["...","...","..."],"watch":"..."}

- headline: 60자 이내 한 줄. 최근 발표 중 가장 큰 회사(recent의 앞쪽)의 결과로 시작합니다.
- points: 2~3개, 각 70자 이내. 상회·하회 비율(stats), 눈에 띄는 서프라이즈, 하회한 대형주를 짚습니다.
  recent가 비어 있으면 「최근 7일 발표 없음」을 한 줄로 쓰고 다가오는 발표로 채웁니다.
- watch: 70자 이내 한 문장. upcoming의 대형주와 날짜(estimated는 「추정」이라고 밝힘).
- 한국어로 씁니다. 회사명은 사실표의 name을 그대로 씁니다.`

func earningsDigestUserPrompt(factsJSON string) string {
	return "사실표:\n" + factsJSON
}

// GenerateEarningsDigest asks for one market's weekly digest.
func (c *Client) GenerateEarningsDigest(ctx context.Context, factsJSON string) (*EarningsDigestLLMOutput, error) {
	content, used, err := c.chatCompletion(ctx, earningsDigestSystemPrompt, earningsDigestUserPrompt(factsJSON), 0.2, true, earningsDigestOutputTokens)
	if err != nil {
		return nil, err
	}
	var out EarningsDigestLLMOutput
	if err := decodeLLMJSON(content, &out); err != nil {
		return nil, err
	}
	out.TotalTokens = used
	out.Model = c.Model()
	out.Headline = strings.TrimSpace(out.Headline)
	out.Watch = strings.TrimSpace(out.Watch)
	points := out.Points[:0]
	for _, p := range out.Points {
		if p = strings.TrimSpace(p); p != "" {
			points = append(points, p)
		}
	}
	out.Points = points
	return &out, nil
}

// EstimateEarningsDigestTokens is the pre-flight cost of one call.
func (c *Client) EstimateEarningsDigestTokens(factsJSON string) int64 {
	return EstimateTokens(earningsDigestSystemPrompt, earningsDigestUserPrompt(factsJSON)) + earningsDigestOutputTokens
}
