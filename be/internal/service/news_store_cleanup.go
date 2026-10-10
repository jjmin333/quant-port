package service

import (
	"context"
	"log"
	"time"
)

const newsCleanupBatchSize = 200

type NewsCleanupResult struct {
	Market  string
	Total   int
	Removed int
	Kept    int
	DryRun  bool
}

// CleanupIrrelevantArticles removes news rows that fail the stock relevance filter.
func (st *NewsStore) CleanupIrrelevantArticles(ctx context.Context, market string, dryRun bool) (NewsCleanupResult, error) {
	result := NewsCleanupResult{Market: market, DryRun: dryRun}
	if st.repo == nil {
		return result, nil
	}

	switch market {
	case "domestic":
		st.rebuildNameIndex(ctx)
	case "us":
		st.rebuildUSNameIndex(ctx)
	default:
		return result, nil
	}

	total, err := st.repo.Count(ctx, market)
	if err != nil {
		return result, err
	}
	result.Total = total

	var toDelete []string
	offset := 0
	for {
		recs, err := st.repo.ListPage(ctx, market, offset, newsCleanupBatchSize)
		if err != nil {
			return result, err
		}
		if len(recs) == 0 {
			break
		}
		for _, rec := range recs {
			// 주제 기사(부동산·금리 …, 2026-10-09)는 종목 관련성으로 판정하지 않는다 — 일부러 들인 기사다.
			keep := (rec.Topic != "" && rec.Topic != "stock") || isStockRelatedRecord(ctx, st, rec)
			if market == "us" {
				keep = isUSStockRelatedRecord(ctx, st, rec) || isFinancialJuiceRecord(rec)
			}
			if keep {
				result.Kept++
			} else {
				toDelete = append(toDelete, rec.ID)
			}
		}
		offset += len(recs)
		if len(recs) < newsCleanupBatchSize {
			break
		}
	}

	result.Removed = len(toDelete)
	if dryRun || len(toDelete) == 0 {
		return result, nil
	}

	deleted, err := st.repo.DeleteByIDs(ctx, toDelete)
	if err != nil {
		return result, err
	}
	result.Removed = int(deleted)
	result.Kept = total - result.Removed
	st.refreshCacheAfterCleanup(ctx, market)
	log.Printf("news cleanup (%s): removed %d/%d irrelevant articles", market, result.Removed, result.Total)
	return result, nil
}

func (st *NewsStore) refreshCacheAfterCleanup(ctx context.Context, market string) {
	if st.redis != nil {
		switch market {
		case "us":
			_ = st.redis.Del(ctx, newsUSSnapshotKey).Err()
		default:
			_ = st.redis.Del(ctx, newsSnapshotKey).Err()
		}
	}
	st.mu.Lock()
	st.articles = nil
	st.updatedAt = time.Time{}
	st.mu.Unlock()
	switch market {
	case "us":
		st.loadUSFromStorage(ctx)
	default:
		st.loadFromStorage(ctx)
	}
}
