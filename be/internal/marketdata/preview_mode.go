package marketdata

import (
	"context"
	"net/http"
	"strings"

	"github.com/redis/go-redis/v9"
)

// PreviewMode describes how the FE should open an article.
type PreviewMode string

const (
	PreviewModeReader  PreviewMode = "reader"
	PreviewModeIframe  PreviewMode = "iframe"
	PreviewModeSummary PreviewMode = "summary"
)

// DecidePreviewMode picks reader / iframe / summary from embed probe + Reader allowlist.
func DecidePreviewMode(ctx context.Context, client *http.Client, redis *redis.Client, originalURL string) PreviewMode {
	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return PreviewModeSummary
	}
	embed := ResolveEmbedPolicy(ctx, client, redis, originalURL)
	if embed == EmbedPolicyOK {
		return PreviewModeIframe
	}
	if ReaderAllowedHost(originalURL) {
		return PreviewModeReader
	}
	return PreviewModeSummary
}
