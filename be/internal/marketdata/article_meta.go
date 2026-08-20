package marketdata

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ogDescMetaRe  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:description["'][^>]+content=["']([^"']+)["']`)
	ogDescMetaRe2 = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:description["']`)
	descMetaRe    = regexp.MustCompile(`(?i)<meta[^>]+name=["']description["'][^>]+content=["']([^"']+)["']`)
	descMetaRe2   = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+name=["']description["']`)
)

// ArticlePreviewMeta is scraped from the article page when RSS fields are too thin.
type ArticlePreviewMeta struct {
	Description string
	ImageURL    string
}

// FetchArticlePreviewMeta loads description/image hints from HTML meta tags.
func FetchArticlePreviewMeta(ctx context.Context, client *http.Client, _ *redis.Client, rawURL string) ArticlePreviewMeta {
	return probeArticlePreviewMeta(ctx, client, rawURL)
}

func probeArticlePreviewMeta(ctx context.Context, client *http.Client, rawURL string) ArticlePreviewMeta {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ArticlePreviewMeta{}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; quant-port-news/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	res, err := client.Do(req)
	if err != nil {
		return ArticlePreviewMeta{}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ArticlePreviewMeta{}
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 256<<10))
	if err != nil {
		return ArticlePreviewMeta{}
	}
	html := string(body)

	desc := firstMetaMatch(html, ogDescMetaRe, ogDescMetaRe2, descMetaRe, descMetaRe2)
	desc = normalizeRSSNewsDescription("", desc)

	imageURL := ""
	for _, re := range []*regexp.Regexp{ogImageMetaRe, ogImageMetaRe2} {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			imageURL = normalizeImageURL(strings.TrimSpace(m[1]), rawURL)
			break
		}
	}

	return ArticlePreviewMeta{
		Description: desc,
		ImageURL:    imageURL,
	}
}

func firstMetaMatch(html string, res ...*regexp.Regexp) string {
	for _, re := range res {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func IsThinNewsSummary(summary string) bool {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return true
	}
	if len(summary) < 80 {
		return true
	}
	lower := strings.ToLower(summary)
	if strings.HasPrefix(lower, "key points") && len(summary) < 180 {
		return true
	}
	return false
}
