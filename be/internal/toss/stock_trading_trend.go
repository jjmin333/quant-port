package toss

import (
	"context"
	"net/url"
	"strconv"
)

/*
종목별 매매동향 — `STOCK_TRADING_TREND` 버킷(2026-09-24 수급 축).

`/stocks/{symbol}/investor-trading`은 경로가 `/stocks`로 시작하지만 **종목 정보(`STOCK`)와
다른 버킷**이다. 접두사로 묶으면 여기서 난 429가 경고 스윕·종목 정보의 failover를
보조 키로 넘긴다(`groups.go`).

수량은 전부 주식 수(정수 문자열)다 — 거래대금 축은 없다.
*/

// TradingVolume is 매수·매도·순매수 주식 수 for one investor class.
type TradingVolume struct {
	BuyVolume    string `json:"buyVolume"`
	SellVolume   string `json:"sellVolume"`
	NetBuyVolume string `json:"netBuyVolume"`
}

// ForeignerHolding is 외국인 보유 현황 for one day.
type ForeignerHolding struct {
	HoldingQuantity string `json:"holdingQuantity"`
	LimitQuantity   string `json:"limitQuantity"`
	HoldingRate     string `json:"holdingRate"` // 0.5089 = 50.89%
}

/*
StockInvestorTradingRecord — 하루치 기록.

`Individual`이 nil이면 **장중 잠정치**다(개인·기타법인·기관 세부·외국인 보유는 확정치가
들어오는 저녁부터 채워진다). 적재는 이 값으로 확정 여부를 가른다.
*/
type StockInvestorTradingRecord struct {
	Date             string            `json:"date"`
	UpdatedAt        string            `json:"updatedAt"`
	Individual       *TradingVolume    `json:"individual"`
	Foreigner        TradingVolume     `json:"foreigner"`
	Institution      TradingVolume     `json:"institution"`
	OtherCorporation *TradingVolume    `json:"otherCorporation"`
	ForeignerHolding *ForeignerHolding `json:"foreignerHolding"`
}

type StockInvestorTradingResponse struct {
	NextUntil *string                      `json:"nextUntil"`
	Records   []StockInvestorTradingRecord `json:"records"`
}

// GetStockInvestorTrading returns up to count (max 100) daily records, newest first.
// symbol is the 6-digit KRX code. Non-KR symbols answer 400 unsupported-market.
func (c *Client) GetStockInvestorTrading(ctx context.Context, symbol string, count int, until string) (*StockInvestorTradingResponse, error) {
	q := url.Values{}
	if count > 0 {
		q.Set("count", strconv.Itoa(count))
	}
	if until != "" {
		q.Set("until", until)
	}
	var out Envelope[StockInvestorTradingResponse]
	path := "/api/v1/stocks/" + url.PathEscape(symbol) + "/investor-trading"
	if err := c.get(ctx, path, q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Result, nil
}
