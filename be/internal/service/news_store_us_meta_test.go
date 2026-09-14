package service

import (
	"context"
	"strings"
	"testing"
)

func TestResolveUSNewsPreviewImage_nasdaqGenericToLogo(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{"tesla": "TSLA", "ko": "KO"}}
	raw := "https://www.nasdaq.com/sites/acquia.prod/files/2019-05/0902-Q19%20Total%20Markets%20photos%20and%20gif_CC8.jpg"
	got := st.resolveUSNewsPreviewImage(
		context.Background(),
		"https://www.nasdaq.com/articles/coca-cola-stock-buy-hold-or-sell",
		"Coca-Cola Stock: Buy, Hold, or Sell?",
		"Coca-Cola (NYSE: KO) stock has been on a tear.",
		nil,
		raw,
	)
	if got == raw {
		t.Fatalf("expected logo fallback, still generic image %q", got)
	}
	if !strings.Contains(got, "KO") {
		t.Fatalf("expected KO logo, got %q", got)
	}
}

func TestResolveUSNewsPreviewImage_teslaFSDWithoutExchangeTag(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{"tesla": "TSLA", "tsla": "TSLA"}}
	raw := "https://www.nasdaq.com/sites/acquia.prod/files/2019-05/0902-Q19%20Total%20Markets%20photos%20and%20gif_CC8.jpg"
	got := st.resolveUSNewsPreviewImage(
		context.Background(),
		"https://www.nasdaq.com/articles/teslas-99-month-fsd-plan-pace-generate-18-billion-year-new-profit-lever-signal-buy-elon",
		"Tesla's $99-a-Month FSD Plan Is on Pace to Generate $1.8 Billion a Year.",
		"Key points: Tesla's active Full Self-Driving subscriptions are growing fast.",
		nil,
		raw,
	)
	if got == raw {
		t.Fatalf("expected TSLA logo, still generic image %q", got)
	}
	if !strings.Contains(strings.ToUpper(got), "TSLA") {
		t.Fatalf("expected TSLA logo url, got %q", got)
	}
}

func TestResolveUSNewsPreviewImage_keepsEditorial(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{"microsoft": "MSFT"}}
	editorial := "https://cloudfront-us-east-2.images.arcpublishing.com/reuters/trump-white-house-2026.jpg"
	got := st.resolveUSNewsPreviewImage(
		context.Background(),
		"https://www.reuters.com/world/us/microsoft-trump-deal-2026/",
		"Microsoft wins Trump administration cloud contract",
		"",
		[]string{"MSFT"},
		editorial,
	)
	if got != editorial {
		t.Fatalf("expected editorial image kept, got %q", got)
	}
}

func TestPickPrimaryUSSymbol_prefersHeadlineLead(t *testing.T) {
	idx := map[string]string{
		"tesla":  "TSLA",
		"rivian": "RIVN",
		"tsla":   "TSLA",
		"rivn":   "RIVN",
	}
	got := pickPrimaryUSSymbol([]string{"TSLA", "RIVN"}, "Is This EV Stock The Next Tesla?", idx)
	if got != "TSLA" {
		t.Fatalf("pickPrimaryUSSymbol() = %q, want TSLA", got)
	}
}

func TestAddUSNameIndexKeys_companyToken(t *testing.T) {
	idx := map[string]string{}
	addUSNameIndexKeys(idx, nil, "TSLA", "Tesla, Inc.", "")
	if idx["tesla"] != "TSLA" {
		t.Fatalf("expected tesla token, got %#v", idx)
	}
}
