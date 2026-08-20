package service

import (
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func paginationBaseInterval(interval model.CandleInterval) (model.CandleInterval, bool) {
	return storageBaseInterval(interval)
}

func normalizeCandlesPageCursor(page CandlesPage, base model.CandleInterval) CandlesPage {
	page.NextBefore = model.CanonicalPaginationCursorPtr(page.NextBefore, base)
	return page
}

func normalizeSnapshotCursor(snap model.CandleSnapshot) model.CandleSnapshot {
	base, ok := storageBaseInterval(snap.Interval)
	if !ok {
		return snap
	}
	snap.NextBefore = model.CanonicalPaginationCursorPtr(snap.NextBefore, base)
	return snap
}

func tossCandleBaseInterval(interval toss.CandleInterval) model.CandleInterval {
	if interval == toss.CandleInterval1d {
		return model.Interval1d
	}
	return model.Interval1m
}

func normalizeTossCandlesParams(params toss.GetCandlesParams) toss.GetCandlesParams {
	if strings.TrimSpace(params.Before) == "" {
		return params
	}
	base := tossCandleBaseInterval(params.Interval)
	params.Before = model.CanonicalPaginationCursor(params.Before, base)
	return params
}

func normalizeTossCandlesResponse(page *toss.CandlePageResponse, interval toss.CandleInterval) {
	if page == nil || page.NextBefore == nil {
		return
	}
	base := tossCandleBaseInterval(interval)
	page.NextBefore = model.CanonicalPaginationCursorPtr(page.NextBefore, base)
}
