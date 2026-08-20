package marketdata

import "testing"

func TestIsKnownGenericNewsImage_nasdaqDefault(t *testing.T) {
	url := "https://www.nasdaq.com/sites/acquia.prod/files/2019-05/0902-Q19%20Total%20Markets%20photos%20and%20gif_CC8.jpg"
	if !IsKnownGenericNewsImage(url) {
		t.Fatal("expected nasdaq default og:image to be generic")
	}
}

func TestShouldReplaceNewsImageWithSymbolLogo_nasdaqDefault(t *testing.T) {
	article := "https://www.nasdaq.com/articles/microsoft-stock-soaring-it-too-late-buy"
	image := "https://www.nasdaq.com/sites/acquia.prod/files/2019-05/0902-Q19%20Total%20Markets%20photos%20and%20gif_CC8.jpg"
	if !ShouldReplaceNewsImageWithSymbolLogo(article, image, "Microsoft Stock is Soaring", 1) {
		t.Fatal("expected generic nasdaq image to use symbol logo")
	}
}

func TestShouldReplaceNewsImageWithSymbolLogo_reutersTrumpPhoto(t *testing.T) {
	article := "https://www.reuters.com/world/us/microsoft-negotiates-trump-administration-contract-2026-07-01/"
	image := "https://cloudfront-us-east-2.images.arcpublishing.com/reuters/7XK2TRUMP-MICROSOFT-2026.jpg"
	headline := "Microsoft and Trump administration reach cloud computing deal"
	if ShouldReplaceNewsImageWithSymbolLogo(article, image, headline, 1) {
		t.Fatal("expected editorial reuters image to be kept")
	}
	if !IsArticleSpecificNewsImage(article, image, headline) {
		t.Fatal("expected reuters off-site image to be article-specific")
	}
}

func TestShouldReplaceNewsImageWithSymbolLogo_slugMatchedImage(t *testing.T) {
	article := "https://www.nasdaq.com/articles/jeff-bezos-stepped-down-amazon-ceo-5-years-ago"
	image := "https://www.nasdaq.com/sites/acquia.prod/files/2026-07/jeff-bezos-stepped-down-amazon-ceo.jpg"
	if ShouldReplaceNewsImageWithSymbolLogo(article, image, "Jeff Bezos stepped down as Amazon CEO", 1) {
		t.Fatal("expected slug-matched editorial upload to be kept")
	}
}

func TestShouldReplaceNewsImageWithSymbolLogo_emptyUsesLogo(t *testing.T) {
	if !ShouldReplaceNewsImageWithSymbolLogo("https://example.com/a", "", "Apple shares rise", 1) {
		t.Fatal("expected empty image with symbol to use logo")
	}
}
