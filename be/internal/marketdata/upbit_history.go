package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

// UpbitDatedClose is one daily candle with the calendar day it belongs to.
type UpbitDatedClose struct {
	Date  time.Time
	Close float64
}

/*
DailyClosesDated는 `DailyCloses`와 같은 응답을 읽되 **날짜를 버리지 않는다**.

기존 메서드는 스파크라인용이라 종가 배열만 돌려준다. 스파크라인에는 그걸로 충분하지만
`index_daily_closes`를 채우려면 각 값이 어느 날 것인지가 있어야 한다 — 날짜 없는 배열을
이력으로 적으려면 날짜를 지어내야 하고, 그러면 전 행이 조용히 어긋난다.

`candle_date_time_kst`를 쓴다. 업비트는 KST 자정을 일봉 경계로 잡고, 우리 쪽 비트코인
일정(`ScheduleAlways`)도 KST 23:59를 종가 확정 시각으로 본다 — 같은 경계다.

업비트 캔들 API는 한 번에 최대 200봉을 준다. 그보다 긴 구간이 필요하면 `to`로 페이지를
넘겨야 하는데, 여기 쓰임(반년 남짓 백필)에는 한 번으로 닿는다.
*/
func (c *UpbitClient) DailyClosesDated(ctx context.Context, market string, count int) ([]UpbitDatedClose, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("upbit client not configured")
	}
	if count <= 0 || count > 200 {
		count = 200
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderUpbit); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf(
		"https://api.upbit.com/v1/candles/days?market=%s&count=%d",
		url.QueryEscape(market), count,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upbit candles %s: status %d", market, res.StatusCode)
	}

	var rows []struct {
		DateKST    string  `json:"candle_date_time_kst"`
		TradePrice float64 `json:"trade_price"`
	}
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		return nil, err
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderUpbit)
	}

	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		seoul = time.FixedZone("KST", 9*3600)
	}

	// 응답은 최신이 앞이다. 이력은 오래된 것부터 쓰므로 뒤집어 담는다.
	out := make([]UpbitDatedClose, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].TradePrice <= 0 {
			continue
		}
		// "2026-08-31T00:00:00" — 타임존 표기가 없고 값은 이미 KST다.
		day, err := time.ParseInLocation("2006-01-02T15:04:05", rows[i].DateKST, seoul)
		if err != nil {
			continue
		}
		out = append(out, UpbitDatedClose{Date: day, Close: rows[i].TradePrice})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("upbit candles %s: no dated closes", market)
	}
	return out, nil
}
