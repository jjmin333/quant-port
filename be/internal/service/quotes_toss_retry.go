package service

import (
	"context"
	"errors"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

// tossRateLimitDefaultWait is used when a 429 carried no usable
// Retry-After / X-RateLimit-Reset header.
const tossRateLimitDefaultWait = 2 * time.Second

// tossRateLimitWait reports how long to wait before retrying, preferring the
// server's own Retry-After / X-RateLimit-Reset (apiErr.RetryAfter) over a
// fixed guess — observed resets are as short as 1s, so a flat default can
// wait far longer than necessary.
func tossRateLimitWait(err error) (time.Duration, bool) {
	var apiErr *toss.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
		return 0, false
	}
	if apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter, true
	}
	return tossRateLimitDefaultWait, true
}

func (s *QuotesService) fetchTossCandlesWithRetry(ctx context.Context, params toss.GetCandlesParams) (*toss.CandlePageResponse, error) {
	params = normalizeTossCandlesParams(params)
	page, err := s.toss.GetCandles(ctx, params)
	if err == nil {
		normalizeTossCandlesResponse(page, params.Interval)
		return page, nil
	}
	if wait, ok := tossRateLimitWait(err); ok {
		time.Sleep(wait)
		page, err = s.toss.GetCandles(ctx, params)
		if err == nil {
			normalizeTossCandlesResponse(page, params.Interval)
		}
		return page, err
	}
	return nil, err
}
