package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
buildUSTopicQueue — 구글 뉴스로 검색할 것들.

# 종목 이름으로도 검색한다 (2026-09-12 사용자 요청 2)

예전에는 **관리자 키워드 27개뿐**이었다. 그래서 「알파벳 A」·「알파벳 C」처럼 종목을
가리키는 한국어 이름으로는 아무도 검색하지 않았고, 그 종목의 한국어 기사는 우연히
다른 키워드에 걸릴 때만 들어왔다.

국내 쪽은 이미 이렇게 하고 있었다(`buildKeywordQueue`) — 추적 종목의 `stocks.name`을
검색어로 쓰고 **`Symbol`을 키워드에 실어 보낸다.** 그래서 검색 결과에 그 종목이
정확히 붙는다(본문 매칭에 의존하지 않는다). 여기서도 같은 구조를 쓴다.

# 왜 본문 매칭보다 이쪽이 나은가

「알파벳 A」로 검색해 받은 기사는 **그 검색어 때문에** 온 기사다. 심볼을 그 자리에서
붙이면 한국어 이름을 자유 본문에서 매칭할 때의 오탐 위험이 아예 없다 — 색인 쪽도
함께 열었지만(`addUSNameIndexKeys`) 이 경로가 더 정확하다.

# 우선순위

종목 이름이 0, 관리자 키워드가 1이다. 국내와 같은 순서이고, 커서가 한 바퀴 도는
동안 종목 쪽이 먼저 돈다 — 「주요 종목」의 뉴스가 주제 키워드보다 값이 크다.
*/
func (st *NewsStore) buildUSTopicQueue(ctx context.Context) []newsKeyword {
	var queue []newsKeyword

	if st.stocks != nil {
		symbols := st.usIndexSymbols(ctx)
		if stocks, err := st.stocks.ListBySymbols(ctx, symbols); err == nil {
			for _, s := range stocks {
				/*
					**한국어 이름만 검색어로 쓴다.** 구글 뉴스는 한국어 질의에 한국어
					기사를 준다. 영어 이름으로 검색하면 영어 기사가 오는데, 그쪽은
					이미 Finnhub·RSS가 받고 있어 같은 기사를 두 경로로 받게 된다.

					이름이 한글이 아닌 종목(AMD·QQQ·SPY처럼 토스도 영문으로 적는 것)은
					여기서 빠진다 — 그 종목은 티커·영어 이름 경로가 이미 덮는다.
				*/
				name := strings.TrimSpace(s.Name)
				if name == "" || !containsHangul(name) {
					continue
				}
				queue = append(queue, newsKeyword{
					Text:     name,
					Priority: 0,
					Symbol:   strings.ToUpper(strings.TrimSpace(s.Symbol)),
				})
			}
			/*
				**상장명이 아니라 사람들이 쓰는 말로도 검색한다** (2026-09-13 사용자 요청 1·4).

				상장명은 「알파벳 A」인데 한국 기사 제목은 「구글」이라고 쓴다. 위
				반복문은 상장명만 넣으므로, 그 기사는 이 경로로 영영 안 들어온다 —
				우연히 관리자 키워드에 걸릴 때만 왔다.

				별칭 표(`stock_aliases`)가 그 대응을 들고 있으므로 여기서 같이 쓴다.
				검색어로 온 기사는 그 자리에서 심볼이 붙으므로(`Symbol`) 본문 매칭의
				오탐 위험이 없다 — 위 머리글의 이유가 그대로 적용된다.

				**추적 중인 종목의 별칭만** 넣는다. 표 전체를 넣으면 아무도 안 보는
				종목이 큐를 늘려 관심 종목의 차례가 늦어진다(호출 수는 커서가 정하므로
				늘지 않지만, 한 바퀴가 길어지는 것은 곧 신선도 손해다).
			*/
			tracked := make(map[string]struct{}, len(stocks))
			for _, s := range stocks {
				tracked[strings.ToUpper(strings.TrimSpace(s.Symbol))] = struct{}{}
			}
			/*
				한 별칭이 여러 종목을 가리킬 수 있다 (2026-09-14 사용자 요청 1).
				검색어는 그래도 **하나**다 — 「구글」을 두 번 검색하면 같은 기사를
				두 번 받아 오고 호출 한도만 두 배로 쓴다. 그래서 질의는 한 번 넣고,
				그 기사에 붙일 심볼로는 **추적 중인 첫 대상**을 쓴다. 추적 중인
				것이 하나도 없으면 큐에 안 넣는다 — 아무도 안 보는 종목이 큐를
				늘리면 관심 종목의 차례가 늦어진다.
			*/
			for alias, syms := range repository.SymbolNameAliases() {
				if alias = strings.TrimSpace(alias); alias == "" || !containsHangul(alias) {
					continue
				}
				for _, sym := range syms {
					sym = strings.ToUpper(strings.TrimSpace(sym))
					if _, ok := tracked[sym]; !ok {
						continue
					}
					queue = append(queue, newsKeyword{Text: alias, Priority: 0, Symbol: sym})
					break
				}
			}
		}
	}

	for _, kw := range st.adminKeywordsForMarket(ctx, "us") {
		queue = append(queue, newsKeyword{Text: kw, Priority: 1})
	}

	sort.SliceStable(queue, func(i, j int) bool {
		if queue[i].Priority != queue[j].Priority {
			return queue[i].Priority < queue[j].Priority
		}
		return queue[i].Text < queue[j].Text
	})
	return dedupeKeywords(queue)
}

func (st *NewsStore) nextUSTopicKeywords(ctx context.Context, count int) []newsKeyword {
	queue := st.buildUSTopicQueue(ctx)
	if len(queue) == 0 || count <= 0 {
		return nil
	}

	st.mu.Lock()
	cursor := st.usGoogleCursor
	if st.redis != nil {
		if n, err := st.redis.Get(ctx, newsUSGoogleKeywordCursorKey).Int(); err == nil && n >= 0 {
			cursor = n
		}
	}
	if cursor >= len(queue) {
		cursor = 0
	}
	end := cursor + count
	if end > len(queue) {
		end = len(queue)
	}
	batch := append([]newsKeyword(nil), queue[cursor:end]...)
	next := end
	if next >= len(queue) {
		next = 0
	}
	st.usGoogleCursor = next
	st.mu.Unlock()

	if st.redis != nil {
		_ = st.redis.Set(ctx, newsUSGoogleKeywordCursorKey, next, 7*24*time.Hour).Err()
	}
	return batch
}

func (st *NewsStore) refreshUSGoogleNews(ctx context.Context) {
	if st.googleNews == nil || !st.googleNews.Enabled() {
		return
	}
	st.rebuildUSNameIndex(ctx)
	keywords := st.nextUSTopicKeywords(ctx, st.pollConfig(ctx).USGoogleKeywordsPerTick)
	var ingested []model.NewsArticle
	for _, kw := range keywords {
		items, err := st.googleNews.Search(ctx, kw.Text, 8)
		if err != nil {
			st.logFetchErr("google_news", err)
			continue
		}
		for _, item := range items {
			// 검색어가 종목 이름이면 그 심볼을 **그 자리에서** 붙인다 — 국내
			// 경로와 같다(buildUSTopicQueue 머리글).
			article, ok := st.ingestItem(ctx, item, []string{kw.Text}, kw.Symbol, "us")
			if ok {
				ingested = append(ingested, article)
			}
		}
	}
	st.mergeArticlesForMarket(ctx, ingested, "us")
}
