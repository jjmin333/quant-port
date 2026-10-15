package llm

import (
	"context"
	"strings"
)

/*
지수 한 줄 해설 — 「오늘의 시장」 열세 칸이 각자 무슨 상황인지.

여기 오기 전까지 지수에 붙어 있던 문장은 규칙이 만든 지표 나열이었다:
「나스닥 — 최근 5거래일 +1.4%, 20거래일 +4.6%, 20일선 위, RSI 56.8(strong),
52주 최고 대비 -2.0%.」 사실은 다 맞지만 **읽어도 무슨 상황인지는 모른다** — RSI 56.8이
좋은 건지, 20일선 위인데 52주 최고 대비 −2%인 건 어떤 국면인지를 읽는 사람이 직접
조립해야 한다.

여기서는 그 조립을 맡긴다. **숫자는 여전히 규칙이 뽑고**(JSON으로 넘긴다), 모델은
그것들을 한 문단으로 엮기만 한다. 새 수치를 만들 여지를 주지 않는 게 핵심이라, 규칙
문장은 지우지 않고 그대로 남는다 — LLM이 꺼져 있거나 실패한 지수는 예전 문장을 계속
보여 준다.
*/

// IndexNoteLLMOutput is one index's narrative read.
type IndexNoteLLMOutput struct {
	// Note는 한 문단이다. 지수 모달이 규칙 문장 아래에 편다.
	Note string `json:"note"`
	/*
		Stance는 국면 라벨이다 — 「상승 · 중립 · 하락」 중 하나로 강제한다.

		브리핑의 시황 배지(「강세 · 중립 · 약세」)와 **같은 자리, 같은 모양**으로 화면에
		서므로 길이도 거기에 맞춘다. 한때 「상승 흐름 · 방향 없음 · 하락 흐름」이었는데,
		알약 배지 안에서 두 어절은 옆의 점수 배지와 균형이 맞지 않았다.
	*/
	Stance string `json:"stance"`

	// TotalTokens is what the provider charged, taken from the response.
	TotalTokens int64 `json:"-"`
}

/*
indexNoteOutputTokens — 한 문단 + 라벨.

지수 열셋을 하루 한 번 도는 작업이라 한 번의 길이가 그대로 하루 비용이 된다.
3~4문장이면 한국어로 200음절 남짓이고, 여유를 두어도 이 정도면 충분하다.
*/
const indexNoteOutputTokens = 400

const indexNoteSystemPrompt = `당신은 한국 투자자를 위한 시장 지수 해설자입니다.

주어진 JSON 지표만 사용하십시오. **JSON에 없는 수치를 언급해서는 안 됩니다.**
전망치·목표치·다른 지수·뉴스 사건을 지어내지 마십시오.

응답은 반드시 아래 형식의 JSON 하나입니다:
{"note":"...","stance":"상승|중립|하락"}

- note: 한국어 3~4문장. 이 지수가 **지금 어떤 국면에 있는지**를 설명하십시오.
  지표를 다시 나열하지 말고(그 목록은 화면에 이미 있습니다), 지표들이 **함께
  가리키는 것**을 말하십시오. 예: 20일선 위인데 52주 최고와는 거리가 있다면 그것이
  회복 국면인지 정체인지. 서로 어긋나는 지표가 있으면 어긋난다고 적으십시오.
- stance: note가 말한 국면의 한 단어 요약. 세 값 중 하나만 씁니다.

지표의 뜻:
- return5d/return20d: 최근 5·20거래일 누적 등락률(%).
- sma20Position: 종가가 20일 이동평균 위인지 아래인지.
- rsi14: 상대강도지수. 70 이상 과열, 30 이하 과매도로 봅니다.
- pctFromHigh52w: 52주 최고 대비 현재 위치(%). 0에 가까울수록 고점 부근입니다.
- direction이 false이면 오르는 것이 좋은 소식이라고 단정할 수 없는 지표입니다
  (원/달러·VIX·미 국채 금리). 그런 지수는 방향만 적고 좋다·나쁘다를 말하지 마십시오.

이것은 정보 제공이며 투자 조언이 아닙니다. 매수·매도를 권하지 마십시오.`

// GenerateIndexNote turns one index's computed indicators into a paragraph.
func (c *Client) GenerateIndexNote(ctx context.Context, factsJSON string) (*IndexNoteLLMOutput, error) {
	content, used, _, err := c.chatCompletionWithFallback(
		ctx,
		indexNoteSystemPrompt,
		factsJSON,
		0.3,
		true,
		indexNoteOutputTokens,
	)
	if err != nil {
		return nil, err
	}
	var out IndexNoteLLMOutput
	if err := decodeLLMJSON(content, &out); err != nil {
		return nil, err
	}
	out.Note = strings.TrimSpace(out.Note)
	out.Stance = normalizeIndexStance(out.Stance)
	out.TotalTokens = used
	return &out, nil
}

/*
normalizeIndexStance keeps the label inside the three values the UI can colour.

**옛 두 어절 표기도 받아 준다.** 프롬프트를 바꾼 뒤에도 모델은 한동안 「상승 흐름」처럼
답할 수 있고, 그때 알 수 없는 값으로 떨어뜨려 전부 「중립」이 되면 배지가 조용히
거짓말을 한다 — 실제로는 상승이라고 답했는데 화면은 중립을 보여 주는 셈이다.
접두사로 갈라 같은 뜻이면 새 표기로 옮긴다.

그래도 못 알아본 값만 「중립」이다 — 색이 없는 쪽이 안전한 기본값이다.
*/
func normalizeIndexStance(raw string) string {
	s := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(s, "상승"):
		return "상승"
	case strings.HasPrefix(s, "하락"):
		return "하락"
	default:
		return "중립"
	}
}
