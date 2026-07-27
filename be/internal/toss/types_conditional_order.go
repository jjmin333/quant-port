package toss

type ConditionalOrderType string

const (
	ConditionalOrderSingle ConditionalOrderType = "SINGLE"
	ConditionalOrderOCO    ConditionalOrderType = "OCO"
	ConditionalOrderOTO    ConditionalOrderType = "OTO"
)

type ConditionalOrderStatus string

const (
	ConditionalStatusWatching  ConditionalOrderStatus = "WATCHING"
	ConditionalStatusPaused    ConditionalOrderStatus = "PAUSED"
	ConditionalStatusOrdering  ConditionalOrderStatus = "ORDERING"
	ConditionalStatusOrdered   ConditionalOrderStatus = "ORDERED"
	ConditionalStatusCompleted ConditionalOrderStatus = "COMPLETED"
	ConditionalStatusExpired   ConditionalOrderStatus = "EXPIRED"
)

type ConditionalOrderLifecycleStatus string

const (
	ConditionalLifecycleOpen   ConditionalOrderLifecycleStatus = "OPEN"
	ConditionalLifecycleClosed ConditionalOrderLifecycleStatus = "CLOSED"
)

type ConditionType string

const (
	ConditionStop       ConditionType = "STOP"
	ConditionProfitRate ConditionType = "PROFIT_RATE"
)

type ConditionLegStatus string

const (
	LegStatusWatching  ConditionLegStatus = "WATCHING"
	LegStatusHolding   ConditionLegStatus = "HOLDING"
	LegStatusPaused    ConditionLegStatus = "PAUSED"
	LegStatusOrdering  ConditionLegStatus = "ORDERING"
	LegStatusOrdered   ConditionLegStatus = "ORDERED"
	LegStatusCompleted ConditionLegStatus = "COMPLETED"
	LegStatusExpired   ConditionLegStatus = "EXPIRED"
	LegStatusCanceled  ConditionLegStatus = "CANCELED"
)

type ConditionRequest struct {
	OrderSide    OrderSide `json:"orderSide"`
	TriggerPrice string    `json:"triggerPrice"`
	OrderPrice   *string   `json:"orderPrice,omitempty"`
}

type ConditionalOrderCreateRequest struct {
	Symbol                string               `json:"symbol"`
	Type                  ConditionalOrderType `json:"type"`
	Quantity              string               `json:"quantity"`
	OrderType             OrderType            `json:"orderType"`
	ClientOrderID         *string              `json:"clientOrderId,omitempty"`
	ExpireDate            string               `json:"expireDate"`
	First                 ConditionRequest     `json:"first"`
	Second                *ConditionRequest    `json:"second,omitempty"`
	ConfirmHighValueOrder *bool                `json:"confirmHighValueOrder,omitempty"`
}

type ConditionalOrderModifyRequest struct {
	Type                  ConditionalOrderType `json:"type"`
	Quantity              string               `json:"quantity"`
	OrderType             OrderType            `json:"orderType"`
	ExpireDate            string               `json:"expireDate"`
	First                 ConditionRequest     `json:"first"`
	Second                *ConditionRequest    `json:"second,omitempty"`
	ConfirmHighValueOrder *bool                `json:"confirmHighValueOrder,omitempty"`
}

type ConditionalOrderCreateResponse struct {
	ConditionalOrderID string  `json:"conditionalOrderId"`
	ClientOrderID      *string `json:"clientOrderId"`
}

type ConditionalOrderCondition struct {
	Type               ConditionType      `json:"type"`
	Status             ConditionLegStatus `json:"status"`
	TriggerPrice       *string            `json:"triggerPrice"`
	TargetProfitRate   *string            `json:"targetProfitRate"`
	OrderPrice         *string            `json:"orderPrice"`
	TriggeredOrderID   *string            `json:"triggeredOrderId"`
}

type ConditionalOrderDetail struct {
	ConditionalOrderID string                 `json:"conditionalOrderId"`
	Type               ConditionalOrderType   `json:"type"`
	Status             ConditionalOrderStatus `json:"status"`
	Symbol             string                 `json:"symbol"`
	Market             MarketCountry          `json:"market"`
	Quantity           string                 `json:"quantity"`
	OrderType          OrderType              `json:"orderType"`
	ExpireDate         *string                `json:"expireDate"`
	First              ConditionalOrderCondition `json:"first"`
	Second             *ConditionalOrderCondition `json:"second"`
	CreatedAt          string                 `json:"createdAt"`
}

type PaginatedConditionalOrderResponse struct {
	ConditionalOrders []ConditionalOrderDetail `json:"conditionalOrders"`
	NextCursor        *string                  `json:"nextCursor"`
	HasNext           bool                     `json:"hasNext"`
}

type ConditionalOrderResponse struct {
	ConditionalOrderID string `json:"conditionalOrderId"`
}
