package service

import (
	"context"
	"sort"
	"strings"
	"time"
)

// trackedSymbolsTTL — 관심·보유가 바뀌고 뉴스 태깅에 반영되기까지의 최대 지연.
//
// 이 목록을 만드는 데 watchlist 전수 조회 + 수동 보유 조회가 들어가는데,
// extractSymbols가 **기사마다** 이걸 부른다. 목록 요청 하나에 기사 수만큼 나가던
// 조회다. 관심종목을 담자마자 뉴스 태그가 갱신돼야 하는 화면은 없다.
const trackedSymbolsTTL = time.Minute

func (st *NewsStore) trackedSymbols(ctx context.Context) []string {
	st.mu.RLock()
	cached, at := st.trackedSymbolsCache, st.trackedSymbolsAt
	st.mu.RUnlock()
	if len(cached) > 0 && time.Since(at) < trackedSymbolsTTL {
		return cached
	}
	out := st.trackedSymbolsUncached(ctx)
	if len(out) > 0 {
		st.mu.Lock()
		st.trackedSymbolsCache = out
		st.trackedSymbolsAt = time.Now()
		st.mu.Unlock()
	}
	return out
}

func (st *NewsStore) trackedSymbolsUncached(ctx context.Context) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(sym string) {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			return
		}
		if _, ok := seen[sym]; ok {
			return
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	if st.watchlist != nil {
		symbols, err := st.watchlist.ListAllSymbols(ctx)
		if err == nil {
			for _, sym := range symbols {
				add(sym)
			}
		}
	}
	if st.holdings != nil {
		symbols, err := st.holdings.ListAllSymbols(ctx)
		if err == nil {
			for _, sym := range symbols {
				add(sym)
			}
		}
	}
	return out
}

func (st *NewsStore) buildKeywordQueue(ctx context.Context) []newsKeyword {
	symbols := st.trackedSymbols(ctx)
	var queue []newsKeyword

	if st.stocks != nil && len(symbols) > 0 {
		stocks, err := st.stocks.ListBySymbols(ctx, symbols)
		if err == nil {
			for _, s := range stocks {
				name := strings.TrimSpace(s.Name)
				if name == "" {
					name = s.Symbol
				}
				queue = append(queue, newsKeyword{
					Text:     name,
					Priority: 0,
					Symbol:   strings.ToUpper(s.Symbol),
				})
			}
		}
	}

	for _, kw := range st.adminKeywordsForMarket(ctx, "domestic") {
		queue = append(queue, newsKeyword{
			Text:     kw,
			Priority: 1,
		})
	}

	sort.SliceStable(queue, func(i, j int) bool {
		if queue[i].Priority != queue[j].Priority {
			return queue[i].Priority < queue[j].Priority
		}
		return queue[i].Text < queue[j].Text
	})

	return dedupeKeywords(queue)
}

func dedupeKeywords(items []newsKeyword) []newsKeyword {
	seen := map[string]struct{}{}
	var out []newsKeyword
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Text))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func (st *NewsStore) nextKeywords(ctx context.Context, count int) []newsKeyword {
	queue := st.buildKeywordQueue(ctx)
	if len(queue) == 0 || count <= 0 {
		return nil
	}

	st.mu.Lock()
	cursor := st.keywordCursor
	if st.redis != nil {
		if n, err := st.redis.Get(ctx, newsKeywordCursorKey).Int(); err == nil && n >= 0 {
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
	st.keywordCursor = next
	st.mu.Unlock()

	if st.redis != nil {
		_ = st.redis.Set(ctx, newsKeywordCursorKey, next, 7*24*time.Hour).Err()
	}
	return batch
}
