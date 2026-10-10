package llm

import (
	"context"
	"strings"
)

/*
실적 「관전 포인트」 (2026-09-27 사용자 결정 C ② — 실적 메뉴의 LLM 활용).

「향후 7일 안에 실적을 내는 종목에서 **이번 발표의 무엇을 봐야 하나**」를 사실표 하나로
정리한다. 숫자는 규칙이 뽑아 JSON으로 넘기고(점수·축·지난 서프라이즈·추정치 변화·가격 위치·
최근 기사 제목), 모델은 그것들이 **함께 가리키는 것**을 세 줄로 엮는다. 사실표에 없는 수치·
전망·목표가는 쓰지 못하게 한다 — 이 문장은 투자 권유가 아니라 「볼 것」의 목록이다.
*/

// EarningsPreviewLLMOutput is the model's JSON.
type EarningsPreviewLLMOutput struct {
	Headline string   `json:"headline"`
	Points   []string `json:"points"`
	Risk     string   `json:"risk"`

	TotalTokens int64  `json:"-"`
	Model       string `json:"-"`
}

// earningsPreviewOutputTokens — 한 줄 + 세 줄 + 한 줄. 한국어 250음절 남짓에 여유.
const earningsPreviewOutputTokens = 450

const earningsPreviewSystemPrompt = `당신은 한국 개인 투자자를 위해 곧 있을 실적 발표의 「관전 포인트」를 정리하는 애널리스트입니다.

주어진 JSON 사실표만 사용하십시오. **사실표에 없는 수치·전망치·목표가·사건을 지어내지 마십시오.**
매수·매도 권유나 주가 예측을 하지 마십시오. 이 글은 「이번 발표에서 무엇을 확인하면 되는가」입니다.

응답은 반드시 아래 형식의 JSON 하나입니다:
{"headline":"...","points":["...","...","..."],"risk":"..."}

- headline: 40자 이내 한 줄. 이번 발표를 한마디로(예: 「기대치 상향 뒤의 발표 — 눈높이 충족이 관건」).
- points: 정확히 3개. 각 60자 이내. 사실표의 근거를 짚어 「무엇을 볼지」를 말합니다
  (예: 지난 네 번 중 세 번 예상 상회 → 이번에도 이어지는지, 추정치가 한 달 새 올랐다 → 높아진 눈높이,
  최근 기사 제목이 말하는 쟁점). 근거가 부족한 항목은 만들지 말고 있는 근거로 채우십시오.
- risk: 60자 이내 한 문장. 사실표가 가리키는 주의점(예: 20일 급등으로 기대가 선반영, 점수 하락 중).
- 한국어로 씁니다. 종목명은 사실표의 name을 그대로 씁니다.`

func earningsPreviewUserPrompt(factsJSON string) string {
	return "사실표:\n" + factsJSON
}

// GenerateEarningsPreview asks for one symbol's preview.
func (c *Client) GenerateEarningsPreview(ctx context.Context, factsJSON string) (*EarningsPreviewLLMOutput, error) {
	content, used, err := c.chatCompletion(ctx, earningsPreviewSystemPrompt, earningsPreviewUserPrompt(factsJSON), 0.3, true, earningsPreviewOutputTokens)
	if err != nil {
		return nil, err
	}
	var out EarningsPreviewLLMOutput
	if err := decodeLLMJSON(content, &out); err != nil {
		return nil, err
	}
	out.TotalTokens = used
	out.Model = c.Model()
	out.Headline = strings.TrimSpace(out.Headline)
	out.Risk = strings.TrimSpace(out.Risk)
	points := out.Points[:0]
	for _, p := range out.Points {
		if p = strings.TrimSpace(p); p != "" {
			points = append(points, p)
		}
	}
	out.Points = points
	return &out, nil
}

// EstimateEarningsPreviewTokens is the pre-flight cost of one call.
func (c *Client) EstimateEarningsPreviewTokens(factsJSON string) int64 {
	return EstimateTokens(earningsPreviewSystemPrompt, earningsPreviewUserPrompt(factsJSON)) + earningsPreviewOutputTokens
}
