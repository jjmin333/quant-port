package service

import (
	"context"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func (st *NewsStore) refreshFinnhubUSGeneral(ctx context.Context) {
	if st.finnhubNews == nil || !st.finnhubNews.Enabled() {
		return
	}
	st.rebuildUSNameIndex(ctx)
	categories := []string{"general", "merger", "forex", "crypto"}
	var ingested []model.NewsArticle
	for _, category := range categories {
		items, err := st.finnhubNews.MarketNews(ctx, category)
		if err != nil {
			st.logFetchErr("finnhub_us_general", err)
			continue
		}
		for _, item := range items {
			linked := strings.ToUpper(strings.TrimSpace(item.RelatedSymbol))
			article, ok := st.ingestItem(ctx, item, nil, linked, "us")
			if ok {
				ingested = append(ingested, article)
			}
		}
	}
	st.mergeArticlesForMarket(ctx, ingested, "us")
}

func (st *NewsStore) refreshFinnhubUSCompany(ctx context.Context) {
	if st.finnhubNews == nil || !st.finnhubNews.Enabled() {
		return
	}
	st.rebuildUSNameIndex(ctx)
	symbols := st.nextUSSymbols(ctx, newsFinnhubCompanySymbolsPerTick)
	if len(symbols) == 0 {
		return
	}
	to := time.Now().UTC().Format("2006-01-02")
	from := time.Now().UTC().Add(-14 * 24 * time.Hour).Format("2006-01-02")

	var ingested []model.NewsArticle
	for _, kw := range symbols {
		if kw.Symbol == "" {
			continue
		}
		items, err := st.finnhubNews.CompanyNews(ctx, kw.Symbol, from, to)
		if err != nil {
			st.logFetchErr("finnhub_us_company", err)
			continue
		}
		for _, item := range items {
			linked := kw.Symbol
			if item.RelatedSymbol != "" {
				linked = strings.ToUpper(strings.TrimSpace(item.RelatedSymbol))
			}
			article, ok := st.ingestItem(ctx, item, []string{kw.Text}, linked, "us")
			if ok {
				ingested = append(ingested, article)
			}
		}
	}
	st.mergeArticlesForMarket(ctx, ingested, "us")
}

func (st *NewsStore) refreshRSSUS(ctx context.Context) {
	if st.rssUS == nil {
		return
	}
	st.rebuildUSNameIndex(ctx)
	items, err := st.rssUS.FetchAll(ctx)
	if err != nil {
		st.logFetchErr("rss_us", err)
		return
	}
	var ingested []model.NewsArticle
	for _, item := range items {
		article, ok := st.ingestItem(ctx, item, nil, "", "us")
		if ok {
			ingested = append(ingested, article)
		}
	}
	st.mergeArticlesForMarket(ctx, ingested, "us")
}

func (st *NewsStore) loadUSFromStorage(ctx context.Context) {
	if snap, at := st.loadSnapshotFromRedis(ctx, "us"); len(snap) > 0 {
		st.mu.Lock()
		st.usArticles = snap
		st.usUpdatedAt = at
		st.mu.Unlock()
		return
	}
	if st.repo == nil {
		return
	}
	recs, err := st.repo.List(ctx, repository.NewsListParams{
		Market: "us",
		Sort:   "latest",
		Limit:  newsUSListLimit,
	})
	if err != nil || len(recs) == 0 {
		return
	}
	snap := st.recordsToArticles(ctx, recs)
	at, _ := st.repo.LatestUpdatedAt(ctx, "us")
	st.mu.Lock()
	st.usArticles = snap
	st.usUpdatedAt = at
	st.mu.Unlock()
	st.persistSnapshotRedis(ctx, snap, at, "us")
}
