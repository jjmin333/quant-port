package service

import (
	"context"
	"fmt"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func (st *NewsStore) adminKeywordsForMarket(ctx context.Context, market string) []string {
	st.mu.RLock()
	cached, ok := st.adminKeywordsCache[market]
	st.mu.RUnlock()
	if ok && time.Since(cached.loadedAt) < newsAdminKeywordCacheTTL {
		return append([]string(nil), cached.items...)
	}

	var items []string
	if st.keywords != nil {
		if loaded, err := st.keywords.ListEnabledTexts(ctx, market); err == nil && len(loaded) > 0 {
			items = loaded
		}
	}
	if len(items) == 0 {
		switch market {
		case "us":
			items = append([]string(nil), defaultUSAdminKeywords...)
		default:
			items = append([]string(nil), defaultDomesticAdminKeywords...)
		}
	}

	st.mu.Lock()
	st.adminKeywordsCache[market] = cachedKeywords{items: items, loadedAt: time.Now()}
	st.mu.Unlock()
	return append([]string(nil), items...)
}

func (st *NewsStore) pollConfig(ctx context.Context) model.NewsPollSettings {
	st.mu.RLock()
	cached := st.pollSettingsCache
	at := st.pollSettingsAt
	st.mu.RUnlock()
	if !at.IsZero() && time.Since(at) < newsAdminKeywordCacheTTL {
		return cached
	}

	settings := model.DefaultNewsPollSettings()
	if st.pollSettings != nil {
		if loaded, err := st.pollSettings.Get(ctx); err == nil {
			settings = loaded
		}
	}

	st.mu.Lock()
	st.pollSettingsCache = settings
	st.pollSettingsAt = time.Now()
	st.mu.Unlock()
	return settings
}

func (st *NewsStore) InvalidatePollSettings() {
	st.mu.Lock()
	st.pollSettingsCache = model.DefaultNewsPollSettings()
	st.pollSettingsAt = time.Time{}
	st.mu.Unlock()
}

func (st *NewsStore) GetPollSettings(ctx context.Context) model.NewsPollSettings {
	return st.pollConfig(ctx)
}

func (st *NewsStore) UpdatePollSettings(ctx context.Context, input model.NewsPollSettings) (model.NewsPollSettings, error) {
	if st.pollSettings == nil {
		return model.NewsPollSettings{}, fmt.Errorf("poll settings repo not configured")
	}
	updated, err := st.pollSettings.Update(ctx, input)
	if err != nil {
		return model.NewsPollSettings{}, err
	}
	st.mu.Lock()
	st.pollSettingsCache = updated
	st.pollSettingsAt = time.Now()
	st.mu.Unlock()
	st.ResetKeywordCursors(ctx)
	return updated, nil
}

func (st *NewsStore) InvalidateAdminKeywords() {
	st.mu.Lock()
	st.adminKeywordsCache = make(map[string]cachedKeywords)
	st.mu.Unlock()
}

func (st *NewsStore) ResetKeywordCursors(ctx context.Context) {
	st.mu.Lock()
	st.keywordCursor = 0
	st.usKeywordCursor = 0
	st.usGoogleCursor = 0
	st.mu.Unlock()

	if st.redis != nil {
		_ = st.redis.Del(ctx, newsKeywordCursorKey, newsUSKeywordCursorKey, newsUSGoogleKeywordCursorKey).Err()
	}
}

func (st *NewsStore) NewsStats(ctx context.Context) (domesticCount, usCount int, domesticUpdated, usUpdated time.Time) {
	if st.repo == nil {
		return 0, 0, time.Time{}, time.Time{}
	}
	domesticCount, _ = st.repo.Count(ctx, "domestic")
	usCount, _ = st.repo.Count(ctx, "us")
	domesticUpdated, _ = st.repo.LatestUpdatedAt(ctx, "domestic")
	usUpdated, _ = st.repo.LatestUpdatedAt(ctx, "us")
	return domesticCount, usCount, domesticUpdated, usUpdated
}
