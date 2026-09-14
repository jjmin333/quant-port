package service

import "testing"

/*
국내 종목에는 ETF 추측을 돌리지 않는다.

돌리면 005930에서 005Q·005U·005G·005X·005D·005L·00XX가 나오는데 **전부 존재할 수
없는 코드**다. 그게 헛수고로 끝나지 않았던 이유: 호출자
(`ensureRelatedCatalogFromToss`)가 「`stocks`에 없는 후보」를 Toss에서 받아 채우는데,
허구 코드는 Toss에도 없어 저장되지 않는다 → `missing`이 줄지 않는다 →
**국내 종목을 검색할 때마다 Toss를 부르고 영영 수렴하지 않았다.**

`/stocks/search`는 비로그인도 부를 수 있는 공개 경로이고, 그 호출은 `STOCK`
한도를 `tickFast`의 `prefetchStockInfo`와 나눠 쓴다 — 검색 트래픽이 시세 갱신을
밀어낼 수 있었다.
*/
func TestRelatedSymbolCandidates_skipsDomesticNumericSymbols(t *testing.T) {
	for _, sym := range []string{"005930", "000660", "035420"} {
		if got := relatedSymbolCandidates(sym); len(got) != 0 {
			t.Fatalf("%s → %v — 국내 종목에는 추측 후보를 만들면 안 된다", sym, got)
		}
	}
}

// 미국 티커에는 그대로 동작해야 한다 — 이 기능의 원래 대상이다.
func TestRelatedSymbolCandidates_keepsUSTickers(t *testing.T) {
	got := relatedSymbolCandidates("TSLA")
	if len(got) == 0 {
		t.Fatal("TSLA → 후보 없음 — 미국 티커에는 추측이 살아 있어야 한다")
	}
	want := map[string]bool{"TSLQ": true, "TSLU": true, "TSLL": true}
	var hit int
	for _, c := range got {
		if want[c] {
			hit++
		}
	}
	if hit == 0 {
		t.Fatalf("TSLA → %v — 레버리지 ETF 접미사 후보가 하나도 없다", got)
	}
}

/*
수동 매핑은 마켓과 무관하게 살아 있어야 한다.

추측을 막는 것이 목적이지 「관련 상품」 기능 자체를 죽이는 것이 아니다.
국내 종목에 실제 관련 상품을 나중에 등록하면 추측 없이 그것만 쓰인다.
*/
func TestRelatedSymbolCandidates_manualOverrideSurvivesForNonLatin(t *testing.T) {
	const sym = "005930"
	knownRelatedETFOverrides[sym] = []string{"069500"}
	defer delete(knownRelatedETFOverrides, sym)

	got := relatedSymbolCandidates(sym)
	if len(got) != 1 || got[0] != "069500" {
		t.Fatalf("%s → %v — 수동 매핑은 그대로 나와야 한다", sym, got)
	}
}

// 반환값을 호출자가 고쳐도 매핑 원본이 오염되면 안 된다.
func TestRelatedSymbolCandidates_manualOverrideNotAliased(t *testing.T) {
	const sym = "000660"
	knownRelatedETFOverrides[sym] = []string{"069500"}
	defer delete(knownRelatedETFOverrides, sym)

	got := relatedSymbolCandidates(sym)
	got[0] = "MUTATED"

	if knownRelatedETFOverrides[sym][0] != "069500" {
		t.Fatal("반환 슬라이스가 매핑 원본을 가리킨다 — 호출자가 지도를 덮어쓴다")
	}
}
