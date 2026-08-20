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
	repo   *repository.WatchlistRepo
	quotes *QuotesService
}

func NewWatchlistService(repo *repository.WatchlistRepo) *WatchlistService {
	return &WatchlistService{repo: repo}
}

func (s *WatchlistService) SetQuotes(quotes *QuotesService) {
	s.quotes = quotes
}

func (s *WatchlistService) List(ctx context.Context) ([]model.WatchlistItem, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := s.repo.ListEntries(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]model.WatchlistItem, 0, len(entries))
	for _, entry := range entries {
		item := model.WatchlistItem{
			ID:        entry.ID,
			Symbol:    entry.Symbol,
			CreatedAt: entry.CreatedAt.UTC().Format(time.RFC3339),
		}
		if entry.AddedPrice.Valid && entry.AddedPrice.Float64 > 0 {
			item.AddedPrice = entry.AddedPrice.Float64
		}
		out = append(out, item)
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
	if err := s.repo.Add(ctx, userID, symbol); err != nil {
		return err
	}
	if s.quotes != nil {
		if price, ok := s.quotes.CurrentPriceForSymbol(ctx, symbol); ok {
			_ = s.repo.SetAddedPrice(ctx, userID, symbol, price)
		}
		s.quotes.InvalidateQuotesListCache(ctx, "watchlist")
	}
	return nil
}

func (s *WatchlistService) Remove(ctx context.Context, symbol string) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if err := s.repo.Remove(ctx, userID, symbol); err != nil {
		return err
	}
	if s.quotes != nil {
		s.quotes.InvalidateQuotesListCache(ctx, "watchlist")
	}
	return nil
}

func (s *WatchlistService) BulkAdd(ctx context.Context, symbols []string) (int, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return 0, err
	}
	added := 0
	for _, sym := range symbols {
		sym = strings.TrimSpace(strings.ToUpper(sym))
		if sym == "" {
			continue
		}
		if err := s.repo.Add(ctx, userID, sym); err == nil {
			if s.quotes != nil {
				if price, ok := s.quotes.CurrentPriceForSymbol(ctx, sym); ok {
					_ = s.repo.SetAddedPrice(ctx, userID, sym, price)
				}
			}
			added++
		}
	}
	if added > 0 && s.quotes != nil {
		s.quotes.InvalidateQuotesListCache(ctx, "watchlist")
	}
	return added, nil
}

func (s *WatchlistService) Reorder(ctx context.Context, symbols []string) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	normalized := make([]string, 0, len(symbols))
	seen := map[string]struct{}{}
	for _, sym := range symbols {
		sym = strings.TrimSpace(strings.ToUpper(sym))
		if sym == "" {
			continue
		}
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		normalized = append(normalized, sym)
	}
	if len(normalized) == 0 {
		return fmt.Errorf("symbols required")
	}
	if err := s.repo.Reorder(ctx, userID, normalized); err != nil {
		return err
	}
	if s.quotes != nil {
		s.quotes.InvalidateQuotesListCache(ctx, "watchlist")
	}
	return nil
}
