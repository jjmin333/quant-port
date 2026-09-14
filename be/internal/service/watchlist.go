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
	stocks *repository.StockRepo
}

func NewWatchlistService(repo *repository.WatchlistRepo) *WatchlistService {
	return &WatchlistService{repo: repo}
}

func (s *WatchlistService) SetQuotes(quotes *QuotesService) {
	s.quotes = quotes
}

// SetCatalog 을 주면 없는 심볼을 관심 목록에 담지 않는다.
func (s *WatchlistService) SetCatalog(stocks *repository.StockRepo) {
	s.stocks = stocks
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

/*
Add puts a symbol on the watchlist.

`addedPrice` 가 0보다 크면 **그 값을 담은 가격으로 쓴다.** 화면의 「실행 취소」가
쓰는 통로다: 관심종목의 담은 가격은 「담은 뒤로 얼마나 움직였나」의 기준선이고,
그것이 이 기능의 존재 이유다. 실수로 지웠다가 다시 담을 때 오늘 시세로 덮어쓰면
기준선이 조용히 리셋된다(실측: 해제→추가 한 번에 257,500 → 258,000으로 바뀌었다).

0이면 예전처럼 현재가를 찍는다 — 처음 담는 경우다.
*/
func (s *WatchlistService) Add(ctx context.Context, symbol string, addedPrice float64) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if symbol == "" {
		return fmt.Errorf("symbol required")
	}
	if err := ensureSymbolInCatalog(ctx, s.stocks, symbol); err != nil {
		return err
	}
	if err := s.repo.Add(ctx, userID, symbol); err != nil {
		return err
	}
	if addedPrice > 0 {
		_ = s.repo.SetAddedPrice(ctx, userID, symbol, addedPrice)
		if s.quotes != nil {
			s.quotes.InvalidateQuotesListCache(ctx, "watchlist")
		}
		return nil
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
		// 일괄 담기는 한 종목 때문에 전체를 세우지 않는다 — 모르는 심볼은 건너뛰고
		// 담긴 개수로 답한다.
		if err := ensureSymbolInCatalog(ctx, s.stocks, sym); err != nil {
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
