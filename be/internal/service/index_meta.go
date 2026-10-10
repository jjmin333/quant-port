package service

import (
	"context"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func dataKindForIndex(indexID string) model.IndexDataKind {
	switch indexID {
	case "usd-krw", "nq100", "sox", "btc", "gold":
		return model.IndexDataKindLive
	default:
		return model.IndexDataKindEOD
	}
}

func eodScheduleForIndex(indexID string) calendar.ScheduleID {
	switch indexID {
	case "kospi", "kosdaq", "kr-base", "ktb3", "ktb10":
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

func (st *MarketIndexStore) krSessionOpen(now time.Time) bool {
	var kr *toss.KrMarketCalendarResponse
	if st.marketStore != nil {
		if cals, ok := st.marketStore.EnsureCalendars(context.Background()); ok {
			kr = cals.kr
		}
	}
	return calendar.ShouldPoll(calendar.ScheduleKRX, now, kr, nil)
}

func (st *MarketIndexStore) finalizeIndexMeta(ctx context.Context, entry indexCatalogEntry, idx model.MarketIndex) model.MarketIndex {
	if idx.Value <= 0 {
		idx.DataKind = dataKindForIndex(entry.id)
		return idx
	}

	now := time.Now()
	if entry.id == "usd-krw" {
		idx.DataKind = model.IndexDataKindLive
		if st.fx != nil {
			if snap, ok := st.fx.Get(ctx); ok && !snap.UpdatedAt.IsZero() {
				idx.UpdatedAt = snap.UpdatedAt.UTC().Format(time.RFC3339)
				return idx
			}
		}
		if t := st.lastRefreshAt(ctx, entry.id); !t.IsZero() {
			idx.UpdatedAt = t.UTC().Format(time.RFC3339)
		} else if idx.UpdatedAt == "" {
			idx.UpdatedAt = now.UTC().Format(time.RFC3339)
		}
		return idx
	}
	// 토스 지표 API로 받는 국내 지표 — 장중에는 실시간이다(국고채는 2026-10-07부터).
	if entry.id == "kospi" || entry.id == "kosdaq" || entry.id == "ktb3" || entry.id == "ktb10" {
		if st.krSessionOpen(now) {
			idx.DataKind = model.IndexDataKindLive
			if t := st.lastRefreshAt(ctx, entry.id); !t.IsZero() {
				idx.UpdatedAt = t.UTC().Format(time.RFC3339)
			} else if idx.UpdatedAt != "" {
				// keep batch refresh time
			} else {
				idx.UpdatedAt = now.UTC().Format(time.RFC3339)
			}
			return idx
		}
	}

	idx.DataKind = dataKindForIndex(entry.id)
	if idx.DataKind == model.IndexDataKindEOD {
		closeAt := calendar.LastSessionCloseAt(eodScheduleForIndex(entry.id), now)
		idx.UpdatedAt = closeAt.Format(time.RFC3339)
		return idx
	}

	if idx.UpdatedAt == "" {
		if t := st.lastRefreshAt(ctx, entry.id); !t.IsZero() {
			idx.UpdatedAt = t.UTC().Format(time.RFC3339)
		} else {
			idx.UpdatedAt = now.UTC().Format(time.RFC3339)
		}
	}
	return idx
}
