package service

import (
	"context"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
「오늘의 키워드」 — 세는 일을 서버로 옮긴 자리.

예전에는 홈이 기사 목록을 통째로 받아 브라우저에서 셌다. 100건일 때는 그게 가장 단순한
답이었지만 표본을 1,000건으로 넓히면 두 군데서 무너진다 — 회선(요약본 1,000건 ≈ 300KB)과
`NewsStore.List`의 국내 200건 상한(실측: `limit=1000`과 `limit=200`의 응답이 바이트
단위로 같았다). 집계는 `repository.TopKeywords`가 SQL로 하고(실측 27ms), 여기서는
**표본 크기와 캐시**만 정한다.

이제 규칙이 서버 한 곳에만 있다. 홈 카드와 뉴스 화면 레일이 같은 응답을 쓰므로, 두
언어에 같은 규칙을 두 벌 적어 놓고 언젠가 조용히 어긋나는 일이 없다.
*/
const (
	/*
		표본 1,000건.

		100건은 국내 기준 **반나절도 못 덮는다** — 조용한 아침에 올라온 단어가 점심이면
		목록에서 사라져, 「오늘의 키워드」가 사실상 「최근 두 시간의 키워드」였다.
		1,000건이면 하루를 넉넉히 덮는다. 비용은 SQL 27ms + 캐시라 늘려도 티가 없다.
	*/
	newsTopKeywordsSample = 1000
	// 두 번 미만은 화제가 아니다 — 화면이 쓰던 문턱 그대로다.
	newsTopKeywordsMinMentions  = 2
	newsTopKeywordsMaxLimit     = 40
	newsTopKeywordsDefaultLimit = 14
	/*
		기사는 2분마다 들어오는데, 1,000건 표본에서 순위가 뒤집히려면 그보다 훨씬 많이
		쌓여야 한다. 5분이면 화면이 늦었다고 느껴질 일이 없으면서 홈을 여는 사람마다
		집계가 돌지는 않는다.
	*/
	newsTopKeywordsCacheTTL = 5 * time.Minute
)

// NewsTopKeywords is the aggregated answer plus the sample it was drawn from.
type NewsTopKeywords struct {
	Keywords []repository.NewsKeywordCount `json:"keywords"`
	// ArticleCount는 **실제로 센 기사 수**다. 화면 각주가 「최근 기사 N건에서」라고
	// 적을 때 요청한 수가 아니라 있었던 수를 써야 한다 — 표본보다 적게 쌓인 시장에서
	// 1,000건이라고 적으면 거짓이 된다.
	ArticleCount int `json:"articleCount"`
	MinMentions  int `json:"minMentions"`
}

type topKeywordsEntry struct {
	value NewsTopKeywords
	at    time.Time
}

var (
	topKeywordsMu    sync.Mutex
	topKeywordsCache = map[string]topKeywordsEntry{}
)

// TopKeywords returns the most repeated keywords across the market's recent
// articles. Reads only — it never fetches news.
func (st *NewsStore) TopKeywords(ctx context.Context, market string, limit int) (NewsTopKeywords, error) {
	if limit <= 0 {
		limit = newsTopKeywordsDefaultLimit
	}
	if limit > newsTopKeywordsMaxLimit {
		limit = newsTopKeywordsMaxLimit
	}
	if st == nil || st.repo == nil {
		return NewsTopKeywords{MinMentions: newsTopKeywordsMinMentions}, nil
	}

	// 캐시는 limit별로 나눈다. 14를 받아 40을 자를 수는 없다.
	key := market + ":" + itoa(limit)

	topKeywordsMu.Lock()
	if hit, ok := topKeywordsCache[key]; ok && time.Since(hit.at) < newsTopKeywordsCacheTTL {
		topKeywordsMu.Unlock()
		return hit.value, nil
	}
	topKeywordsMu.Unlock()

	rows, sampled, err := st.repo.TopKeywords(
		ctx, market, newsTopKeywordsSample, newsTopKeywordsMinMentions, limit,
	)
	if err != nil {
		return NewsTopKeywords{}, err
	}
	out := NewsTopKeywords{
		Keywords:     rows,
		ArticleCount: sampled,
		MinMentions:  newsTopKeywordsMinMentions,
	}
	if out.Keywords == nil {
		out.Keywords = []repository.NewsKeywordCount{}
	}

	topKeywordsMu.Lock()
	topKeywordsCache[key] = topKeywordsEntry{value: out, at: time.Now()}
	topKeywordsMu.Unlock()

	return out, nil
}
