package llm

import (
	"context"
	"errors"
	"strings"
)

/*
부동산·금융상품 AI 해설 (2026-10-01 사용자 요청). 키는 GEMINI_SUB_API_KEY 전용(버킷 llm_sub) —
주식·뉴스가 쓰는 KR/US 키와 섞지 않는다.

**토큰 최적화** (사용자 요구):
  - 한 번에 여러 항목(5~10개)을 묶어 보낸다 — 시스템 프롬프트를 항목마다 다시 내지 않는다.
  - 사실표는 짧은 키의 압축 JSON(공백 없음), 자유 텍스트(우대조건)는 400자에서 자른다.
  - 출력은 항목당 짧은 JSON(문장 1~2개, 120자 이내) — max_tokens를 항목 수에 비례해 작게.
  - 입력 해시가 같으면 부르지 않는다(서비스 쪽).
모든 프롬프트는 사실표 밖의 숫자를 만들지 말고, 매수·매도 권유를 하지 말라고 못박는다.
*/

const assetCommonRules = `규칙: 주어진 JSON의 값만 쓰십시오. 없는 숫자·사건을 만들지 마십시오. 매수·매도·가입 권유나 가격 예측을 하지 마십시오. 한국어. 응답은 JSON 하나.`

// ── 우대조건 구조화 ──────────────────────────────────────────────────────────

// FinCondsItem — 한 상품의 우대조건 정리.
type FinCondsItem struct {
	ID    string    `json:"id"`
	Conds []FinCond `json:"conds"`
	Easy  string    `json:"easy"` // 쉬움 · 보통 · 어려움
	Note  string    `json:"note"`
}

type FinCond struct {
	Label string  `json:"l"` // 8자 이내
	Bonus float64 `json:"b"` // %p (모르면 0)
}

const finCondsSystem = `예·적금 상품의 「우대조건」 원문을 짧게 구조화합니다.
입력: [{"id","n"(상품명),"b"(기본금리%),"m"(최고금리%),"t"(우대조건 원문)}]
출력: {"items":[{"id","conds":[{"l":"조건 이름 8자 이내","b":가산 %p 숫자}],"easy":"쉬움|보통|어려움","note":"40자 이내 한 줄"}]}
- conds는 원문에 나온 조건만, 최대 5개. 가산폭이 원문에 없으면 b=0.
- easy: 누구나 쉽게 채우는 조건(비대면 가입·마케팅 동의)만이면 쉬움, 급여이체·카드실적처럼 거래가 필요하면 보통, 신규고객 한정·특정 직업·높은 실적이면 어려움.
- note: 최고금리를 받으려면 무엇이 필요한지 한 줄.
` + assetCommonRules

// GenerateFinConds structures 우대조건 for a batch of products.
func (c *Client) GenerateFinConds(ctx context.Context, itemsJSON string, n int) ([]FinCondsItem, int64, error) {
	var out struct {
		Items []FinCondsItem `json:"items"`
	}
	used, err := c.assetJSON(ctx, finCondsSystem, itemsJSON, int64(90*n+40), &out)
	for i := range out.Items {
		out.Items[i].Note = clip(out.Items[i].Note, 60)
		if len(out.Items[i].Conds) > 5 {
			out.Items[i].Conds = out.Items[i].Conds[:5]
		}
		for k := range out.Items[i].Conds {
			out.Items[i].Conds[k].Label = clip(out.Items[i].Conds[k].Label, 12)
		}
	}
	return out.Items, used, err
}

// ── 지역·단지 해설 ──────────────────────────────────────────────────────────

// AssetNoteItem — 지역 또는 단지 한 줄 해설.
type AssetNoteItem struct {
	Key  string   `json:"k"`
	Note string   `json:"note"`
	Tags []string `json:"tags"`
}

const regionNotesSystem = `한국 아파트 시장의 시군구별 주간 동향을 한두 문장으로 요약합니다.
입력: [{"k"(지역코드),"n"(지역명),"w1","w4","w12"(주간 매매지수 1·4·12주 변화 %),"j4"(전세지수 4주 %),"vol"(최근 3개월 거래량 변화 %),"top"([단지명, 최근 3개월 ㎡가 변화 %] 최대 3),"news"(그 지역 최근 기사 제목, 있을 때만)}]
출력: {"items":[{"k","note":"90자 이내 1~2문장","tags":["2~6자 최대 3개"]}]}
- note: 방향(상승/보합/하락)과 속도, 거래량, 전세 흐름 중 두드러진 것. 단지는 top에 있는 것만 언급.
- news는 「무슨 이야기가 오가나」의 참고다. 숫자와 맞을 때만 짧게 덧붙이고, 기사 속 숫자는 옮기지 마십시오.
` + assetCommonRules

const areaNotesSystem = `한국 아파트 시장을 전국 또는 시도 단위로 요약합니다(개인 투자자가 지역을 고르기 전 큰 흐름을 보는 자리).
입력: [{"k"(코드, 00=전국),"n"(이름),"w1","w4","w12"(주간 매매지수 1·4·12주 변화 %),"j4"(전세지수 4주 %),"vol"(최근 3개월 거래량 변화 %),
"up","dn"(4주 지수가 오른·내린 시군구 수),"sub"(시군구 수),"top","bot"([시군구, 4주 %] 많이 오른·내린 곳),"sido"(전국만: [시도, 4주 %])}]
출력: {"items":[{"k","note":"160자 이내 2~3문장","tags":["2~6자 최대 3개"]}]}
- note: ① 전체 방향과 속도 ② 오른·내린 지역 수로 본 확산 정도 ③ 어디가 끌고 어디가 처지는지(top·bot·sido에 있는 이름만) — 거래량·전세 흐름이 두드러지면 함께.
- 시도 항목(k≠00)에는 다른 시도의 숫자가 없습니다 — 「전국에서 가장」「조사 지역 중 가장」처럼 다른 시도와 비교·순위를 말하지 마십시오.
` + assetCommonRules

const complexNotesSystem = `한국 아파트 단지의 최근 실거래 흐름을 한두 문장으로 요약합니다.
입력: [{"k","n"(단지명),"r"(지역),"mom"(최근 3개월 ㎡가 vs 그 전 9개월 %),"tr"(6개월 거래 건수),"jr"(전세가율 %),"pk"(12개월 월별 최고 대비 %),"pr"(시군구 평균 대비 %),"by"(준공연도)}]
출력: {"items":[{"k","note":"90자 이내 1~2문장","tags":["2~6자 최대 3개"]}]}
- note: 가격 흐름, 거래 활발도, 전세가율(80% 넘으면 주의, 50% 아래면 갭이 큼)을 사실대로.
` + assetCommonRules

// GenerateAssetNotes writes short notes for regions ("region") or complexes ("complex").
func (c *Client) GenerateAssetNotes(ctx context.Context, kind, itemsJSON string, n int) ([]AssetNoteItem, int64, error) {
	system, perItem, clipAt := regionNotesSystem, int64(80), 140
	switch kind {
	case "complex":
		system = complexNotesSystem
	case "area":
		system, perItem, clipAt = areaNotesSystem, 140, 220
	}
	var out struct {
		Items []AssetNoteItem `json:"items"`
	}
	used, err := c.assetJSON(ctx, system, itemsJSON, perItem*int64(n)+30, &out)
	for i := range out.Items {
		out.Items[i].Note = clip(out.Items[i].Note, clipAt)
		out.Items[i].Tags = trimList(out.Items[i].Tags, 3, func(s string) string { return clip(strings.TrimPrefix(s, "#"), 10) })
	}
	return out.Items, used, err
}

// ── 금리 한 줄 ──────────────────────────────────────────────────────────────

// RatesNote — 금리 환경 요약.
type RatesNote struct {
	Note   string   `json:"note"`
	Points []string `json:"points"`
}

const ratesNoteSystem = `한국 금리 환경을 개인 투자자에게 요약합니다.
입력: {"d"(기준일),"r":[[이름, 현재 %, 1주 전 %, 3개월 전 %]]}
출력: {"note":"80자 이내 한 문장","points":["예금·대출·채권 관점 각 40자 이내, 최대 3개"]}
- 숫자는 입력의 것만. 기준금리 변화가 없으면 없다고 쓰지 말고 시장금리 흐름을 쓰십시오.
` + assetCommonRules

func (c *Client) GenerateRatesNote(ctx context.Context, factsJSON string) (*RatesNote, int64, error) {
	var out RatesNote
	used, err := c.assetJSON(ctx, ratesNoteSystem, factsJSON, 220, &out)
	out.Note = clip(out.Note, 120)
	out.Points = trimList(out.Points, 3, func(s string) string { return clip(s, 60) })
	return &out, used, err
}

// ── 홈 브리핑(부동산·금리) ──────────────────────────────────────────────────

// AssetBrief — 홈 「부동산·금리 브리핑」 한 쪽(2026-10-03 사용자 결정 B). 분위기는 서비스가 규칙으로 정한다.
type AssetBrief struct {
	Headline string   `json:"headline"`
	Body     []string `json:"body"`
	Watch    []string `json:"watch"`
}

const briefREInput = `입력: {"d"(지수 기준일),"w1","w4","w12"(전국 아파트 주간 매매지수 1·4·12주 변화 %),"j4"(전세지수 4주 %),"vol"(최근 3개월 거래량 변화 %),
"up","dn","sub"(4주 지수가 오른·내린 시군구 수 / 전체),"sido"([시도, 4주 %]),"top","bot"([시군구, 4주 %] 많이 오른·내린 곳),"mort"([주담대 평균 %, 3개월 전 %]),
"news"(최근 3일 부동산·대출 기사 제목, 있을 때만)}`

const briefRatesInput = `입력: {"d"(기준일),"r":[[이름, 현재 %, 1주 전 %, 3개월 전 %]],"news"(최근 3일 금리·예적금·대출 기사 제목, 있을 때만)}`

const briefSystemTail = `
출력: {"headline":"25자 이내 제목(숫자 없이 흐름을 한마디로)","body":["첫 문단 160자 이내","둘째 문단 120자 이내"],"watch":["지켜볼 것 40자 이내, 최대 3개"]}
- body는 신문 시황처럼 사실 → 해석 순서. 숫자는 입력의 것만 쓰고 소수 둘째 자리까지.
- watch는 다음 발표에서 확인할 만한 것(방향이 바뀌는지, 번지는지). 예측·권유 금지.
- news(기사 제목)는 숫자가 말하는 흐름의 배경으로만 한 줄 이내로 씁니다 — 기사 속 숫자·전망은 옮기지 않습니다.
` + assetCommonRules

const briefRESystem = `한국 아파트 시장의 이번 주 흐름을 홈 화면 브리핑으로 씁니다(개인 투자자가 하루 한 번 보는 자리).
` + briefREInput + `
- 첫 문단: 전국 방향·속도, 오른 지역 수로 본 확산, 어디가 끌고(sido·top) 어디가 처지는지(bot). 둘째 문단: 거래량·전세, mort가 있으면 대출 금리 부담.` + briefSystemTail

const briefRatesSystem = `한국 금리 환경을 홈 화면 브리핑으로 씁니다(예금·대출·채권을 가진 개인이 보는 자리).
` + briefRatesInput + `
- 첫 문단: 기준금리와 시장금리(국고채·CD)의 흐름과 장단기 차. 둘째 문단: 예금·대출 금리가 개인에게 뜻하는 것.
- 기준금리 변화가 없으면 「동결」이라 쓰지 말고 시장금리 흐름을 쓰십시오.` + briefSystemTail

// GenerateAssetBrief writes the home briefing for kind "re" or "rates".
func (c *Client) GenerateAssetBrief(ctx context.Context, kind, factsJSON string) (*AssetBrief, int64, error) {
	var out AssetBrief
	used, err := c.assetJSON(ctx, briefSystemFor(kind), factsJSON, 380, &out)
	out.Headline = clip(out.Headline, 40)
	out.Body = trimList(out.Body, 2, func(s string) string { return clip(s, 220) })
	out.Watch = trimList(out.Watch, 3, func(s string) string { return clip(s, 60) })
	if err == nil && (out.Headline == "" || len(out.Body) == 0) {
		err = errors.New("asset brief: empty headline or body")
	}
	return &out, used, err
}

func briefSystemFor(kind string) string {
	switch kind {
	case "re":
		return briefRESystem
	case "fin_savings":
		return briefFinSavingsSystem
	case "fin_loans":
		return briefFinLoansSystem
	}
	return briefRatesSystem
}

// 금융상품 분석 화면 맨 위(2026-10-09 요청 6) — 상품 비교가 아니라 「지금 시장이 어떤가」를 읽어 주는 자리.
const briefFinSavingsSystem = `한국 예금·적금 시장을 금융상품 분석 화면 맨 위 요약으로 씁니다(목돈·적립을 고민하는 개인이 보는 자리).
입력: {"deposit"·"saving":{"n"(12개월 상품 수),"bank"·"sb"([기본금리 평균 %, 최고금리 평균 %] — 은행·저축은행),"top"([상품명, 최고금리 %] 상위 3)},
"baseRate"(한국은행 기준금리 %),"news"(최근 3일 예·적금·금리 기사 제목, 있을 때만)}
- 첫 문단: 은행과 저축은행의 금리 수준과 차이, 기준금리 대비 위치. 둘째 문단: 적금의 최고금리와 기본금리 차이(우대조건 의존), 상위 상품의 성격.
- watch: 다음 공시에서 확인할 것(금리 방향, 특판 여부). 특정 상품 가입 권유 금지.` + briefSystemTail

const briefFinLoansSystem = `한국 대출 상품 시장을 금융상품 분석 화면 맨 위 요약으로 씁니다(대출을 알아보는 개인이 보는 자리).
입력: {"mortgage"(주택담보)·"rent"(전세자금)·"credit"(신용):{"n"(상품 수),"avg"(평균금리의 평균 %),"min"(공시 최저 %),"low"([상품명, 평균금리 %] 낮은 순 3)},
"baseRate"(한국은행 기준금리 %),"news"(최근 3일 대출·금리 기사 제목, 있을 때만)}
- 첫 문단: 종류별 평균 수준과 기준금리 대비 가산 폭. 둘째 문단: 최저·평균의 차이(신용·조건에 따라 달라지는 폭), 낮은 상품의 공통점.
- watch: 확인할 것(금리 방향, 규제 기사). 특정 상품 가입 권유 금지.` + briefSystemTail

// EstimateAssetTokens is the pre-flight cost of one asset call.
func EstimateAssetTokens(kind, input string, n int) int64 {
	system, perItem := finCondsSystem, int64(90)
	switch kind {
	case "region":
		system, perItem = regionNotesSystem, 80
	case "complex":
		system, perItem = complexNotesSystem, 80
	case "area":
		system, perItem = areaNotesSystem, 140
	case "rates":
		return EstimateTokens(ratesNoteSystem, input) + 220
	case "brief_re", "brief_rates":
		return EstimateTokens(briefSystemFor(strings.TrimPrefix(kind, "brief_")), input) + 380
	}
	return EstimateTokens(system, input) + perItem*int64(n) + 40
}

func (c *Client) assetJSON(ctx context.Context, system, input string, maxOut int64, dest any) (int64, error) {
	// 상한은 예상의 2배 + 여유 — 잘리면 JSON이 깨져 규칙 문장으로 떨어진다(돈을 더 쓰지 않는다).
	content, used, err := c.chatCompletion(WithMaxTokens(ctx, maxOut*2+256), system, input, 0.2, true, maxOut)
	if err != nil {
		return used, err
	}
	return used, decodeLLMJSON(wrapBareArray(content), dest)
}

// wrapBareArray — 모델이 가끔 {"items":[…]} 대신 배열만 준다(2026-10-02 실측). 이미 돈을 낸 응답이니 감싸서 받는다.
func wrapBareArray(content string) string {
	if s := strings.TrimSpace(stripMarkdownFence(strings.TrimSpace(content))); strings.HasPrefix(s, "[") {
		return `{"items":` + s + `}`
	}
	return content
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
