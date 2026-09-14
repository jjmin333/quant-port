package service

import (
	"context"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
「많이 언급된 종목」 — 「오늘의 키워드」(news_top_keywords.go)와 같은 규칙, 같은 표본.

두 집계가 같은 자리에서 같은 크기의 표본을 보는 것이 중요하다. 한쪽은 최근 1,000건,
다른 쪽은 화면에 떠 있는 100건을 보고 있으면 같은 화면의 두 줄이 서로 다른 하루를
말한다 — 실제로 그랬다. 뉴스 목록 100건이 덮는 시간이 34분이라(실측 2026-09-01)
종목 언급 순위만 30분마다 뒤집혔다.

집계는 `repository.TopSymbols`가 SQL로 하고, 여기서는 **표본 크기와 캐시**만 정한다.
*/
const (
	// 키워드와 같은 표본을 쓴다. 두 집계가 다른 크기를 보면 같은 화면의 두 줄이
	// 서로 다른 기간을 말하게 된다.
	newsTopSymbolsSample = newsTopKeywordsSample
	// 한 번 언급된 종목은 화제가 아니다 — 키워드와 같은 문턱.
	newsTopSymbolsMinMentions  = 2
	newsTopSymbolsMaxLimit     = 30
	newsTopSymbolsDefaultLimit = 6
	newsTopSymbolsCacheTTL     = newsTopKeywordsCacheTTL
)

// NewsTopSymbols is the aggregated answer plus the sample it was drawn from.
type NewsTopSymbols struct {
	Symbols []repository.NewsSymbolCount `json:"symbols"`
	// ArticleCount는 **실제로 센 기사 수**다 — 요청한 표본보다 적을 수 있다.
	ArticleCount int `json:"articleCount"`
	MinMentions  int `json:"minMentions"`
}

type topSymbolsEntry struct {
	value NewsTopSymbols
	at    time.Time
}

var (
	topSymbolsMu    sync.Mutex
	topSymbolsCache = map[string]topSymbolsEntry{}
)

// TopSymbols returns the most-mentioned stocks across the market's recent
// articles, each already carrying its display name. Reads only — it never
// fetches news.
func (st *NewsStore) TopSymbols(ctx context.Context, market string, limit int) (NewsTopSymbols, error) {
	return st.TopSymbolsAbove(ctx, market, newsTopSymbolsMinMentions, limit)
}

/*
TopSymbolsAbove는 문턱을 부르는 쪽이 정하는 형태다.

뉴스 화면의 필터 칩과 홈 브리핑의 「많이 언급된 종목」이 **같은 표본(최근 1,000건)을**
보되 **다른 문턱**을 쓴다. 칩은 좁히는 도구라 2회면 충분하지만, 브리핑에서 「많이
언급됐다」고 적을 때 2회는 많이가 아니다 — 거기서는 10회다.

표본을 인자로 열지 않은 것은 일부러다. 두 화면이 다른 크기를 보면 같은 앱의 두 줄이
서로 다른 하루를 말한다 — 이 파일이 애초에 고치려던 문제가 그것이다.
*/
func (st *NewsStore) TopSymbolsAbove(
	ctx context.Context, market string, minMentions, limit int,
) (NewsTopSymbols, error) {
	if limit <= 0 {
		limit = newsTopSymbolsDefaultLimit
	}
	if minMentions <= 0 {
		minMentions = newsTopSymbolsMinMentions
	}
	if limit > newsTopSymbolsMaxLimit {
		limit = newsTopSymbolsMaxLimit
	}
	if st == nil || st.repo == nil {
		return NewsTopSymbols{
			Symbols:     []repository.NewsSymbolCount{},
			MinMentions: minMentions,
		}, nil
	}

	// 캐시는 limit·문턱별로 나눈다. 6을 받아 30을 자를 수 없고, 2회 이상으로
	// 센 목록에서 10회 이상을 골라낼 수도 없다(잘린 뒤라 아래가 없다).
	key := market + ":" + itoa(minMentions) + ":" + itoa(limit)

	topSymbolsMu.Lock()
	if hit, ok := topSymbolsCache[key]; ok && time.Since(hit.at) < newsTopSymbolsCacheTTL {
		topSymbolsMu.Unlock()
		return hit.value, nil
	}
	topSymbolsMu.Unlock()

	rows, sampled, err := st.repo.TopSymbols(
		ctx, market, newsTopSymbolsSample, minMentions, limit,
	)
	if err != nil {
		return NewsTopSymbols{}, err
	}
	out := NewsTopSymbols{
		Symbols:      rows,
		ArticleCount: sampled,
		MinMentions:  minMentions,
	}
	if out.Symbols == nil {
		out.Symbols = []repository.NewsSymbolCount{}
	}

	topSymbolsMu.Lock()
	topSymbolsCache[key] = topSymbolsEntry{value: out, at: time.Now()}
	topSymbolsMu.Unlock()

	return out, nil
}
