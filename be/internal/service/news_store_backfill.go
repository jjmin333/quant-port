package service

import (
	"context"
	"log"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/repository"
)

const (
	newsOGBackfillLimit = 200
	newsOGReprobeLimit  = 120
)

func (st *NewsStore) backfillMissingImages(ctx context.Context) {
	if st.repo == nil {
		return
	}
	st.backfillEmptyImages(ctx)
	st.reprobeRecentImages(ctx)
}

func (st *NewsStore) backfillEmptyImages(ctx context.Context) {
	recs, err := st.repo.ListMissingImages(ctx, "domestic", newsOGBackfillLimit)
	if err != nil {
		st.logFetchErr("backfill", err)
		return
	}
	if len(recs) == 0 {
		return
	}

	updated := 0
	for _, rec := range recs {
		imageURL := st.fetchArticleOGImage(ctx, rec)
		if imageURL == "" {
			continue
		}
		if err := st.repo.UpdateImageURL(ctx, rec.ID, imageURL); err != nil {
			st.logFetchErr("backfill", err)
			continue
		}
		st.patchArticleImage(rec.ID, imageURL)
		updated++
	}
	if updated > 0 {
		log.Printf("news backfill: og:image updated %d rows", updated)
	}
}

func (st *NewsStore) reprobeRecentImages(ctx context.Context) {
	recs, err := st.repo.ListRecentForImageReprobe(ctx, "domestic", newsOGReprobeLimit)
	if err != nil {
		st.logFetchErr("reprobe", err)
		return
	}
	if len(recs) == 0 {
		return
	}

	updated := 0
	for _, rec := range recs {
		imageURL := st.fetchArticleOGImage(ctx, rec)
		if imageURL == "" || imageURL == strings.TrimSpace(rec.ImageURL) {
			continue
		}
		if err := st.repo.UpdateImageURL(ctx, rec.ID, imageURL); err != nil {
			st.logFetchErr("reprobe", err)
			continue
		}
		st.patchArticleImage(rec.ID, imageURL)
		updated++
	}
	if updated > 0 {
		log.Printf("news reprobe: og:image corrected %d rows", updated)
	}
}

func (st *NewsStore) fetchArticleOGImage(ctx context.Context, rec repository.NewsArticleRecord) string {
	fetchURL := strings.TrimSpace(rec.OriginalURL)
	if fetchURL == "" {
		fetchURL = strings.TrimSpace(rec.URL)
	}
	if fetchURL == "" {
		return ""
	}
	return marketdata.FetchOGImage(ctx, st.ogHTTP, st.redis, fetchURL)
}

func (st *NewsStore) patchArticleImage(id, imageURL string) {
	st.mu.Lock()
	for i := range st.articles {
		if st.articles[i].ID == id {
			st.articles[i].ImageURL = imageURL
			break
		}
	}
	st.mu.Unlock()
}

func (st *NewsStore) IncrementView(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" || st.repo == nil {
		return nil
	}
	if err := st.repo.IncrementViewCount(ctx, id); err != nil {
		return err
	}
	st.mu.Lock()
	for i := range st.articles {
		if st.articles[i].ID == id {
			st.articles[i].ViewCount++
			break
		}
	}
	st.mu.Unlock()
	return nil
}
