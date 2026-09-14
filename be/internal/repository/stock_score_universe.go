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
	clause := fmt.Sprintf(
		" AND %s.asset_type <> ? AND UPPER(TRIM(%s.security_type)) NOT IN (%s)",
		alias, alias, placeholders,
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
	query := `
		SELECT sc.symbol
		  FROM stock_scores sc
		  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.trade_date = ?
		   AND s.asset_type IN (` + placeholders + `)` +
		productClause + scoreMinAxesClause("sc")

	args := make([]any, 0, len(assetTypes)+len(productArgs)+1)
	args = append(args, tradeDate)
	for _, t := range assetTypes {
		args = append(args, t)
	}
	args = append(args, productArgs...)

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
