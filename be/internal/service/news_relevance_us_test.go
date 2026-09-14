package service

import (
	"context"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/translate"
)

func TestPassesUSStockRelevance_strongMarket(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	item := marketdata.RawNewsItem{
		Title:       "Nasdaq falls as tech stocks slide on rate fears",
		Description: "Wall Street investors sold shares ahead of the Federal Reserve meeting.",
		SourceID:    "rss",
	}
	if !st.passesUSStockRelevance(context.Background(), item, nil, "") {
		t.Fatal("expected US market headline to pass")
	}
}

func TestPassesUSStockRelevance_excludeSports(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	item := marketdata.RawNewsItem{
		Title:       "NBA playoffs schedule announced for next week",
		Description: "Sports fans can watch games on national TV.",
		SourceID:    "rss",
	}
	if st.passesUSStockRelevance(context.Background(), item, nil, "") {
		t.Fatal("expected sports headline to be rejected")
	}
}

func TestPassesUSStockRelevance_linkedSymbolWithFinance(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{"apple": "AAPL"}}
	item := marketdata.RawNewsItem{
		Title:       "Apple shares rise after earnings beat",
		Description: "Revenue topped analyst estimates in the latest quarter.",
		SourceID:    "finnhub",
	}
	if !st.passesUSStockRelevance(context.Background(), item, []string{"Apple"}, "AAPL") {
		t.Fatal("expected linked symbol article with finance context to pass")
	}
}

func TestPassesUSStockRelevance_linkedSymbolWithoutFinance(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{"apple": "AAPL"}}
	item := marketdata.RawNewsItem{
		Title:       "Apple unveils new product line",
		Description: "The company hosted an event in Cupertino.",
		SourceID:    "finnhub",
	}
	if st.passesUSStockRelevance(context.Background(), item, []string{"Apple"}, "AAPL") {
		t.Fatal("expected product launch without finance context to be rejected")
	}
}

func TestIsUSStockRelated_personalFinanceNoise(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	ctx := context.Background()

	cases := []struct {
		name    string
		title   string
		summary string
		want    bool
	}{
		{
			name:    "fidelity death certificate excluded",
			title:   "My ex-husband's sister died — so why is Fidelity asking me for her death certificate?",
			summary: "A reader asks about brokerage paperwork after a family death.",
			want:    false,
		},
		{
			name:    "passive income listicle excluded",
			title:   "Here Are 3 Things Every Successful Passive Income Investor Has in Common -- and They're Simpler Than You Might Think",
			summary: "Personal finance advice for building side income streams.",
			want:    false,
		},
		{
			name:    "macro rss headline allowed",
			title:   "S&P 500 falls as inflation data surprises markets",
			summary: "Treasury yields rose after the CPI report.",
			want:    true,
		},
		{
			name:    "earnings headline allowed",
			title:   "Microsoft beats quarterly earnings estimates",
			summary: "Shares rose in after-hours trading on revenue growth.",
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isUSStockRelated(ctx, st, tc.title, tc.summary, "", nil, nil, "")
			if got != tc.want {
				t.Fatalf("isUSStockRelated() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsUSStockRelated_fedOperationalNoise(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	ctx := context.Background()

	cases := []struct {
		name    string
		title   string
		summary string
		url     string
		want    bool
	}{
		{
			name:  "triennial payments study excluded",
			title: "Federal Reserve issues initial findings from its 2025 triennial payments study",
			url:   "https://www.federalreserve.gov/newsevents/pressreleases/other20260701a.htm",
			want:  false,
		},
		{
			name:  "enforcement action excluded",
			title: "Federal Reserve Board issues enforcement action with Regions Bank",
			url:   "https://www.federalreserve.gov/newsevents/pressreleases/enforcement20260730a.htm",
			want:  false,
		},
		{
			name:    "fomc statement allowed",
			title:   "Federal Reserve issues FOMC statement",
			summary: "The committee decided to maintain the target range for the federal funds rate.",
			url:     "https://www.federalreserve.gov/newsevents/pressreleases/monetary20260729a.htm",
			want:    true,
		},
		{
			name:    "fed inflation credibility allowed",
			title:   "US Treasury sell-off raises questions about Fed inflation credibility",
			summary: "Bond yields climbed after the interview.",
			url:     "https://www.reuters.com/markets/example",
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isUSStockRelated(ctx, st, tc.title, tc.summary, "", nil, nil, tc.url)
			if got != tc.want {
				t.Fatalf("isUSStockRelated() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsUSRSSNoiseItem_motleyFool(t *testing.T) {
	item := marketdata.RawNewsItem{
		Title:         "Starbucks vs. Chipotle: Which Stock Is the Better Buy Today?",
		Description:   "Key points: Chipotle and Starbucks saw better-than-expected same-store sales last quarter.",
		Link:          "https://www.nasdaq.com/articles/starbucks-vs-chipotle",
		PublisherName: "The Motley Fool",
	}
	if !isUSRSSNoiseItem(item) {
		t.Fatal("expected motley fool syndication to be skipped")
	}
}

// 「재고」 보정은 translate 패키지의 용어 사전으로 옮겼다(ApplyGlossary).
// 여기 남겨 두는 이유는 이 실측 문장이 그 규칙이 생긴 이유이기 때문이다 —
// 규칙만 옮기고 근거를 버리면 다음 사람이 「왜 이런 치환이 있지」 하고 지운다.
func TestFixUSStockTranslation_inventoryToEquity(t *testing.T) {
	en := "Starbucks vs. Chipotle: Which Stock Is the Better Buy Today?"
	ko := "스타벅스 대. 치폴레: 두 식당 모두 반전을 맞고 있는데, 오늘 어떤 재고가 더 좋은 매수처일까요?"
	got := translate.ApplyGlossary(en, ko)
	if strings.Contains(got, "재고") {
		t.Fatalf("expected stock mistranslation fix, got %q", got)
	}
	if !strings.Contains(got, "주식") {
		t.Fatalf("expected 주식 in translation, got %q", got)
	}
}

func TestIsUSStockRelated_geopoliticsMacro(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	ctx := context.Background()

	cases := []struct {
		title   string
		summary string
		want    bool
	}{
		{
			title:   "China commerce ministry refutes US forced labor claims",
			summary: "Trade tensions continue ahead of tariff review.",
			want:    true,
		},
		{
			title:   "US Treasury sell-off raises questions about Fed inflation credibility",
			summary: "Bond yields climbed after the FT interview.",
			want:    true,
		},
		{
			title:   "Japan finance ministry comments on currency intervention tools",
			summary: "Yen volatility picked up in Tokyo trading.",
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			got := isUSStockRelated(ctx, st, tc.title, tc.summary, "", nil, nil, "https://www.reuters.com/markets/example")
			if got != tc.want {
				t.Fatalf("isUSStockRelated() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsUSStockRelated_lowQualitySourceBlocked(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	ctx := context.Background()

	got := isUSStockRelated(ctx, st,
		"5 tips for a happier retirement",
		"Personal finance advice from our experts.",
		"", nil, nil,
		"https://www.fool.com/investing/2026/example",
	)
	if got {
		t.Fatal("expected motley fool lifestyle article to be blocked")
	}
}

func TestIsUSStockRelatedArticle_rssUsesStrictFilter(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	ctx := context.Background()

	article := model.NewsArticle{
		Headline: "My ex-husband's sister died — so why is Fidelity asking me for her death certificate?",
		Summary:  "Brokerage customer service question.",
		SourceID: "rss",
	}
	if isUSStockRelatedArticle(ctx, st, article) {
		t.Fatal("expected RSS personal finance article to be excluded at list time")
	}
}

func TestPassesUSStockRelevance_googleNewsRequiresTitleAnchor(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	ctx := context.Background()

	item := marketdata.RawNewsItem{
		SourceID:    "google_news",
		Title:       "Local community festival draws large crowds",
		Description: "City officials reported record attendance.",
		Link:        "https://news.google.com/rss/articles/example",
	}
	if st.passesUSStockRelevance(ctx, item, []string{"Apple"}, "") {
		t.Fatal("expected non-market google news article to be excluded")
	}

	item = marketdata.RawNewsItem{
		SourceID:    "google_news",
		Title:       "S&P 500 futures rise ahead of jobs report",
		Description: "Wall Street is watching labor data.",
		Link:        "https://news.google.com/rss/articles/example2",
	}
	if !st.passesUSStockRelevance(ctx, item, nil, "") {
		t.Fatal("expected market headline from google news to pass")
	}
}
