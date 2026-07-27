package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

type WatchlistService struct {
	repo *repository.WatchlistRepo
}

func NewWatchlistService(repo *repository.WatchlistRepo) *WatchlistService {
	return &WatchlistService{repo: repo}
}

func (s *WatchlistService) List(ctx context.Context) ([]model.WatchlistItem, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	symbols, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]model.WatchlistItem, 0, len(symbols))
	for i, sym := range symbols {
		out = append(out, model.WatchlistItem{
			ID:        int64(i + 1),
			Symbol:    sym,
			CreatedAt: time.Now().Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *WatchlistService) Symbols(ctx context.Context) ([]string, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, nil
	}
	return s.repo.List(ctx, userID)
}

func (s *WatchlistService) Add(ctx context.Context, symbol string) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if symbol == "" {
		return fmt.Errorf("symbol required")
	}
	return s.repo.Add(ctx, userID, symbol)
}

func (s *WatchlistService) Remove(ctx context.Context, symbol string) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return s.repo.Remove(ctx, userID, strings.TrimSpace(strings.ToUpper(symbol)))
}
