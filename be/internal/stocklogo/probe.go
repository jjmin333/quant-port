package stocklogo

import (
	"context"
	"net/http"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// ProbeFMPImage reports whether the FMP image-stock URL returns HTTP 200.
func ProbeFMPImage(ctx context.Context, client *http.Client, symbol string, currency model.QuoteCurrency, assetType model.QuoteAssetType) bool {
	url := FMPImageURL(symbol, currency, assetType)
	if url == "" {
		return false
	}
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}
