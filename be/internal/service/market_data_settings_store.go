package service

import (
	"context"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

type MarketDataSettingsStore struct {
	repo   *repository.MarketDataSettingsRepo
	mu     sync.RWMutex
	cached model.MarketDataSettings
}

func NewMarketDataSettingsStore(repo *repository.MarketDataSettingsRepo) *MarketDataSettingsStore {
	st := &MarketDataSettingsStore{
		repo:   repo,
		cached: model.DefaultMarketDataSettings(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s, err := st.load(ctx); err == nil {
		st.cached = s
	}
	return st
}

func (st *MarketDataSettingsStore) load(ctx context.Context) (model.MarketDataSettings, error) {
	if st.repo == nil {
		return model.DefaultMarketDataSettings(), nil
	}
	return st.repo.Get(ctx)
}

func (st *MarketDataSettingsStore) Get(ctx context.Context) model.MarketDataSettings {
	st.mu.RLock()
	cached := st.cached
	st.mu.RUnlock()

	if st.repo == nil {
		return cached
	}
	s, err := st.load(ctx)
	if err != nil {
		return cached
	}
	st.mu.Lock()
	st.cached = s
	st.mu.Unlock()
	return s
}

func (st *MarketDataSettingsStore) Update(ctx context.Context, input model.MarketDataSettings) (model.MarketDataSettings, error) {
	if st.repo == nil {
		return model.MarketDataSettings{}, nil
	}
	updated, err := st.repo.Update(ctx, input)
	if err != nil {
		return model.MarketDataSettings{}, err
	}
	st.mu.Lock()
	st.cached = updated
	st.mu.Unlock()
	return updated, nil
}

func (st *MarketDataSettingsStore) RankingsPollInterval(d toss.RankingDuration) time.Duration {
	s := st.Get(context.Background())
	switch d {
	case toss.RankingRealtime:
		return time.Duration(s.RankingsPollRealtimeMinutes) * time.Minute
	case toss.Ranking1d, toss.Ranking1w:
		return time.Duration(s.RankingsPollShortMinutes) * time.Minute
	default:
		return time.Duration(s.RankingsPollLongMinutes) * time.Minute
	}
}
