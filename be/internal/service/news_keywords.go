package service

import (
	"context"
	"sort"
	"strings"
	"time"
)

func (st *NewsStore) trackedSymbols(ctx context.Context) []string {
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
	if st.portfolio != nil {
		for _, sym := range st.portfolio.AllTrackedSymbols(ctx) {
			add(sym)
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

	for _, kw := range adminNewsKeywords {
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
