package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
외부 신호 세 가지의 호출부 (2026-09-28 사용자 결정 — 요청 6·7). 배치(worker-filings)만 부른다.

	Finnhub  /stock/metric         해외 지표 보조(본국 상장 기준 비율)
	         /stock/recommendation 애널리스트 추천 추이(무료). 목표가(/stock/price-target)는 유료라 없다.
	네이버    m.stock.naver.com …/integration  국내 평균 목표가·투자의견(공개 API 아님)
	ApeWisdom /api/v1.0/filter/all-stocks     Reddit 종목 언급 수(키 없음)
*/

// ErrExternalNotFound — 그 종목 자료가 없다(빈 응답). 실패가 아니라 「없음」이다.
var ErrExternalNotFound = errors.New("external: no data")

func getJSON(ctx context.Context, hc *http.Client, budget *apibudget.Manager, provider apibudget.Provider, u string, out any) error {
	body, err := getRaw(ctx, hc, budget, provider, u, "application/json", "")
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// maxExternalBody — 외부 응답 상한(ApeWisdom 종목 페이지 ~30KB · Reddit RSS ~60KB).
const maxExternalBody = 2 << 20

// getRaw is getJSON without the decode. ua가 비면 기본 UA.
func getRaw(ctx context.Context, hc *http.Client, budget *apibudget.Manager, provider apibudget.Provider, u, accept, ua string) ([]byte, error) {
	if budget != nil {
		if err := budget.Check(ctx, provider); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if ua == "" {
		ua = "Mozilla/5.0 (compatible; quant-port/1.0)"
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", ua)
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if budget != nil {
		budget.Record(ctx, provider)
	}
	if res.StatusCode == http.StatusTooManyRequests {
		if budget != nil {
			wait := apibudget.ParseRetryAfter(res.Header.Get("Retry-After"))
			// Reddit은 Retry-After 대신 x-ratelimit-reset(초)을 준다.
			if n, err := strconv.Atoi(strings.TrimSpace(res.Header.Get("X-Ratelimit-Reset"))); err == nil && n > 0 {
				wait = time.Duration(n) * time.Second
			}
			budget.BlockFor(ctx, provider, wait)
		}
		return nil, fmt.Errorf("%s: status 429", provider)
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrExternalNotFound
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 200))
		return nil, fmt.Errorf("%s: status %d %s", provider, res.StatusCode, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(io.LimitReader(res.Body, maxExternalBody))
}

// FinnhubMetrics — /stock/metric에서 쓰는 값들. 퍼센트는 퍼센트 단위 그대로(39.86 = 39.86%).
type FinnhubMetrics struct {
	PER, PBR, PSR                     *float64
	ROE, OperatingMargin, NetMargin   *float64
	RevenueGrowth, EPSGrowth          *float64
	CurrentRatio, DividendYield, Beta *float64
}

// Empty is true when Finnhub had nothing for the symbol.
func (m FinnhubMetrics) Empty() bool {
	return m.PER == nil && m.PBR == nil && m.PSR == nil && m.ROE == nil && m.OperatingMargin == nil
}

func (c *FinnhubClient) Metrics(ctx context.Context, symbol string) (FinnhubMetrics, error) {
	var out FinnhubMetrics
	if !c.Enabled() {
		return out, fmt.Errorf("finnhub not configured")
	}
	var payload struct {
		Metric map[string]any `json:"metric"`
	}
	u := fmt.Sprintf("https://finnhub.io/api/v1/stock/metric?symbol=%s&metric=all&token=%s",
		url.QueryEscape(symbol), url.QueryEscape(c.apiKey))
	if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderFinnhubMetrics, u, &payload); err != nil {
		return out, err
	}
	pick := func(keys ...string) *float64 {
		for _, k := range keys {
			if v, ok := payload.Metric[k].(float64); ok {
				x := v
				return &x
			}
		}
		return nil
	}
	out = FinnhubMetrics{
		PER:             positive(pick("peTTM", "peBasicExclExtraTTM", "peAnnual")),
		PBR:             positive(pick("pbQuarterly", "pbAnnual")),
		PSR:             positive(pick("psTTM", "psAnnual")),
		ROE:             pick("roeTTM", "roeRfy"),
		OperatingMargin: pick("operatingMarginTTM", "operatingMarginAnnual"),
		NetMargin:       pick("netProfitMarginTTM", "netProfitMarginAnnual"),
		RevenueGrowth:   pick("revenueGrowthTTMYoy"),
		EPSGrowth:       pick("epsGrowthTTMYoy"),
		CurrentRatio:    positive(pick("currentRatioQuarterly", "currentRatioAnnual")),
		DividendYield:   pick("dividendYieldIndicatedAnnual", "currentDividendYieldTTM"),
		Beta:            pick("beta"),
	}
	if out.Empty() {
		return out, ErrExternalNotFound
	}
	return out, nil
}

// PER·PBR·PSR은 0 이하면 뜻이 없다(적자 PER 등) — 값 없음으로.
func positive(v *float64) *float64 {
	if v == nil || *v <= 0 {
		return nil
	}
	return v
}

// FinnhubRecommendation — 가장 최근 달의 추천 명수.
type FinnhubRecommendation struct {
	Period                                 string
	StrongBuy, Buy, Hold, Sell, StrongSell int
}

// Recommendation — 최근 몇 달 치 추천 추이, **최신 달이 먼저**(2026-10-07부터 전부 돌려준다 — 이력 표가 쓴다).
// 명수가 전부 0인 달은 뺀다. 남는 달이 없으면 ErrExternalNotFound.
func (c *FinnhubClient) Recommendation(ctx context.Context, symbol string) ([]FinnhubRecommendation, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("finnhub not configured")
	}
	var rows []struct {
		Period     string `json:"period"`
		StrongBuy  int    `json:"strongBuy"`
		Buy        int    `json:"buy"`
		Hold       int    `json:"hold"`
		Sell       int    `json:"sell"`
		StrongSell int    `json:"strongSell"`
	}
	u := fmt.Sprintf("https://finnhub.io/api/v1/stock/recommendation?symbol=%s&token=%s",
		url.QueryEscape(symbol), url.QueryEscape(c.apiKey))
	if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderFinnhubMetrics, u, &rows); err != nil {
		return nil, err
	}
	out := make([]FinnhubRecommendation, 0, len(rows))
	for _, r := range rows {
		if r.StrongBuy+r.Buy+r.Hold+r.Sell+r.StrongSell == 0 {
			continue
		}
		out = append(out, FinnhubRecommendation{r.Period, r.StrongBuy, r.Buy, r.Hold, r.Sell, r.StrongSell})
	}
	if len(out) == 0 {
		return nil, ErrExternalNotFound
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Period > out[j].Period })
	return out, nil
}

// RecommendationRating — 명수를 1(강력매도)~5(강력매수) 평균으로. 네이버 투자의견과 같은 척도.
func RecommendationRating(r FinnhubRecommendation) (float64, bool) {
	n := r.StrongBuy + r.Buy + r.Hold + r.Sell + r.StrongSell
	if n == 0 {
		return 0, false
	}
	sum := 5*r.StrongBuy + 4*r.Buy + 3*r.Hold + 2*r.Sell + r.StrongSell
	return float64(sum) / float64(n), true
}

// NaverConsensus — 국내 평균 목표가·투자의견(1~5) + 같은 응답의 최근 증권사 리포트 목록.
type NaverConsensus struct {
	AsOf        string // YYYY-MM-DD
	Rating      *float64
	TargetPrice *float64
	Reports     []NaverReport
}

// NaverReport — 증권사 리포트 한 건(제목·증권사·날짜만, 본문은 링크로).
type NaverReport struct {
	ID     int64
	Broker string
	Title  string
	Date   string // YYYY-MM-DD
}

// NaverStockClient reads m.stock.naver.com. 공개 API가 아니라 모양이 바뀌면 빈 값으로 떨어진다.
type NaverStockClient struct {
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewNaverStockClient(hc *http.Client, budget *apibudget.Manager) *NaverStockClient {
	return &NaverStockClient{httpClient: hc, budget: budget}
}

func (c *NaverStockClient) Consensus(ctx context.Context, code string) (NaverConsensus, error) {
	var out NaverConsensus
	var payload struct {
		ConsensusInfo *struct {
			CreateDate      string `json:"createDate"`
			RecommMean      string `json:"recommMean"`
			PriceTargetMean string `json:"priceTargetMean"`
		} `json:"consensusInfo"`
		Researches []struct {
			ID     int64  `json:"id"`
			Broker string `json:"bnm"`
			Title  string `json:"tit"`
			Date   string `json:"wdt"` // YYYYMMDD
		} `json:"researches"`
	}
	u := "https://m.stock.naver.com/api/stock/" + url.PathEscape(code) + "/integration"
	if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderNaverStock, u, &payload); err != nil {
		return out, err
	}
	for _, r := range payload.Researches {
		if r.ID <= 0 || strings.TrimSpace(r.Title) == "" {
			continue
		}
		d := strings.TrimSpace(r.Date)
		if len(d) == 8 {
			d = d[:4] + "-" + d[4:6] + "-" + d[6:]
		} else {
			d = ""
		}
		out.Reports = append(out.Reports, NaverReport{ID: r.ID, Broker: strings.TrimSpace(r.Broker), Title: strings.TrimSpace(r.Title), Date: d})
	}
	ci := payload.ConsensusInfo
	if ci == nil {
		// 컨센서스는 없어도 리포트가 있으면 돌려준다 — 커버리지가 얇은 종목도 리포트는 나온다.
		if len(out.Reports) > 0 {
			return out, nil
		}
		return out, ErrExternalNotFound
	}
	out.AsOf = strings.TrimSpace(ci.CreateDate)
	out.Rating = parseLooseFloat(ci.RecommMean)
	out.TargetPrice = positive(parseLooseFloat(ci.PriceTargetMean))
	if out.Rating == nil && out.TargetPrice == nil && len(out.Reports) == 0 {
		return out, ErrExternalNotFound
	}
	return out, nil
}

// "493,864" · "4.00" · "" → 숫자.
func parseLooseFloat(s string) *float64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" || s == "-" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v == 0 {
		return nil
	}
	return &v
}

// ApeWisdomRow — Reddit(주식 서브레딧 전체) 24시간 언급.
type ApeWisdomRow struct {
	Ticker       string
	Name         string
	Mentions     int
	MentionsPrev int
	Upvotes      int
	Rank         int
	RankPrev     *int
}

// ApeWisdomClient — 키 없는 공개 API.
type ApeWisdomClient struct {
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewApeWisdomClient(hc *http.Client, budget *apibudget.Manager) *ApeWisdomClient {
	return &ApeWisdomClient{httpClient: hc, budget: budget}
}

// maxApeWisdomPages — 전 종목 목록이 실측 7페이지(692종목). 모양이 바뀌어도 폭주하지 않게 상한.
const maxApeWisdomPages = 12

// AllStocks walks every page of the all-stocks filter. pause는 페이지 사이 간격(버킷 MinInterval).
func (c *ApeWisdomClient) AllStocks(ctx context.Context, pause func(context.Context) error) ([]ApeWisdomRow, error) {
	return c.Filter(ctx, "all-stocks", pause)
}

/*
ApeWisdomSubreddits — 종목 레딧 탭의 「어느 커뮤니티에서」 재료(2026-09-28). ApeWisdom은 과거를 주지
않으므로 **오늘부터 쌓아야** 나중에 쓸 수 있다. 실측 페이지 수: wsb 5 · 나머지 1~2 → 하루 약 18호출.
*/
var ApeWisdomSubreddits = []string{
	"wallstreetbets", "stocks", "investing", "options", "StockMarket", "Daytrading", "pennystocks", "Shortsqueeze", "SPACs",
}

// Filter walks every page of one ApeWisdom filter (all-stocks 또는 서브레딧 이름).
func (c *ApeWisdomClient) Filter(ctx context.Context, filter string, pause func(context.Context) error) ([]ApeWisdomRow, error) {
	var out []ApeWisdomRow
	pages := 1
	for page := 1; page <= pages && page <= maxApeWisdomPages; page++ {
		if page > 1 && pause != nil {
			if err := pause(ctx); err != nil {
				return out, err
			}
		}
		var payload struct {
			Pages   int `json:"pages"`
			Results []struct {
				Ticker       string `json:"ticker"`
				Name         string `json:"name"`
				Mentions     any    `json:"mentions"`
				MentionsPrev any    `json:"mentions_24h_ago"`
				Upvotes      any    `json:"upvotes"`
				Rank         any    `json:"rank"`
				RankPrev     any    `json:"rank_24h_ago"`
			} `json:"results"`
		}
		u := fmt.Sprintf("https://apewisdom.io/api/v1.0/filter/%s/page/%d", url.PathEscape(filter), page)
		if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderApeWisdom, u, &payload); err != nil {
			return out, err
		}
		pages = payload.Pages
		for _, r := range payload.Results {
			row := ApeWisdomRow{
				Ticker:       strings.ToUpper(strings.TrimSpace(r.Ticker)),
				Name:         html.UnescapeString(r.Name),
				Mentions:     looseInt(r.Mentions),
				MentionsPrev: looseInt(r.MentionsPrev),
				Upvotes:      looseInt(r.Upvotes),
				Rank:         looseInt(r.Rank),
			}
			if r.RankPrev != nil {
				v := looseInt(r.RankPrev)
				row.RankPrev = &v
			}
			if row.Ticker != "" {
				out = append(out, row)
			}
		}
	}
	return out, nil
}

// ApeWisdom은 숫자를 숫자로도, 문자열로도 준다.
func looseInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(x))
		return n
	}
	return 0
}
