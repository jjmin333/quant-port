package toss

import (
	"context"
	"net/url"
	"strconv"
)

// --- Order API ---

func (c *Client) CreateOrder(ctx context.Context, accountSeq int64, req OrderCreateRequest) (*OrderResponse, error) {
	var out Envelope[OrderResponse]
	if err := c.postJSON(ctx, "/api/v1/orders", req, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) ModifyOrder(ctx context.Context, accountSeq int64, orderID string, req OrderModifyRequest) (*OrderOperationResponse, error) {
	var out Envelope[OrderOperationResponse]
	path := "/api/v1/orders/" + url.PathEscape(orderID) + "/modify"
	if err := c.postJSON(ctx, path, req, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) CancelOrder(ctx context.Context, accountSeq int64, orderID string) (*OrderOperationResponse, error) {
	var out Envelope[OrderOperationResponse]
	path := "/api/v1/orders/" + url.PathEscape(orderID) + "/cancel"
	if err := c.postJSON(ctx, path, map[string]any{}, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

// --- Order History API ---

type GetOrdersParams struct {
	Status OrderLifecycleStatus
	Symbol string
	From   string
	To     string
	Cursor string
	Limit  int
}

func (c *Client) GetOrders(ctx context.Context, accountSeq int64, p GetOrdersParams) (*PaginatedOrderResponse, error) {
	q := url.Values{"status": {string(p.Status)}}
	if p.Symbol != "" {
		q.Set("symbol", p.Symbol)
	}
	if p.From != "" {
		q.Set("from", p.From)
	}
	if p.To != "" {
		q.Set("to", p.To)
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	var out Envelope[PaginatedOrderResponse]
	if err := c.get(ctx, "/api/v1/orders", q, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) GetOrder(ctx context.Context, accountSeq int64, orderID string) (*Order, error) {
	var out Envelope[Order]
	path := "/api/v1/orders/" + url.PathEscape(orderID)
	if err := c.get(ctx, path, nil, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

// --- Order Info API ---

func (c *Client) GetBuyingPower(ctx context.Context, accountSeq int64, currency Currency) (*BuyingPowerResponse, error) {
	q := url.Values{"currency": {string(currency)}}
	var out Envelope[BuyingPowerResponse]
	if err := c.get(ctx, "/api/v1/buying-power", q, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) GetSellableQuantity(ctx context.Context, accountSeq int64, symbol string) (*SellableQuantityResponse, error) {
	q := url.Values{"symbol": {symbol}}
	var out Envelope[SellableQuantityResponse]
	if err := c.get(ctx, "/api/v1/sellable-quantity", q, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) GetCommissions(ctx context.Context, accountSeq int64) ([]Commission, error) {
	var out Envelope[[]Commission]
	if err := c.get(ctx, "/api/v1/commissions", nil, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}
