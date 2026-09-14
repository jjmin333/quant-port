package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
실적 발표일 — SEC EDGAR 8-K Item 2.02.

미국 상장사는 분기 실적을 **8-K Item 2.02**(Results of Operations and Financial
Condition)로 공시한다. `data.sec.gov/submissions/CIK{...}.json`이 그 제출일과
접수 시각을 함께 주고, 종목당 1호출이다.

이게 필요한 이유는 Finnhub `/stock/earnings`가 **발표일을 안 주기** 때문이다
(주는 건 달력 분기로 반올림한 기간말). migrations/056 헤더 참고.
*/

// EarningsAnnouncement is one 8-K Item 2.02 filing.
type EarningsAnnouncement struct {
	// Filed — 제출일(미 동부 달력일). 이게 우리가 쓰는 「발표일」이다.
	Filed time.Time
	// Accepted — SEC 접수 시각(ET). **보도자료 시각이 아니다** — 실측 WMT는
	// 장전 발표인데 접수가 10:59였다. 장후 판정에만 쓴다(056 헤더).
	Accepted  time.Time
	Accession string
}

// earnings8KItem is the item code that marks a results release.
const earnings8KItem = "2.02"

// FetchEarningsAnnouncements returns this company's 8-K Item 2.02 filings,
// newest first.
func (e *EdgarClient) FetchEarningsAnnouncements(
	ctx context.Context, ticker string,
) ([]EarningsAnnouncement, error) {
	if e == nil || !e.cfg.Enabled() {
		return nil, fmt.Errorf("edgar: user agent not configured")
	}
	cik, err := e.LookupCIK(ctx, ticker)
	if err != nil {
		return nil, err
	}
	if e.budget != nil {
		if err := e.budget.Check(ctx, e.provider); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", e.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	res, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		if e.budget != nil {
			e.budget.BlockFor(ctx, e.provider, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, errEdgarRateLimited
	}
	if res.StatusCode == http.StatusNotFound {
		// EDGAR에 없는 종목(ADR·해외 발행사 등)이 있다. 실패가 아니라 「없음」이다.
		return nil, nil
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("edgar submissions %s: status %d %s",
			ticker, res.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Filings struct {
			Recent struct {
				AccessionNumber []string `json:"accessionNumber"`
				FilingDate      []string `json:"filingDate"`
				Form            []string `json:"form"`
				Items           []string `json:"items"`
				AcceptanceTime  []string `json:"acceptanceDateTime"`
			} `json:"recent"`
		} `json:"filings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("edgar submissions %s: %w", ticker, err)
	}
	if e.budget != nil {
		e.budget.Record(ctx, e.provider)
	}

	return parse8KEarnings(
		payload.Filings.Recent.Form,
		payload.Filings.Recent.Items,
		payload.Filings.Recent.FilingDate,
		payload.Filings.Recent.AcceptanceTime,
		payload.Filings.Recent.AccessionNumber,
	), nil
}

var errEdgarRateLimited = fmt.Errorf("edgar: status 429")

// parse8KEarnings pulls the Item 2.02 filings out of the parallel arrays SEC
// returns.
//
// **배열 네 개가 인덱스로만 묶여 있다.** 길이가 다를 수 있으므로 매 접근마다
// 범위를 확인한다 — 한 배열이 짧으면 panic이 나고, 그건 3천 종목 배치 한복판이다.
func parse8KEarnings(forms, items, filed, accepted, accessions []string) []EarningsAnnouncement {
	out := make([]EarningsAnnouncement, 0, 16)
	for i := range forms {
		if forms[i] != "8-K" {
			continue
		}
		if i >= len(items) || !strings.Contains(items[i], earnings8KItem) {
			continue
		}
		if i >= len(filed) {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", filed[i], time.UTC)
		if err != nil {
			continue
		}
		a := EarningsAnnouncement{Filed: day}
		if i < len(accessions) {
			a.Accession = accessions[i]
		}
		if i < len(accepted) {
			// "2026-05-20T20:21:33.000Z" — Z가 붙어 있지만 SEC는 ET로 낸다.
			// UTC로 해석하면 장후 판정이 5시간 밀린다.
			if t, err := time.Parse("2006-01-02T15:04:05.000Z", accepted[i]); err == nil {
				a.Accepted = t
			}
		}
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Filed.After(out[j].Filed) })
	return out
}

// 장후 경계 — ET 16:00.
//
// 실측 분포가 뚜렷이 갈린다: 장후 발표사의 접수는 20:03~21:31, 그 밖은
// 10:16~12:12였다. 그 사이 어디에 선을 그어도 같은 결과라 장 마감 시각을 쓴다.
const edgarAfterCloseHour = 16

// sessionFromAcceptance maps the SEC acceptance clock to our session vocabulary.
//
// 장전과 장중을 정확히 가르지는 못한다(접수가 보도자료보다 늦다). **그래도 어떤
// 숫자도 틀리지 않는다** — 발표 후 주가 계산이 장전과 장중을 똑같이 다루기
// 때문이다(earnings_reaction.go의 sameDay 분기). 갈리는 것은 장후뿐이다.
func sessionFromAcceptance(accepted time.Time) string {
	if accepted.IsZero() {
		return ""
	}
	if accepted.Hour() >= edgarAfterCloseHour {
		return "amc"
	}
	if accepted.Hour()*60+accepted.Minute() < 9*60+30 {
		return "bmo"
	}
	return "dmh"
}
