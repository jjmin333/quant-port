package marketdata

import (
	"context"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

// CryptoTicker — 원화 마켓 한 종목의 지금 값 (순자산 코인 평가, 2026-10-01).
type CryptoTicker struct {
	Market     string  `json:"market"`
	KoreanName string  `json:"koreanName"`
	Price      float64 `json:"price"`
	ChangeRate float64 `json:"changeRate"`
}

// KRWBoard returns every KRW market's last price in **two calls** (ticker/all + market/all) —
// 보유 코인 수와 상관없이 호출 수가 같다. 부르는 쪽(WealthService)이 1분 캐시를 둔다.
func (c *UpbitClient) KRWBoard(ctx context.Context) ([]CryptoTicker, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("upbit client not configured")
	}
	var names []struct {
		Market     string `json:"market"`
		KoreanName string `json:"korean_name"`
	}
	if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderUpbitBoard, "https://api.upbit.com/v1/market/all?is_details=false", &names); err != nil {
		return nil, err
	}
	var rows []struct {
		Market     string  `json:"market"`
		TradePrice float64 `json:"trade_price"`
		ChangeRate float64 `json:"signed_change_rate"`
	}
	if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderUpbitBoard, "https://api.upbit.com/v1/ticker/all?quote_currencies=KRW", &rows); err != nil {
		return nil, err
	}
	byMarket := map[string]string{}
	for _, n := range names {
		byMarket[n.Market] = n.KoreanName
	}
	out := make([]CryptoTicker, 0, len(rows))
	for _, r := range rows {
		if !strings.HasPrefix(r.Market, "KRW-") || r.TradePrice <= 0 {
			continue
		}
		out = append(out, CryptoTicker{Market: r.Market, KoreanName: byMarket[r.Market], Price: r.TradePrice, ChangeRate: r.ChangeRate * 100})
	}
	return out, nil
}
