package service

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

const (
	remigrateMarketDoneKey = "news:remigrate-market:v1:domestic"
	remigrateMarketTTL     = 90 * 24 * time.Hour
)

// RemigrateMarketResult reports one domestic→US sweep outcome.
type RemigrateMarketResult struct {
	Scanned  int    `json:"scanned"`
	Moved    int    `json:"moved"`
	Failed   int    `json:"failed"`
	Duration string `json:"duration"`
}

// RemigrateDomesticToUS reclassifies stored domestic articles whose headline
// evidence points to US equities (e.g. "엔비디아" without "NVDA" ticker).
func (st *NewsStore) RemigrateDomesticToUS(ctx context.Context) (RemigrateMarketResult, error) {
	started := time.Now()
	res := RemigrateMarketResult{}
	if st.repo == nil {
		return res, nil
	}

	st.rebuildUSNameIndex(ctx)
	st.mu.RLock()
	ready := len(st.usNameIndex) > 0
	st.mu.RUnlock()
	if !ready {
		return res, nil
	}

	for offset := 0; ; offset += retagPageSize {
		if ctx.Err() != nil {
			break
		}
		recs, err := st.repo.ListPage(ctx, "domestic", offset, retagPageSize)
		if err != nil {
			return res, err
		}
		if len(recs) == 0 {
			break
		}
		for _, rec := range recs {
			res.Scanned++
			if st.inferNewsMarket(ctx, rec.Headline, rec.Summary, "", "domestic") != "us" {
				continue
			}
			if err := st.moveArticleToUSMarket(ctx, rec); err != nil {
				res.Failed++
				continue
			}
			res.Moved++
		}
	}

	res.Duration = time.Since(started).Round(time.Millisecond).String()
	if res.Moved > 0 {
		if st.redis != nil {
			_ = st.redis.Del(ctx, snapshotKeyForMarket("domestic"), snapshotKeyForMarket("us")).Err()
		}
		st.loadFromStorage(ctx)
		st.loadUSFromStorage(ctx)
	}
	return res, nil
}

func (st *NewsStore) moveArticleToUSMarket(ctx context.Context, rec repository.NewsArticleRecord) error {
	if err := st.repo.UpdateMarket(ctx, rec.ID, "us"); err != nil {
		return err
	}
	headlineKo := strings.TrimSpace(rec.HeadlineKo)
	if headlineKo == "" && containsHangulText(rec.Headline) {
		if err := st.repo.UpdateTranslation(ctx, rec.ID, rec.Headline, rec.Summary); err != nil {
			return err
		}
	}
	symbols, tags := st.retagRecord(ctx, repository.NewsArticleRecord{
		ID:       rec.ID,
		Market:   "us",
		Headline: rec.Headline,
		Summary:  rec.Summary,
		Tags:     rec.Tags,
	})
	if err := st.repo.UpdateSymbols(ctx, rec.ID, symbols); err != nil {
		return err
	}
	return st.repo.UpdateTags(ctx, rec.ID, tags)
}

func (st *NewsStore) remigrateMarketOnceIfNeeded(ctx context.Context) {
	if st.redis != nil {
		if done, err := st.redis.Get(ctx, remigrateMarketDoneKey).Result(); err == nil && done != "" {
			return
		}
	}
	res, err := st.RemigrateDomesticToUS(ctx)
	if err != nil {
		st.logFetchErr("remigrate-market", err)
		return
	}
	if res.Scanned == 0 {
		return
	}
	log.Printf("news remigrate domestic→us: scanned=%d moved=%d failed=%d in %s",
		res.Scanned, res.Moved, res.Failed, res.Duration)
	if st.redis != nil {
		_ = st.redis.Set(ctx, remigrateMarketDoneKey, time.Now().Format(time.RFC3339), remigrateMarketTTL).Err()
	}
}
