package service

import (
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

func TestBotRewritePromptUsesStyle(t *testing.T) {
	d := repository.BotDraft{Kind: repository.BotDraftCommunity, Length: "short", Symbol: "NVDA", SymbolName: "엔비디아", Facts: "사실"}
	sys, in := botRewritePrompt(d, "50대 은퇴자, 느긋한 말투")
	if !strings.Contains(sys, "50대 은퇴자") || !strings.Contains(in, "엔비디아 (NVDA)") || !strings.Contains(in, "사실") {
		t.Errorf("prompt missing parts:\n%s\n%s", sys, in)
	}
	if sys, _ := botRewritePrompt(d, ""); !strings.Contains(sys, botDefaultStyle[repository.BotDraftCommunity]) {
		t.Error("empty style must fall back to the default")
	}
}
