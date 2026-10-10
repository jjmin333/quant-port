package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
금리·공매 공공데이터 호출부 (2026-10-01). 배치(worker-filings)만 부른다.

	한국은행 ECOS  ecos.bok.or.kr/api/StatisticSearch/{KEY}/json/kr/{시작}/{끝}/{표}/{주기}/{from}/{to}/{항목}
	캠코 온비드    apis.data.go.kr/B010003/{서비스}/{오퍼레이션}   (차세대, 2026-01 신설 · 개발계정 API당 1,000/일)

온비드 이용 조건: 공공데이터포털 「이용허락범위 제한 없음」. ECOS는 출처 「한국은행 경제통계시스템」 표시.
*/

// EcosObs — ECOS 한 시점 값. Time은 원천 그대로(20260930 · 202608).
type EcosObs struct {
	Time  string
	Value float64
}

// EcosSeries fetches stat/item for cycle (D·M) between from and to (원천 시점 형식).
func (c *PublicDataClient) EcosSeries(ctx context.Context, stat, cycle, item, from, to string) ([]EcosObs, error) {
	if !c.EcosEnabled() {
		return nil, fmt.Errorf("ecos: ECOS_API_KEY not configured")
	}
	var payload struct {
		Search *struct {
			Rows []struct {
				Time  string `json:"TIME"`
				Value string `json:"DATA_VALUE"`
			} `json:"row"`
		} `json:"StatisticSearch"`
		Result *struct {
			Code    string `json:"CODE"`
			Message string `json:"MESSAGE"`
		} `json:"RESULT"`
	}
	u := fmt.Sprintf("https://ecos.bok.or.kr/api/StatisticSearch/%s/json/kr/1/10000/%s/%s/%s/%s/%s",
		url.PathEscape(c.ecosKey), stat, cycle, from, to, item)
	if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderEcos, u, &payload); err != nil {
		return nil, c.redact(fmt.Errorf("ecos %s/%s: %w", stat, item, err))
	}
	if payload.Result != nil {
		if payload.Result.Code == "INFO-200" {
			return nil, nil
		}
		return nil, fmt.Errorf("ecos %s/%s: %s %s", stat, item, payload.Result.Code, payload.Result.Message)
	}
	if payload.Search == nil {
		return nil, nil
	}
	out := make([]EcosObs, 0, len(payload.Search.Rows))
	for _, r := range payload.Search.Rows {
		v, err := strconv.ParseFloat(strings.TrimSpace(r.Value), 64)
		if err != nil {
			continue
		}
		out = append(out, EcosObs{Time: r.Time, Value: v})
	}
	return out, nil
}

// ── 캠코 온비드 ────────────────────────────────────────────────────────────────

// OnbidPage calls one Onbid next-gen operation and returns its items as raw maps.
// service 예: "OnbidPbancListSrvc2/getPbancList2".
func (c *PublicDataClient) OnbidPage(ctx context.Context, service string, q url.Values) ([]map[string]any, int, error) {
	if !c.DataEnabled() {
		return nil, 0, fmt.Errorf("onbid: DATA_GO_KR_API_KEY not configured")
	}
	q.Set("serviceKey", c.dataKey)
	q.Set("resultType", "json")
	var payload struct {
		Header *struct {
			Code string `json:"resultCode"`
			Msg  string `json:"resultMsg"`
		} `json:"header"`
		Result *struct {
			Code string `json:"resultCode"`
			Msg  string `json:"resultMsg"`
		} `json:"result"`
		Body *struct {
			Total any `json:"totalCount"`
			Items *struct {
				Item json.RawMessage `json:"item"`
			} `json:"items"`
		} `json:"body"`
		Fault *struct {
			Header struct {
				Msg string `json:"returnAuthMsg"`
			} `json:"cmmMsgHeader"`
		} `json:"OpenAPI_ServiceResponse"`
	}
	u := "https://apis.data.go.kr/B010003/" + service + "?" + q.Encode()
	if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderOnbid, u, &payload); err != nil {
		return nil, 0, c.redact(fmt.Errorf("onbid %s: %w", service, err))
	}
	if payload.Fault != nil {
		return nil, 0, fmt.Errorf("onbid %s: %s", service, payload.Fault.Header.Msg)
	}
	// 「03 NODATA_ERROR」는 결과 0건이다.
	if payload.Result != nil {
		if payload.Result.Code == "03" {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("onbid %s: %s %s", service, payload.Result.Code, payload.Result.Msg)
	}
	if payload.Header != nil && payload.Header.Code != "00" {
		if payload.Header.Code == "03" {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("onbid %s: %s %s", service, payload.Header.Code, payload.Header.Msg)
	}
	if payload.Body == nil || payload.Body.Items == nil || len(payload.Body.Items.Item) == 0 {
		return nil, 0, nil
	}
	total := anyInt(payload.Body.Total)
	raw := payload.Body.Items.Item
	// 한 건이면 배열이 아니라 객체로 온다.
	var items []map[string]any
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		var one map[string]any
		if err := json.Unmarshal(raw, &one); err != nil {
			return nil, 0, err
		}
		items = []map[string]any{one}
	} else if err := json.Unmarshal(raw, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// AnyStr reads a field of a raw JSON row as a string (숫자도 문자로).
func AnyStr(row map[string]any, key string) string {
	switch v := row[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// AnyInt64 reads a numeric field (숫자 문자열 포함). ok=false면 없음.
func AnyInt64(row map[string]any, key string) (int64, bool) {
	switch v := row[key].(type) {
	case float64:
		return int64(v), true
	case string:
		n, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v), ",", ""), 64)
		if err != nil {
			return 0, false
		}
		return int64(n), true
	}
	return 0, false
}
