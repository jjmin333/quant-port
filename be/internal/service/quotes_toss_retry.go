package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

func tossRateLimitWait(err error) (time.Duration, bool) {
	var apiErr *toss.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
		return 0, false
	}
	return 2 * time.Second, true
}

func (s *QuotesService) fetchTossCandlesWithRetry(ctx context.Context, params toss.GetCandlesParams) (*toss.CandlePageResponse, error) {
	page, err := s.toss.GetCandles(ctx, params)
	if err == nil {
		return page, nil
	}
	if wait, ok := tossRateLimitWait(err); ok {
		time.Sleep(wait)
		return s.toss.GetCandles(ctx, params)
	}
	return nil, err
}

func parseRetryAfterHeader(raw string) time.Duration {
	if raw == "" {
		return 2 * time.Second
	}
	if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	return 2 * time.Second
}
