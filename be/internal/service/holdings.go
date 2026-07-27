package service

import (
	"context"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/toss"
)

type HoldingsService struct {
	tossReg *TossRegistry
}

func NewHoldingsService(tossReg *TossRegistry) *HoldingsService {
	return &HoldingsService{tossReg: tossReg}
}

func (s *HoldingsService) Get(ctx context.Context, accountSeq int64, symbol string) (*toss.HoldingsOverview, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	client, err := s.tossReg.ClientForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return client.GetHoldings(ctx, accountSeq, symbol)
}
