package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
)

const dartCorpCodeCachePath = "data/cache/dart_corp_code.zip"

func loadDartCorpCodeCache() ([]byte, error) {
	return os.ReadFile(dartCorpCodeCachePath)
}

type DartConfig struct {
	APIKey string
}

func (c DartConfig) Enabled() bool {
	return c.APIKey != ""
}

type corpCodeEntry struct {
	CorpCode  string `xml:"corp_code"`
	CorpName  string `xml:"corp_name"`
	StockCode string `xml:"stock_code"`
}

type corpCodeXML struct {
	List []corpCodeEntry `xml:"list"`
}

type DartClient struct {
	cfg        DartConfig
	httpClient *http.Client
	budget     *apibudget.Manager
	mu         sync.RWMutex
	bySymbol   map[string]string
	loadedAt   time.Time
	// corpCodeErr — 직전 corpCode 받기 실패. corpCodeRetryAfter 동안은 다시 받지 않고 이것을 돌려준다.
	corpCodeErr   error
	corpCodeErrAt time.Time
	// corpCodeURL overrides the corpCode.xml endpoint (tests only).
	corpCodeURL string
	// financialAPIBase overrides fnlttSinglAcntAll endpoint (tests only).
	financialAPIBase string

	// paceMu는 DART의 최소 호출 간격을 **기다림으로** 지킨다.
	// `apibudget.Check`는 간격이 안 지나면 에러만 낸다 — 재무 적재는 종목당
	// 20회 넘게 연달아 부르므로, 그대로 두면 두 번째 호출부터 전부 거절되고
	// 호출자는 그것을 「그 분기 미제출」로 오해해 조용히 건너뛴다.
	// `MinInterval`이 있는 이유가 이것이다(budget.go 주석 참조).
	paceMu sync.Mutex
}

func NewDartClient(cfg DartConfig, budget *apibudget.Manager) *DartClient {
	return &DartClient{
		cfg:        cfg,
		httpClient: newDartHTTPClient(),
		budget:     budget,
		bySymbol:   map[string]string{},
	}
}

func newDartHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			ForceAttemptHTTP2:     false,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
}

func (d *DartClient) TodayDisclosures(ctx context.Context, symbols []string) ([]model.BriefingEvent, error) {
	if !d.cfg.Enabled() || len(symbols) == 0 {
		return nil, nil
	}
	if err := d.ensureCorpCodes(ctx); err != nil {
		return nil, err
	}

	// "오늘" must be KST — DART is a Korean regulatory feed and the process's
	// own zone is whatever the container runs as (UTC in Docker), so
	// time.Now() without .In(KST) queries the wrong calendar day for part of
	// every 24h cycle.
	now := time.Now().In(model.KST)
	bgn := now.Format("20060102")
	var out []model.BriefingEvent

	for _, symbol := range symbols {
		corpCode := d.lookupCorpCode(symbol)
		if corpCode == "" {
			continue
		}
		items, err := d.fetchDisclosureList(ctx, corpCode, bgn, bgn)
		if err != nil {
			continue
		}
		for _, item := range items {
			out = append(out, model.BriefingEvent{
				Date:   "오늘",
				Symbol: symbol,
				Event:  item,
			})
		}
	}
	return out, nil
}

func (d *DartClient) lookupCorpCode(symbol string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.bySymbol[normalizeKRSymbol(symbol)]
}

func (d *DartClient) ensureCorpCodes(ctx context.Context) error {
	d.mu.RLock()
	if len(d.bySymbol) > 0 && time.Since(d.loadedAt) < 24*time.Hour {
		d.mu.RUnlock()
		return nil
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.bySymbol) > 0 && time.Since(d.loadedAt) < 24*time.Hour {
		return nil
	}
	/*
		**실패를 잠깐 기억한다 (2026-10-11).** 이 함수는 종목마다 불린다. DART 점검(800)
		중에 목록이 없으면 1,500종목 배치가 corpCode(3.6MB)를 종목마다 다시 받으려 했다 —
		10-09·10-10 insider·financials가 각 15~40분을 그렇게 쓰고 전 종목 실패로 끝났다.
	*/
	if d.corpCodeErr != nil && time.Since(d.corpCodeErrAt) < corpCodeRetryAfter {
		return d.corpCodeErr
	}
	next, err := d.downloadCorpCodes(ctx)
	if err != nil {
		if len(d.bySymbol) > 0 {
			// 옛 목록이 없는 것보다 낫다 — corp_code는 신규 상장 몇 건 말고는 바뀌지 않는다.
			// 받기만 실패한 것이니 corpCodeRetryAfter 뒤에 다시 받아 본다.
			log.Printf("dart corpCode: refresh failed, keeping %d codes from %s: %v", len(d.bySymbol), d.loadedAt.Format(time.RFC3339), err)
			d.loadedAt = time.Now().Add(corpCodeRetryAfter - 24*time.Hour)
			return nil
		}
		d.corpCodeErr, d.corpCodeErrAt = err, time.Now()
		return err
	}
	d.bySymbol = next
	d.loadedAt = time.Now()
	d.corpCodeErr = nil
	return nil
}

const corpCodeRetryAfter = 10 * time.Minute

// downloadCorpCodes — corpCode.xml을 받아 종목코드 → corp_code 맵을 만든다. 빈 맵은 오류다.
func (d *DartClient) downloadCorpCodes(ctx context.Context) (map[string]string, error) {
	u := fmt.Sprintf("https://opendart.fss.or.kr/api/corpCode.xml?crtfc_key=%s", url.QueryEscape(d.cfg.APIKey))
	if d.corpCodeURL != "" {
		u = d.corpCodeURL
	}
	body, err := d.fetchURL(ctx, u)
	// **거절도 HTTP 200으로 온다.** 한도가 소진되면 DART는 ZIP 대신
	// `{"status":"020"}` JSON을 200으로 돌려준다 — 전송 계층은 성공이라
	// `err == nil`이고, 그대로 두면 그 JSON을 ZIP으로 열려다 실패하거나
	// **빈 corp_code 맵**이 만들어진다. 그러면 그 뒤 모든 종목이
	// 「corp_code not found」로 떨어지고, 배치는 그것을 종목 문제로 오해한다
	// (2026-09-02 실측: 705종목이 전부 그 이유로 아무것도 못 쓰고 ok로 보고됐다).
	if err == nil {
		if status, msg, isErr := dartErrorPayload(body); isErr {
			err = fmt.Errorf("dart corpCode: %s %s", status, msg)
			if status == "020" {
				err = fmt.Errorf("%w (corpCode)", ErrDartQuotaExceeded)
			}
		}
	}
	if err != nil {
		// Local cache fallback when live download fails (e.g. Go TLS on Windows).
		if cached, cacheErr := loadDartCorpCodeCache(); cacheErr == nil {
			body = cached
		} else {
			return nil, err
		}
	}

	xmlBody := body
	if len(body) > 2 && body[0] == 'P' && body[1] == 'K' {
		xmlBody, err = unzipFirstFile(body)
		if err != nil {
			return nil, err
		}
	}

	var doc corpCodeXML
	if err := xml.Unmarshal(xmlBody, &doc); err != nil {
		return nil, err
	}

	next := map[string]string{}
	for _, item := range doc.List {
		code := strings.TrimSpace(item.StockCode)
		if len(code) != 6 {
			continue
		}
		next[code] = strings.TrimSpace(item.CorpCode)
	}
	// 빈 맵을 설치하지 않는다. 한 번 설치되면 24시간 캐시로 굳어 그 사이 모든
	// 종목이 「corp_code not found」가 된다 — 원인과 한참 떨어진 곳에서 터진다.
	if len(next) == 0 {
		return nil, fmt.Errorf("dart corpCode: 목록이 비어 있다 (%d바이트 응답)", len(xmlBody))
	}
	return next, nil
}

func (d *DartClient) fetchDisclosureList(ctx context.Context, corpCode, bgn, end string) ([]string, error) {
	items, err := d.fetchDisclosureItems(ctx, corpCode, bgn, end, 10)
	if err != nil {
		return nil, err
	}
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.ReportName)
	}
	return titles, nil
}

type dartDisclosureItem struct {
	ReportName string
	Date       string // rcept_dt, YYYYMMDD
	RceptNo    string
}

func (d *DartClient) fetchDisclosureItems(ctx context.Context, corpCode, bgn, end string, pageCount int) ([]dartDisclosureItem, error) {
	u := fmt.Sprintf(
		"https://opendart.fss.or.kr/api/list.json?crtfc_key=%s&corp_code=%s&bgn_de=%s&end_de=%s&page_count=%d",
		url.QueryEscape(d.cfg.APIKey),
		url.QueryEscape(corpCode),
		bgn,
		end,
		pageCount,
	)
	body, err := d.fetchURL(ctx, u)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		List    []struct {
			ReportName  string `json:"report_nm"`
			ReceiptDate string `json:"rcept_dt"`
			ReceiptNo   string `json:"rcept_no"`
		} `json:"list"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Status != "000" && payload.Status != "" {
		return nil, fmt.Errorf("dart list: %s", payload.Message)
	}
	items := make([]dartDisclosureItem, 0, len(payload.List))
	for _, item := range payload.List {
		if item.ReportName == "" {
			continue
		}
		items = append(items, dartDisclosureItem{
			ReportName: item.ReportName,
			Date:       item.ReceiptDate,
			RceptNo:    item.ReceiptNo,
		})
	}
	return items, nil
}

// earningsDisclosureKeywords selects filings that carry earnings information.
// "잠정실적"/"영업(잠정)실적" is DART's fair-disclosure preliminary-results
// filing — the closest domestic equivalent to a US earnings release.
var earningsDisclosureKeywords = []string{
	"잠정실적", "영업(잠정)실적", "실적공시", "사업보고서", "분기보고서", "반기보고서",
}

// FetchEarningsDisclosures returns recent earnings-related DART filings for
// symbol, most recent first, capped at 8. DART has no forward-looking
// schedule API, so every event here is Kind:"filed" (already published),
// unlike the US Finnhub calendar which is forward-looking.
func (d *DartClient) FetchEarningsDisclosures(ctx context.Context, symbol string, days int) ([]model.EarningsCalendarEvent, error) {
	if !d.cfg.Enabled() {
		return nil, fmt.Errorf("dart api key not configured")
	}
	if err := d.ensureCorpCodes(ctx); err != nil {
		return nil, err
	}
	corpCode := d.lookupCorpCode(symbol)
	if corpCode == "" {
		return nil, fmt.Errorf("dart: corp_code not found for %s", symbol)
	}

	nowKST := time.Now().In(model.KST)
	end := nowKST.Format("20060102")
	bgn := nowKST.AddDate(0, 0, -days).Format("20060102")
	items, err := d.fetchDisclosureItems(ctx, corpCode, bgn, end, 50)
	if err != nil {
		return nil, err
	}

	var out []model.EarningsCalendarEvent
	for _, item := range items {
		if !matchesEarningsKeyword(item.ReportName) {
			continue
		}
		out = append(out, model.EarningsCalendarEvent{
			Date:   formatDartDisclosureDate(item.Date),
			Label:  item.ReportName,
			Source: "dart",
			Kind:   "filed",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	if len(out) > 8 {
		out = out[:8]
	}
	return out, nil
}

func matchesEarningsKeyword(reportName string) bool {
	for _, kw := range earningsDisclosureKeywords {
		if strings.Contains(reportName, kw) {
			return true
		}
	}
	return false
}

func formatDartDisclosureDate(raw string) string {
	t, err := time.Parse("20060102", strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	return t.Format("2006-01-02")
}

func normalizeKRSymbol(symbol string) string {
	s := strings.TrimSpace(symbol)
	if len(s) > 6 {
		return s[len(s)-6:]
	}
	return s
}
