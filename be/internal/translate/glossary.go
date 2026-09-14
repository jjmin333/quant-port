package translate

import "strings"

/*
번역 뒤 손보는 자리 — **원문에 그 말이 있을 때만** 고친다.

**증거가 코드에 있었다.** 이 파일의 첫 규칙은 원래 `news_translate.go`의
`fixUSStockTranslation`이었는데, Papago가 "stock"을 「재고」로 옮겨서 생긴 것이다.
기계 번역은 금융 문맥을 모른다 — 어느 엔진을 쓰든 이 자리는 필요하다.

**규칙은 반드시 조건부다.** 「재고 → 주식」을 무조건 적용하면 진짜 재고 기사
(「반도체 재고가 쌓였다」)가 망가진다. 그래서 모든 규칙이 「원문에 이 영어 표현이
있을 때」를 함께 본다. 조건 없이 늘 고치는 것은 표기 통일 몇 개뿐이다.

**여기서 고치는 것은 낱말이 문맥을 잘못 잡은 경우뿐이다.** 문장이 통째로 이상한
것은 여기서 못 고치고, 고치려 들면 안 된다 — 그건 `Accept`가 막고 다음 엔진이 받는다.

목록의 근거는 실측이다(2026-09-03, 저장된 Papago 번역 20건과 나란히 비교). 자체
모델이 Papago보다 뚜렷하게 나빴던 건들에서 **낱말 하나만 틀린** 것을 추렸다.
*/

// contextFix는 「원문에 any가 있으면 from을 to로」다.
type contextFix struct {
	// any 중 하나라도 원문(소문자)에 있어야 적용한다.
	any []string
	// pairs는 번역문에서 바꿀 것들. 앞에서부터 순서대로 적용한다.
	pairs [][2]string
}

var contextFixes = []contextFix{
	{
		// Papago가 "stock"을 「재고」로 옮기던 것. 이 파일이 생긴 이유다.
		any: []string{"stock", "shares", "share"},
		pairs: [][2]string{
			{"재고가", "주식이"}, {"재고는", "주식은"}, {"재고를", "주식을"},
			{"재고의", "주식의"}, {" 재고 ", " 주식 "},
		},
	},
	{
		// "Rates"는 금융 헤드라인에서 거의 언제나 금리다. 실측: "Rates Still
		// Poised to Go Higher"가 자체 모델에서 「가격」, Papago에서 「요금」이 됐다.
		any: []string{"rates", "interest rate", "yields", "fed funds"},
		pairs: [][2]string{
			{"요금", "금리"}, {"이자율", "금리"},
		},
	},
	{
		// 실측: "If a Bear Market Is Coming"이 「암울한 시장」이 됐다.
		any:   []string{"bear market", "bear-market"},
		pairs: [][2]string{{"암울한 시장", "약세장"}, {"곰 시장", "약세장"}, {"약세 시장", "약세장"}},
	},
	{
		any:   []string{"bull market", "bull-market"},
		pairs: [][2]string{{"황소 시장", "강세장"}, {"강세 시장", "강세장"}},
	},
	{
		// 실측: "Student loan repayment plans"가 「학생 대출」이 됐다.
		any:   []string{"student loan"},
		pairs: [][2]string{{"학생 대출", "학자금 대출"}},
	},
	{
		// 실측: "Stock Market Midday"가 「주식 시장 중간」이 됐다.
		any:   []string{"midday"},
		pairs: [][2]string{{"중간:", "정오:"}, {"중간,", "정오,"}, {" 중간 ", " 정오 "}},
	},
	{
		// 실측: "US Equity Indexes Retreat"가 「회귀」가 됐다 — 방향이 안 읽힌다.
		any:   []string{"retreat", "retreats"},
		pairs: [][2]string{{"회귀", "하락"}, {"후퇴", "하락"}},
	},
	{
		// 실측: "Sector Laggards"가 「랭거」로 음차됐다.
		any:   []string{"laggard", "laggards"},
		pairs: [][2]string{{"랭거", "부진 종목"}, {"래거드", "부진 종목"}},
	},
	{
		// "small caps"는 소형주다. 「소규모 캡」은 캡 모자로 읽힌다.
		any:   []string{"small cap", "small-cap", "small caps"},
		pairs: [][2]string{{"소규모 캡", "소형주"}, {"작은 캡", "소형주"}},
	},
	{
		// "Defense"가 방위산업일 때 「방어」로 나온다. 섹터 이야기일 때만 고친다.
		any:   []string{"defense stocks", "defense sector", "defence stocks"},
		pairs: [][2]string{{"방어,", "방위산업,"}, {"방어 주식", "방위산업주"}},
	},
	{
		// 실측: "Enters Oversold Territory"가 「과**매출** 지역」이 됐다.
		// 과매도와 과매출은 뜻이 반대에 가깝다 — 뒤집히면 없느니만 못하다.
		any:   []string{"oversold"},
		pairs: [][2]string{{"과매출", "과매도"}, {"초과 매도", "과매도"}},
	},
	{
		any:   []string{"overbought"},
		pairs: [][2]string{{"과매수", "과매수"}, {"초과 매수", "과매수"}},
	},
	{
		// 실측: "After Wildfire Legislation"이 「**야간 화재** 법안」이 됐다.
		// wild를 옮기려다 생긴 오역인데, 산불 기사가 야간 화재 기사가 된다.
		any:   []string{"wildfire", "wildfires"},
		pairs: [][2]string{{"야간 화재", "산불"}, {"야생 화재", "산불"}, {"들불", "산불"}},
	},
}

// alwaysFixes — 문맥과 무관하게 늘 같은 표기를 쓰게 한다.
//
// 회사 이름은 여기 넣지 않는다. 이름을 표에 적기 시작하면 상장 종목 수만큼 늘어나
// 결국 유지되지 않는다 — 이름은 **번역 전에 잠근다**(names.go). 그쪽이 표를
// 늘리지 않고도 정확한 유일한 방법이다.
var alwaysFixes = [][2]string{
	{"연방준비제도이사회", "연준"},
	{"연방 준비 제도", "연준"},
	{"페드", "연준"},
}

/*
달 이름은 **사실이다.** 틀리면 문장이 어색한 게 아니라 날짜가 틀린 기사가 된다.

실측: "Secured Overnight Financing Rate 3.66% September 1"이 「**10월** 1일」로 나왔다.
번역기가 달을 하나씩 밀어 쓰는 실수는 드물지 않은데, 뜻이 통해 보여서 읽는 사람이
못 알아챈다 — 금리·실적 기사에서 날짜가 한 달 어긋나면 기사가 거짓이 된다.

원문에 달 이름이 하나만 있을 때만 고친다. 둘 이상이면 어느 것이 어디로 갔는지
알 수 없어 손대지 않는다 — 잘못 고치면 지금보다 나빠진다.
*/
var monthNames = []struct {
	en string
	ko string
}{
	{"january", "1월"}, {"february", "2월"}, {"march", "3월"},
	{"april", "4월"}, {"may", "5월"}, {"june", "6월"},
	{"july", "7월"}, {"august", "8월"}, {"september", "9월"},
	{"october", "10월"}, {"november", "11월"}, {"december", "12월"},
}

func fixMonth(source, translated string) string {
	en := strings.ToLower(source)
	var want string
	found := 0
	for _, m := range monthNames {
		// "may"는 조동사이기도 하다. 뒤에 숫자나 쉼표가 오는 날짜 표기만 본다.
		if m.en == "may" && !strings.Contains(en, "may ") {
			continue
		}
		if strings.Contains(en, m.en) {
			found++
			want = m.ko
		}
	}
	if found != 1 {
		return translated
	}
	for _, m := range monthNames {
		if m.ko != want && strings.Contains(translated, m.ko) {
			return strings.ReplaceAll(translated, m.ko, want)
		}
	}
	return translated
}

// ApplyGlossary fixes the finance-context word choices machine translation gets wrong.
func ApplyGlossary(source, translated string) string {
	out := strings.TrimSpace(translated)
	if out == "" {
		return out
	}
	en := strings.ToLower(source)

	for _, fix := range contextFixes {
		if !containsAny(en, fix.any) {
			continue
		}
		for _, p := range fix.pairs {
			// 낱말만 갈면 뒤 조사가 어긋난다(「요금이」 → 「금리이」). particle.go 참고.
			out = replaceNoun(out, p[0], p[1])
		}
	}
	// 어미 자리라 앞뒤 공백으로는 안 잡히는 한 가지.
	if containsAny(en, []string{"stock", "shares", "share"}) {
		if rest, ok := strings.CutSuffix(out, " 재고"); ok {
			out = rest + " 주식"
		}
	}
	for _, p := range alwaysFixes {
		out = replaceNoun(out, p[0], p[1])
	}
	return fixMonth(source, out)
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
