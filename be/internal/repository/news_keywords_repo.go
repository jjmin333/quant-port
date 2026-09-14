package repository

import (
	"context"
	"database/sql"
)

/*
「오늘의 키워드」 집계 — **표본은 DB에 두고 세는 것도 DB가 한다.**

예전에는 화면이 기사 목록을 통째로 받아 브라우저에서 셌다. 100건일 때는 그게 가장
단순한 답이었다(집계 규칙이 한 곳에만 있다). 그런데 표본을 1,000건으로 넓히자 그
방식이 두 군데서 무너진다:

  - **회선.** 요약본 1,000건이 300KB가 넘는다. 홈은 그중 여덟 줄만 그리는데, 나머지
    992건은 단어를 세려고 모바일 회선을 건너온다.
  - **상한.** `NewsStore.List`가 국내를 200건에서 자른다. 화면이 1,000을 달라고 해도
    200이 왔고, 그래서 `limit=1000`과 `limit=200`의 응답이 **바이트 단위로 같았다**.

여기서는 답만 건너간다 — 14줄, 2KB 남짓. 실측(도메스틱 1,000건 표본): 27ms.

집계 규칙은 화면이 쓰던 것 그대로다:
  - **한 기사가 같은 단어를 여러 번 달아도 1로 센다** (`COUNT(DISTINCT a.id)`).
    태깅이 후한 기사 하나가 「오늘의 키워드」를 혼자 정하지 않게.
  - 한 글자는 버린다. 한 번만 나온 단어도 버린다 — 한 번 나온 건 화제가 아니다.
*/

// NewsKeywordCount is one aggregated keyword row.
type NewsKeywordCount struct {
	Keyword string `json:"keyword"`
	Count   int    `json:"count"`
}

/*
TopKeywords는 최신 `sample`건에서 상위 키워드를 센다.

파생 테이블(최신 N건)을 먼저 좁히는 것이 핵심이다. `idx_news_market_published`가
(market, published_at DESC)라 그 N건은 인덱스를 앞에서부터 N행 읽으면 끝나고,
JSON_TABLE은 그 N행에만 돈다 — 10만 행짜리 표 전체를 펼치지 않는다.

두 번째 반환값은 **실제로 센 기사 수**다. 화면 각주가 「최근 기사 N건에서」라고
적으려면 요청한 수가 아니라 있었던 수를 알아야 한다 — 해외처럼 표본보다 적게 쌓인
시장에서 1,000건이라고 적으면 그건 거짓이다.
*/
func (r *NewsRepo) TopKeywords(
	ctx context.Context, market string, sample, minMentions, limit int,
) ([]NewsKeywordCount, int, error) {
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

	rows, err := r.db.QueryContext(ctx, `
		SELECT j.kw, COUNT(DISTINCT a.id) AS n
		  FROM (
		        SELECT id, keywords_json FROM news_articles
		         WHERE market = ?
		         ORDER BY published_at DESC
		         LIMIT ?
		       ) a
		  JOIN JSON_TABLE(
		        a.keywords_json, '$[*]' COLUMNS (kw VARCHAR(64) PATH '$')
		       ) j ON TRUE
		 WHERE CHAR_LENGTH(TRIM(j.kw)) >= 2
		 GROUP BY j.kw
		HAVING n >= ?
		 ORDER BY n DESC, j.kw ASC
		 LIMIT ?`, market, sample, minMentions, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]NewsKeywordCount, 0, limit)
	for rows.Next() {
		var (
			kw sql.NullString
			n  int
		)
		if err := rows.Scan(&kw, &n); err != nil {
			return nil, 0, err
		}
		if !kw.Valid || kw.String == "" {
			continue
		}
		out = append(out, NewsKeywordCount{Keyword: kw.String, Count: n})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, sampled, nil
}
