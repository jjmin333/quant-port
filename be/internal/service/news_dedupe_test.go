package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
)

func TestCanonicalNewsLink_financialJuice(t *testing.T) {
	raw := "HTTPS://WWW.FinancialJuice.com/News/abc?ref=rss&utm=1#frag"
	got := canonicalNewsLink(raw)
	want := "https://www.financialjuice.com/news/abc"
	if got != want {
		t.Fatalf("canonicalNewsLink() = %q, want %q", got, want)
	}
}

func TestDedupeNewsArticles_prefersTranslation(t *testing.T) {
	published := time.Now().UTC()
	items := []model.NewsArticle{
		{
			ID:          "aaaa",
			Headline:    "Turkish energy minister: transit deal includes daily capacity",
			PublishedAt: published,
		},
		{
			ID:          "bbbb",
			Headline:    "Turkish energy minister: transit deal includes daily capacity",
			HeadlineKo:  "터키 에너지 장관: 통과 협정에 하루 75만 배럴 용량 포함",
			PublishedAt: published,
		},
	}
	got := dedupeNewsArticles(items)
	if len(got) != 1 {
		t.Fatalf("dedupeNewsArticles() len = %d, want 1", len(got))
	}
	if got[0].ID != "bbbb" {
		t.Fatalf("dedupeNewsArticles() kept id %q, want bbbb", got[0].ID)
	}
}

func TestDedupeNewsArticles_mergesPublisherSuffix(t *testing.T) {
	published := time.Now().UTC()
	ko := "미국 대사관들, 중동 전역에 여행 경보 발령"
	items := []model.NewsArticle{
		{
			ID:          "fj",
			SourceID:    "financial-juice",
			Headline:    "US embassies issue travel warnings across Middle East",
			HeadlineKo:  ko,
			PublishedAt: published,
		},
		{
			ID:          "google",
			SourceID:    "google_news",
			Headline:    "US embassies issue travel warnings across Middle East - Fox News",
			HeadlineKo:  ko,
			PublishedAt: published,
		},
	}
	got := dedupeNewsArticles(items)
	if len(got) != 1 {
		t.Fatalf("dedupeNewsArticles() len = %d, want 1", len(got))
	}
	if got[0].ID != "fj" {
		t.Fatalf("dedupeNewsArticles() kept id %q, want fj", got[0].ID)
	}
}

func TestNormalizeNewsHeadlineKey(t *testing.T) {
	a := marketdata.NormalizeNewsHeadlineKey("Turkish  energy   minister:")
	b := marketdata.NormalizeNewsHeadlineKey("turkish energy minister:")
	if a != b {
		t.Fatalf("keys differ: %q vs %q", a, b)
	}
}

func TestIsSyndicatedFinancialJuiceIngest(t *testing.T) {
	item := marketdata.RawNewsItem{
		SourceID: "google_news",
		Link:     "https://www.financialjuice.com/news/example",
	}
	if !isSyndicatedFinancialJuiceIngest(item, canonicalNewsLink(item.Link)) {
		t.Fatal("expected google news FJ URL to be treated as syndicated duplicate")
	}
}
