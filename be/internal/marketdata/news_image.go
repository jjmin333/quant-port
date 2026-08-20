package marketdata

import (
	"net/url"
	"regexp"
	"strings"
)

var knownGenericNewsImageSubstrings = []string{
	"nasdaq.com/sites/acquia.prod/files/2019-",
	"nasdaq.com/sites/default/files/styles/hero",
	"/default-news-image",
	"/default-news-image.",
	"/placeholder",
	"/placeholder.",
	"/generic-",
	"/stock-photo-default",
}

// Editorial image hosts — og:image on these CDNs is usually article-specific photography.
var editorialNewsImageHostSubstrings = []string{
	"reuters.com",
	"reutersmedia.net",
	"bloomberg.com",
	"wsj.net",
	"dj.com",
	"gettyimages.com",
	"media.gettyimages.com",
	"shutterstock.com",
	"apnews.com",
	"cnbcfm.com",
	"image.cnbcfm.com",
	"marketwatch.com",
	"financialtimes.com",
	"nytimes.com",
	"washingtonpost.com",
	"politico.com",
	"afp.com",
}

var recentUploadPathRe = regexp.MustCompile(`/20(2[4-9]|[3-9][0-9])[-/]`)

// IsKnownGenericNewsImage detects site-wide fallback og:image URLs (not article photography).
func IsKnownGenericNewsImage(imageURL string) bool {
	imageURL = strings.ToLower(strings.TrimSpace(imageURL))
	if imageURL == "" {
		return false
	}
	for _, fragment := range knownGenericNewsImageSubstrings {
		if strings.Contains(imageURL, fragment) {
			return true
		}
	}
	return false
}

// IsArticleSpecificNewsImage returns true when og:image likely belongs to this story
// (e.g. politician photo, wire photo) and should not be replaced by a ticker logo.
func IsArticleSpecificNewsImage(articleURL, imageURL, headline string) bool {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return false
	}
	if IsKnownGenericNewsImage(imageURL) {
		return false
	}

	imageHost := newsImageHost(imageURL)
	articleHost := newsImageHost(articleURL)
	if imageHost != "" && imageHost != articleHost {
		for _, fragment := range editorialNewsImageHostSubstrings {
			if strings.Contains(imageHost, fragment) {
				return true
			}
		}
		// Off-site image that is not a known logo CDN → treat as editorial.
		if !isLikelyLogoCDN(imageHost) {
			return true
		}
	}

	if imageOverlapsArticleSlug(articleURL, imageURL) {
		return true
	}
	if imageOverlapsHeadline(headline, imageURL) {
		return true
	}
	if recentUploadPathRe.MatchString(strings.ToLower(imageURL)) {
		return true
	}
	return false
}

// ShouldReplaceNewsImageWithSymbolLogo decides logo fallback for preview thumbnails.
func ShouldReplaceNewsImageWithSymbolLogo(articleURL, imageURL, headline string, symbolCount int) bool {
	if symbolCount == 0 {
		return false
	}
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return true
	}
	if IsArticleSpecificNewsImage(articleURL, imageURL, headline) {
		return false
	}
	return IsKnownGenericNewsImage(imageURL)
}

func newsImageHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return strings.ToLower(rawURL)
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	return host
}

func isLikelyLogoCDN(host string) bool {
	for _, fragment := range []string{
		"financialmodelingprep.com",
		"logo.clearbit.com",
		"companiesmarketcap.com",
	} {
		if strings.Contains(host, fragment) {
			return true
		}
	}
	return false
}

func imageOverlapsArticleSlug(articleURL, imageURL string) bool {
	slug := articleSlugToken(articleURL)
	if slug == "" {
		return false
	}
	imgLower := strings.ToLower(imageURL)
	matches := 0
	for _, part := range slugParts(slug) {
		if len(part) < 5 {
			continue
		}
		if strings.Contains(imgLower, part) {
			matches++
		}
	}
	return matches >= 1
}

func imageOverlapsHeadline(headline, imageURL string) bool {
	headline = strings.ToLower(strings.TrimSpace(headline))
	imageURL = strings.ToLower(strings.TrimSpace(imageURL))
	if headline == "" || imageURL == "" {
		return false
	}
	matches := 0
	for _, word := range strings.Fields(headline) {
		word = strings.Trim(word, ".,!?\"'()[]")
		if len(word) < 5 || isCommonImageToken(word) {
			continue
		}
		if strings.Contains(imageURL, word) {
			matches++
		}
	}
	return matches >= 1
}

func articleSlugToken(articleURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(articleURL))
	if err != nil {
		return ""
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return strings.ToLower(parts[len(parts)-1])
}

func slugParts(slug string) []string {
	slug = strings.Trim(slug, "/")
	if slug == "" {
		return nil
	}
	raw := strings.FieldsFunc(slug, func(r rune) bool {
		return r == '-' || r == '_' || r == '/'
	})
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func isCommonImageToken(word string) bool {
	switch word {
	case "stock", "stocks", "share", "shares", "market", "markets", "today",
		"buy", "hold", "sell", "news", "article", "articles", "investor", "investors",
		"company", "companies", "year", "month", "week", "about", "after", "before":
		return true
	default:
		return false
	}
}
