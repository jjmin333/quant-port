package service

import (
	"context"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const (
	quotesIndicesSnapshotKey = "quotes:indices:snapshot:v2"
	indicesSnapshotTTL       = 10 * time.Minute
	indicesRefreshTimeout    = 90 * time.Second
)

type indicesSnapshotPayload struct {
	Indices   []model.MarketIndex `json:"indices"`
	UpdatedAt time.Time           `json:"updatedAt"`
}

func (s *QuotesService) ListIndices(ctx context.Context) ([]model.MarketIndex, time.Time) {
	if s.indexStore != nil {
		return s.indexStore.List(ctx)
	}
	return emptyIndicesFromCatalog(), time.Time{}
}

// IndicesCheckedAt is when the index refresh loop last completed a cycle.
func (s *QuotesService) IndicesCheckedAt() time.Time {
	if s.indexStore == nil {
		return time.Time{}
	}
	return s.indexStore.CheckedAt()
}

func (s *QuotesService) ListIndexGroups(ctx context.Context) map[string]model.IndexGroupMeta {
	if s.indexStore != nil {
		return s.indexStore.GroupMeta()
	}
	return nil
}

func (s *QuotesService) IndicesUpdatedAt(ctx context.Context) time.Time {
	if s.indexStore != nil {
		_, at := s.indexStore.List(ctx)
		return at
	}
	return time.Time{}
}
