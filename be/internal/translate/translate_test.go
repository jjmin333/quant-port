package translate

import (
	"context"
	"strings"
	"testing"
)

func TestAccept(t *testing.T) {
	cases := []struct {
		name       string
		source     string
		translated string
		want       bool
	}{
		{"정상", "Nvidia stock slips as investors weigh AI outlook",
			"AI 전망을 저울질하는 투자자들에 엔비디아 주가 하락", true},
		{"빈 값", "Nvidia stock slips", "", false},
		{"한글 없음 — 원문을 그대로 뱉었다", "Nvidia stock slips", "Nvidia stock slips", false},
		{"한글 없음 — 다른 언어", "Nvidia stock slips", "Nvidia 株価下落", false},
		{
			// 실측: LMT-60-0.6B가 없는 말을 붙였다.
			"너무 길다 — 지어냈다",
			"Fed holds rates steady",
			"연준은 인플레이션 우려와 고용 지표 둔화 그리고 국제 유가 상승과 지정학적 " +
				"리스크를 모두 고려하여 이번 회의에서 기준금리를 현재 수준으로 동결하기로 " +
				"결정했으며 시장은 이를 비둘기파적 신호로 해석하고 있다는 분석이 나온다",
			false,
		},
		{"너무 짧다 — 잘렸다", "Broadcom beats earnings estimates on strong AI chip demand", "브로드컴", false},
		{"같은 말 반복 — 루프", "Fed holds rates steady", "금리 금리 금리 금리 안정", false},
	}
	for _, tc := range cases {
		if got := Accept(tc.source, tc.translated); got != tc.want {
			t.Errorf("%s: Accept(%q) = %v, want %v", tc.name, tc.translated, got, tc.want)
		}
	}
}

func TestNeedsTranslation(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"Nvidia stock slips", true},
		{"엔비디아 주가 하락", false},
		{"엔비디아 stock slips", false}, // 이미 한글이 섞였으면 옮길 것이 없다
		{"", false},
		{"2026-09-03 +1.52%", false}, // 라틴 글자가 없다
		// 「3Q」의 Q는 라틴 글자라 true다. 보내 봐야 Accept가 막으므로 손해가 없고,
		// 여기서 걸러내려 들면 진짜 짧은 제목("AI hype")까지 같이 막힌다.
		{"2026 3Q", true},
	}
	for _, tc := range cases {
		if got := NeedsTranslation(tc.text); got != tc.want {
			t.Errorf("NeedsTranslation(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

/*
이름 잠그기 — **엔비디아 기사가 인텔 기사가 되는 것**을 막는 장치다.

실측(LMT-60-1.7B): "…for Nvidia stock investors"가 「인텔 주식 투자자들」이 됐고,
길이·한글·반복 검사를 전부 통과했다. 문장이 자연스러워서 어떤 검사로도 못 잡는다.
*/
func TestMaskAndRestoreNames(t *testing.T) {
	pairs := []NamePair{
		{English: "Nvidia", Korean: "엔비디아"},
		{English: "Eli Lilly", Korean: "일라이 릴리"},
	}

	masked, restore := MaskNames("Jim Cramer has a strong message for Nvidia stock investors", pairs)
	if masked == "Jim Cramer has a strong message for Nvidia stock investors" {
		t.Fatalf("이름이 안 잠겼다: %q", masked)
	}
	if len(restore) != 1 || restore[0] != "엔비디아" {
		t.Fatalf("되돌릴 이름표가 틀렸다: %v", restore)
	}

	// 모델이 자리표시자를 그대로 옮겨 온 경우.
	out, ok := RestoreNames("짐 크라머가 "+namePlaceholder(0)+" 주식 투자자들에게 강한 메시지를 보냈다", restore)
	if !ok {
		t.Fatal("되돌리기가 실패했다")
	}
	if out != "짐 크라머가 엔비디아 주식 투자자들에게 강한 메시지를 보냈다" {
		t.Fatalf("되돌린 문장이 틀렸다: %q", out)
	}

	// 모델이 자리표시자를 지워 버린 경우 — 그 기사는 주어가 없어진다. 실패로 친다.
	if _, ok := RestoreNames("짐 크라머가 주식 투자자들에게 강한 메시지를 보냈다", restore); ok {
		t.Fatal("자리표시자가 사라졌는데 성공으로 쳤다")
	}
}

// 긴 이름을 먼저 잠그지 않으면 앞부분만 잘리고 뒤에 찌꺼기가 남는다.
func TestMaskNamesPrefersLongerName(t *testing.T) {
	pairs := []NamePair{
		{English: "Alto", Korean: "앨토"},
		{English: "Alto Ingredients", Korean: "앨토 인그리디언츠"},
	}
	masked, restore := MaskNames("Alto Ingredients reports Q3 results", pairs)
	if len(restore) == 0 || restore[0] != "앨토 인그리디언츠" {
		t.Fatalf("긴 이름이 먼저 잠기지 않았다: masked=%q restore=%v", masked, restore)
	}
	if got, _ := RestoreNames(masked, restore); got != "앨토 인그리디언츠 reports Q3 results" {
		t.Fatalf("되돌린 결과: %q", got)
	}
}

// 낱말 경계를 안 보면 이름이 다른 낱말 속에 걸린다.
func TestMaskNamesRespectsWordBoundary(t *testing.T) {
	pairs := []NamePair{{English: "Live", Korean: "라이브 벤처스"}}
	if masked, restore := MaskNames("Delivery volumes rose", pairs); len(restore) != 0 {
		t.Fatalf("Delivery 안의 live가 잠겼다: %q", masked)
	}
	// 소유격은 이름의 일부가 아니다.
	pairs = []NamePair{{English: "Lilly", Korean: "일라이 릴리"}}
	if _, restore := MaskNames("Lilly's record stock price", pairs); len(restore) != 1 {
		t.Fatal("소유격 앞의 이름을 못 잡았다")
	}
}

func TestApplyGlossary(t *testing.T) {
	// Papago가 "stock"을 「재고」로 옮기던 것 — 이 함수가 생긴 이유다.
	got := ApplyGlossary("Apple stock rises", "애플 재고가 상승")
	if got != "애플 주식이 상승" {
		t.Errorf("stock 보정 실패: %q", got)
	}
	// 재고 이야기를 하는 기사는 건드리지 않는다.
	got = ApplyGlossary("Chip inventory builds up", "반도체 재고가 쌓인다")
	if got != "반도체 재고가 쌓인다" {
		t.Errorf("재고 기사를 건드렸다: %q", got)
	}
}

func TestCleanModelOutput(t *testing.T) {
	cases := [][2]string{
		{"엔비디아 주가 하락", "엔비디아 주가 하락"},
		{"Korean: 엔비디아 주가 하락", "엔비디아 주가 하락"},
		{`"엔비디아 주가 하락"`, "엔비디아 주가 하락"},
		{"엔비디아 주가 하락\n(설명을 덧붙임)", "엔비디아 주가 하락"},
		{`  엔비디아 "AI" 주가 하락  `, `엔비디아 "AI" 주가 하락`}, // 안쪽 인용부호는 원문의 일부
	}
	for _, c := range cases {
		if got := cleanModelOutput(c[0]); got != c[1] {
			t.Errorf("cleanModelOutput(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

/*
Chain — 앞 엔진이 「받아들일 수 없는」 번역을 내면 뒤 엔진이 받는다.

오류만 보고 넘기면 안 되는 이유가 여기 있다: 자체 모델이 **살아 있는 채로** 이상한
문장을 꾸준히 뱉는 상황이 제일 위험한데, 그건 오류로 안 잡힌다.
*/
func TestChainFallsBackOnRejectedTranslation(t *testing.T) {
	bad := &stubTranslator{name: "bad", out: []string{"Nvidia stock slips", "금리 금리 금리 금리"}}
	good := &stubTranslator{name: "good", out: []string{"엔비디아 주가 하락", "연준 금리 동결"}}
	chain := NewChain(bad, good)

	got, err := chain.TranslateENtoKO(t.Context(),
		[]string{"Nvidia stock slips", "Fed holds rates steady"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got[0] != "엔비디아 주가 하락" || got[1] != "연준 금리 동결" {
		t.Fatalf("폴백 결과가 틀렸다: %v", got)
	}
	// 두 번째 엔진에는 **못 채운 자리만** 간다 — 전부 다시 보내면 비용을 아낀 값이 없다.
	if bad.lastLen != 2 || good.lastLen != 2 {
		t.Fatalf("넘어간 건수: bad=%d good=%d", bad.lastLen, good.lastLen)
	}
	snap := chain.Snapshot()
	if snap[0].Rejected != 2 || snap[1].Accepted != 2 {
		t.Fatalf("집계가 틀렸다: %+v", snap)
	}
}

func TestChainOnlyRetriesWhatFailed(t *testing.T) {
	partial := &stubTranslator{name: "partial", out: []string{"엔비디아 주가 하락", ""}}
	backup := &stubTranslator{name: "backup", out: []string{"연준 금리 동결"}}
	chain := NewChain(partial, backup)

	got, _ := chain.TranslateENtoKO(t.Context(),
		[]string{"Nvidia stock slips", "Fed holds rates steady"})
	if got[0] != "엔비디아 주가 하락" || got[1] != "연준 금리 동결" {
		t.Fatalf("결과: %v", got)
	}
	if backup.lastLen != 1 {
		t.Fatalf("폴백에 %d건이 갔다 — 실패한 1건만 가야 한다", backup.lastLen)
	}
}

type stubTranslator struct {
	name    string
	out     []string
	lastLen int
}

func (s *stubTranslator) Name() string    { return s.name }
func (s *stubTranslator) Available() bool { return true }
func (s *stubTranslator) TranslateENtoKO(_ context.Context, texts []string) ([]string, error) {
	s.lastLen = len(texts)
	out := make([]string, len(texts))
	copy(out, s.out)
	return out, nil
}

/*
용어 규칙은 **조건부**여야 한다.

「재고 → 주식」을 무조건 적용하면 진짜 재고 기사가 망가진다. 아래 표는 실측에서
자체 모델이 Papago보다 뚜렷하게 나빴던 건들 — 낱말 하나만 틀린 것들이다.
*/
func TestApplyGlossaryContextRules(t *testing.T) {
	cases := []struct{ name, en, ko, want string }{
		{"금리", "Fortress's Burton Says Rates Still Poised to Go Higher",
			"버턴, 요금이 더 오를 전망", "버턴, 금리가 더 오를 전망"},
		{"약세장", "If a Bear Market Is Coming, This Growth Stock Might Be a Great Buy",
			"만일 암울한 시장이 온다면", "만일 약세장이 온다면"},
		{"학자금", "Student loan repayment plans: What are your options now?",
			"학생 대출 상환 계획", "학자금 대출 상환 계획"},
		{"정오", "Stock Market Midday, Sept. 1: Stocks Slide",
			"주식 시장 중간: 주가 하락", "주식 시장 정오: 주가 하락"},
		{"하락", "US Equity Indexes Retreat Amid Surging Treasury Yields",
			"미국 주식 지수, 국채 수익률 급등에 회귀", "미국 주식 지수, 국채 수익률 급등에 하락"},
		{"소형주", "September seasonality puts S&P 500 and small caps in focus",
			"S&P 500과 소규모 캡이 초점", "S&P 500과 소형주가 초점"},

		// 조건이 안 맞으면 건드리지 않는다 — 이게 규칙을 조건부로 둔 이유다.
		{"재고 기사는 그대로", "Chip inventory builds up", "반도체 재고가 쌓인다", "반도체 재고가 쌓인다"},
		{"요금 기사는 그대로", "Utility bills climb this winter", "이번 겨울 요금이 오른다", "이번 겨울 요금이 오른다"},
		{"곰 기사는 그대로", "A bear wandered into the town", "곰 시장에 나타났다", "곰 시장에 나타났다"},
	}
	for _, c := range cases {
		if got := ApplyGlossary(c.en, c.ko); got != c.want {
			t.Errorf("%s: ApplyGlossary(%q) = %q, want %q", c.name, c.ko, got, c.want)
		}
	}
}

// 낱말을 바꾸면 뒤 조사도 같이 바뀌어야 한다 — 표를 다섯 배로 늘리지 않으려는 장치.
func TestReplaceNounFixesParticle(t *testing.T) {
	cases := []struct{ text, from, to, want string }{
		// 받침 있음 → 없음
		{"요금이 오른다", "요금", "금리", "금리가 오른다"},
		{"요금은 오른다", "요금", "금리", "금리는 오른다"},
		{"요금을 올린다", "요금", "금리", "금리를 올린다"},
		// 받침 없음 → 있음
		{"금리가 오른다", "금리", "요금", "요금이 오른다"},
		// ㄹ 받침은 「로」를 쓴다
		{"부산으로 간다", "부산", "서울", "서울로 간다"},
		// 조사가 아니라 낱말의 일부면 건드리지 않는다
		{"요금인상 소식", "요금", "금리", "금리인상 소식"},
		// 조사가 없으면 낱말만 바꾼다
		{"요금 인상 소식", "요금", "금리", "금리 인상 소식"},
	}
	for _, c := range cases {
		if got := replaceNoun(c.text, c.from, c.to); got != c.want {
			t.Errorf("replaceNoun(%q, %q, %q) = %q, want %q", c.text, c.from, c.to, got, c.want)
		}
	}
}

// 달 이름은 사실이다 — 틀리면 날짜가 틀린 기사가 된다.
func TestFixMonth(t *testing.T) {
	cases := []struct{ name, en, ko, want string }{
		{"밀려 쓴 달을 되돌린다",
			"Secured Overnight Financing Rate 3.66% September 1 vs 3.68% August 31.",
			"10월 1일 보장된 밤 자금 조달 비율은 3.66%", "10월 1일 보장된 밤 자금 조달 비율은 3.66%"},
		{"달이 하나면 고친다", "Fed minutes from September meeting", "10월 회의 의사록", "9월 회의 의사록"},
		{"이미 맞으면 그대로", "Fed minutes from September meeting", "9월 회의 의사록", "9월 회의 의사록"},
		{"달이 없으면 그대로", "Fed holds rates steady", "연준 금리 동결", "연준 금리 동결"},
		{"may는 조동사일 수 있다", "Stocks may rise", "주가가 오를 수 있다", "주가가 오를 수 있다"},
	}
	for _, c := range cases {
		if got := ApplyGlossary(c.en, c.ko); got != c.want {
			t.Errorf("%s: = %q, want %q", c.name, got, c.want)
		}
	}
}

// 뜻이 뒤집히는 오역 — 어색한 것과 다르다. 실측에서 나온 것만 표에 넣는다.
func TestApplyGlossaryReversedMeanings(t *testing.T) {
	cases := []struct{ name, en, ko, want string }{
		{"과매도", "Enterprise Financial Services Enters Oversold Territory",
			"엔터프라이즈 금융 서비스가 과매출 지역에 진출", "엔터프라이즈 금융 서비스가 과매도 지역에 진출"},
		{"산불", "These California Stocks Plummet After Wildfire Legislation Agreement",
			"야간 화재 법안 합의 후 캘리포니아 주식 폭락", "산불 법안 합의 후 캘리포니아 주식 폭락"},
		{"조건이 없으면 그대로", "Night fire drill at the plant", "야간 화재 훈련", "야간 화재 훈련"},
	}
	for _, c := range cases {
		if got := ApplyGlossary(c.en, c.ko); got != c.want {
			t.Errorf("%s: = %q, want %q", c.name, got, c.want)
		}
	}
}

/*
자리표시자에 **숫자가 없어야** 한다 — 이 시험이 이 파일에 있는 이유는 실측 사고다.

Azure로 엔진을 바꾼 뒤 이틀 동안 해외 제목 번역이 사실상 전부 실패했다. 원인은
Azure가 글자 뒤의 숫자를 낱말에서 떼어낸 것이었다(`$QPZ0` → `$QPZ 0`, 때로는
`$QPZ년`). 되돌리기가 실패하면 조용히 영어로 남으므로 **아무 오류도 안 났고**,
같은 24건이 2분마다 다시 나가 하루 100만 자를 태웠다.

그래서 「숫자를 안 쓴다」를 규칙으로 못박는다. 다음 사람이 `strconv.Itoa`로
되돌리면 여기서 걸린다.
*/
func TestNamePlaceholderHasNoDigits(t *testing.T) {
	for i := 0; i < 30; i++ {
		ph := namePlaceholder(i)
		if ph == "" {
			t.Fatalf("%d번 자리표시자가 비었다", i)
		}
		for _, r := range ph {
			if r >= '0' && r <= '9' {
				t.Fatalf("자리표시자에 숫자가 있다: %q (Azure가 떼어낸다)", ph)
			}
		}
	}
}

// 고정폭이라야 서로의 접두가 되지 않는다 — 27번째 이름이 나오는 순간의 문제다.
func TestNamePlaceholderIsNotAPrefixOfAnother(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 26*26; i++ {
		ph := namePlaceholder(i)
		if ph == "" {
			t.Fatalf("%d번 자리표시자가 비었다", i)
		}
		if seen[ph] {
			t.Fatalf("자리표시자가 겹친다: %q", ph)
		}
		seen[ph] = true
	}
	if len(seen) != 26*26 {
		t.Fatalf("자리표시자 개수: %d", len(seen))
	}
	// 전부 같은 길이면 어느 하나가 다른 하나의 접두가 될 수 없다.
	n := len(namePlaceholder(0))
	for ph := range seen {
		if len(ph) != n {
			t.Fatalf("길이가 다르다: %q (%d) vs %d", ph, len(ph), n)
		}
	}
	// 범위를 넘으면 패닉이 아니라 빈 문자열이다.
	if namePlaceholder(26*26) != "" || namePlaceholder(-1) != "" {
		t.Fatal("범위 밖에서 빈 문자열이 아니다")
	}
}

/*
번역기가 자리표시자 안에 공백을 넣어도 되돌린다.

지금 쓰는 두 글자 접미사는 Azure에서 쪼개지지 않지만, 다음 엔진이 무엇을 할지는
모르고 **이 실패는 조용하다**. 안전망을 시험으로 잡아 둔다.
*/
func TestRestoreNamesToleratesSplitPlaceholder(t *testing.T) {
	restore := []string{"엔비디아", "마이크론"}
	ph0, ph1 := namePlaceholder(0), namePlaceholder(1)
	// "$QPZAA" → "$QPZ AA"
	spaced := func(s string) string { return s[:4] + " " + s[4:] }

	out, ok := RestoreNames(spaced(ph0)+"의 신호가 "+spaced(ph1)+"에 위험을 시사한다", restore)
	if !ok {
		t.Fatal("공백이 낀 자리표시자를 못 되돌렸다")
	}
	if out != "엔비디아의 신호가 마이크론에 위험을 시사한다" {
		t.Fatalf("되돌린 문장이 틀렸다: %q", out)
	}

	// 대소문자만 달라진 것도 그대로 받는다.
	out, ok = RestoreNames(strings.ToLower(ph0)+"가 올랐다", restore[:1])
	if !ok || out != "엔비디아가 올랐다" {
		t.Fatalf("소문자 자리표시자: ok=%v out=%q", ok, out)
	}

	// 글자가 **바뀐** 것은 여전히 실패다 — 어느 회사였는지 알 수 없다.
	if _, ok := RestoreNames("$QPZ년 주식 보유 증가", restore[:1]); ok {
		t.Fatal("글자가 망가진 자리표시자를 성공으로 쳤다")
	}
}

// 공백 건너뛰기가 **아무 글자나** 건너뛰면 안 된다 — 다른 자리표시자에 맞아 버린다.
func TestRestoreNamesDoesNotMatchAcrossOtherCharacters(t *testing.T) {
	if _, _, ok := indexSpaced("$QPZxAA", "$QPZAA"); ok {
		t.Fatal("공백이 아닌 글자를 건너뛰었다")
	}
	if _, _, ok := indexSpaced("$QPZ\tAA", "$QPZAA"); !ok {
		t.Fatal("탭을 공백으로 안 봤다")
	}
}
