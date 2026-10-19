package service

import (
	"math/rand/v2"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
봇 초안 작성자 유형 (2026-10-19 사용자 요청) — 특정 계정의 말투를 흉내 내는 게 아니라, 실제 커뮤니티처럼
각양각색의 사람이 쓴 것처럼 보이게 한다. 초안마다 유형을 가중 랜덤으로 뽑고, 유형이 길이·말투·형식을 정한다.

가중치는 실제 종목 게시판의 모양을 따른다 — 한 줄·짧은 글이 대부분이고, 공들인 긴 글은 가끔이다.
Long은 템플릿(사실 목록)을 긴 판으로 만들지다. 짧은 유형도 사실은 넉넉히 받고 골라 쓴다.
MinBody·MaxBody는 검증의 길이 범위다(한 줄 글은 5자부터).
*/

type botArchetype struct {
	Key, Label string
	Weight     float64
	Long       bool
	MinBody    int
	MaxBody    int
	Who        string // 어떤 사람인가(말투)
	Form       string // 무엇을 얼마나 쓰나
}

var botCommunityArchetypes = []botArchetype{
	{Key: "oneline", Label: "한 줄", Weight: 22, MinBody: 5, MaxBody: 120,
		Who:  "게시판을 대충 보고 한마디 툭 던지는 사람. 반말·줄임말·「ㅋㅋ」·「ㄷㄷ」·「ㅠ」 정도는 쓴다. 맞춤법을 일부러 틀리진 않는다.",
		Form: "제목 20자 이내, 본문은 한 문장(길어야 두 문장). 숫자는 하나 쓰거나 아예 안 써도 된다. 질문으로 안 끝나도 된다."},
	{Key: "feeling", Label: "감정형", Weight: 14, MinBody: 15, MaxBody: 300,
		Who:  "주가에 기분이 오르내리는 개인투자자. 「아 진짜」, 「하…」, 「설레네요」 같은 감정 표현이 앞선다. 근거는 한두 개만.",
		Form: "제목 25자 이내, 본문 2~3문장. 분석보다 기분. 남한테 묻는 말로 끝나도 되고 아니어도 된다."},
	{Key: "question", Label: "초보 질문", Weight: 14, MinBody: 20, MaxBody: 400,
		Who:  "주식을 시작한 지 얼마 안 된 사람. 용어를 조심스럽게 쓰고 「이거 원래 이런가요?」처럼 묻는다. 존댓말.",
		Form: "제목은 질문형 30자 이내, 본문 2~4문장. 사실 하나를 보고 생긴 궁금증 하나를 묻는다."},
	{Key: "office", Label: "직장인 짧은 글", Weight: 14, MinBody: 40, MaxBody: 600,
		Who:  "폰으로 짧게 쓰는 30~40대 직장인. 존댓말 기반에 「~더라고요」, 「~인 듯」 같은 구어체.",
		Form: "제목 30자 이내, 본문 3~5문장. 본 것 하나와 자기 생각 하나, 끝에 가벼운 질문."},
	{Key: "news", Label: "뉴스 공유", Weight: 10, MinBody: 30, MaxBody: 500,
		Who:  "기사를 보고 공유하는 사람. 「기사 보니까」, 「~라네요」 식으로 전해 듣는 말투.",
		Form: "제목 35자 이내, 본문 2~4문장. 사실 속 사건·뉴스 흐름 하나를 옮기고 짧은 소감."},
	{Key: "numbers", Label: "숫자 정리", Weight: 9, MinBody: 40, MaxBody: 700,
		Who:  "말수는 적고 숫자로 말하는 사람. 군더더기 없이 정리한다.",
		Form: "제목 30자 이내, 본문은 「·」로 시작하는 3~5줄 목록 + 마지막에 한 줄 코멘트. 숫자는 사실 그대로."},
	{Key: "skeptic", Label: "신중파", Weight: 8, MinBody: 40, MaxBody: 800,
		Who:  "분위기에 휩쓸리지 않으려는 사람. 좋은 소식에도 「근데」로 반대쪽을 짚는다. 차분한 존댓말.",
		Form: "제목 35자 이내, 본문 3~6문장. 사실 중 걱정되는 쪽을 짚고, 단정은 하지 않는다."},
	{Key: "longform", Label: "장문 분석", Weight: 9, Long: true, MinBody: 400, MaxBody: 3000,
		Who:  "시간을 들여 공들여 쓰는 고수 느낌의 개인투자자. 차분하지만 자기 시각이 있다. 존댓말.",
		Form: "제목 40자 이내, 본문은 짧은 문단 5~8개(소제목 없이 문단으로). 무슨 일 → 숫자로 보면 → 걸리는 점 → 지켜볼 것 순. 끝에 의견을 묻는다."},
}

var botResearchArchetypes = []botArchetype{
	{Key: "memo", Label: "짧은 메모", Weight: 25, MinBody: 150, MaxBody: 900,
		Who:  "바쁜 개인 애널리스트가 남기는 메모. 짧고 건조하다.",
		Form: "제목 40자 이내, 본문은 마크다운 ## 소제목 2~3개, 300~600자."},
	{Key: "checklist", Label: "체크리스트", Weight: 20, MinBody: 200, MaxBody: 1500,
		Who:  "확인할 것부터 챙기는 꼼꼼한 투자자.",
		Form: "제목 45자 이내, 본문은 ## 소제목 3개 안팎, 각 소제목 아래 「- 」 목록 위주. 500~1,000자."},
	{Key: "analyst", Label: "정석 리포트", Weight: 25, Long: true, MinBody: 800, MaxBody: 5000,
		Who:  "증권사 출신 같은 차분한 개인 애널리스트. 단정하지 않고 근거 → 해석 → 위험 순.",
		Form: "제목 60자 이내, 본문은 ## 소제목 4~6개(요약·근거·해석·위험 포함), 1,200~2,500자."},
	{Key: "column", Label: "컬럼", Weight: 15, Long: true, MinBody: 800, MaxBody: 5000,
		Who:  "경제 칼럼을 쓰듯 이야기로 풀어내는 사람. 도입에서 장면이나 질문으로 시작한다.",
		Form: "제목 50자 이내, 본문은 ## 소제목 3~4개를 둔 수필형 글, 1,200~2,200자. 문장은 매끄럽게, 나열보다 흐름."},
	{Key: "risk", Label: "위험 점검", Weight: 15, MinBody: 300, MaxBody: 2500,
		Who:  "좋은 점보다 깨질 수 있는 지점을 먼저 보는 신중한 투자자.",
		Form: "제목 45자 이내, 본문은 ## 소제목 3~4개, 위험·확인할 것 비중이 절반 이상. 700~1,400자."},
}

func botArchetypesFor(kind string) []botArchetype {
	if kind == repository.BotDraftResearch {
		return botResearchArchetypes
	}
	return botCommunityArchetypes
}

func pickBotArchetype(rnd *rand.Rand, kind string) botArchetype {
	list := botArchetypesFor(kind)
	total := 0.0
	for _, a := range list {
		total += a.Weight
	}
	x := rnd.Float64() * total
	for _, a := range list {
		if x -= a.Weight; x < 0 {
			return a
		}
	}
	return list[len(list)-1]
}

// botArchetypeOf — 저장된 키의 유형. 키가 없는 옛 초안(10-19 이전)은 길이로 고른다.
func botArchetypeOf(kind, key, length string) botArchetype {
	for _, a := range botArchetypesFor(kind) {
		if a.Key == key {
			return a
		}
	}
	switch {
	case kind == repository.BotDraftResearch && length == "long":
		return botResearchArchetypes[2] // analyst
	case kind == repository.BotDraftResearch:
		return botResearchArchetypes[0] // memo
	case length == "long":
		return botCommunityArchetypes[len(botCommunityArchetypes)-1] // longform
	}
	return botCommunityArchetypes[3] // office
}
