package llm

import (
	"context"
	"strings"
)

// NewsBriefingLLMOutput is the 「AI 시장 브리핑」 payload for one market.
//
// The narrative is deliberately long — the card clamps it and a modal shows the
// rest — so this is the largest completion the client asks for after the
// per-symbol AI 요약.
type NewsBriefingLLMOutput struct {
	Summary     string   `json:"summary"`
	Mood        string   `json:"mood"`
	MoodReason  string   `json:"moodReason"`
	Highlights  []string `json:"highlights"`
	WatchPoints []string `json:"watchPoints"`

	// TotalTokens is what the provider charged, taken from the response rather
	// than the model's JSON.
	TotalTokens int64 `json:"-"`
}

// newsBriefingOutputTokens is the pre-flight ceiling for the completion. The
// prompt asks for 6–10 Korean sentences plus lists; Korean costs roughly one
// token per syllable, so this is ~1,000 syllables of prose with headroom.
const newsBriefingOutputTokens = 1_500

const newsBriefingSystemPrompt = `당신은 한국 투자자를 위한 시장 뉴스 브리핑 작성자입니다.

주어진 JSON 사실만 사용하십시오. 사건·가격·수치를 새로 만들어내지 마십시오.
JSON에 없는 종목·지수·수치를 언급해서는 안 됩니다.

응답은 반드시 아래 형식의 JSON 하나입니다:
{"summary":"...","mood":"강세|중립|약세","moodReason":"...","highlights":["..."],"watchPoints":["..."]}

- summary: 한국어 6~10문장. 오늘 뉴스가 무엇을 말하는지, 그것이 지수·종목 움직임과
  어떻게 이어지는지 설명하십시오. 첫 두 문장에 가장 중요한 내용을 담으십시오
  (화면에는 앞부분만 보이고 나머지는 펼쳐 봅니다).
- mood: 지수 등락과 뉴스 논조를 함께 보고 판단한 오늘 시장의 성격.
- moodReason: mood를 그렇게 본 근거 한 문장.
- highlights: 오늘의 핵심 사건 3~5개. 각 항목은 한 문장.
- watchPoints: 앞으로 지켜볼 것 2~3개. 각 항목은 한 문장.

headlines가 오늘의 뉴스 본문입니다. earnings·flow·지수는 숫자 근거로만 쓰십시오.
earnings의 imp는 서버가 고른 우선순위(0–100)이며 문장에 인용하지 마십시오.
flow가 있으면 코스피 외국인(frNet)·기관(insNet) 순매수 억원입니다. p가 true이면
장중 잠정치입니다.

previousBriefings가 있으면 흐름의 변화(이어짐·반전·소강)를 summary에 반영하되,
지난 브리핑의 문장을 반복하지 마십시오. 최신 항목의 watchPoints가 있으면 그것이
이어졌는지·해소됐는지를 반영하십시오. 이력이 없으면 오늘 사실만으로 작성하십시오.

이것은 정보 제공이며 투자 조언이 아닙니다. 매수·매도를 권하지 마십시오.`

// GenerateNewsBriefing produces one market's briefing from assembled facts.
// factsJSON must carry every number the prose may cite — the model is told not
// to invent any, so anything missing simply will not be mentioned.
func (c *Client) GenerateNewsBriefing(ctx context.Context, factsJSON string) (*NewsBriefingLLMOutput, error) {
	content, used, err := c.chatCompletion(
		ctx,
		newsBriefingSystemPrompt,
		factsJSON,
		0.3,
		true,
		newsBriefingOutputTokens,
	)
	if err != nil {
		return nil, err
	}
	var out NewsBriefingLLMOutput
	if err := decodeLLMJSON(content, &out); err != nil {
		return nil, err
	}
	out.Summary = strings.TrimSpace(out.Summary)
	out.Mood = normalizeBriefingMood(out.Mood)
	out.TotalTokens = used
	return &out, nil
}

// EstimateNewsBriefingTokens reports what one call is expected to cost, input
// plus completion, so a scheduler can decide whether it still fits the day's
// budget before spending it.
func (c *Client) EstimateNewsBriefingTokens(factsJSON string) int64 {
	return EstimateTokens(newsBriefingSystemPrompt, factsJSON) + newsBriefingOutputTokens
}

// normalizeBriefingMood keeps the label inside the three values the UI knows
// how to colour. Anything else becomes 중립 rather than leaking an unstyled
// string onto the card.
func normalizeBriefingMood(raw string) string {
	switch strings.TrimSpace(raw) {
	case "강세", "약세", "중립":
		return strings.TrimSpace(raw)
	default:
		return "중립"
	}
}
