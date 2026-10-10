package marketdata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
ETFDailyRow — KRX `etf_bydd_trd` 한 행의 숫자들(ETF 점검 재료, 2026-09-24).

카탈로그 적재(`ListDomesticETP`)는 같은 응답에서 이름·코드만 꺼낸다. 이쪽은 NAV·순자산·
거래대금·기초지수를 꺼낸다. 실측(2026-09-22) 필드: TDD_CLSPRC · NAV · ACC_TRDVAL ·
INVSTASST_NETASST_TOTAMT · IDX_IND_NM · OBJ_STKPRC_IDX · FLUC_RT_IDX.
*/
type ETFDailyRow struct {
	Symbol          string
	Close           float64
	NAV             *float64
	NetAssets       *int64
	TradeValue      *int64
	IndexName       string
	IndexValue      *float64
	IndexChangeRate *float64
}

// krxNumber accepts KRX's string numbers ("1,234.5", "-", "") and JSON numbers.
type krxNumber struct {
	v  float64
	ok bool
}

func (n *krxNumber) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	s = strings.ReplaceAll(s, ",", "")
	if s == "" || s == "-" || s == "null" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil // 숫자가 아니면 없는 값이다 — 0으로 읽지 않는다.
	}
	n.v, n.ok = v, true
	return nil
}

func (n krxNumber) float() *float64 {
	if !n.ok {
		return nil
	}
	v := n.v
	return &v
}

func (n krxNumber) int64() *int64 {
	if !n.ok {
		return nil
	}
	v := int64(n.v)
	return &v
}

type krxETFDailyRaw struct {
	ISUCD    string    `json:"ISU_CD"`
	ISUSRTCD string    `json:"ISU_SRT_CD"`
	Close    krxNumber `json:"TDD_CLSPRC"`
	NAV      krxNumber `json:"NAV"`
	NetAsset krxNumber `json:"INVSTASST_NETASST_TOTAMT"`
	TrdVal   krxNumber `json:"ACC_TRDVAL"`
	IdxName  string    `json:"IDX_IND_NM"`
	IdxValue krxNumber `json:"OBJ_STKPRC_IDX"`
	IdxRate  krxNumber `json:"FLUC_RT_IDX"`
}

/*
ETFDaily returns every ETF row for basDd (YYYYMMDD). An empty slice means KRX has
nothing for that day — a holiday, or (for yesterday) not published yet. 호출 하나다.
*/
func (c *KRXClient) ETFDaily(ctx context.Context, basDd string) ([]ETFDailyRow, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("krx api key not configured")
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderKRX); err != nil {
			return nil, err
		}
	}
	body, _ := json.Marshal(map[string]string{"basDd": basDd})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, krxETFTradeURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("AUTH_KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "quant-port-etf-metrics/1.0")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderKRX)
	}
	if res.StatusCode == http.StatusTooManyRequests {
		if c.budget != nil {
			c.budget.BlockFor(ctx, apibudget.ProviderKRX, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("krx: status 429")
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("krx: status %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	var wrapped struct {
		Out []krxETFDailyRaw `json:"OutBlock_1"`
	}
	if err := json.NewDecoder(res.Body).Decode(&wrapped); err != nil {
		return nil, fmt.Errorf("krx etf daily json: %w", err)
	}
	return mapETFDaily(wrapped.Out), nil
}

func mapETFDaily(raw []krxETFDailyRaw) []ETFDailyRow {
	out := make([]ETFDailyRow, 0, len(raw))
	for _, r := range raw {
		sym := strings.ToUpper(strings.TrimSpace(r.ISUSRTCD))
		if sym == "" {
			sym = strings.ToUpper(strings.TrimSpace(r.ISUCD))
		}
		if sym == "" || !r.Close.ok || r.Close.v <= 0 {
			continue
		}
		out = append(out, ETFDailyRow{
			Symbol: sym, Close: r.Close.v, NAV: r.NAV.float(), NetAssets: r.NetAsset.int64(),
			TradeValue: r.TrdVal.int64(), IndexName: strings.TrimSpace(r.IdxName),
			IndexValue: r.IdxValue.float(), IndexChangeRate: r.IdxRate.float(),
		})
	}
	return out
}
