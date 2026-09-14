package repository

import (
	"sort"
	"strings"
	"unicode"

	"github.com/enact/quant-port/be/internal/model"
)

/*
후보 상한. 랭킹은 Go에서 하므로 이 수를 넘는 매치는 순위에 참여하지 못한다.

실측 (2026-08-26, stocks 17,911행): 흔한 검색어의 매치 수는 「삼성」98 · 「전자」48 ·
「코리아」62 · 「인버스」160 · 「KODEX」239 · 「스팩」74 로 전부 여유가 있다. 500에서
1000으로 올린 것은 정확도를 위해서가 아니라, 후보 절단이 순위를 바꾸는 상황을 아예
만들지 않기 위한 여유다. 후보 질의의 정렬(`appendSearchOrder`)이 심볼 일치·이름 접두를
앞으로 당기고 마지막에 `symbol`로 묶으므로, 절단이 일어나도 **같은 입력에 같은 결과**가
나오고 강한 매치가 먼저 들어온다.
*/
const searchCandidateCap = 1000

/*
relevanceTierSize 는 관련도 점수를 묶는 폭이다.

왜 필요한가: 예전에는 `score`를 그대로 1차 정렬 기준으로 썼다. 그런데 관련도 점수에는
`english_name` 매치 같은 **작은 가산점**이 섞여 있어서, 그 1,000점 남짓의 차이가 「본주 vs 파생상품」이라는 훨씬 큰 신호(`SearchInstrumentPriority`)를
뒤집었다. 실측: 「삼성」 검색 상위 20개가 전부 ETN이고 삼성전자는 21번째였다.

점수를 1,000 단위로 묶으면 정확 일치(+50,000)·심볼 일치(+10,000)처럼 **의미 있는 차이는
그대로 살아 있고**, 자잘한 가산점은 같은 칸에 묶여 그 다음 기준(본주 우선 → 시가총액)이
판단하게 된다.
*/
const relevanceTierSize = 1000

/*
IsLatinOnlyQuery is true when the query has no Hangul (English ticker/name search).

`ContainsHangul`\uACFC \uAC19\uC740 \uD310\uC815\uC744 \uC4F4\uB2E4. \uC608\uC804\uC5D0\uB294 \uC5EC\uAE30\uC11C\uB9CC \uC644\uC131\uD615 \uC74C\uC808(AC00~D7A3)\uC744 \uBD10\uC11C
\uC790\uBAA8\uB9CC \uB0A8\uC740 \uC9C8\uC758(\u300C\u3145\u300D, \u300C\u3131\u3147\u300D \u2014 \uD55C\uAE00 IME \uC870\uD569 \uC911\uC5D0 \uADF8\uB300\uB85C \uB0A0\uC544\uC628\uB2E4)\uAC00 \uB77C\uD2F4 \uC9C8\uC758\uB85C
\uBD84\uB958\uB410\uACE0, \uADF8\uB7EC\uBA74 \uB77C\uD2F4 \uADDC\uCE59(\uC2EC\uBCFC\u00B7\uC601\uBB38 \uD1A0\uD070)\uC5D0 \uAC78\uB9B4 \uB9AC\uAC00 \uC5C6\uC5B4 \uACB0\uACFC\uAC00 0\uAC74\uC774\uC5C8\uB2E4.
*/
func IsLatinOnlyQuery(q string) bool {
	return strings.TrimSpace(q) != "" && !ContainsHangul(q)
}

/*
LooksLikeDomesticCode matches Korean listing codes typed into search (005930, 0194M0).

왜 따로 판별하는가: 한국 종목코드는 한글이 없으므로 `IsLatinOnlyQuery`가 참이 되고,
예전 필터는 라틴 질의를 전부 `asset_type != 'domestic'`으로 몰아 **국내 종목을 아예
후보에서 제외**했다. 실측: `005930` 검색 결과 0건 — 삼성전자가 DB에 있는데도 못 찾았다
(`000660`, `00593` 같은 접두어도 동일).

판별 규칙은 「첫 글자가 숫자이고 6자 이하의 영숫자」다. 미국 티커는 숫자로 시작하는
경우가 사실상 없고(거래소 규칙상 알파벳으로 시작한다), 국내 코드는 `0044K0`처럼 뒤에
영문이 붙는 것까지 이 규칙에 들어온다. 접두어 검색을 살리려고 하한을 2로 뒀다.
*/
func LooksLikeDomesticCode(q string) bool {
	q = strings.TrimSpace(q)
	runes := []rune(q)
	if len(runes) < 2 || len(runes) > 6 {
		return false
	}
	if runes[0] < '0' || runes[0] > '9' {
		return false
	}
	for _, r := range runes {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			continue
		}
		return false
	}
	return true
}

// LooksLikeTickerSymbol matches short exchange tickers (e.g. ARM, ORCL).
func LooksLikeTickerSymbol(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" || len(q) > 12 {
		return false
	}
	for _, r := range q {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func tokenizeEnglishName(s string) []string {
	s = strings.ToLower(s)
	replacer := strings.NewReplacer(",", " ", ".", " ", "(", " ", ")", " ", "-", " ", "/", " ")
	return strings.Fields(replacer.Replace(s))
}

/*
latinSearchTokens 는 문자열에서 **영숫자 덩어리**만 뽑는다 (소문자).

	"KT&G"       -> ["kt", "g"]
	"SK하이닉스"  -> ["sk"]
	"JYP Ent."   -> ["jyp", "ent"]

`tokenizeEnglishName`과 달리 한글·기호를 경계로도 끊는다. 그래서 **한글 이름 안에 섞인
영문**(국내 상장명이 이렇다)을 라틴 질의로 찾을 수 있다.
*/
func latinSearchTokens(s string) []string {
	runs := latinRuns(s)
	out := make([]string, 0, len(runs))
	for _, run := range runs {
		out = append(out, strings.ToLower(run))
		// 붙여 쓴 이름을 낱말로도 본다: "SamsungElec" -> samsung/elec.
		// 카탈로그의 국내 `english_name`이 이렇게 압축돼 있어서, 이게 없으면
		// 「samsung」이 삼성전자를 접두(약한 점수)로만 잡았다.
		if parts := splitCamelCase(run); len(parts) > 1 {
			out = append(out, parts...)
		}
	}
	return out
}

// latinRuns returns the maximal latin/digit runs of s, original case.
func latinRuns(s string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// splitCamelCase splits "SamsungElec" into ["samsung", "elec"]; a single-case run yields one part.
func splitCamelCase(run string) []string {
	var out []string
	var cur strings.Builder
	prevLower := false
	for _, r := range run {
		if prevLower && r >= 'A' && r <= 'Z' && cur.Len() > 0 {
			out = append(out, strings.ToLower(cur.String()))
			cur.Reset()
		}
		cur.WriteRune(r)
		prevLower = r >= 'a' && r <= 'z'
	}
	if cur.Len() > 0 {
		out = append(out, strings.ToLower(cur.String()))
	}
	return out
}

// condenseLatin joins every latin/digit run of s ("KT&G" -> "ktg", "SK하이닉스" -> "sk").
func condenseLatin(s string) string {
	return strings.ToLower(strings.Join(latinRuns(s), ""))
}

/*
latinNameScore 는 라틴 질의가 **표시 이름**과 얼마나 맞는지 점수화한다.

이름의 영문이 **질의 그 자체**면 9,000 · 단어 일치 3,500 · 단어 접두 1,800.
부분 문자열(Contains)은 쓰지 않는다 — 「arm」이 「Ph**arm**a」를 끌어오는 류의 오탐이
거기서 나왔다.

9,000 칸이 따로 있는 이유: 그게 없으면 심볼 접두(5,000)가 이름 전체 일치를 이겨서,
「LG」·「HD」 검색 상위가 LGIH·HDSN 같은 남의 티커로 채워지고 정작 LG전자·HD현대는
한참 뒤였다. 이름의 영문이 통째로 질의와 같다는 건 심볼 접두보다 강한 신호다.
*/
func latinNameScore(qNorm, name string) int {
	if qNorm == "" {
		return 0
	}
	toks := latinSearchTokens(name)
	// 기호·한글을 지운 전체 형태. 「KT&G」·「LG전자」처럼 토큰이 쪼개지거나
	// 한글이 뒤에 붙는 이름을 살린다.
	cond := condenseLatin(name)
	if cond == qNorm {
		return 9000
	}

	best := 0
	for _, t := range toks {
		if t == qNorm {
			return 3500
		}
		if len(qNorm) >= 2 && strings.HasPrefix(t, qNorm) {
			best = 1800
		}
	}
	if len(qNorm) >= 2 && strings.HasPrefix(cond, qNorm) && best < 1800 {
		best = 1800
	}
	return best
}

/*
domesticEnglishNameScore 는 국내 종목의 `english_name` 매치를 **한 칸 아래로** 눌러 준다.

`english_name`은 자동 번역된 영문 표기라 신뢰도가 낮다. 그렇다고 아예 안 보면
「samsung」·「hyundai」 같은 로마자 검색이 국내 종목을 하나도 못 찾는다. 그래서
보되, 표시 이름 매치(3,500)나 해외 종목 매치보다 항상 뒤(2,500/1,200)에 놓는다.
*/
func domesticEnglishNameScore(qNorm, englishName string) int {
	switch latinNameScore(qNorm, englishName) {
	case 9000, 3500:
		return 2500
	case 1800:
		return 1200
	default:
		return 0
	}
}

func searchPopularity(st model.Stock) int64 {
	if st.SharesOutstanding != nil && *st.SharesOutstanding > 0 {
		return *st.SharesOutstanding
	}
	return 0
}

func searchRelevanceScore(q string, st model.Stock, latinOnly bool) int {
	q = strings.TrimSpace(q)
	if q == "" {
		return 1
	}
	qLower := strings.ToLower(q)
	sym := strings.ToLower(st.Symbol)
	en := strings.ToLower(st.EnglishName)
	name := strings.ToLower(st.Name)

	score := 0

	/*
		국내 종목코드를 그대로 친 경우. 자산군을 가리지 않고 심볼로 판단한다.

		접두 일치에서 **곧바로 반환하지 않는다.** 「3M」·「180」처럼 숫자로 시작하는
		미국 종목명은 `LooksLikeDomesticCode`가 참이 되는데, 예전에는 여기서 끝나
		이름 일치를 아예 보지 못했다(코드 접두에 걸리는 게 없으면 0점).
	*/
	if LooksLikeDomesticCode(q) {
		switch {
		case sym == qLower:
			return 60000
		case strings.HasPrefix(sym, qLower):
			score += 20000
		}
	}

	if sym == qLower {
		score += 10000
	} else if strings.HasPrefix(sym, qLower) {
		score += 5000
	} else if len(qLower) >= 3 && strings.Contains(sym, qLower) {
		score += 1200
	}

	/*
		라틴 질의는 **표시 이름(`name`)** 을 먼저 본다.

		예전에는 라틴 질의가 `english_name`만 봤고, 국내 종목은 3글자 이하일 때만
		그 예외를 받았다. 그래서 상장명 자체가 영문인 종목이 **이름으로는 검색되지
		않았다**: HPSP·NAVER·KT&G·SOOP·LIG넥스원… (「HPSP」 → 코드 403870으로는
		나오는데 이름으로는 0건). 국내 ETF도 `english_name`이 비어 있으면 「KODEX」가
		안 잡혔다.

		국내 종목의 `english_name`은 자동 번역 칸이라 **한 칸 아래**로 눌러서 본다
		(`domesticEnglishNameScore`). 「Pharma」가 국내 제약주를 통째로 끌어오던 오탐은
		부분 문자열 매치를 없앤 것으로 해결됐고, 로마자 검색(「samsung」)은 살렸다.
	*/
	if latinOnly {
		qNorm := condenseLatin(q)
		best := latinNameScore(qNorm, st.Name)
		enScore := latinNameScore(qNorm, st.EnglishName)
		if st.AssetType == model.AssetDomestic {
			enScore = domesticEnglishNameScore(qNorm, st.EnglishName)
		}
		if enScore > best {
			best = enScore
		}
		return score + best
	}

	if strings.Contains(name, q) || strings.Contains(name, qLower) {
		score += 4500
		if strings.EqualFold(strings.TrimSpace(st.Name), strings.TrimSpace(q)) {
			score += 50000
		}
	}

	/*
		한글 질의에는 `english_name`을 보지 않는다.

		`english_name`은 이름 그대로 영문 표기 칸이지만, 카탈로그를 아직 보강하지 못한
		행에는 **한글 이름이 그대로 복사돼 있다**. 그 행에서만 아래 두 가산점이 붙어
		같은 이름 매치인데 +1,200점을 더 받았고, 그 차이로 파생상품이 본주를 넘어섰다.

		실측으로 확인한 증상 두 가지가 모두 여기서 나왔다:

		 1. 「삼성」 검색 상위 20개가 전부 ETN (삼성전자는 21번째).
		    530127~530129는 `english_name`이 「삼성 미국다우존스 TR ETN」처럼 한글이라
		    5,700점, 삼성전자는 4,500점이었다.

		 2. **같은 검색어인데 호출마다 결과가 한 칸씩 밀렸다.** 검색은 1위 종목을
		    카탈로그 보강 대상으로 올리는데(`triggerCatalogBackfill`), 보강이 그 행의
		    `english_name`을 제대로 된 영문으로 바꾸면 점수가 5,700 → 4,500으로 떨어져
		    다음 호출에서는 아직 보강 안 된 다음 ETN이 1위가 됐다. 검색할수록 순위가
		    흘러가는 구조였다.

		한글 질의가 영문 표기와 일치할 일은 없으므로, 이 조건이 참이 된다는 것 자체가
		「english_name에 한글이 들어 있다」는 뜻이다 — 즉 `name` 매치의 중복 계산이다.
	*/
	if !ContainsHangul(q) {
		for _, word := range tokenizeEnglishName(en) {
			if word == qLower || strings.HasPrefix(word, qLower) {
				score += 800
			}
		}
		if strings.Contains(en, qLower) {
			score += 400
		}
	}

	/*
		한글 발음으로 영문 상장명을 찾는 경로(`hangulSearchAliases`).

		「네이버」·「포스코」·「숲」은 `name`에도 `english_name`에도 없는 읽기라 위의
		규칙에 하나도 걸리지 않는다(실측 0건). 별칭 점수는 이름 매치의 절반이라
		직접 이름이 걸린 행을 밀어내지 않는다 — 자세한 이유는 `searchAliasScore`.
	*/
	if alias := searchAliasScore(q, st); alias > score {
		score = alias
	}
	/*
		심볼 별칭은 **이름 일치만큼** 쳐준다 (2026-09-13).

		브랜드 별칭을 절반으로 누른 이유는 그것이 「이 영문 표기가 든 종목 전부」를
		가리키는 넓은 추측이기 때문이다. 심볼 별칭은 반대다 — 사람이 종목 하나를
		지목해 적은 값이라 이름이 실제로 그렇게 적혀 있는 것과 확신의 크기가 같다.
		절반으로 누르면 「구글」에서 이름에 「구글」이 든 ETF가 GOOGL 위로 올라간다.
	*/
	if sym := searchAliasSymbolScore(q, st); sym > score {
		score = sym
	}
	return score
}

type scoredStock struct {
	stock model.Stock
	score int
	pop   int64
	pri   int
	// KRW 환산 시가총액. 알 수 없으면 0이고, 그때는 `pop`이 대신 판단한다.
	mcap float64
}

// SearchInstrumentPriority ranks listing types for search tie-breaks (higher = 본주 우선).
func SearchInstrumentPriority(st model.Stock) int {
	combined := strings.ToLower(st.Name + " " + st.EnglishName + " " + st.SecurityType)
	sec := strings.ToUpper(strings.TrimSpace(st.SecurityType))

	if st.AssetType == model.AssetETF || strings.Contains(sec, "ETF") || strings.Contains(sec, "ETN") {
		if isLeveragedOrStructuredETP(combined) {
			return 10
		}
		return 25
	}
	if strings.Contains(combined, "adr") {
		return 55
	}
	if isLeveragedOrStructuredETP(combined) {
		return 15
	}
	if st.AssetType == model.AssetDomestic || st.AssetType == model.AssetUS {
		return 100
	}
	return 40
}

/*
isLeveragedOrStructuredETP 는 이름/종류에서 파생·구조화 상품 표지를 찾는다.

한글 표지는 부분 문자열로 본다(한글은 띄어쓰기 없이 붙는다). **영문 표지는 단어 단위로**
본다 — 예전에는 부분 문자열이라 「short」·「etn」·「ultra」가 우연히 들어간 이름
(Ultragenyx, Aetna, Shoreline…)까지 파생상품으로 몰아 본주를 검색 뒤로 밀었다.
합성어는 목록에 그대로 적는다(UltraPro/UltraShort). 접두 일치를 쓰지 않는 이유가
방금 그 오탐이다.
*/
func isLeveragedOrStructuredETP(s string) bool {
	koreanMarkers := []string{
		"레버리지", "레버", "인버스", "2배", "3배", "단일종목", "선물단일", "밸류체인",
	}
	for _, m := range koreanMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}

	for _, tok := range latinSearchTokens(s) {
		if _, hit := etpLatinMarkers[tok]; hit {
			return true
		}
	}
	return false
}

var etpLatinMarkers = map[string]struct{}{
	"etn": {}, "2x": {}, "3x": {}, "ultra": {}, "ultrapro": {}, "ultrashort": {},
	"leverage": {}, "leveraged": {}, "short": {}, "inverse": {},
}

func rankSearchResults(items []model.Stock, q string, latinOnly bool) []model.Stock {
	return rankSearchResultsWithCaps(items, q, latinOnly, nil)
}

/*
rankSearchResultsWithCaps orders search hits.

정렬 기준을 순서대로:

	1. 관련도 **칸** (`relevanceTierSize` 단위) — 정확 일치가 부분 일치를 이긴다
	2. 상품 우선순위 — **본주 > 일반 ETF/ETN > 레버리지·인버스** (`SearchInstrumentPriority`)
	3. **시가총액** (KRW 환산) — 같은 칸·같은 종류면 큰 회사가 먼저
	4. 상장주식수 — 시총을 모르는 종목(종가 미수집)의 대체 기준
	5. 심볼 — 최종 결정자. 여기까지 오면 **입력이 같으면 결과가 같다**

2번을 1번 바로 다음에 둔 것이 이번 수정의 핵심이다. 파생상품을 목록에서 빼지 않는
이유는 그러면 그 상품을 검색으로 찾을 방법이 사라지기 때문이고, 대신 본주 뒤로 보낸다.

5번을 명시적으로 둔 이유: 예전에는 `pop`(상장주식수)까지만 비교했는데 ETN들이 대부분
1,000,000주로 같아서 그 뒤가 정해지지 않았다. Go의 `sort.SliceStable`은 입력 순서를
보존하지만 그 입력은 `LIMIT`에 `ORDER BY`가 없던 SQL이 정한 것이라, 결국 순서가
DB 사정에 따라 흔들렸다.
*/
func rankSearchResultsWithCaps(
	items []model.Stock,
	q string,
	latinOnly bool,
	caps map[string]float64,
) []model.Stock {
	capOf := func(st model.Stock) float64 {
		if caps == nil {
			return 0
		}
		return caps[strings.ToUpper(strings.TrimSpace(st.Symbol))]
	}

	q = strings.TrimSpace(q)
	if q == "" {
		sort.SliceStable(items, func(i, j int) bool {
			ci, cj := capOf(items[i]), capOf(items[j])
			if ci != cj {
				return ci > cj
			}
			pi, pj := searchPopularity(items[i]), searchPopularity(items[j])
			if pi != pj {
				return pi > pj
			}
			return items[i].Symbol < items[j].Symbol
		})
		return items
	}

	scored := make([]scoredStock, 0, len(items))
	for _, st := range items {
		rel := searchRelevanceScore(q, st, latinOnly)
		if rel <= 0 {
			continue
		}
		scored = append(scored, scoredStock{
			stock: st,
			score: rel,
			pop:   searchPopularity(st),
			pri:   SearchInstrumentPriority(st),
			mcap:  capOf(st),
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		a, b := scored[i], scored[j]
		if ta, tb := a.score/relevanceTierSize, b.score/relevanceTierSize; ta != tb {
			return ta > tb
		}
		if a.pri != b.pri {
			return a.pri > b.pri
		}
		if a.mcap != b.mcap {
			return a.mcap > b.mcap
		}
		if a.pop != b.pop {
			return a.pop > b.pop
		}
		return a.stock.Symbol < b.stock.Symbol
	})

	out := make([]model.Stock, len(scored))
	for i, s := range scored {
		out[i] = s.stock
	}
	return out
}

func paginateStocks(items []model.Stock, limit, offset int) []model.Stock {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return nil
	}
	items = items[offset:]
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func appendSearchQueryFilter(query *string, args *[]any, q string, latinOnly bool) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	esc := escapeLikePattern(q)
	like := "%" + esc + "%"

	/*
		후보 조건은 **자산군을 가리지 않는다.**

		예전에는 라틴 질의가 4글자 이상이면 `AND asset_type != 'domestic'`을 테이블
		전체에 걸었다. 그 규칙의 목적은 자동 번역된 `english_name` 때문에 생기는
		오탐(「Pharma」→ 국내 제약주 전부)을 막는 것이었는데, 국내 행을 통째로
		후보에서 빼 버려서 상장명이 영문인 종목(HPSP·NAVER·KT&G·SOOP…)과 종목코드
		(`005930`)까지 같이 사라졌다. 3글자 이하 분기가 따로 있었던 것도 같은 규칙을
		부분적으로 되돌린 땜질이었다.

		정밀도는 SQL이 아니라 Go 랭킹이 책임진다 — `searchRelevanceScore`가 단어 일치·
		접두만 인정하고 나머지를 0점으로 걸러 낸다. 여기서는 놓치는 행이 없도록 넓게
		뽑고, 후보 상한에 잘릴 때 강한 매치가 먼저 들어오도록 `appendSearchOrder`가
		순서를 잡는다.
	*/
	if latinOnly {
		*query += ` AND (
			UPPER(symbol) = UPPER(?)
			OR UPPER(symbol) LIKE UPPER(?)
			OR name LIKE ?
			OR english_name LIKE ?
		)`
		*args = append(*args, q, like, like, like)
		return
	}

	// 한글 질의. 별칭이 있으면 영문 표기도 같이 후보로 끌어온다 — 「네이버」로 NAVER를
	// 찾는 유일한 경로다(DB에 그 읽기가 없다).
	conds := []string{`symbol LIKE ?`, `name LIKE ?`, `english_name LIKE ?`}
	*args = append(*args, like, like, like)
	for _, alias := range SearchAliasTokens(q) {
		aliasLike := "%" + escapeLikePattern(alias) + "%"
		conds = append(conds, `name LIKE ?`, `english_name LIKE ?`)
		*args = append(*args, aliasLike, aliasLike)
	}
	/*
		심볼 별칭은 **이름이 아니라 종목을 가리킨다** (2026-09-13 사용자 요청 1·4).

		브랜드 별칭은 영문 표기를 주므로 LIKE로 이어지지만, 「구글」은 상장명
		「알파벳 A」와 글자가 하나도 겹치지 않는다. 실측: 별칭 표를 넣기 전
		「구글」 검색 결과는 ACE 구글밸류체인·TIGER 구글밸류체인 같은 **ETF뿐**이고
		GOOGL은 없었다 — 이름에 「구글」이 든 상품만 걸린 것이다.

		그래서 이 갈래는 심볼을 직접 후보에 넣는다. 점수는 `searchAliasSymbolScore`가
		준다(여기서 후보로만 넣고 점수를 안 주면 랭킹이 0점으로 걸러 낸다).
	*/
	if symbols := SearchAliasSymbols(q); len(symbols) > 0 {
		holes := make([]string, len(symbols))
		for i, sym := range symbols {
			holes[i] = "?"
			*args = append(*args, sym)
		}
		conds = append(conds, `symbol IN (`+strings.Join(holes, ", ")+`)`)
	}
	*query += ` AND (` + strings.Join(conds, " OR ") + `)`
}

/*
appendSearchOrder 는 후보 질의의 순서를 정한다.

표시 순서는 Go(`rankSearchResultsWithCaps`)가 정한다. 여기서 순서를 두는 이유는 두 가지다.

 1. **결정성**: 정렬 없는 `LIMIT`은 어떤 행이 들어올지 스토리지 엔진에 맡긴다. 검색이
    1위 종목을 카탈로그 보강 대상으로 올려 그 행을 갱신하기 때문에 테이블은 검색할
    때마다 조금씩 움직였고, 같은 검색어의 결과가 호출마다 달라졌다.
 2. **후보 상한(`searchCandidateCap`)에 잘릴 때 강한 매치를 먼저 넣기**: 짧은 질의는
    `LIKE '%q%'`에 수천 행이 걸릴 수 있는데, 심볼 일치·이름 접두를 앞으로 당겨 두면
    상한에 잘려도 정작 찾던 종목이 사라지지 않는다.
*/
func appendSearchOrder(query *string, args *[]any, q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		*query += ` ORDER BY symbol ASC`
		return
	}
	prefix := escapeLikePattern(q) + "%"
	strong := []string{`UPPER(symbol) LIKE UPPER(?)`, `name LIKE ?`, `english_name LIKE ?`}
	strongArgs := []any{prefix, prefix, prefix}
	// 별칭도 강한 매치로 친다 — 「에스케이」의 `sk`처럼 흔한 조각은 후보 상한에
	// 걸리기 쉬운데, 이름이 그 영문으로 시작하는 행이 먼저 들어와야 한다.
	for _, alias := range SearchAliasTokens(q) {
		aliasPrefix := escapeLikePattern(alias) + "%"
		strong = append(strong, `name LIKE ?`)
		strongArgs = append(strongArgs, aliasPrefix)
	}

	*query += `
		ORDER BY (CASE
			WHEN UPPER(symbol) = UPPER(?) THEN 0
			WHEN ` + strings.Join(strong, " OR ") + ` THEN 1
			ELSE 2
		END), symbol ASC`
	*args = append(*args, q)
	*args = append(*args, strongArgs...)
}

// escapeLikePattern neutralizes LIKE wildcards typed into search ("%", "_").
func escapeLikePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(q)
}

func normalizeSearchLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

// CountRankedSearch returns the number of stocks matching ranked search rules.
func CountRankedSearch(items []model.Stock, q string, latinOnly bool) int {
	q = strings.TrimSpace(q)
	if q == "" {
		return len(items)
	}
	n := 0
	for _, st := range items {
		if searchRelevanceScore(q, st, latinOnly) > 0 {
			n++
		}
	}
	return n
}

// ContainsHangul reports whether s has any Hangul syllable or jamo.
func ContainsHangul(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Hangul, r) {
			return true
		}
	}
	return false
}
