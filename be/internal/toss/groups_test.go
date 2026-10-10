package toss

import "testing"

func TestMetricPathFoldsVariableSegments(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/api/v1/prices?symbols=005930", "/api/v1/prices"},
		{"/api/v1/orderbook", "/api/v1/orderbook"},
		{"/api/v1/trades?symbol=AAPL&size=50", "/api/v1/trades"},
		{"/api/v1/stocks", "/api/v1/stocks"},
		{"/api/v1/stocks/005930/warnings", "/api/v1/stocks/{symbol}/warnings"},
		{"/api/v1/stocks/AAPL", "/api/v1/stocks/{symbol}"},
		{"/api/v1/market-indicators/KOSPI/candles", "/api/v1/market-indicators/{symbol}/candles"},
		// prices는 심볼이 아니라 고정 엔드포인트다 — 접으면 안 된다.
		{"/api/v1/market-indicators/prices", "/api/v1/market-indicators/prices"},
		{"/api/v1/market-calendar/KR", "/api/v1/market-calendar/KR"},
	}
	for _, c := range cases {
		if got := metricPath(c.in); got != c.want {
			t.Errorf("metricPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 라벨 카디널리티가 심볼 수로 폭발하지 않는지 — 지표를 넣은 이유가 무의미해진다.
func TestMetricPathIsSymbolIndependent(t *testing.T) {
	a := metricPath("/api/v1/stocks/005930/warnings")
	b := metricPath("/api/v1/stocks/AAPL/warnings")
	if a != b {
		t.Fatalf("심볼이 라벨에 남았다: %q vs %q", a, b)
	}
}

// 매매동향은 경로가 /stocks로 시작해도 STOCK이 아니다 — 묶으면 수급 적재의 429가
// 경고 스윕을 보조 키로 넘긴다.
func TestStockTradingTrendHasItsOwnGroup(t *testing.T) {
	cases := map[string]string{
		"/api/v1/stocks/005930/investor-trading?count=5": groupStockTradingTrend,
		"/api/v1/stocks/005930/short-selling":            groupStockTradingTrend,
		"/api/v1/stocks/005930/warnings":                 groupStock,
		"/api/v1/stocks?symbols=005930":                  groupStock,
		"/api/v1/stocks/all":                             groupStockAll,
	}
	for in, want := range cases {
		if got := rateLimitGroup(in); got != want {
			t.Errorf("rateLimitGroup(%q) = %q, want %q", in, got, want)
		}
	}
}
