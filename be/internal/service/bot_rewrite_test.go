package service

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestValidateBotRewrite(t *testing.T) {
	d := repository.BotDraft{
		Kind: repository.BotDraftCommunity, Symbol: "005930", SymbolName: "삼성전자", Length: "short",
		Reason: "뉴스 언급 34건",
		Facts:  "· 10/16 거래량 7.4배 (등락 -4.12%)\n· 등락률 +0.80% (10/16 종가)\n· AI 점수 64점 (B+) · 상위 21%\n· 내부자 순매수 36,828,256주",
	}
	long := strings.Repeat("문장이 이어집니다. ", 6)
	cases := []struct {
		name, title, body, wantNote string
	}{
		{"ok", "삼성전자 거래량 터졌네요", "10월 16일에 거래량이 7.4배였고 그날 -4.12%였어요. 점수는 64점. 36,828,256주 순매수도 있었고요. " + long + "어떻게 보세요?", ""},
		{"made-up number", "삼성전자 이야기", "주가가 -3.5% 빠졌더라고요. " + long, "사실에 없는 숫자: 3.5"},
		{"rounded number", "삼성전자 이야기", "등락이 0.8%였어요. " + long, "사실에 없는 숫자: 0.8"},
		{"banned", "삼성전자 이야기", "지금 사세요. 무조건 오릅니다. " + long, "금지 표현: 사세요"},
		{"too short", "삼성전자", "짧아요.", "본문 길이"},
		{"single digit ok", "삼성전자 볼 것 3가지", "포인트가 3가지 있어요. 점수 64점. " + long + "#해시 @멘션", ""},
	}
	for _, c := range cases {
		body, note := validateBotRewrite(d, llm.BotDraftRewrite{Title: c.title, Body: c.body})
		if c.wantNote == "" && note != "" {
			t.Errorf("%s: rejected %q", c.name, note)
		}
		if c.wantNote != "" && !strings.HasPrefix(note, c.wantNote) {
			t.Errorf("%s: note %q, want prefix %q", c.name, note, c.wantNote)
		}
		if note == "" && strings.ContainsAny(body, "#@") {
			t.Errorf("%s: community body keeps tag characters: %q", c.name, body)
		}
	}
	// 리포트는 ## 소제목을 살리고 @만 바꾼다.
	r := d
	r.Kind = repository.BotDraftResearch
	body, note := validateBotRewrite(r, llm.BotDraftRewrite{Title: "삼성전자 점검", Body: "## 요약\n\n점수 64점. @x " + strings.Repeat("근거를 차분히 적는다. ", 20)})
	if note != "" || !strings.Contains(body, "## 요약") || strings.Contains(body, "@") {
		t.Errorf("research: note=%q body=%q", note, body[:40])
	}
}

func TestBotRewritePromptUsesArchetype(t *testing.T) {
	d := repository.BotDraft{Kind: repository.BotDraftCommunity, Archetype: "oneline", Symbol: "NVDA", SymbolName: "엔비디아", Facts: "사실"}
	sys, in := botRewritePrompt(d)
	one := botArchetypeOf(repository.BotDraftCommunity, "oneline", "")
	if !strings.Contains(sys, one.Who) || !strings.Contains(sys, one.Form) || !strings.Contains(in, "엔비디아 (NVDA)") {
		t.Errorf("prompt missing parts:\n%s\n%s", sys, in)
	}
	// 한 줄 글은 5자부터 받는다 — 일반 글 기준(40자)으로 버리면 한 줄 유형이 늘 템플릿으로 남는다.
	if _, note := validateBotRewrite(d, llm.BotDraftRewrite{Title: "엔비디아 ㄷㄷ", Body: "오늘 뉴스 엄청 나오네"}); note != "" {
		t.Errorf("one-liner rejected: %s", note)
	}
	// 키가 없는 옛 초안은 길이로 유형을 고른다.
	if a := botArchetypeOf(repository.BotDraftResearch, "", "long"); a.Key != "analyst" {
		t.Errorf("legacy long research → %s", a.Key)
	}
}

func TestPickBotArchetypeCoversAll(t *testing.T) {
	rnd := rand.New(rand.NewPCG(7, 8))
	for _, kind := range []string{repository.BotDraftCommunity, repository.BotDraftResearch} {
		seen := map[string]int{}
		for i := 0; i < 3000; i++ {
			seen[pickBotArchetype(rnd, kind).Key]++
		}
		for _, a := range botArchetypesFor(kind) {
			if seen[a.Key] == 0 {
				t.Errorf("%s archetype %s never picked", kind, a.Key)
			}
		}
	}
}
