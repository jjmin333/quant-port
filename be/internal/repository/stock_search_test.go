package repository

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestSearchRelevanceScore_qqqRelatedTickers(t *testing.T) {
	qqq := model.Stock{Symbol: "QQQ", EnglishName: "Invesco QQQ Trust", AssetType: model.AssetETF}
	tqqq := model.Stock{Symbol: "TQQQ", EnglishName: "ProShares UltraPro QQQ", AssetType: model.AssetETF}
	qld := model.Stock{Symbol: "QLD", EnglishName: "ProShares Ultra QQQ", AssetType: model.AssetETF}
	qqqm := model.Stock{Symbol: "QQQM", EnglishName: "Invesco NASDAQ 100 ETF", AssetType: model.AssetETF}

	if got := searchRelevanceScore("qqq", qqq, true); got < 10000 {
		t.Fatalf("QQQ exact: got %d", got)
	}
	if got := searchRelevanceScore("qqq", qqqm, true); got < 5000 {
		t.Fatalf("QQQM prefix: got %d", got)
	}
	if got := searchRelevanceScore("qqq", tqqq, true); got < 1200 {
		t.Fatalf("TQQQ contains: got %d", got)
	}
	if got := searchRelevanceScore("qqq", qld, true); got < 3500 {
		t.Fatalf("QLD english word: got %d", got)
	}
}

func TestRankSearchResults_qqqOrdersExactBeforeContains(t *testing.T) {
	items := []model.Stock{
		{Symbol: "TQQQ", EnglishName: "ProShares UltraPro QQQ", AssetType: model.AssetETF},
		{Symbol: "QQQM", EnglishName: "Invesco NASDAQ 100 ETF", AssetType: model.AssetETF},
		{Symbol: "QQQ", EnglishName: "Invesco QQQ Trust", AssetType: model.AssetETF},
		{Symbol: "QLD", EnglishName: "ProShares Ultra QQQ", AssetType: model.AssetETF},
	}
	ranked := rankSearchResults(items, "qqq", true)
	if len(ranked) < 4 {
		t.Fatalf("expected 4 results, got %d", len(ranked))
	}
	if ranked[0].Symbol != "QQQ" {
		t.Fatalf("expected QQQ first, got %s", ranked[0].Symbol)
	}
}

func TestSearchRelevanceScore_armSkipsPharmaSubstring(t *testing.T) {
	pharma := model.Stock{
		Symbol:      "123456",
		Name:        "Some Pharma",
		EnglishName: "ABC Pharmaceutical Co Ltd",
		AssetType:   model.AssetDomestic,
	}
	if got := searchRelevanceScore("arm", pharma, true); got > 0 {
		t.Fatalf("expected 0 for pharma english substring, got %d", got)
	}

	arm := model.Stock{
		Symbol:      "ARM",
		Name:        "Arm Holdings",
		EnglishName: "Arm Holdings plc American Depositary Shares",
		AssetType:   model.AssetUS,
	}
	if got := searchRelevanceScore("arm", arm, true); got < 5000 {
		t.Fatalf("expected high score for ARM ticker, got %d", got)
	}
}

func TestSearchRelevanceScore_shortLatinMatchesDomesticEnglishPrefix(t *testing.T) {
	skHynix := model.Stock{
		Symbol:      "000660",
		Name:        "SK하이닉스",
		EnglishName: "SK hynix Inc.",
		AssetType:   model.AssetDomestic,
	}
	if got := searchRelevanceScore("sk", skHynix, true); got < 3000 {
		t.Fatalf("expected SK english prefix match for domestic, got %d", got)
	}
}

/*
국내 종목의 `english_name` 은 자동 번역 칸이라 **한 칸 아래**로만 인정한다.

로마자 검색(「samsung」)은 되어야 하지만, 상장명이 실제로 그 영문인 종목보다는
항상 뒤여야 한다.
*/
func TestSearchRelevanceScore_domesticEnglishNameRanksBelowDisplayName(t *testing.T) {
	domestic := model.Stock{
		Symbol: "005930", Name: "삼성전자", EnglishName: "Samsung Electronics Co Ltd",
		AssetType: model.AssetDomestic,
	}
	viaEnglish := searchRelevanceScore("samsung", domestic, true)
	if viaEnglish <= 0 {
		t.Fatal("romanized query should still find the domestic row")
	}

	us := model.Stock{Symbol: "SSNLF", Name: "Samsung", AssetType: model.AssetUS}
	if viaDisplay := searchRelevanceScore("samsung", us, true); viaDisplay <= viaEnglish {
		t.Fatalf("display-name match must outrank domestic english_name: %d vs %d", viaDisplay, viaEnglish)
	}
}

/*
상장명이 영문인 국내 종목은 **이름으로도** 검색돼야 한다.

회귀 대상: 라틴 질의가 4글자 이상이면 SQL이 국내 행을 통째로 뺐고, 점수 쪽도 3글자
이하일 때만 `english_name`을 봤다. 그래서 「HPSP」는 코드 403870으로는 나오는데
이름으로는 0건이었다.
*/
func TestSearchRelevanceScore_latinMatchesDomesticDisplayName(t *testing.T) {
	cases := []struct {
		q    string
		name string
	}{
		{"HPSP", "HPSP"},
		{"hpsp", "HPSP"},
		{"naver", "NAVER"},
		{"kt&g", "KT&G"},
		{"ktg", "KT&G"},
		{"lig", "LIG넥스원"},
		{"soop", "SOOP"},
	}
	for _, c := range cases {
		st := model.Stock{Symbol: "403870", Name: c.name, AssetType: model.AssetDomestic}
		if got := searchRelevanceScore(c.q, st, true); got < 1800 {
			t.Fatalf("%q vs %q: expected a name match, got %d", c.q, c.name, got)
		}
	}
}

// 국내 ETF 는 `english_name` 이 비어 있는 행이 많다 — 이름으로 찾을 수 있어야 한다.
func TestSearchRelevanceScore_latinMatchesEtfDisplayNameWithoutEnglishName(t *testing.T) {
	kodex := model.Stock{Symbol: "069500", Name: "KODEX 200", SecurityType: "ETF", AssetType: model.AssetETF}
	if got := searchRelevanceScore("kodex", kodex, true); got < 3500 {
		t.Fatalf("expected KODEX word match, got %d", got)
	}
}

// `english_name` 이 아직 없는 해외 행은 `name` 에 영문이 들어 있다.
func TestSearchRelevanceScore_latinMatchesUsDisplayName(t *testing.T) {
	pltr := model.Stock{Symbol: "PLTR", Name: "Palantir Technologies", AssetType: model.AssetUS}
	if got := searchRelevanceScore("palantir", pltr, true); got < 3500 {
		t.Fatalf("expected display-name match, got %d", got)
	}
}

/*
숫자로 시작하는 해외 종목명도 이름으로 찾아야 한다.

회귀 대상: 「3M」·「180」은 `LooksLikeDomesticCode` 가 참이라, 심볼 접두에 걸리는 게
없으면 그대로 0점으로 끝났다 (이름을 아예 보지 않았다).
*/
func TestSearchRelevanceScore_numericLatinQueryStillMatchesName(t *testing.T) {
	mmm := model.Stock{Symbol: "MMM", Name: "3M", EnglishName: "3M Company", AssetType: model.AssetUS}
	if got := searchRelevanceScore("3M", mmm, true); got < 3500 {
		t.Fatalf("expected 3M name match, got %d", got)
	}
}

// 자모만 남은 질의(IME 조합 중)는 라틴 질의가 아니다.
func TestIsLatinOnlyQuery_jamo(t *testing.T) {
	if IsLatinOnlyQuery("ㅅ") || IsLatinOnlyQuery("ㄱㅇ") {
		t.Fatal("jamo query must not be treated as latin")
	}
}

// 영문 표지는 단어 단위로만 본다 — 이름에 우연히 든 「ultra」·「etn」에 걸리면 안 된다.
func TestSearchInstrumentPriority_latinMarkersAreWordScoped(t *testing.T) {
	for _, common := range []model.Stock{
		{Symbol: "RARE", Name: "울트라제닉스", EnglishName: "Ultragenyx Pharmaceutical Inc",
			SecurityType: "STOCK", AssetType: model.AssetUS},
		{Symbol: "AET", Name: "에트나", EnglishName: "Aetna Inc", SecurityType: "STOCK", AssetType: model.AssetUS},
	} {
		if got := SearchInstrumentPriority(common); got != 100 {
			t.Fatalf("%s should stay 본주 priority, got %d", common.Symbol, got)
		}
	}
	lev := model.Stock{Symbol: "TQQQ", Name: "ProShares UltraPro QQQ", SecurityType: "ETF", AssetType: model.AssetETF}
	if got := SearchInstrumentPriority(lev); got != 10 {
		t.Fatalf("leveraged ETF should rank last, got %d", got)
	}
}

func TestSearchRelevanceScore_koreanNameMatch(t *testing.T) {
	oracle := model.Stock{
		Symbol:      "ORCL",
		Name:        "오라클",
		EnglishName: "Oracle Corporation Common Stock",
		AssetType:   model.AssetUS,
	}
	if got := searchRelevanceScore("오라클", oracle, false); got < 4000 {
		t.Fatalf("expected korean name match, got %d", got)
	}
}

func TestRankSearchResults_skHynixCommonStockBeforeDerivatives(t *testing.T) {
	items := []model.Stock{
		{Symbol: "0192L0", Name: "RISE SK하이닉스단일종목레버리지", AssetType: model.AssetETF, SecurityType: "ETF"},
		{Symbol: "000660", Name: "SK하이닉스", EnglishName: "SK hynix Inc.", AssetType: model.AssetDomestic, SecurityType: "STOCK"},
		{Symbol: "SKHY", Name: "SK하이닉스(ADR)", AssetType: model.AssetUS, SecurityType: "STOCK"},
		{Symbol: "520101", Name: "미래에셋 레버리지 SK하이닉스 단일종목ETN", AssetType: model.AssetETF, SecurityType: "ETN"},
	}
	ranked := rankSearchResults(items, "sk하이닉스", false)
	if len(ranked) < 4 {
		t.Fatalf("expected 4 results, got %d", len(ranked))
	}
	if ranked[0].Symbol != "000660" {
		t.Fatalf("expected 000660 first, got %s", ranked[0].Symbol)
	}
}

func TestRankSearchResults_popularityTiebreak(t *testing.T) {
	large := int64(10_000_000_000)
	small := int64(100_000_000)
	items := []model.Stock{
		{Symbol: "AAA", EnglishName: "Alpha Corporation", AssetType: model.AssetUS, SharesOutstanding: &small},
		{Symbol: "AAB", EnglishName: "Alpha Beta Corporation", AssetType: model.AssetUS, SharesOutstanding: &large},
	}
	ranked := rankSearchResults(items, "alpha", true)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 results, got %d", len(ranked))
	}
	if ranked[0].Symbol != "AAB" {
		t.Fatalf("expected larger-cap AAB first, got %s", ranked[0].Symbol)
	}
}

func TestIsLatinOnlyQuery(t *testing.T) {
	if !IsLatinOnlyQuery("oracle") {
		t.Fatal("oracle should be latin-only")
	}
	if IsLatinOnlyQuery("오라클") {
		t.Fatal("hangul query should not be latin-only")
	}
}

/*
「삼성」 검색은 삼성전자가 1위여야 한다.

회귀 대상: 예전에는 `english_name`에 한글이 그대로 남아 있는 ETN 행만 +1,200점을
더 받아 본주를 넘어섰다. 실측으로 상위 20개가 전부 ETN이었고 삼성전자는 21번째였다.
*/
func TestRankSearchResults_koreanQueryPutsCommonStockFirst(t *testing.T) {
	shares := func(v int64) *int64 { return &v }
	items := []model.Stock{
		// english_name 이 한글인 행 — 여기가 예전의 가산점 출처다.
		{Symbol: "530127", Name: "삼성 미국다우존스 TR ETN", EnglishName: "삼성 미국다우존스 TR ETN",
			SecurityType: "ETN", AssetType: model.AssetDomestic, SharesOutstanding: shares(1_000_000)},
		{Symbol: "530128", Name: "삼성 레버리지 미국다우존스 선물 ETN", EnglishName: "삼성 레버리지 미국다우존스 선물 ETN",
			SecurityType: "ETN", AssetType: model.AssetDomestic, SharesOutstanding: shares(1_000_000)},
		{Symbol: "005930", Name: "삼성전자", EnglishName: "SamsungElec",
			SecurityType: "STOCK", AssetType: model.AssetDomestic, SharesOutstanding: shares(5_846_278_608)},
		{Symbol: "000810", Name: "삼성화재", EnglishName: "SamsungFire",
			SecurityType: "주권", AssetType: model.AssetDomestic, SharesOutstanding: shares(44_647_473)},
	}
	caps := map[string]float64{
		"005930": 1.49e15,
		"000810": 2.0e13,
		"530127": 1.0e9,
		"530128": 1.0e9,
	}
	ranked := rankSearchResultsWithCaps(items, "삼성", false, caps)
	if len(ranked) != 4 {
		t.Fatalf("expected 4 results, got %d", len(ranked))
	}
	if ranked[0].Symbol != "005930" {
		t.Fatalf("expected 005930 first, got %s", ranked[0].Symbol)
	}
	if ranked[1].Symbol != "000810" {
		t.Fatalf("expected 000810 (본주, 작은 시총) second, got %s", ranked[1].Symbol)
	}
	// 파생상품은 걷어내지 않는다 — 뒤로 갈 뿐이다.
	if ranked[2].Symbol != "530127" || ranked[3].Symbol != "530128" {
		t.Fatalf("ETN should remain, ordered by symbol: %s %s", ranked[2].Symbol, ranked[3].Symbol)
	}
}

// 같은 입력이면 같은 순서. 예전에는 상장주식수가 같은 ETN들의 순서가 정해지지 않았다.
func TestRankSearchResults_isDeterministicForTiedRows(t *testing.T) {
	shares := func(v int64) *int64 { return &v }
	mk := func(sym string) model.Stock {
		return model.Stock{Symbol: sym, Name: "삼성 " + sym + " ETN", EnglishName: "Samsung " + sym,
			SecurityType: "ETN", AssetType: model.AssetDomestic, SharesOutstanding: shares(1_000_000)}
	}
	first := ""
	for i := range 8 {
		items := []model.Stock{mk("530130"), mk("530121"), mk("530145"), mk("530102")}
		ranked := rankSearchResultsWithCaps(items, "삼성", false, nil)
		if i == 0 {
			first = ranked[0].Symbol
			continue
		}
		if ranked[0].Symbol != first {
			t.Fatalf("run %d changed head: %s != %s", i, ranked[0].Symbol, first)
		}
	}
	if first != "530102" {
		t.Fatalf("tie-break should fall to the lowest symbol, got %s", first)
	}
}

// 시가총액은 같은 우선순위 안에서만 작동해야 한다 — 큰 ETF가 작은 본주를 이기면 안 된다.
func TestRankSearchResults_marketCapDoesNotBeatCommonStock(t *testing.T) {
	items := []model.Stock{
		{Symbol: "069500", Name: "KODEX 코리아200", EnglishName: "KODEX Korea 200", SecurityType: "ETF", AssetType: model.AssetETF},
		{Symbol: "000210", Name: "코리아소형주", EnglishName: "Korea Small", SecurityType: "주권", AssetType: model.AssetDomestic},
	}
	caps := map[string]float64{"069500": 9.0e14, "000210": 1.0e11}
	ranked := rankSearchResultsWithCaps(items, "코리아", false, caps)
	if len(ranked) != 2 {
		t.Fatalf("expected both to match, got %d", len(ranked))
	}
	if ranked[0].Symbol != "000210" {
		t.Fatalf("common stock must outrank a much larger ETF, got %s", ranked[0].Symbol)
	}
}

func TestLooksLikeDomesticCode(t *testing.T) {
	for _, ok := range []string{"005930", "000660", "00593", "0194M0", "0044K0"} {
		if !LooksLikeDomesticCode(ok) {
			t.Fatalf("%q should look like a domestic code", ok)
		}
	}
	for _, bad := range []string{"", "A", "NVDA", "AAPL", "삼성", "0059301", "TSLA1"} {
		if LooksLikeDomesticCode(bad) {
			t.Fatalf("%q should not look like a domestic code", bad)
		}
	}
}

// 종목코드 검색은 자산군 필터에 막히지 않아야 한다 (실측: `005930` → 0건이었다).
func TestSearchRelevanceScore_domesticCodeMatches(t *testing.T) {
	samsung := model.Stock{Symbol: "005930", Name: "삼성전자", EnglishName: "SamsungElec", AssetType: model.AssetDomestic}
	if got := searchRelevanceScore("005930", samsung, true); got < 50000 {
		t.Fatalf("exact code should score high, got %d", got)
	}
	if got := searchRelevanceScore("00593", samsung, true); got < 10000 {
		t.Fatalf("code prefix should match, got %d", got)
	}

	var q, args = "", []any(nil)
	appendSearchQueryFilter(&q, &args, "005930", true)
	// 국내 제외는 `english_name` 절 안에서만 허용된다 — 테이블 전체에 걸리면 안 된다.
	if strings.Contains(q, "AND asset_type != 'domestic'") {
		t.Fatalf("latin query must not exclude domestic rows outright: %s", q)
	}
	if !strings.Contains(q, "UPPER(symbol) = UPPER(?)") || !strings.Contains(q, "name LIKE ?") {
		t.Fatalf("latin filter must match symbol and display name: %s", q)
	}
	if len(args) != 4 {
		t.Fatalf("expected symbol/name/english_name args, got %v", args)
	}
}

/*
4글자 이상 라틴 질의도 국내 행을 후보에서 빼면 안 된다 (「HPSP」 회귀).

이름 매치의 정밀도는 Go 랭킹이 맡고, SQL 은 후보만 넓게 뽑는다.
*/
func TestAppendSearchQueryFilter_longLatinKeepsDomestic(t *testing.T) {
	var q, args = "", []any(nil)
	appendSearchQueryFilter(&q, &args, "HPSP", true)
	if strings.Contains(q, "AND asset_type != 'domestic'") {
		t.Fatalf("long latin query must not exclude domestic rows: %s", q)
	}
	if !strings.Contains(q, "name LIKE ?") {
		t.Fatalf("long latin query must search the display name: %s", q)
	}
	if len(args) != 4 {
		t.Fatalf("unexpected args: %v", args)
	}
}
