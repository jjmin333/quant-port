package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
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

	now := time.Now()
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

	u := fmt.Sprintf("https://opendart.fss.or.kr/api/corpCode.xml?crtfc_key=%s", url.QueryEscape(d.cfg.APIKey))
	body, err := d.fetchURL(ctx, u)
	if err != nil {
		// Local cache fallback when live download fails (e.g. Go TLS on Windows).
		if cached, cacheErr := loadDartCorpCodeCache(); cacheErr == nil {
			body = cached
		} else {
			return err
		}
	}

	xmlBody := body
	if len(body) > 2 && body[0] == 'P' && body[1] == 'K' {
		xmlBody, err = unzipFirstFile(body)
		if err != nil {
			return err
		}
	}

	var doc corpCodeXML
	if err := xml.Unmarshal(xmlBody, &doc); err != nil {
		return err
	}

	next := map[string]string{}
	for _, item := range doc.List {
		code := strings.TrimSpace(item.StockCode)
		if len(code) != 6 {
			continue
		}
		next[code] = strings.TrimSpace(item.CorpCode)
	}
	d.bySymbol = next
	d.loadedAt = time.Now()
	return nil
}

func (d *DartClient) fetchDisclosureList(ctx context.Context, corpCode, bgn, end string) ([]string, error) {
	u := fmt.Sprintf(
		"https://opendart.fss.or.kr/api/list.json?crtfc_key=%s&corp_code=%s&bgn_de=%s&end_de=%s&page_count=10",
		url.QueryEscape(d.cfg.APIKey),
		url.QueryEscape(corpCode),
		bgn,
		end,
	)
	body, err := d.fetchURL(ctx, u)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		List    []struct {
			ReportName string `json:"report_nm"`
		} `json:"list"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Status != "000" && payload.Status != "" {
		return nil, fmt.Errorf("dart list: %s", payload.Message)
	}
	var titles []string
	for _, item := range payload.List {
		if item.ReportName != "" {
			titles = append(titles, item.ReportName)
		}
	}
	return titles, nil
}

func normalizeKRSymbol(symbol string) string {
	s := strings.TrimSpace(symbol)
	if len(s) > 6 {
		return s[len(s)-6:]
	}
	return s
}
