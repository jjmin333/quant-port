package repository

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

type seedRow struct{ alias, kind, target string }

func goSeedRows() []seedRow {
	var out []seedRow
	for alias, tokens := range seedBrandAliases {
		for _, tok := range tokens {
			out = append(out, seedRow{alias, "brand", tok})
		}
	}
	for alias, sym := range seedSymbolAliases {
		out = append(out, seedRow{alias, "symbol", sym})
	}
	sortSeedRows(out)
	return out
}

func sortSeedRows(rows []seedRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.alias != b.alias {
			return a.alias < b.alias
		}
		return a.target < b.target
	})
}

var migrationRowRe = regexp.MustCompile(`\('([^']*)', '([^']*)', '([^']*)', '([^']*)'\)`)

func migrationSeedRows(t *testing.T) []seedRow {
	t.Helper()
	path := filepath.Join("..", "..", "migrations", "070_stock_aliases.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("마이그레이션을 못 읽는다: %v", err)
	}
	var out []seedRow
	for _, m := range migrationRowRe.FindAllStringSubmatch(string(body), -1) {
		if m[4] != "seed" {
			continue
		}
		out = append(out, seedRow{m[1], m[2], m[3]})
	}
	sortSeedRows(out)
	return out
}

/*
씨앗(Go)과 마이그레이션(SQL)이 **같은 내용**이어야 한다 (2026-09-13 사용자 요청 1·4).

같은 자료가 두 파일에 적혀 있는 구조라, 한쪽만 고치면 「새로 붓는 DB와 이미 도는
DB가 다른 별칭을 쓴다」가 된다. 그런 어긋남은 검색 결과가 환경마다 다른 모양으로만
드러나서 알아채기 어렵다. SQL 행은 Go에서 **생성**된 것이고, 이 시험이 그 생성이
최신인지를 센다.
*/
func TestSeedMatchesMigration(t *testing.T) {
	got := migrationSeedRows(t)
	want := goSeedRows()

	if len(got) != len(want) {
		t.Fatalf("행 수가 다르다: 마이그레이션 %d · Go 씨앗 %d — 생성기를 다시 돌려라", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%d번째 행이 다르다: 마이그레이션 %+v · Go 씨앗 %+v", i, got[i], want[i])
		}
	}
}

/*
별칭이 표를 타고 들어와도 **검색이 그것을 쓴다.**

스냅샷을 갈아 끼워도 `SearchAliasTokens`가 예전 map을 보고 있으면 표로 옮긴 뜻이
없다 — 배포 없이 고치려고 옮긴 것이기 때문이다.
*/
func TestSearchAliasTokens_usesSnapshot(t *testing.T) {
	restore := aliasSnap.Load()
	t.Cleanup(func() { aliasSnap.Store(restore) })

	if got := SearchAliasTokens("네이버"); len(got) == 0 || got[0] != "naver" {
		t.Fatalf("씨앗이 안 먹는다: %v", got)
	}
	if got := SearchAliasTokens("테스트브랜드"); len(got) != 0 {
		t.Fatalf("없는 별칭이 잡혔다: %v", got)
	}

	snap := seedSnapshot()
	snap.brand["테스트브랜드"] = []string{"testbrand"}
	aliasSnap.Store(snap)

	if got := SearchAliasTokens("테스트브랜드"); len(got) != 1 || got[0] != "testbrand" {
		t.Fatalf("표에서 온 별칭이 검색에 안 쓰인다: %v", got)
	}
	// 접두 입력도 같은 스냅샷을 본다 — 검색창이 글자마다 질의를 보낸다.
	if got := SearchAliasTokens("테스트"); len(got) != 1 || got[0] != "testbrand" {
		t.Fatalf("접두 입력이 표를 못 본다: %v", got)
	}
}

/*
사용자가 지목한 바로 그 건 — 「알파벳 A」·「알파벳 C」 (2026-09-13 사용자 요청 1).

토스가 준 상장명은 「알파벳 A」이고 흔히 쓰는 말은 「구글」이다. 둘 다 GOOGL로
가야 하고, 종류 꼬리를 뗀 「알파벳」도 **정해진 한쪽**으로 가야 한다 — 기계로
떼면 A주와 C주가 같은 열쇠를 다퉈 매번 다른 답이 나온다.
*/
func TestSymbolAliases_alphabetAndGoogle(t *testing.T) {
	aliases := SymbolNameAliases()
	for alias, want := range map[string]string{
		"구글":   "GOOGL",
		"알파벳":  "GOOGL",
		"구글씨":  "GOOG",
		"페이스북": "META",
	} {
		got := aliases[alias]
		if len(got) != 1 || got[0] != want {
			t.Fatalf("%q → %v, want [%s]", alias, got, want)
		}
	}
}

/*
일상어와 부딪히는 별칭은 들어 있으면 안 된다.

오태깅은 사용자가 본다 — 재현율 한 종목보다 비싸다. 여기 적힌 것들은 실제로
한 번씩 후보에 올랐다가 뺀 값이라, 다음 사람이 다시 넣지 않도록 시험으로 못박는다.
*/
func TestSymbolAliases_excludesEverydayWords(t *testing.T) {
	banned := []string{"비자", "델", "티", "릴리", "블록", "타깃"}
	aliases := SymbolNameAliases()
	for _, word := range banned {
		if syms, ok := aliases[word]; ok {
			t.Fatalf("%q 가 %v 별칭으로 들어 있다 — 일상어와 부딪힌다 (seed 머리글 참조)", word, syms)
		}
	}
}

/*
별칭은 **소문자·공백 없는** 꼴로만 적는다.

매칭하는 쪽이 본문을 소문자로 내려 비교하므로(`nameTokenMatch`), 대문자가 섞인
별칭은 영원히 안 걸린다 — 그런 행은 있어도 아무 일도 안 하고, 없는 것과 구분되지
않아 「왜 안 되지」로 남는다.
*/
func TestSeedAliases_areNormalized(t *testing.T) {
	check := func(kind, alias string) {
		t.Helper()
		if alias != strings.ToLower(alias) {
			t.Fatalf("%s 별칭 %q 에 대문자가 있다", kind, alias)
		}
		if strings.TrimSpace(alias) != alias || strings.Contains(alias, " ") {
			t.Fatalf("%s 별칭 %q 에 공백이 있다", kind, alias)
		}
	}
	for alias := range seedBrandAliases {
		check("brand", alias)
	}
	for alias := range seedSymbolAliases {
		check("symbol", alias)
	}
}

/*
「구글」로 GOOGL이 나와야 한다 (2026-09-13 사용자 요청 1).

표를 넣기 전 실측: 「구글」 검색 결과는 ACE 구글밸류체인·TIGER 구글밸류체인 같은
**ETF뿐**이었다 — 이름에 「구글」이 든 상품만 LIKE에 걸렸고, 정작 상장명이
「알파벳 A」인 GOOGL은 글자가 하나도 겹치지 않아 후보에도 못 들어갔다.
*/
func TestSearchAliasSymbols_googleReachesAlphabet(t *testing.T) {
	if got := SearchAliasSymbols("구글"); len(got) != 1 || got[0] != "GOOGL" {
		t.Fatalf("구글 → %v, want [GOOGL]", got)
	}
	// 접두 입력도 닿는다 — 검색창은 글자마다 질의를 보낸다.
	if got := SearchAliasSymbols("알파"); len(got) == 0 || got[0] != "GOOGL" {
		t.Fatalf("알파 → %v, want GOOGL 먼저", got)
	}
	// 라틴 질의는 이 경로를 타지 않는다(티커·이름이 이미 그 일을 한다).
	if got := SearchAliasSymbols("GOOGL"); len(got) != 0 {
		t.Fatalf("라틴 질의가 별칭 경로를 탔다: %v", got)
	}
}

/*
심볼 별칭으로 지목된 종목이 **이름에 그 말이 든 상품보다 위**여야 한다.

「구글」에서 GOOGL이 ACE 구글밸류체인 아래로 내려가면 표를 넣은 뜻이 없다.
*/
func TestSearchAliasSymbolScore_outranksNameContainingProduct(t *testing.T) {
	googl := model.Stock{Symbol: "GOOGL", Name: "알파벳 A", EnglishName: "Alphabet A"}
	etf := model.Stock{Symbol: "483340", Name: "ACE 구글밸류체인액티브", AssetType: model.AssetETF}

	got := searchRelevanceScore("구글", googl, false)
	want := searchRelevanceScore("구글", etf, false)
	if got <= want {
		t.Fatalf("GOOGL %d점 ≤ 구글밸류체인 ETF %d점 — 별칭이 이름 일치에 밀린다", got, want)
	}
	// 다른 종목에는 점수가 붙지 않는다.
	if s := searchAliasSymbolScore("구글", model.Stock{Symbol: "AAPL", Name: "애플"}); s != 0 {
		t.Fatalf("엉뚱한 종목에 별칭 점수 %d", s)
	}
}

/*
같은 별칭에 대상을 **여럿** 달 수 있다 (2026-09-14 사용자 요청 1).

사용자가 적은 증상 그대로다: 「구글」에 GOOGL과 GOOG을 만들어 뒀는데 검색에는
GOOG만 나왔다. 원인은 표가 아니라 스냅샷이었다 — 값이 문자열 하나라 나중에 읽힌
행이 앞 행을 덮었고, 어느 쪽이 살아남을지는 읽는 순서가 정했다.

검색은 **둘 다** 봐야 하고, 뉴스 태깅은 **하나만** 봐야 한다. 그 갈림이 이 시험의
전부다 — 둘을 한 함수로 합치면 검색이 좁아지거나 태깅이 두 벌로 붙는다.
*/
func TestSymbolAliases_multipleTargetsPerAlias(t *testing.T) {
	restore := aliasSnap.Load()
	t.Cleanup(func() { aliasSnap.Store(restore) })

	snap := seedSnapshot()
	// 표에서 읽힌 모양 — id 오름차순이라 먼저 적어 둔 GOOGL이 앞이다.
	snap.symbol["구글"] = []string{"GOOGL", "GOOG"}
	aliasSnap.Store(snap)

	got := SearchAliasSymbols("구글")
	if len(got) != 2 || got[0] != "GOOGL" || got[1] != "GOOG" {
		t.Fatalf("구글 → %v, want [GOOGL GOOG] — 둘 다 검색돼야 한다", got)
	}

	// 둘 다 **정확 일치**다. 뒤에 적힌 쪽이 접두 매치로 밀리면 목록에서 네 칸
	// 아래로 내려간다.
	for _, sym := range []string{"GOOGL", "GOOG"} {
		if s := searchAliasSymbolScore("구글", model.Stock{Symbol: sym}); s != 9000 {
			t.Fatalf("%s 정확 일치 점수 %d, want 9000", sym, s)
		}
	}

	// 태깅은 대표 하나만 쓴다 — 기사 한 줄에 같은 회사가 두 번 서면 안 된다.
	if sym := PrimarySymbolNameAliases()["구글"]; sym != "GOOGL" {
		t.Fatalf("대표 심볼 %q, want GOOGL (표에 먼저 적힌 것)", sym)
	}
}

/*
끈 행 **하나**가 별칭 전체를 지우면 안 된다 (2026-09-14).

대상이 하나뿐이던 시절에는 「끄기 = 별칭 삭제」가 같은 뜻이었다. 여럿이 되면
다르다 — GOOG만 끄고 싶었는데 GOOGL까지 사라지면, 끄는 사람은 자기가 무엇을
껐는지 모른 채 검색이 통째로 죽는 것을 본다.
*/
func TestReloadStockAliases_disablingOneTargetKeepsTheOthers(t *testing.T) {
	restore := aliasSnap.Load()
	t.Cleanup(func() { aliasSnap.Store(restore) })

	// `ReloadStockAliases`가 쓰는 바로 그 접기다 — 규칙 사본이 아니라 같은 코드다.
	fold := newAliasFold()
	// 끈 행이 **뒤**에 온다 — 앞의 켜진 대상이 살아남아야 한다.
	fold.add("구글", "symbol", "GOOGL", true)
	fold.add("구글", "symbol", "GOOG", false)
	// 반대 순서도 같아야 한다 — 끈 행이 앞에 온다고 뒤의 켜진 것이 죽으면 안 된다.
	fold.add("에스케이", "brand", "skhynix", false)
	fold.add("에스케이", "brand", "sk", true)
	// 모든 행이 꺼진 별칭은 씨앗에서도 사라진다 — 스위치가 스위치이려면 그래야 한다.
	fold.add("페이스북", "symbol", "META", false)
	aliasSnap.Store(fold.snap)

	if got := SearchAliasSymbols("구글"); len(got) != 1 || got[0] != "GOOGL" {
		t.Fatalf("구글 → %v, want [GOOGL] — 켜 둔 대상까지 사라졌다", got)
	}
	if got := SearchAliasTokens("에스케이"); len(got) != 1 || got[0] != "sk" {
		t.Fatalf("에스케이 → %v, want [sk] — 켜 둔 대상까지 사라졌다", got)
	}
	if got := SearchAliasSymbols("페이스북"); len(got) != 0 {
		t.Fatalf("페이스북 → %v, want [] — 전부 껐는데 씨앗이 살아 있다", got)
	}
	if fold.disabled != 3 {
		t.Fatalf("끈 행 %d개로 셌다, want 3", fold.disabled)
	}
}

/*
표의 값이 씨앗을 **이긴다** — 대상이 여럿이어도.

배포 없이 고치려고 표로 옮긴 것이므로, 운영에서 적은 것이 소스에 적힌 것을 덮어야
한다. 목록이 되면서 「덮는다」의 뜻이 「첫 행에서 비우고 그때부터 쌓는다」로
바뀌었는데, 그 첫 행을 안 비우면 씨앗 값이 표의 값과 나란히 남는다.
*/
func TestAliasFold_tableReplacesSeedList(t *testing.T) {
	restore := aliasSnap.Load()
	t.Cleanup(func() { aliasSnap.Store(restore) })

	fold := newAliasFold()
	// 씨앗의 「구글 → GOOGL」을 표가 GOOG 하나로 바꾼다.
	fold.add("구글", "symbol", "goog", true)
	aliasSnap.Store(fold.snap)

	if got := SearchAliasSymbols("구글"); len(got) != 1 || got[0] != "GOOG" {
		t.Fatalf("구글 → %v, want [GOOG] — 씨앗이 표 옆에 남았다", got)
	}
}
