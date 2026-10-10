package growth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

/*
Bing Webmaster API — 키 하나로 도는 JSON 엔드포인트 둘(검색어별 · 페이지별).

Bing은 날짜를 `/Date(1695600000000-0700)/`로 준다. 행은 대체로 **주 단위로 묶여** 오고
(날짜가 그 주를 대표한다), 최근 6개월쯤을 한 번에 준다 — 그래서 매번 통째로 받아 덮는다.
*/
type bingClient struct {
	key     string
	siteURL string
	hc      *http.Client
}

type bingRow struct {
	Day         time.Time
	Key         string // 검색어 또는 페이지 URL
	Clicks      int
	Impressions int
	Position    float64
}

var bingDateRe = regexp.MustCompile(`/Date\((-?\d+)`)

func parseBingDate(s string) (time.Time, bool) {
	m := bingDateRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	ms, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return dayOf(time.UnixMilli(ms)), true
}

func (b *bingClient) stats(ctx context.Context, method string) ([]bingRow, error) {
	q := url.Values{"siteUrl": {b.siteURL + "/"}, "apikey": {b.key}}
	endpoint := "https://ssl.bing.com/webmaster/api.svc/json/" + method + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bing %s: request failed", method) // 오류 문구에 키가 든 URL을 싣지 않는다
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bing %s: %s: %s", method, resp.Status, truncate(string(raw), 200))
	}
	var body struct {
		D []struct {
			Query                 string  `json:"Query"`
			Page                  string  `json:"Page"`
			Clicks                int     `json:"Clicks"`
			Impressions           int     `json:"Impressions"`
			AvgImpressionPosition float64 `json:"AvgImpressionPosition"`
			Date                  string  `json:"Date"`
		} `json:"d"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("bing %s: %w", method, err)
	}
	out := make([]bingRow, 0, len(body.D))
	for _, r := range body.D {
		day, ok := parseBingDate(r.Date)
		if !ok {
			continue
		}
		key := r.Query
		if key == "" {
			key = r.Page
		}
		out = append(out, bingRow{Day: day, Key: key, Clicks: r.Clicks, Impressions: r.Impressions, Position: r.AvgImpressionPosition})
	}
	return out, nil
}
