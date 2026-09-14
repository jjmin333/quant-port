package toss

import "strings"

// Toss rate-limit groups from docs/toss-openapi.json. Failover is per group:
// a candles 429 must not send prices to secondary, and vice versa.
const (
	groupMarketData      = "MARKET_DATA"
	groupMarketDataChart = "MARKET_DATA_CHART"
	groupStock = "STOCK"
	/*
		groupStockAll — /api/v1/stocks/all 전용 버킷.

		STOCK(종목 상세·경고)과 **다른 버킷**이라는 사실이 이 엔드포인트의 값이다.
		경고 스윕이 STOCK 버킷을 하루 14,500회 쓰는데(관측 한도 5/s), 유니버스 조회를
		같은 버킷에 두면 그 스윕과 다툰다. 스펙이 갈라 놓은 것을 우리도 갈라 센다.
	*/
	groupStockAll = "STOCK_ALL"
	groupMarketInfo      = "MARKET_INFO"
	groupMarketIndicator = "MARKET_INDICATOR"
	groupRanking         = "RANKING"
	groupOther           = "OTHER"
)

/*
metricPath — 지표 라벨용으로 경로의 **가변 구간을 지운다**.

그룹(`rateLimitGroup`)은 한도를 보는 눈이고, 이쪽은 「어느 엔드포인트가 호출을
쓰는가」를 보는 눈이다. 최적화 대상은 항상 후자로 드러난다 — `MARKET_DATA`가
늘었다는 것만으로는 `/orderbook`인지 `/trades`인지 `/prices`인지 알 수 없다.

심볼이 라벨에 들어가면 카디널리티가 종목 수만큼 폭발하므로 `{symbol}`로 접는다.
*/
func metricPath(path string) string {
	p := path
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	const stocks = "/api/v1/stocks/"
	if strings.HasPrefix(p, stocks) {
		rest := p[len(stocks):]
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			return stocks + "{symbol}" + rest[i:]
		}
		return stocks + "{symbol}"
	}
	const indicators = "/api/v1/market-indicators/"
	if strings.HasPrefix(p, indicators) && p != indicators+"prices" {
		rest := p[len(indicators):]
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			return indicators + "{symbol}" + rest[i:]
		}
		return indicators + "{symbol}"
	}
	return p
}

func rateLimitGroup(path string) string {
	p := path
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	switch {
	case p == "/api/v1/candles" || strings.HasPrefix(p, "/api/v1/candles/"):
		return groupMarketDataChart
	case strings.HasPrefix(p, "/api/v1/orderbook"),
		strings.HasPrefix(p, "/api/v1/prices"),
		strings.HasPrefix(p, "/api/v1/trades"),
		strings.HasPrefix(p, "/api/v1/price-limits"):
		return groupMarketData
	// /stocks/all 은 /stocks 보다 **먼저** 봐야 한다 — 접두사가 겹친다.
	case p == "/api/v1/stocks/all":
		return groupStockAll
	case strings.HasPrefix(p, "/api/v1/stocks"):
		return groupStock
	case strings.HasPrefix(p, "/api/v1/rankings"):
		return groupRanking
	case strings.HasPrefix(p, "/api/v1/exchange-rate"),
		strings.HasPrefix(p, "/api/v1/market-calendar"):
		return groupMarketInfo
	case strings.HasPrefix(p, "/api/v1/market-indicators"):
		return groupMarketIndicator
	default:
		return groupOther
	}
}
