package service

import (
	"context"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// LLMSettingsStore caches the AI 요약 pilot symbol list, re-reading the DB
// on every Get (single-row table, cheap) so admin edits via /admin/llm take
// effect immediately for the next request — no restart, no cache TTL.
type LLMSettingsStore struct {
	repo   *repository.LLMSettingsRepo
	mu     sync.RWMutex
	cached model.LLMSettings
}

func NewLLMSettingsStore(repo *repository.LLMSettingsRepo) *LLMSettingsStore {
	st := &LLMSettingsStore{repo: repo, cached: model.DefaultLLMSettings()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s, err := st.load(ctx); err == nil {
		st.cached = s
	}
	return st
}

func (st *LLMSettingsStore) load(ctx context.Context) (model.LLMSettings, error) {
	if st.repo == nil {
		return model.DefaultLLMSettings(), nil
	}
	return st.repo.Get(ctx)
}

func (st *LLMSettingsStore) Get(ctx context.Context) model.LLMSettings {
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

// PilotSymbols is the always-include symbol list each batch run prepends to
// its ranking-derived targets.
func (st *LLMSettingsStore) PilotSymbols(ctx context.Context) []string {
	return st.Get(ctx).PilotSymbols
}

func (st *LLMSettingsStore) Update(ctx context.Context, in model.LLMSettings) (model.LLMSettings, error) {
	if st.repo == nil {
		return model.LLMSettings{}, nil
	}
	updated, err := st.repo.Update(ctx, in)
	if err != nil {
		return model.LLMSettings{}, err
	}
	st.mu.Lock()
	st.cached = updated
	st.mu.Unlock()
	return updated, nil
}
