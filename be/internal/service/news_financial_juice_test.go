package service

import (
	"context"
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata"
)

func TestIsFinancialJuiceItem(t *testing.T) {
	item := marketdata.RawNewsItem{
		Title:       "Turkey energy minister: transshipment deal includes 750k bpd",
		Link:        "https://www.financialjuice.com/news/123",
		SourceID:    "financial-juice",
		Description: "Turkey and Iraq agreed on a one-year pipeline contract.",
	}
	if !isFinancialJuiceItem(item) {
		t.Fatal("expected financial juice rss item to be detected")
	}
}

func TestIngestItem_financialJuiceBypassesUSRelevance(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	item := marketdata.RawNewsItem{
		Title:       "Turkey energy minister: transshipment deal includes 750k bpd",
		Link:        "https://www.financialjuice.com/news/123",
		SourceID:    "financial-juice",
		Description: "Turkey and Iraq agreed on a one-year pipeline contract.",
	}
	article, ok := st.ingestItem(context.Background(), item, nil, "", "us")
	if !ok {
		t.Fatal("expected financial juice wire to ingest without US stock relevance match")
	}
	if article.Market != "us" {
		t.Fatalf("market = %q, want us", article.Market)
	}
	if len(article.Tags) != 1 || article.Tags[0] != "속보" {
		t.Fatalf("tags = %v, want [속보]", article.Tags)
	}
}

func TestClassifyNewsTags_financialJuice(t *testing.T) {
	got := classifyNewsTags(
		"Fed's Barkin: It's a close call whether rates are high enough",
		"financial-juice",
		"https://www.financialjuice.com/news/456",
	)
	if len(got) != 1 || got[0] != "속보" {
		t.Fatalf("classifyNewsTags() = %v, want [속보]", got)
	}
}
