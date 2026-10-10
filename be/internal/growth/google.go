package growth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

/*
Google 서비스 계정 인증 — 표준 라이브러리만으로.

oauth2 패키지를 들이지 않은 이유: 필요한 것은 「JWT 하나 서명 → 토큰 교환」 한 가지뿐이고,
이 워커만 쓴다. 의존성 하나가 API 이미지 전체에 들어가는 것보다 60줄이 싸다.

범위는 읽기 전용 둘뿐이다(webmasters.readonly · analytics.readonly). 서비스 계정에도
Search Console에서 「제한」 권한만 주면 된다 — 이 워커는 사이트맵 제출조차 하지 않는다.
*/
const (
	scopeWebmastersRO = "https://www.googleapis.com/auth/webmasters.readonly"
	scopeAnalyticsRO  = "https://www.googleapis.com/auth/analytics.readonly"
)

type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type googleAuth struct {
	email    string
	key      *rsa.PrivateKey
	tokenURI string
	scope    string
	hc       *http.Client

	mu    sync.Mutex
	token string
	exp   time.Time
}

func loadGoogleAuth(path string, hc *http.Client, scopes ...string) (*googleAuth, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sa serviceAccount
	if err := json.Unmarshal(raw, &sa); err != nil {
		return nil, fmt.Errorf("service account json: %w", err)
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, errors.New("service account: private_key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("service account key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("service account key is not RSA")
	}
	if sa.TokenURI == "" {
		sa.TokenURI = "https://oauth2.googleapis.com/token"
	}
	return &googleAuth{
		email: sa.ClientEmail, key: key, tokenURI: sa.TokenURI,
		scope: strings.Join(scopes, " "), hc: hc,
	}, nil
}

func (a *googleAuth) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && time.Until(a.exp) > 2*time.Minute {
		return a.token, nil
	}
	now := time.Now()
	assertion, err := a.signJWT(now)
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google token: %s: %s", resp.Status, truncate(string(body), 300))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", err
	}
	a.token = tok.AccessToken
	a.exp = now.Add(time.Duration(tok.ExpiresIn) * time.Second)
	return a.token, nil
}

func (a *googleAuth) signJWT(now time.Time) (string, error) {
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{
		"iss":   a.email,
		"scope": a.scope,
		"aud":   a.tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	signingInput := header + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, a.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + enc.EncodeToString(sig), nil
}

// googleJSON — 인증을 붙여 JSON을 주고받는다. body가 nil이면 GET.
func googleJSON(ctx context.Context, a *googleAuth, method, endpoint string, body any, out any) error {
	token, err := a.Token(ctx)
	if err != nil {
		return err
	}
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: %s: %s", method, endpointLabel(endpoint), resp.Status, truncate(string(raw), 300))
	}
	return json.Unmarshal(raw, out)
}

// endpointLabel — 오류 문구에 쿼리(키 등)를 싣지 않는다.
func endpointLabel(endpoint string) string {
	if i := strings.IndexByte(endpoint, '?'); i >= 0 {
		return endpoint[:i]
	}
	return endpoint
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ── Search Console ─────────────────────────────────────────────────────────

type gscClient struct {
	auth     *googleAuth
	property string
}

type gscRow struct {
	Keys        []string `json:"keys"`
	Clicks      float64  `json:"clicks"`
	Impressions float64  `json:"impressions"`
	Position    float64  `json:"position"`
}

// searchAnalytics — 한 구간 · 한 차원 조합을 페이지를 넘기며 전부 받는다.
func (g *gscClient) searchAnalytics(ctx context.Context, start, end time.Time, dims []string) ([]gscRow, error) {
	endpoint := "https://www.googleapis.com/webmasters/v3/sites/" + url.PathEscape(g.property) + "/searchAnalytics/query"
	const pageSize = 25000
	var out []gscRow
	for startRow := 0; ; startRow += pageSize {
		var resp struct {
			Rows []gscRow `json:"rows"`
		}
		err := googleJSON(ctx, g.auth, http.MethodPost, endpoint, map[string]any{
			"startDate":  ymd(start),
			"endDate":    ymd(end),
			"dimensions": dims,
			"rowLimit":   pageSize,
			"startRow":   startRow,
			// "all"은 확정 전(최근 1~2일) 값도 준다. 매일 최근 구간을 다시 받아 덮으므로
			// 잠정치가 나중에 확정치로 갈린다.
			"dataState": "all",
		}, &resp)
		if err != nil {
			return nil, err
		}
		out = append(out, resp.Rows...)
		if len(resp.Rows) < pageSize {
			return out, nil
		}
	}
}

type inspection struct {
	Verdict         string
	CoverageState   string
	IndexingState   string
	RobotsState     string
	FetchState      string
	GoogleCanonical string
	UserCanonical   string
	LastCrawl       *time.Time
}

func (g *gscClient) inspect(ctx context.Context, pageURL string) (inspection, error) {
	var resp struct {
		InspectionResult struct {
			IndexStatusResult struct {
				Verdict         string `json:"verdict"`
				CoverageState   string `json:"coverageState"`
				RobotsTxtState  string `json:"robotsTxtState"`
				IndexingState   string `json:"indexingState"`
				LastCrawlTime   string `json:"lastCrawlTime"`
				PageFetchState  string `json:"pageFetchState"`
				GoogleCanonical string `json:"googleCanonical"`
				UserCanonical   string `json:"userCanonical"`
			} `json:"indexStatusResult"`
		} `json:"inspectionResult"`
	}
	err := googleJSON(ctx, g.auth, http.MethodPost,
		"https://searchconsole.googleapis.com/v1/urlInspection/index:inspect",
		map[string]any{"inspectionUrl": pageURL, "siteUrl": g.property, "languageCode": "ko-KR"}, &resp)
	if err != nil {
		return inspection{}, err
	}
	r := resp.InspectionResult.IndexStatusResult
	out := inspection{
		Verdict: r.Verdict, CoverageState: r.CoverageState, IndexingState: r.IndexingState,
		RobotsState: r.RobotsTxtState, FetchState: r.PageFetchState,
		GoogleCanonical: r.GoogleCanonical, UserCanonical: r.UserCanonical,
	}
	if t, err := time.Parse(time.RFC3339, r.LastCrawlTime); err == nil {
		t = t.In(kst)
		out.LastCrawl = &t
	}
	return out, nil
}

type sitemapStatus struct {
	Path           string `json:"path"`
	LastDownloaded string `json:"lastDownloaded"`
	IsPending      bool   `json:"isPending"`
	Errors         string `json:"errors"`
	Warnings       string `json:"warnings"`
	Contents       []struct {
		Type      string `json:"type"`
		Submitted string `json:"submitted"`
	} `json:"contents"`
}

func (g *gscClient) sitemaps(ctx context.Context) ([]sitemapStatus, error) {
	var resp struct {
		Sitemap []sitemapStatus `json:"sitemap"`
	}
	err := googleJSON(ctx, g.auth, http.MethodGet,
		"https://www.googleapis.com/webmasters/v3/sites/"+url.PathEscape(g.property)+"/sitemaps", nil, &resp)
	return resp.Sitemap, err
}

// ── GA4 Data API ───────────────────────────────────────────────────────────

type ga4Client struct {
	auth     *googleAuth
	property string
}

type ga4Row struct {
	Dims    []string
	Metrics []float64
}

// runReport — 차원·지표 이름만 받는 얇은 껍데기. filter는 GA4 FilterExpression 그대로(없으면 nil).
func (g *ga4Client) runReport(ctx context.Context, start, end time.Time, dims, metrics []string, filter map[string]any, limit int) ([]ga4Row, error) {
	body := map[string]any{
		"dateRanges": []map[string]string{{"startDate": ymd(start), "endDate": ymd(end)}},
	}
	var d []map[string]string
	for _, name := range dims {
		d = append(d, map[string]string{"name": name})
	}
	var m []map[string]string
	for _, name := range metrics {
		m = append(m, map[string]string{"name": name})
	}
	body["dimensions"], body["metrics"] = d, m
	if filter != nil {
		body["dimensionFilter"] = filter
	}
	if limit > 0 {
		body["limit"] = limit
	}
	var resp struct {
		Rows []struct {
			DimensionValues []struct {
				Value string `json:"value"`
			} `json:"dimensionValues"`
			MetricValues []struct {
				Value string `json:"value"`
			} `json:"metricValues"`
		} `json:"rows"`
	}
	endpoint := "https://analyticsdata.googleapis.com/v1beta/properties/" + url.PathEscape(g.property) + ":runReport"
	if err := googleJSON(ctx, g.auth, http.MethodPost, endpoint, body, &resp); err != nil {
		return nil, err
	}
	out := make([]ga4Row, 0, len(resp.Rows))
	for _, r := range resp.Rows {
		row := ga4Row{}
		for _, v := range r.DimensionValues {
			row.Dims = append(row.Dims, v.Value)
		}
		for _, v := range r.MetricValues {
			var f float64
			_, _ = fmt.Sscan(v.Value, &f)
			row.Metrics = append(row.Metrics, f)
		}
		out = append(out, row)
	}
	return out, nil
}
