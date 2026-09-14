package toss

import (
	"context"
	"net/url"
	"strings"
)

// --- Stock Info API ---

func (c *Client) GetStocks(ctx context.Context, symbols []string) ([]StockInfo, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	q := url.Values{"symbols": {strings.Join(symbols, ",")}}
	var out Envelope[[]StockInfo]
	if err := c.get(ctx, "/api/v1/stocks", q, nil, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func (c *Client) GetStock(ctx context.Context, symbol string) (*StockInfo, error) {
	items, err := c.GetStocks(ctx, []string{symbol})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (c *Client) GetStockWarnings(ctx context.Context, symbol string) ([]StockWarning, error) {
	var out Envelope[[]StockWarning]
	if err := c.get(ctx, "/api/v1/stocks/"+url.PathEscape(symbol)+"/warnings", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

// ListedStock is one row of the tradable universe from /api/v1/stocks/all.
//
// `Name`은 **한국어 종목명**이고 주식 종류까지 들어 있다(알파벳 A · 알파벳 C ·
// 버크셔 해서웨이 A). 뉴스 검색의 별칭이 이 값에서 나온다.
type ListedStock struct {
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	SecurityType  string `json:"securityType"`
	IsCommonShare bool   `json:"isCommonShare"`
	IsinCode      string `json:"isinCode"`
}

// TossMarket는 /stocks/all 의 market 파라미터가 받는 값이다.
type TossMarket string

const (
	TossMarketKOSPI  TossMarket = "KOSPI"
	TossMarketKOSDAQ TossMarket = "KOSDAQ"
	TossMarketNYSE   TossMarket = "NYSE"
	TossMarketNASDAQ TossMarket = "NASDAQ"
	TossMarketAMEX   TossMarket = "AMEX"
	TossMarketKrEtc  TossMarket = "KR_ETC"
	TossMarketUsEtc  TossMarket = "US_ETC"
)

// AllTossMarkets는 유니버스 한 바퀴가 도는 마켓 전부다. 일곱 호출이 한 바퀴다.
func AllTossMarkets() []TossMarket {
	return []TossMarket{
		TossMarketKOSPI, TossMarketKOSDAQ,
		TossMarketNYSE, TossMarketNASDAQ, TossMarketAMEX,
		TossMarketKrEtc, TossMarketUsEtc,
	}
}

/*
GetListedStocks — 한 마켓에서 **토스가 거래를 지원하는 종목 전부**.

# 왜 이 엔드포인트가 특별한가

우리 카탈로그(`stocks`, status=NORMAL)는 토스가 취급하는 집합의 **상위집합**이다.
그 차이를 예전에는 종목마다 404를 한 번씩 맞아 가며 배웠다 — 실측으로 경고 스윕의
19%(3,388건)와 해외 종가 구멍 메우기의 회차당 3,250건이 그 404였다.

이 호출은 그 집합을 **한 번에** 준다. 스펙이 「마켓당 최대 수천 건 · gzip 30KB ·
하루 1회 조회 후 로컬 캐싱 권장」이라고 적고 있고, 마켓이 일곱이니 **하루 7호출**이다.

페이지네이션이 없다 — 필터에 맞는 종목을 한 응답에 다 담아 준다.
*/
func (c *Client) GetListedStocks(ctx context.Context, market TossMarket) ([]ListedStock, error) {
	q := url.Values{
		"market": {string(market)},
		// ACTIVE 가 기본값이지만 명시한다 — 이 호출의 뜻은 「지금 거래 가능한 것」이고,
		// 기본값이 바뀌면 조용히 다른 집합을 받게 된다.
		"status": {"ACTIVE"},
	}
	var out Envelope[[]ListedStock]
	if err := c.get(ctx, "/api/v1/stocks/all", q, nil, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}
