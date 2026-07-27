package service

import (
	"context"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/toss"
)

type OrdersService struct {
	tossReg *TossRegistry
}

func NewOrdersService(tossReg *TossRegistry) *OrdersService {
	return &OrdersService{tossReg: tossReg}
}

func (s *OrdersService) clientForCtx(ctx context.Context) (*toss.Client, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.tossReg.ClientForUser(ctx, userID)
}

func (s *OrdersService) Create(ctx context.Context, accountSeq int64, req toss.OrderCreateRequest) (*toss.OrderResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.CreateOrder(ctx, accountSeq, req)
}

func (s *OrdersService) Modify(ctx context.Context, accountSeq int64, orderID string, req toss.OrderModifyRequest) (*toss.OrderOperationResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.ModifyOrder(ctx, accountSeq, orderID, req)
}

func (s *OrdersService) Cancel(ctx context.Context, accountSeq int64, orderID string) (*toss.OrderOperationResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.CancelOrder(ctx, accountSeq, orderID)
}

func (s *OrdersService) List(ctx context.Context, accountSeq int64, p toss.GetOrdersParams) (*toss.PaginatedOrderResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetOrders(ctx, accountSeq, p)
}

func (s *OrdersService) Get(ctx context.Context, accountSeq int64, orderID string) (*toss.Order, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetOrder(ctx, accountSeq, orderID)
}

func (s *OrdersService) BuyingPower(ctx context.Context, accountSeq int64, currency toss.Currency) (*toss.BuyingPowerResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetBuyingPower(ctx, accountSeq, currency)
}

func (s *OrdersService) SellableQuantity(ctx context.Context, accountSeq int64, symbol string) (*toss.SellableQuantityResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetSellableQuantity(ctx, accountSeq, symbol)
}

func (s *OrdersService) Commissions(ctx context.Context, accountSeq int64) ([]toss.Commission, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetCommissions(ctx, accountSeq)
}

func (s *OrdersService) CreateConditional(ctx context.Context, accountSeq int64, req toss.ConditionalOrderCreateRequest) (*toss.ConditionalOrderCreateResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.CreateConditionalOrder(ctx, accountSeq, req)
}

func (s *OrdersService) ListConditional(ctx context.Context, accountSeq int64, p toss.GetConditionalOrdersParams) (*toss.PaginatedConditionalOrderResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetConditionalOrders(ctx, accountSeq, p)
}

func (s *OrdersService) GetConditional(ctx context.Context, accountSeq int64, id string) (*toss.ConditionalOrderDetail, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetConditionalOrder(ctx, accountSeq, id)
}

func (s *OrdersService) CancelConditional(ctx context.Context, accountSeq int64, id string) (*toss.ConditionalOrderResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.CancelConditionalOrder(ctx, accountSeq, id)
}

func (s *OrdersService) ModifyConditional(ctx context.Context, accountSeq int64, id string, req toss.ConditionalOrderModifyRequest) (*toss.ConditionalOrderResponse, error) {
	client, err := s.clientForCtx(ctx)
	if err != nil {
		return nil, err
	}
	return client.ModifyConditionalOrder(ctx, accountSeq, id, req)
}
