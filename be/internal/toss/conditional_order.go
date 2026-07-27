package toss

import (
	"context"
	"net/url"
	"strconv"
)

// --- Conditional Order API ---

func (c *Client) CreateConditionalOrder(ctx context.Context, accountSeq int64, req ConditionalOrderCreateRequest) (*ConditionalOrderCreateResponse, error) {
	var out Envelope[ConditionalOrderCreateResponse]
	if err := c.postJSON(ctx, "/api/v1/conditional-orders", req, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

type GetConditionalOrdersParams struct {
	Status ConditionalOrderLifecycleStatus
	Symbol string
	Cursor string
	Limit  int
}

func (c *Client) GetConditionalOrders(ctx context.Context, accountSeq int64, p GetConditionalOrdersParams) (*PaginatedConditionalOrderResponse, error) {
	q := url.Values{"status": {string(p.Status)}}
	if p.Symbol != "" {
		q.Set("symbol", p.Symbol)
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	var out Envelope[PaginatedConditionalOrderResponse]
	if err := c.get(ctx, "/api/v1/conditional-orders", q, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) GetConditionalOrder(ctx context.Context, accountSeq int64, conditionalOrderID string) (*ConditionalOrderDetail, error) {
	var out Envelope[ConditionalOrderDetail]
	path := "/api/v1/conditional-orders/" + url.PathEscape(conditionalOrderID)
	if err := c.get(ctx, path, nil, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) CancelConditionalOrder(ctx context.Context, accountSeq int64, conditionalOrderID string) (*ConditionalOrderResponse, error) {
	var out Envelope[ConditionalOrderResponse]
	path := "/api/v1/conditional-orders/" + url.PathEscape(conditionalOrderID)
	if err := c.delete(ctx, path, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}

func (c *Client) ModifyConditionalOrder(ctx context.Context, accountSeq int64, conditionalOrderID string, req ConditionalOrderModifyRequest) (*ConditionalOrderResponse, error) {
	var out Envelope[ConditionalOrderResponse]
	path := "/api/v1/conditional-orders/" + url.PathEscape(conditionalOrderID) + "/modify"
	if err := c.postJSON(ctx, path, req, WithAccount(accountSeq), &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}
