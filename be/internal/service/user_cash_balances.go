package service

import (
	"context"
	"fmt"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

type UserCashBalancesService struct {
	repo *repository.UserCashBalancesRepo
}

func NewUserCashBalancesService(repo *repository.UserCashBalancesRepo) *UserCashBalancesService {
	return &UserCashBalancesService{repo: repo}
}

func (s *UserCashBalancesService) Get(ctx context.Context) (model.UserCashBalances, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.UserCashBalances{}, err
	}
	return s.repo.Get(ctx, userID)
}

func (s *UserCashBalancesService) Replace(ctx context.Context, balances model.UserCashBalances) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	if balances.KRW < 0 || balances.USD < 0 {
		return fmt.Errorf("cash amount must be >= 0")
	}
	return s.repo.Upsert(ctx, userID, balances)
}
