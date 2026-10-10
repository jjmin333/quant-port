package service

import (
	"strings"
	"testing"
)

func TestResolveMentions(t *testing.T) {
	known := map[string]string{
		"삼성전자": "005930", "005930": "005930", "SK하이닉스": "000660",
		"AAPL": "AAPL", "TIGER 미국S&P500": "360750",
	}
	lookup := func(name string) (string, bool) {
		if s, ok := known[name]; ok {
			return s, true
		}
		s, ok := known[strings.ToUpper(name)]
		return s, ok
	}
	body := "반도체는 @삼성전자가 좋고 @삼성전자:점수가 70, #SK하이닉스 는 카드.\n" +
		"메일 me@삼성전자.com 은 태그 아님. @aapl. @[TIGER 미국S&P500]:PER 그리고 @없는종목 @삼성전자:모르는지표"
	toks, names := resolveMentions(parseMentions(body), lookup)

	type want struct {
		raw   string
		len   int
		sym   string
		field string
		card  bool
	}
	wants := []want{
		{"@삼성전자가", 5, "005930", "", false},
		{"@삼성전자:점수가", 8, "005930", "score", false},
		{"#SK하이닉스", 7, "000660", "", true},
		{"@aapl.", 5, "AAPL", "", false},
		{"@[TIGER 미국S&P500]:PER", 21, "360750", "per", false},
		{"@삼성전자:모르는지표", 5, "005930", "", false},
	}
	if len(toks) != len(wants) {
		t.Fatalf("got %d tokens: %+v", len(toks), toks)
	}
	for i, w := range wants {
		g := toks[i]
		if g.Raw != w.raw || g.Len != w.len || g.Symbol != w.sym || g.Field != w.field || g.Card != w.card {
			t.Errorf("#%d got %+v want %+v", i, g, w)
		}
		if n := len([]rune(g.Raw)); g.Len > n {
			t.Errorf("#%d len %d beyond raw %d", i, g.Len, n)
		}
	}
	if got := mentionSymbolOrder(toks); strings.Join(got, ",") != "005930,000660,AAPL,360750" {
		t.Errorf("order %v", got)
	}
	if strings.Join(names["005930"], ",") != "삼성전자" {
		t.Errorf("names %v", names["005930"])
	}
}

func TestPlainMentionText(t *testing.T) {
	got := plainMentionText("@삼성전자:점수 좋고 #[TIGER 미국S&P500] 메일 a@b.com")
	if got != "삼성전자 점수 좋고 TIGER 미국S&P500 메일 a@b.com" {
		t.Fatalf("got %q", got)
	}
}
