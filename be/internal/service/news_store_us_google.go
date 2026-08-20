package service

import (
	"context"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func (st *NewsStore) buildUSTopicQueue(ctx context.Context) []newsKeyword {
	keywords := st.adminKeywordsForMarket(ctx, "us")
	queue := make([]newsKeyword, 0, len(keywords))
	for _, kw := range keywords {
		queue = append(queue, newsKeyword{
			Text:     kw,
			Priority: 1,
		})
	}
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
			article, ok := st.ingestItem(ctx, item, []string{kw.Text}, "", "us")
			if ok {
				ingested = append(ingested, article)
			}
		}
	}
	st.mergeArticlesForMarket(ctx, ingested, "us")
}
