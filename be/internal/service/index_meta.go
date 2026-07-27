package service

import (
	"context"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
)

func dataKindForIndex(indexID string) model.IndexDataKind {
	switch indexID {
	case "nq100", "sox", "btc", "gold":
		return model.IndexDataKindLive
	default:
		return model.IndexDataKindEOD
	}
}

func eodScheduleForIndex(indexID string) calendar.ScheduleID {
	switch indexID {
	case "kospi", "kosdaq", "usd-krw":
		return calendar.ScheduleKRX
	default:
		return calendar.ScheduleUS
	}
}

func indexDisplayTime(entry indexCatalogEntry, q marketdata.Quote, fetchedAt time.Time) time.Time {
	if dataKindForIndex(entry.id) == model.IndexDataKindLive {
		if !q.AsOf.IsZero() {
			return q.AsOf.UTC()
		}
		if !fetchedAt.IsZero() {
			return fetchedAt.UTC()
		}
		return time.Now().UTC()
	}

	sched := eodScheduleForIndex(entry.id)
	if !q.AsOf.IsZero() {
		return calendar.SessionCloseAt(sched, q.AsOf)
	}
	ref := fetchedAt
	if ref.IsZero() {
		ref = time.Now()
	}
	return calendar.LastSessionCloseAt(sched, ref)
}

func applyIndexMeta(idx *model.MarketIndex, entry indexCatalogEntry, q marketdata.Quote, fetchedAt time.Time) {
	idx.DataKind = dataKindForIndex(entry.id)
	if idx.Value <= 0 {
		return
	}
	idx.UpdatedAt = indexDisplayTime(entry, q, fetchedAt).Format(time.RFC3339)
}

func (st *MarketIndexStore) enrichIndicesForAPI(ctx context.Context, indices []model.MarketIndex) []model.MarketIndex {
	out := make([]model.MarketIndex, len(indices))
	for i, idx := range indices {
		entry := marketIndexCatalog[i]
		out[i] = st.finalizeIndexMeta(ctx, entry, idx)
	}
	return out
}

func (st *MarketIndexStore) finalizeIndexMeta(ctx context.Context, entry indexCatalogEntry, idx model.MarketIndex) model.MarketIndex {
	idx.DataKind = dataKindForIndex(entry.id)
	if idx.Value <= 0 {
		return idx
	}

	if idx.DataKind == model.IndexDataKindEOD {
		closeAt := calendar.LastSessionCloseAt(eodScheduleForIndex(entry.id), time.Now())
		idx.UpdatedAt = closeAt.Format(time.RFC3339)
		return idx
	}

	if idx.UpdatedAt == "" {
		if t := st.lastRefreshAt(ctx, entry.id); !t.IsZero() {
			idx.UpdatedAt = t.UTC().Format(time.RFC3339)
		} else {
			idx.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	return idx
}
