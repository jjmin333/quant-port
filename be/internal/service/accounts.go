package service

import (
	"context"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/toss"
)

type AccountsService struct {
	tossReg *TossRegistry
}

func NewAccountsService(tossReg *TossRegistry) *AccountsService {
	return &AccountsService{tossReg: tossReg}
}

func (s *AccountsService) List(ctx context.Context) ([]toss.Account, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	client, err := s.tossReg.ClientForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return client.GetAccounts(ctx)
}
