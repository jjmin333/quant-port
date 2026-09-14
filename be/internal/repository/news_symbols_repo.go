package repository

import (
	"context"
	"database/sql"
)

/*
「많이 언급된 종목」 집계 — `TopKeywords`와 같은 자리, 같은 방식이다.

화면이 세던 것을 서버로 옮긴 이유가 키워드 때와 똑같다. 뉴스 목록은 최신 100건만
내려오는데, 그 100건이 덮는 시간이 **34분이었다**(실측 2026-09-01: newest 16:47,
oldest 16:14). 그 위에서 센 「15건·13건」은 「오늘 많이 언급된 종목」이 아니라
「지난 34분에 많이 언급된 종목」이었고, 그래서 새로고침할 때마다 순위가 뒤집혔다.
표본을 1,000건으로 넓히면 하루를 덮는다.

`stocks`를 조인하는 것이 이 쿼리의 두 번째 값어치다:

  - **이름이 함께 온다.** 화면이 시세 배치가 돌아오기를 기다리는 동안 종목코드를
    플레이스홀더로 쓰던 것이 사라진다 — 사용자에게는 그게 「종목명 대신 코드가
    뜨는 버그」로 보였다.
  - **해외 티커가 걸러진다.** 국내 기사가 엔비디아를 자주 언급해서 NVDA가 국내
    뉴스의 3위 필터로 올라와 있었다(실측 100건 중 6건). 국내 화면의 종목 필터는
    국내 종목이어야 한다. 기사 하단 관련 종목 칩은 이 쿼리를 쓰지 않으므로
    거기서는 지금처럼 NVDA가 그대로 보인다.
*/

// NewsSymbolCount is one aggregated symbol row: the code, its display name, and
// how many of the sampled articles mentioned it.
type NewsSymbolCount struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Count  int    `json:"count"`
}

/*
TopSymbols는 최신 `sample`건에서 상위 언급 종목을 센다.

`TopKeywords`와 같은 형태다 — 파생 테이블로 최신 N건을 먼저 좁히면
`idx_news_market_published`(market, published_at DESC)를 앞에서부터 N행 읽고 끝나므로
JSON_TABLE이 표 전체를 펼치지 않는다.

`stocks`는 INNER JOIN이다. 카탈로그에 없는 심볼(해외 티커·상장폐지·오탐 태그)은
여기서 떨어져 나간다 — 이름을 못 붙일 종목을 필터 칩으로 내놓을 이유가 없다.
`market == "domestic"`이면 `asset_type`까지 걸어 파생·해외 ETP를 마저 덜어낸다.

두 번째 반환값은 **실제로 센 기사 수**다. 각주가 「최근 기사 N건에서」라고 적을 때
요청한 수가 아니라 있었던 수를 써야 한다.
*/
func (r *NewsRepo) TopSymbols(
	ctx context.Context, market string, sample, minMentions, limit int,
) ([]NewsSymbolCount, int, error) {
	if r == nil || r.db == nil || market == "" {
		return nil, 0, nil
	}

	var sampled int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT id FROM news_articles
			 WHERE market = ?
			 ORDER BY published_at DESC
			 LIMIT ?
		) a`, market, sample).Scan(&sampled)
	if err != nil {
		return nil, 0, err
	}
	if sampled == 0 {
		return nil, 0, nil
	}

	// 국내 화면에서는 국내 상장 종목만 남긴다. 해외 화면은 카탈로그에 있기만 하면
	// 되므로(미국 티커의 asset_type은 'us') 자산 유형을 걸지 않는다.
	assetClause := ""
	if market == "domestic" {
		assetClause = " AND s.asset_type = 'domestic'"
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT j.sym, s.name, COUNT(DISTINCT a.id) AS n
		  FROM (
		        SELECT id, symbols_json FROM news_articles
		         WHERE market = ?
		         ORDER BY published_at DESC
		         LIMIT ?
		       ) a
		  JOIN JSON_TABLE(
		        a.symbols_json, '$[*]' COLUMNS (sym VARCHAR(16) PATH '$')
		       ) j ON TRUE
		  JOIN stocks s ON s.symbol = j.sym
		 WHERE s.status = 'NORMAL'
		   AND s.name <> ''`+assetClause+`
		 GROUP BY j.sym, s.name
		HAVING n >= ?
		 ORDER BY n DESC, j.sym ASC
		 LIMIT ?`, market, sample, minMentions, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]NewsSymbolCount, 0, limit)
	for rows.Next() {
		var (
			sym  sql.NullString
			name sql.NullString
			n    int
		)
		if err := rows.Scan(&sym, &name, &n); err != nil {
			return nil, 0, err
		}
		if !sym.Valid || sym.String == "" {
			continue
		}
		out = append(out, NewsSymbolCount{
			Symbol: sym.String,
			Name:   name.String,
			Count:  n,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, sampled, nil
}
