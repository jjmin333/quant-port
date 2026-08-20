package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/redis/go-redis/v9"
)

const userHoldingsCacheTTL = 24 * time.Hour

type UserHoldingsService struct {
	repo   *repository.UserHoldingsRepo
	redis  *redis.Client
	trades *repository.HoldingTradesRepo
}

func NewUserHoldingsService(repo *repository.UserHoldingsRepo, redisClient *redis.Client) *UserHoldingsService {
	return &UserHoldingsService{repo: repo, redis: redisClient}
}

// SetTradesRepo lets deleting a holding also drop its buy/sell history. Without
// it, re-adding the same symbol later would silently inherit the old cost basis
// and the valuation chart would show shares the user no longer claims to own.
func (s *UserHoldingsService) SetTradesRepo(repo *repository.HoldingTradesRepo) {
	s.trades = repo
}

func userHoldingsCacheKey(userID int64) string {
	return fmt.Sprintf("holdings:manual:v1:user:%d", userID)
}

func (s *UserHoldingsService) invalidate(ctx context.Context, userID int64) {
	if s.redis == nil {
		return
	}
	_ = s.redis.Del(ctx, userHoldingsCacheKey(userID)).Err()
}

func (s *UserHoldingsService) List(ctx context.Context) ([]model.UserHolding, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, userHoldingsCacheKey(userID)).Bytes(); err == nil && len(cached) > 0 {
			var rows []model.UserHolding
			if json.Unmarshal(cached, &rows) == nil {
				return rows, nil
			}
		}
	}
	rows, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.redis != nil {
		if payload, err := json.Marshal(rows); err == nil {
			_ = s.redis.Set(ctx, userHoldingsCacheKey(userID), payload, userHoldingsCacheTTL).Err()
		}
	}
	return rows, nil
}

func (s *UserHoldingsService) HeldSymbols(ctx context.Context) []string {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil
	}
	symbols, err := s.repo.Symbols(ctx, userID)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(symbols))
	seen := map[string]struct{}{}
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}

func (s *UserHoldingsService) ReplaceAll(ctx context.Context, rows []model.UserHolding) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.ReplaceAll(ctx, userID, rows); err != nil {
		return err
	}
	s.invalidate(ctx, userID)
	return nil
}

func (s *UserHoldingsService) UpsertMany(ctx context.Context, rows []model.UserHolding) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return s.UpsertForUser(ctx, userID, rows)
}

func (s *UserHoldingsService) DeleteSymbol(ctx context.Context, symbol string) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return s.DeleteSymbolForUser(ctx, userID, symbol)
}

// The *ForUser variants exist for callers that already resolved the user and
// need to write holdings as a side effect of their own operation — trade
// history derives quantity and average cost, and must land in the same place
// every other reader looks.

func (s *UserHoldingsService) UpsertForUser(ctx context.Context, userID int64, rows []model.UserHolding) error {
	if err := s.repo.UpsertMany(ctx, userID, rows); err != nil {
		return err
	}
	s.invalidate(ctx, userID)
	return nil
}

func (s *UserHoldingsService) DeleteSymbolForUser(ctx context.Context, userID int64, symbol string) error {
	if err := s.repo.DeleteSymbol(ctx, userID, symbol); err != nil {
		return err
	}
	if s.trades != nil {
		_ = s.trades.DeleteSymbol(ctx, userID, symbol)
	}
	s.invalidate(ctx, userID)
	return nil
}

// RemoveHoldingRowForUser drops only the holdings row and keeps the trade log.
// This is the "sold everything" case: the position is closed, but the history of
// how it was bought and sold is exactly what the realized-P&L view needs.
func (s *UserHoldingsService) RemoveHoldingRowForUser(ctx context.Context, userID int64, symbol string) error {
	if err := s.repo.DeleteSymbol(ctx, userID, symbol); err != nil {
		return err
	}
	s.invalidate(ctx, userID)
	return nil
}

func (s *UserHoldingsService) ListForUser(ctx context.Context, userID int64) ([]model.UserHolding, error) {
	return s.repo.List(ctx, userID)
}

func (s *UserHoldingsService) CommitImportRows(ctx context.Context, req model.ImportCommitRequest) (int, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return 0, err
	}

	var rows []model.UserHolding
	for _, row := range req.Rows {
		if !row.Selected {
			continue
		}
		sym := strings.TrimSpace(strings.ToUpper(row.Symbol))
		if sym == "" || row.Quantity <= 0 {
			continue
		}
		rows = append(rows, model.UserHolding{
			Symbol:       sym,
			Quantity:     row.Quantity,
			AvgPrice:     row.AvgPrice,
			CostCurrency: strings.TrimSpace(row.CostCurrency),
			Source:       "screenshot",
		})
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("no rows selected")
	}

	if req.ReplaceExisting {
		if err := s.repo.ReplaceAll(ctx, userID, rows); err != nil {
			return 0, err
		}
	} else {
		if err := s.repo.UpsertMany(ctx, userID, rows); err != nil {
			return 0, err
		}
	}
	s.invalidate(ctx, userID)
	return len(rows), nil
}
