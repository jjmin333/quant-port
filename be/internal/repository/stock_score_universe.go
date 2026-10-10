package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
AI 추천 랭킹에 **올라갈 수 있는 종목**의 정의. 두 조회 경로가 같은 잣대를 쓰도록
한 곳에 모아 둔다.

랭킹은 두 갈래로 나간다 — 「종합·실시간」은 `StockScoreRepo.TopScores`가 그 자리에서
계산하고, 나머지 47개 조합은 배치가 만들어 둔 것을 읽는다(`StockScoreRankingRepo`).
조건을 각자 적으면 **같은 화면의 탭을 옮겼을 뿐인데 종목 구성 규칙이 달라진다.**
실제로 그런 일이 이미 한 번 있었다: 「최근 시세 없는 종목 제외」가 TopScores에만
있고 사전계산 쪽에는 빠져서, 가격 칸이 빈 채로 랭킹에 오른 종목이 생겼다
(`StockScoreRankingRepo.DisplayRows`의 LATERAL 주석 참조).

# 규칙 둘

① **ETF·ETN·ETP 제외** (2026-09-04 사용자 요청). 「AI 추천 종목」 목록에 지수
추종 상품이 섞이면 추천의 뜻이 흐려진다.

**`asset_type`만으로는 못 거른다.** 실측(2026-09-04) 분포:

	asset_type  security_type  건수
	etf         ETP/ETF        4,308   ← asset_type으로 걸린다
	us          ETP            2,418   ← 안 걸린다 (미국 상장 ETF)
	domestic    ETF            1,156   ← 안 걸린다 (국내 ETF)
	domestic    ETN              376   ← 안 걸린다 (국내 ETN)

그래서 `security_type`을 함께 본다. 이 표는 상장 정보에서 오는 값이라 종목 이름
문자열을 뒤지는 것보다 안전하다.

② **점수 축 최소 3개** (같은 요청). 이유가 실측에 그대로 있다 — 09-03 해외
「수익성·1개월」 목록 상위에 `ADSEW`(애즈 텍 에너지 콜 워런트)·`IRAB`(SPAC)
같은 종목이 앉아 있었고, 그날 breakdown은 각각 `{technical}` 하나, `{quality,
technical}` 둘뿐이었다. 축 하나짜리 점수는 「이 종목이 좋다」가 아니라 「이
종목에 대해 아는 게 그것뿐이다」라는 뜻이다.

**최신 점수일 기준으로 센다**(사용자 선택). 기간 창 전체로 세면 창 안 어느
하루에라도 값이 있으면 통과하는데, 실측으로 ADSEW는 1개월 창에서 4축이라 그대로
남는다 — 걸러 달라고 지목된 바로 그 종목이 안 걸린다.

실측 생존 종목 수(09-03): 국내 2,722 · 해외 4,569. 목록은 최대 100줄이라 넉넉하다.
`asset_type='etf'`는 **0개**가 살아남는다 — ETF에는 애초에 `technical` 하나만 붙는다.
*/

// scoreRankingMinAxes — 랭킹에 오르려면 이만큼의 축에 점수가 있어야 한다.
const scoreRankingMinAxes = 3

/*
scoreRankingExcludedSecurityTypes — 지수 추종 상품의 `stocks.security_type` 값.

대소문자를 섞어 쓰는 공급자가 있어 비교는 SQL에서 `UPPER()`로 맞춘다.
*/
var scoreRankingExcludedSecurityTypes = []string{"ETF", "ETN", "ETP"}

// scoreRankingExcludedAssetType — asset_type 축의 ETF 묶음.
const scoreRankingExcludedAssetType = string(model.AssetETF)

/*
scoreRankableProductClause — 「지수 추종 상품이 아니다」를 SQL로 옮긴 것.

`s`는 `stocks`의 별칭이다. 인자는 반환된 placeholder 수만큼 뒤에 붙여야 한다.
*/
func scoreRankableProductClause(alias string) (string, []any) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(scoreRankingExcludedSecurityTypes)), ",")
	/*
		**국내 우선주도 뺀다** (2026-09-24 사용자 결정). 우선주는 보통주의 EPS·자본으로
		가치·수익성 지표를 받아 가격만 싸 보인다 — 기본 랭킹 1위가 대덕전자1우였고 국내 상위
		50에 4개가 섰다. 점수는 그대로 두고 비교 대상(랭킹·스크리너·백테스트·성적표·변화)에서만
		뺀다. 판별은 종목코드 끝자리다: 실측 끝자리가 0이 아닌 국내 종목 114개가 **전부**
		이름이 우선주였고, 이름에 「우」가 든 보통주 3개는 끝자리가 0이었다.
	*/
	clause := fmt.Sprintf(
		" AND %s.asset_type <> ? AND UPPER(TRIM(%s.security_type)) NOT IN (%s)"+
			" AND NOT (%s.asset_type = 'domestic' AND RIGHT(%s.symbol, 1) <> '0')",
		alias, alias, placeholders, alias, alias,
	)
	args := make([]any, 0, len(scoreRankingExcludedSecurityTypes)+1)
	args = append(args, scoreRankingExcludedAssetType)
	for _, t := range scoreRankingExcludedSecurityTypes {
		args = append(args, t)
	}
	return clause, args
}

/*
scoreAxisCountExpr — breakdown에 값이 있는 축의 개수를 세는 SQL 식.

`model.ScoreAxes`에서 만든다 — 축이 늘거나 이름이 바뀌면 이 식도 같이 따라간다.
JSON 경로가 없으면 `JSON_EXTRACT`가 NULL이라 `IS NOT NULL`이 0/1을 준다.

**JSON null도 없는 것으로 친다.** 점수 배치는 값이 없는 축을 아예 안 넣지만,
옛 행 중에 `"sentiment": null`로 적힌 것이 있으면 그것까지 「있다」로 세면 안 된다.
*/
func scoreAxisCountExpr(alias string) string {
	parts := make([]string, 0, len(model.ScoreAxes))
	for _, axis := range model.ScoreAxes {
		parts = append(parts, fmt.Sprintf(
			"(JSON_EXTRACT(%s.breakdown, '$.%s') IS NOT NULL"+
				" AND JSON_TYPE(JSON_EXTRACT(%s.breakdown, '$.%s')) <> 'NULL')",
			alias, axis, alias, axis,
		))
	}
	return "(" + strings.Join(parts, " + ") + ")"
}

// scoreMinAxesClause is the ≥N-axes gate for a query already pinned to one
// trade_date. `alias` is the stock_scores alias.
func scoreMinAxesClause(alias string) string {
	return fmt.Sprintf(" AND %s >= %d", scoreAxisCountExpr(alias), scoreRankingMinAxes)
}

/*
# 셋째 규칙 — 추천할 수 있는 크기 (2026-09-26 사용자 결정 「중간」)

**랭킹 1위가 살 수 없는 종목이었다.** 09-24 점수 기준 해외 상위 20 중 10종목이 하루
거래대금 100만 달러 미만이었고, 1위 SKYA는 $1.88·29만 달러, 2·3위(AESP·WTG)는
SPAC이라 거래가 거의 없었다. 국내 상위 20 중 12종목이 하루 2억 원 이하·시총 200~400억
원이었다. 처음 온 사람이 「AI 1위」로 이름 모를 동전주를 보면 점수 전체를 믿지 않는다.

그래서 셋을 더 본다. 점수는 그대로 두고 **추천 목록**(실시간·사전계산 랭킹, 내 기준
랭킹, 스크리너 자격)에서만 뺀다 — 우선주와 같은 방식이다. 변화 감지·성적표·수급은
「이 종목에 무슨 일이 있었나」「점수가 맞았나」를 묻는 곳이라 우주를 좁히지 않는다.

  - **SPAC 제외** — 해외는 영문명 `ACQUISITION`(489종목, 전수 확인 전부 SPAC·그 유닛·
    워런트), 국내는 이름 `스팩`(73종목).
  - **미국 우선주 제외** — 국내 우선주를 뺀 것과 같은 이유다. 보통주의 주식 수로 시총이
    잡혀 BMNP(비트마인 9.5% 영구 우선주)가 시총 576억 달러로 하한을 넘고 해외 2위에 섰다.
    판별은 **한글명의 `우선주`**(107종목)다. 영문명 `PREFERRED`는 Preferred Bank(PFBC,
    실제 은행)를 잡는다. ADR은 남긴다 — 브라질 은행·전력(BBD·CIG)은 우선주가 주력 상장이다.
  - **거래대금 하한** — 점수일까지 30일(≈20거래일) 평균 `종가×거래량`. 국내 5억 원 ·
    해외 200만 달러. 값은 점수 행 `inputs.avgTradeValue`에 있다(`FillAvgTradeValue`).
  - **시가총액 하한** — `inputs.marketCap`. 국내 500억 원 · 해외 3억 달러. 주식 수를
    모르는 종목(해외 3,157)은 시총이 없으므로 거래대금만 본다.

`avgTradeValue` 키가 **없는** 행(이 규칙 전에 매긴 날)은 통과시킨다. 지난 날짜의 랭킹
스냅샷·백테스트가 소급해서 바뀌면 「그때 보여 준 랭킹」이 아니게 된다.

실측(09-24): 자격 종목 국내 2,767 → 1,109 · 해외 6,780 → 3,142. 목록은 최대 100줄이다.
*/
const (
	scoreMinTradeValueDomestic = 5e8  // 원
	scoreMinTradeValueUS       = 2e6  // 달러
	scoreMinMarketCapDomestic  = 5e10 // 원
	scoreMinMarketCapUS        = 3e8  // 달러
)

/*
scoreRecommendableClause — 위 규칙 셋을 SQL로 옮긴 것. `sc`는 stock_scores, `st`는
stocks의 별칭이다. `scoreRankableProductClause`·`scoreMinAxesClause` **뒤에** 붙인다.
*/
func scoreRecommendableClause(sc, st string) (string, []any) {
	tv := fmt.Sprintf("CAST(%s.inputs->>'$.avgTradeValue' AS DOUBLE)", sc)
	mcap := fmt.Sprintf("CAST(%s.inputs->>'$.marketCap' AS DOUBLE)", sc)
	isDomestic := fmt.Sprintf("%s.asset_type = 'domestic'", st)
	clause := fmt.Sprintf(
		" AND UPPER(COALESCE(%s.english_name, '')) NOT LIKE '%%ACQUISITION%%'"+
			" AND %s.name NOT LIKE '%%스팩%%'"+
			" AND NOT (%s.asset_type = 'us' AND %s.name LIKE '%%우선주%%' AND %s.name NOT LIKE '%%ADR%%')"+
			" AND (JSON_EXTRACT(%s.inputs, '$.avgTradeValue') IS NULL OR ("+
			"%s >= IF(%s, ?, ?)"+
			" AND (JSON_EXTRACT(%s.inputs, '$.marketCap') IS NULL OR %s >= IF(%s, ?, ?))))",
		st, st, st, st, st, sc, tv, isDomestic, sc, mcap, isDomestic,
	)
	return clause, []any{
		scoreMinTradeValueDomestic, scoreMinTradeValueUS,
		scoreMinMarketCapDomestic, scoreMinMarketCapUS,
	}
}

/*
RankableSymbols — 최신 점수일에 랭킹 자격을 갖춘 종목들.

사전계산 배치가 쓴다. 기간 창을 훑는 `WindowBests`는 창 전체를 집계하므로 「최신
점수일의 축 개수」를 그 쿼리 안에서 묻기 어렵다(행마다 다른 날짜의 상관 서브쿼리가
붙는다). 대신 자격 집합을 **한 번** 받아 Go가 거른다 — 시장당 쿼리 하나이고,
`WindowBests`가 어차피 종목당 한 행을 돌려주므로 거르는 비용은 맵 조회뿐이다.

상위 N을 자르기 **전에** 걸러야 한다는 점이 중요하다. 잘라 낸 뒤에 거르면 100줄을
뽑아 놓고 절반을 버려 목록이 짧아진다.
*/
func (r *StockScoreRankingRepo) RankableSymbols(
	ctx context.Context,
	tradeDate string,
	assetTypes []string,
) (map[string]struct{}, error) {
	if r == nil || r.db == nil || tradeDate == "" || len(assetTypes) == 0 {
		return map[string]struct{}{}, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(assetTypes)), ",")
	productClause, productArgs := scoreRankableProductClause("s")
	sizeClause, sizeArgs := scoreRecommendableClause("sc", "s")
	query := `
		SELECT sc.symbol
		  FROM stock_scores sc
		  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.trade_date = ?
		   AND s.asset_type IN (` + placeholders + `)` +
		productClause + scoreMinAxesClause("sc") + sizeClause

	args := make([]any, 0, len(assetTypes)+len(productArgs)+len(sizeArgs)+1)
	args = append(args, tradeDate)
	for _, t := range assetTypes {
		args = append(args, t)
	}
	args = append(args, productArgs...)
	args = append(args, sizeArgs...)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("rankable symbols: %w", err)
	}
	defer rows.Close()

	out := map[string]struct{}{}
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, fmt.Errorf("rankable symbols scan: %w", err)
		}
		out[sym] = struct{}{}
	}
	return out, rows.Err()
}
