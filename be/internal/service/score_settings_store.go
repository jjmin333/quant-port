package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// ScoreSettingsStore holds the admin-editable scoring weights.
//
// Same pattern as LLMSettingsStore: the tables are tiny and read once per
// batch or request, so it re-reads on access and admin edits take effect
// without a restart. The last good value is cached only as a fallback for a
// transient DB error.
type ScoreSettingsStore struct {
	repo *repository.ScoreSettingsRepo

	mu              sync.RWMutex
	cached          model.ScoreSettings
	cachedOverrides map[string]model.ScoreWeights
}

func NewScoreSettingsStore(repo *repository.ScoreSettingsRepo) *ScoreSettingsStore {
	st := &ScoreSettingsStore{
		repo:            repo,
		cached:          model.DefaultScoreSettings(),
		cachedOverrides: map[string]model.ScoreWeights{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s, err := st.repo.Get(ctx); err == nil {
		st.cached = s
	}
	if list, err := st.repo.ListSymbolWeights(ctx); err == nil {
		st.cachedOverrides = overrideMap(list)
	}
	return st
}

func overrideMap(list []model.SymbolScoreWeights) map[string]model.ScoreWeights {
	out := make(map[string]model.ScoreWeights, len(list))
	for _, it := range list {
		out[strings.ToUpper(strings.TrimSpace(it.Symbol))] = it.Weights
	}
	return out
}

func (st *ScoreSettingsStore) Get(ctx context.Context) model.ScoreSettings {
	if st == nil {
		return model.DefaultScoreSettings()
	}
	st.mu.RLock()
	cached := st.cached
	st.mu.RUnlock()
	if st.repo == nil {
		return cached
	}
	s, err := st.repo.Get(ctx)
	if err != nil {
		return cached
	}
	st.mu.Lock()
	st.cached = s
	st.mu.Unlock()
	return s
}

// SymbolOverrides returns every per-symbol weight override, keyed by symbol.
// The batch calls this once per run rather than per symbol.
func (st *ScoreSettingsStore) SymbolOverrides(ctx context.Context) map[string]model.ScoreWeights {
	if st == nil {
		return map[string]model.ScoreWeights{}
	}
	st.mu.RLock()
	cached := st.cachedOverrides
	st.mu.RUnlock()
	if st.repo == nil {
		return cached
	}
	list, err := st.repo.ListSymbolWeights(ctx)
	if err != nil {
		return cached
	}
	m := overrideMap(list)
	st.mu.Lock()
	st.cachedOverrides = m
	st.mu.Unlock()
	return m
}

func (st *ScoreSettingsStore) List(ctx context.Context) ([]model.SymbolScoreWeights, error) {
	if st == nil || st.repo == nil {
		return nil, nil
	}
	return st.repo.ListSymbolWeights(ctx)
}

func (st *ScoreSettingsStore) Update(ctx context.Context, s model.ScoreSettings) (model.ScoreSettings, error) {
	if st == nil || st.repo == nil {
		return model.DefaultScoreSettings(), nil
	}
	updated, err := st.repo.Update(ctx, s)
	if err != nil {
		return model.ScoreSettings{}, err
	}
	st.mu.Lock()
	st.cached = updated
	st.mu.Unlock()
	return updated, nil
}

func (st *ScoreSettingsStore) UpsertSymbol(ctx context.Context, it model.SymbolScoreWeights) error {
	if st == nil || st.repo == nil {
		return nil
	}
	if err := st.repo.UpsertSymbolWeights(ctx, it); err != nil {
		return err
	}
	st.refreshOverrides(ctx)
	return nil
}

func (st *ScoreSettingsStore) DeleteSymbol(ctx context.Context, symbol string) error {
	if st == nil || st.repo == nil {
		return nil
	}
	if err := st.repo.DeleteSymbolWeights(ctx, symbol); err != nil {
		return err
	}
	st.refreshOverrides(ctx)
	return nil
}

func (st *ScoreSettingsStore) refreshOverrides(ctx context.Context) {
	list, err := st.repo.ListSymbolWeights(ctx)
	if err != nil {
		return
	}
	m := overrideMap(list)
	st.mu.Lock()
	st.cachedOverrides = m
	st.mu.Unlock()
}
