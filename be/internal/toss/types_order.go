package toss

type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeMarket OrderType = "MARKET"
)

type TimeInForce string

const (
	TimeInForceDay TimeInForce = "DAY"
	TimeInForceCLS TimeInForce = "CLS"
)

type OrderStatus string

const (
	OrderStatusPending         OrderStatus = "PENDING"
	OrderStatusPendingCancel   OrderStatus = "PENDING_CANCEL"
	OrderStatusPendingReplace  OrderStatus = "PENDING_REPLACE"
	OrderStatusPartialFilled   OrderStatus = "PARTIAL_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
	OrderStatusRejected        OrderStatus = "REJECTED"
	OrderStatusCancelRejected  OrderStatus = "CANCEL_REJECTED"
	OrderStatusReplaceRejected OrderStatus = "REPLACE_REJECTED"
	OrderStatusReplaced        OrderStatus = "REPLACED"
)

type OrderLifecycleStatus string

const (
	OrderLifecycleOpen   OrderLifecycleStatus = "OPEN"
	OrderLifecycleClosed OrderLifecycleStatus = "CLOSED"
)

type OrderCreateRequest struct {
	ClientOrderID         *string      `json:"clientOrderId,omitempty"`
	Symbol                string       `json:"symbol"`
	Side                  OrderSide    `json:"side"`
	OrderType             OrderType    `json:"orderType"`
	TimeInForce           *TimeInForce `json:"timeInForce,omitempty"`
	Quantity              *string      `json:"quantity,omitempty"`
	Price                 *string      `json:"price,omitempty"`
	OrderAmount           *string      `json:"orderAmount,omitempty"`
	ConfirmHighValueOrder *bool        `json:"confirmHighValueOrder,omitempty"`
}

type OrderModifyRequest struct {
	OrderType             OrderType `json:"orderType"`
	Quantity              *string   `json:"quantity,omitempty"`
	Price                 *string   `json:"price,omitempty"`
	ConfirmHighValueOrder *bool     `json:"confirmHighValueOrder,omitempty"`
}

type OrderExecution struct {
	FilledQuantity      string  `json:"filledQuantity"`
	AverageFilledPrice  *string `json:"averageFilledPrice"`
	FilledAmount        *string `json:"filledAmount"`
	Commission          *string `json:"commission"`
	Tax                 *string `json:"tax"`
	FilledAt            *string `json:"filledAt"`
	SettlementDate      *string `json:"settlementDate"`
}

type Order struct {
	OrderID     string         `json:"orderId"`
	Symbol      string         `json:"symbol"`
	Side        OrderSide      `json:"side"`
	OrderType   OrderType      `json:"orderType"`
	TimeInForce TimeInForce    `json:"timeInForce"`
	Status      OrderStatus    `json:"status"`
	Price       *string        `json:"price"`
	Quantity    string         `json:"quantity"`
	OrderAmount *string        `json:"orderAmount"`
	Currency    Currency       `json:"currency"`
	OrderedAt   string         `json:"orderedAt"`
	CanceledAt  *string        `json:"canceledAt"`
	Execution   OrderExecution `json:"execution"`
}

type OrderResponse struct {
	OrderID       string  `json:"orderId"`
	ClientOrderID *string `json:"clientOrderId"`
}

type OrderOperationResponse struct {
	OrderID string `json:"orderId"`
}

type PaginatedOrderResponse struct {
	Orders     []Order `json:"orders"`
	NextCursor *string `json:"nextCursor"`
	HasNext    bool    `json:"hasNext"`
}

type BuyingPowerResponse struct {
	Currency        Currency `json:"currency"`
	CashBuyingPower string   `json:"cashBuyingPower"`
}

type SellableQuantityResponse struct {
	SellableQuantity string `json:"sellableQuantity"`
}

type Commission struct {
	MarketCountry   MarketCountry `json:"marketCountry"`
	CommissionRate  string        `json:"commissionRate"`
	StartDate       *string       `json:"startDate"`
	EndDate         *string       `json:"endDate"`
}
