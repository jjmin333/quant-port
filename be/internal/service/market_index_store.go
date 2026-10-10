package service

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

const (
	marketIndexPollInterval = 1 * time.Minute
	tossIndexMinRefresh     = 1 * time.Minute
)

type MarketIndexStore struct {
	toss        *toss.Client
	external    *marketdata.Clients
	fx          *ExchangeRateService
	redis       *redis.Client
	repo        *repository.MarketIndexRepo
	marketStore *MarketDataStore

	// 실시간 시세가 스파크라인·등락률을 안 주는 지수를 메우는 종가 표.
	// 자세한 사정은 market_index_history.go.
	closes *repository.IndexDailyCloseRepo
	// 한국 금리 지수(ecos)가 읽는 표 — worker-asset이 채운다.
	rates *repository.PublicDataRepo
	histMu sync.Mutex
	hist   map[string][]repository.DailyClosePoint
	histAt time.Time

	mu        sync.RWMutex
	indices   []model.MarketIndex
	updatedAt time.Time
	// checkedAt — 마지막으로 갱신 회차를 끝낸 시각. updatedAt은 **값이 바뀐** 시각이라
	// 주말·밤에는 몇 분씩 멈춘다(비트코인·금만 움직인다). 「낡은 값」 판정은 이쪽을 본다.
	checkedAt    time.Time
	lastRefresh  map[string]time.Time
	groupMu      sync.RWMutex
	groupRefresh map[string]time.Time
	groupMeta    map[string]model.IndexGroupMeta
}

func NewMarketIndexStore(
	tossClient *toss.Client,
	external *marketdata.Clients,
	fx *ExchangeRateService,
	redisClient *redis.Client,
	repo *repository.MarketIndexRepo,
) *MarketIndexStore {
	return &MarketIndexStore{
		toss:         tossClient,
		external:     external,
		fx:           fx,
		redis:        redisClient,
		repo:         repo,
		lastRefresh:  make(map[string]time.Time),
		groupRefresh: make(map[string]time.Time),
		groupMeta:    make(map[string]model.IndexGroupMeta),
	}
}

// CheckedAt is when the refresh loop last finished a cycle (zero before the first).
func (st *MarketIndexStore) CheckedAt() time.Time {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.checkedAt
}

func (st *MarketIndexStore) SetMarketStore(store *MarketDataStore) {
	st.marketStore = store
}

func (st *MarketIndexStore) Run(ctx context.Context) {
	st.loadFromStorage(ctx)
	log.Println("market indices: warmup started")
	st.refreshAll(ctx, true)
	log.Println("market indices: warmup complete")

	ticker := time.NewTicker(marketIndexPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			st.refreshAll(ctx, false)
		}
	}
}

func (st *MarketIndexStore) List(ctx context.Context) ([]model.MarketIndex, time.Time) {
	st.mu.RLock()
	snap := st.indices
	at := st.updatedAt
	st.mu.RUnlock()

	if len(snap) == 0 {
		snap, at = st.loadSnapshotFromRedis(ctx)
	}
	if len(snap) == 0 {
		snap, at = st.loadFromDB(ctx)
	}
	if len(snap) == 0 {
		return emptyIndicesFromCatalog(), time.Time{}
	}
	return st.enrichIndicesForAPI(ctx, st.mergeZeros(ctx, alignIndicesToCatalog(snap))), at
}

func (st *MarketIndexStore) loadFromStorage(ctx context.Context) {
	if snap, at := st.loadSnapshotFromRedis(ctx); len(snap) > 0 {
		st.mu.Lock()
		st.indices = snap
		st.updatedAt = at
		st.mu.Unlock()
		for _, idx := range snap {
			if idx.Value > 0 {
				st.noteRefresh(idx.ID, at)
			}
		}
		return
	}
	if snap, at := st.loadFromDB(ctx); len(snap) > 0 {
		st.mu.Lock()
		st.indices = snap
		st.updatedAt = at
		st.mu.Unlock()
		st.persistSnapshotRedis(ctx, snap, at)
		for _, idx := range snap {
			if idx.Value > 0 {
				st.noteRefresh(idx.ID, idxUpdatedAt(ctx, st, idx.ID, at))
			}
		}
	}
}

func idxUpdatedAt(ctx context.Context, st *MarketIndexStore, id string, fallback time.Time) time.Time {
	if st.repo != nil {
		rec, err := st.repo.Get(ctx, id)
		if err == nil && rec != nil && !rec.UpdatedAt.IsZero() {
			return rec.UpdatedAt
		}
	}
	return fallback
}

func indicesSnapshotEqual(a, b []model.MarketIndex) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Value != b[i].Value ||
			a[i].Change != b[i].Change ||
			a[i].ChangeRate != b[i].ChangeRate ||
			len(a[i].Sparkline) != len(b[i].Sparkline) {
			return false
		}
	}
	return true
}

func (st *MarketIndexStore) lastRefreshAt(ctx context.Context, id string) time.Time {
	st.mu.RLock()
	if t, ok := st.lastRefresh[id]; ok {
		st.mu.RUnlock()
		return t
	}
	st.mu.RUnlock()

	var cached cachedMarketIndex
	if st.cacheGetJSON(ctx, indexCacheKey(id), &cached) && !cached.UpdatedAt.IsZero() {
		return cached.UpdatedAt
	}
	if st.repo != nil {
		rec, err := st.repo.Get(ctx, id)
		if err == nil && rec != nil {
			return rec.UpdatedAt
		}
	}
	return time.Time{}
}

func (st *MarketIndexStore) noteRefresh(id string, at time.Time) {
	st.mu.Lock()
	st.lastRefresh[id] = at
	st.mu.Unlock()
}

func (st *MarketIndexStore) snapshotOrCatalog() []model.MarketIndex {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if len(st.indices) > 0 {
		// 카탈로그 위치로 덮어쓰므로 길이·순서를 카탈로그에 맞춘다 — Redis에서 되살린 옛 스냅샷(지수 13종)이
		// 그대로 오면 새 지수(spy·dia)의 칸이 없어 패닉했다(2026-10-01 배포 직후 재시작 루프).
		return alignIndicesToCatalog(st.indices)
	}
	return emptyIndicesFromCatalog()
}

func (st *MarketIndexStore) persistOne(ctx context.Context, entry indexCatalogEntry, idx model.MarketIndex, updatedAt time.Time) {
	st.cacheGoodIndexAt(ctx, idx, updatedAt)

	if st.repo != nil {
		if err := st.repo.Upsert(ctx, repository.MarketIndexRecord{
			IndexID:    idx.ID,
			Name:       idx.Name,
			Currency:   string(idx.Currency),
			Value:      idx.Value,
			Change:     idx.Change,
			ChangeRate: idx.ChangeRate,
			Badge:      idx.Badge,
			Sparkline:  idx.Sparkline,
			Source:     string(entry.source),
		}); err != nil {
			log.Printf("market index store: upsert %s: %v", idx.ID, err)
		}
	}
}

func (st *MarketIndexStore) persistSnapshotRedis(ctx context.Context, indices []model.MarketIndex, at time.Time) {
	st.cacheSetJSON(ctx, quotesIndicesSnapshotKey, indicesSnapshotPayload{
		Indices:   indices,
		UpdatedAt: at,
	}, indicesSnapshotTTL)
}

func (st *MarketIndexStore) loadSnapshotFromRedis(ctx context.Context) ([]model.MarketIndex, time.Time) {
	var payload indicesSnapshotPayload
	if !st.cacheGetJSON(ctx, quotesIndicesSnapshotKey, &payload) || len(payload.Indices) == 0 {
		return nil, time.Time{}
	}
	out := make([]model.MarketIndex, len(payload.Indices))
	copy(out, payload.Indices)
	return out, payload.UpdatedAt
}

func (st *MarketIndexStore) loadFromDB(ctx context.Context) ([]model.MarketIndex, time.Time) {
	if st.repo == nil {
		return nil, time.Time{}
	}
	rows, err := st.repo.ListAll(ctx)
	if err != nil || len(rows) == 0 {
		return nil, time.Time{}
	}
	byID := map[string]model.MarketIndex{}
	var latest time.Time
	for _, rec := range rows {
		if rec.Value <= 0 {
			continue
		}
		idx := model.MarketIndex{
			ID:         rec.IndexID,
			Name:       rec.Name,
			Value:      rec.Value,
			Change:     rec.Change,
			ChangeRate: rec.ChangeRate,
			Currency:   model.QuoteCurrency(rec.Currency),
			Badge:      rec.Badge,
			Sparkline:  rec.Sparkline,
			UpdatedAt:  rec.UpdatedAt.UTC().Format(time.RFC3339),
			DataKind:   dataKindForIndex(rec.IndexID),
		}
		byID[rec.IndexID] = idx
		if rec.UpdatedAt.After(latest) {
			latest = rec.UpdatedAt
		}
	}
	if len(byID) == 0 {
		return nil, time.Time{}
	}
	out := emptyIndicesFromCatalog()
	for i, entry := range marketIndexCatalog {
		if idx, ok := byID[entry.id]; ok {
			out[i] = idx
		}
	}
	return out, latest
}

func (st *MarketIndexStore) mergeZeros(ctx context.Context, indices []model.MarketIndex) []model.MarketIndex {
	out := make([]model.MarketIndex, len(indices))
	copy(out, indices)
	for i, idx := range out {
		if idx.Value > 0 {
			continue
		}
		entry := marketIndexCatalog[i]
		out[i] = st.resolveIndex(ctx, entry, idx)
	}
	return out
}

func (st *MarketIndexStore) cacheGetJSON(ctx context.Context, key string, dest any) bool {
	if st.redis == nil {
		return false
	}
	raw, err := st.redis.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dest) == nil
}

func (st *MarketIndexStore) cacheSetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if st.redis == nil {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = st.redis.Set(ctx, key, payload, ttl).Err()
}
